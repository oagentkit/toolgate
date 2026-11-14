// Package toolprior 工具优先级决策层：给 ReAct agent 的工具表加上
// 提示词指引（软）、结构顺序（隐式）、调用限制（硬）三层约束。
//
// 问题背景：扁平工具表里所有工具平等竞争模型注意力——模型可能跳过
// 核心证据工具（get_file）直接刷外部 MCP 工具，且外部调用无止损。
//
// 三层机制：
//  1. StrategyPrompt()——把"何时用哪个、成本多高"渲染成提示词段，注入 instruction
//  2. Ordered()——工具表按优先级稳定排序（模型对表顺序有注意力偏好）
//  3. LimitCalls()——调用次数硬上限，超限返回固定提示文本软止损（模型可见，
//     防 runaway 循环）
package toolprior

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"encoding/json"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/oagentkit/guardx/fence"
	"github.com/oagentkit/mcpkit/mcp"
)

// 优先级档位（数值越小越优先；自定义档位可插在中间）。
const (
	// PriorityCore 核心证据工具：判定必需，必须最先使用（get_file 等）。
	PriorityCore = 0
	// PrioritySupport 支撑工具：核心证据不足时补充（knowledge/skill）。
	PrioritySupport = 1
	// PriorityExternal 外部工具：有网络/进程开销，本地证据穷尽后再用（MCP）。
	PriorityExternal = 2
)

// Entry 一条工具注册项。
type Entry struct {
	Tool tool.BaseTool
	// Priority 优先级（越小越先；同值保持注册序）。
	Priority int
	// When 何时使用（决策指引，注入提示词；空 = 只排序不渲染指引）。
	When string
	// Cost 成本提示（low/medium/high；空 = 不标注）。
	Cost string
}

// Table 工具优先级表。构建期写入（Add）、构建后只读（Ordered/StrategyPrompt）——
// 写侧非并发安全，调用方需保证"先构建后共享"；首读解析 Info 并缓存排序快照，
// 此后两入口（含并发读）共享同一份视图。
type Table struct {
	entries []Entry
	// mu/sorted/sortedOK 排序快照（toolgate#122 单源视图 + toolgate#125 构建
	// 纪律）：首次 Ordered/StrategyPrompt 解析 Info 并缓存，两入口此后同源
	// （#122：各算一次时两次解析之间 Info（MCP tools/list）瞬时失败翻转 ⇒
	// 提示词声明顺序与真实工具表分裂）。mutex 兼作读侧初始化的并发安全
	// （先构建后共享的读契约）。
	//
	// 相对 #122 原实现（sync.Once）的两处偏差，均为 #125：
	//  1. 构建在**解耦 ctx** 上做（context.WithoutCancel）——首读者的
	//     deadline/cancel 不再传播进 Info 解析，短命请求不污染后续全部读者；
	//  2. **失败不锁死**：sync.Once 的「一次」与可失败的 IO 初始化天然冲突，
	//     Info 瞬时错误（网络抖动/文件半写）被冻死 = 快照固定在降级态直到
	//     进程重启。改为「全部 Info 成功才落快照；有失败则本次视图照常返回
	//     但不落盘，下次读重试」——成功路径的共享/不重放语义（#122 核心
	//     保证）原样保留。
	mu       sync.Mutex
	sorted   []resolvedEntry
	sortedOK bool
}

// NewTable 创建空表。
func NewTable() *Table { return &Table{} }

// Add 注册一条（链式）。Tool 为 nil 时 panic——注册期配置错误应尽早暴露，
// 运行期才炸的 nil Tool 难排查。首读即冻结快照，故须按"构建期写入"契约在
// 首次 Ordered/StrategyPrompt 之前完成全部 Add（重置快照，防御违约态旧视图）。
func (t *Table) Add(e Entry) *Table {
	if e.Tool == nil {
		panic("toolprior: Entry.Tool 不能为 nil")
	}
	t.entries = append(t.entries, e)
	t.mu.Lock()
	t.sorted = nil // 重置快照（未并发使用的构建期写入面）
	t.sortedOK = false
	t.mu.Unlock()
	return t
}

// Len 当前条目数。
func (t *Table) Len() int { return len(t.entries) }

// resolvedEntry 一条经 Info 解析的注册项（排序视图元素）。
type resolvedEntry struct {
	e       Entry
	idx     int    // 注册序（同优先级的稳定 tiebreak）
	name    string // Info 失败为空串
	missing bool   // Info 失败/无名：排末尾，不遮挡可用工具
}

// sortedEntries 统一排序视图（Ordered 与 StrategyPrompt 真共用同一份）：
// priority 升序 → Info 失败的排末尾 → 同档保持注册序。
// 首次调用解析 Info 并缓存快照（构建后只读契约的自然延伸），成功建成后重读
// 不再重放 Info——两入口读同一份，提示词宣称的顺序与真实工具表顺序才不会在
// 工具元数据瞬时缺失时互相矛盾（toolgate#122：各算一次时两次解析间成败翻转
// 即分裂）。
//
// 两条构建纪律（toolgate#125）：
//   - **ctx 解耦**：Info 在 context.WithoutCancel(ctx) 上解析——首读者若是
//     短命请求（带 deadline/cancel），其取消语义只影响他自己那次调用，不冻结
//     进快照被后续全部读者继承；
//   - **失败不锁死**：任一 Info 报错 ⇒ 本次视图照常返回（降级态：缺名回退
//     tool#N、排末尾）但**不落快照**，下次读重试——sync.Once 的「一次」语义
//     与可失败的 IO 初始化天然冲突，瞬时抖动不该把优先级排序永久降级到进程
//     重启。全部成功才置 sortedOK（此后与 #122 同：共享、不重放）。
func (t *Table) sortedEntries(ctx context.Context) []resolvedEntry {
	t.mu.Lock()
	if t.sortedOK {
		rs := t.sorted
		t.mu.Unlock()
		return rs
	}
	t.mu.Unlock()

	// 快照构建与调用方 ctx 解耦（#125）：WithoutCancel 保留 ctx 值（trace 等）
	// 但摘掉 cancel/deadline——Info（MCP tools/list 等）跑在不失效的 ctx 上。
	buildCtx := context.WithoutCancel(ctx)
	rs := make([]resolvedEntry, len(t.entries))
	hadInfoErr := false
	for i, e := range t.entries {
		name := ""
		info, err := e.Tool.Info(buildCtx)
		if err != nil {
			hadInfoErr = true
		} else if info != nil && info.Name != "" {
			name = info.Name
		}
		rs[i] = resolvedEntry{e: e, idx: i, name: name, missing: name == ""}
	}
	slices.SortStableFunc(rs, func(a, b resolvedEntry) int {
		if c := cmp.Compare(a.e.Priority, b.e.Priority); c != 0 {
			return c
		}
		if a.missing != b.missing {
			if a.missing {
				return 1 // Info 失败的排末尾，不遮挡可用工具
			}
			return -1
		}
		return cmp.Compare(a.idx, b.idx)
	})

	t.mu.Lock()
	defer t.mu.Unlock()
	if hadInfoErr {
		if t.sortedOK {
			// 并发竞态：另一读者已建成成功视图——共享它，不回退到本次降级视图。
			return t.sorted
		}
		return rs // 失败不落快照：本次照常返回，下次读重试（#125）
	}
	t.sorted = rs
	t.sortedOK = true
	return t.sorted
}

// Ordered 按优先级稳定排序返回工具表（同优先级保持注册序）。
// Info 失败的条目在本优先级档内排后（跨档仍按优先级——与 StrategyPrompt 共用
// 首读缓存的同一份视图，toolgate#122；瞬时失败不冻死、下次读重试，
// toolgate#125）（排序键不可得时不遮挡可用工具）。
func (t *Table) Ordered(ctx context.Context) []tool.BaseTool {
	rs := t.sortedEntries(ctx)
	out := make([]tool.BaseTool, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.e.Tool)
	}
	return out
}

// StrategyPrompt 渲染"工具使用策略"提示词段（按优先级序，与 Ordered 共用首读
// 缓存的同一份视图——提示词声明顺序即真实工具表顺序，toolgate#122；瞬时失败
// 不冻死、下次读重试，toolgate#125）。
// 空表返回空串。调用方拼进 agent instruction。
//
// 攻击者可影响字段消毒（toolgate#135）：Info().Name 对 MCP 工具是服务端可影响
// 字段（被攻陷/敌意 server 注册怪名工具），When/Cost 可来自外部配置——原样拼
// 进提示词段，内嵌换行即可插入顶格伪指令行（混入 instruction 的「系统注入」
// 版式）。渲染前全部过 sanitizePromptFragment（换行折叠单行 + fence 结构中和），
// 与 tools websearch #49/#63、guardx fence 同族口径。
func (t *Table) StrategyPrompt(ctx context.Context) string {
	if len(t.entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("===【工具使用策略（按优先级决策）】===\n")
	for _, r := range t.sortedEntries(ctx) {
		name := r.name
		if name == "" {
			name = fmt.Sprintf("tool#%d", r.idx)
		}
		var parts []string
		parts = append(parts, fmt.Sprintf("priority=%d", r.e.Priority))
		if r.e.Cost != "" {
			parts = append(parts, "cost="+sanitizePromptFragment(r.e.Cost))
		}
		if r.e.When != "" {
			parts = append(parts, sanitizePromptFragment(r.e.When))
		}
		fmt.Fprintf(&b, "- %s（%s）\n", sanitizePromptFragment(name), strings.Join(parts, "；"))
	}
	b.WriteString("原则：先用优先级数值小的工具取得基础证据；优先级数值大的外部工具（有网络/进程开销）仅在本地信息不足时调用，避免不必要的外部开销。")
	return b.String()
}

// sanitizePromptFragment 提示词片段消毒（toolgate#135）：换行/行终止符折叠为
// 单行（strings.Fields 单行化——列表项字段内嵌换行即伪造顶格行/伪条目），
// 再经 guardx/fence.EscapeUntrusted 中和行首结构字符（标题/引用/表格/分隔线）
// ——fence 同族口径：承重内容保留原样，结构位中和。
func sanitizePromptFragment(s string) string {
	if s == "" {
		return ""
	}
	return fence.EscapeUntrusted(strings.Join(strings.Fields(s), " "))
}

// limitedTool 调用次数受限的工具包装（原子计数，并发安全）。
type limitedTool struct {
	inner tool.InvokableTool
	max   int64
	calls atomic.Int64
}

// LimitCalls 包装工具：调用超过 max 次后返回固定提示文本（nil error）软止损。
// 必须返回文本而非 Go error——eino ToolsNode 会把工具 error 直接上抛中止整个
// agent 运行，模型永远看不到提示、此前轮次的部分结论全部丢弃；返回文本则模型
// 可见，可基于已有信息收尾（软止损 + 保留部分进展）。
// max<=0 = 不限制（原样返回）。包装每次新建——计数不跨任务共享。
// 入参需 InvokableTool（嵌入 BaseTool，Info/RawToolName 由内层透出，#73）。
func LimitCalls(t tool.InvokableTool, max int) tool.BaseTool {
	if max <= 0 {
		return t
	}
	return &limitedTool{inner: t, max: int64(max)}
}

func (l *limitedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return l.inner.Info(ctx)
}

// RawToolName 透传原始工具名（oagentkit/toolgate#73，契约单源 mcpkit RawNamer
// /oagentkit/mcpkit#46）：宿主自然堆叠 invokableTool → AuditGate → LimitCalls →
// 模型时最外层是本包装，不透传则 RawToolNameOf 的类型断言断到外层即断链，按名
// 判定的消费方（动词启发式/只读白名单/审计归类）静默拿到带命名空间的暴露名。
// 内层非 RawNamer 返回空串（同 mcpkit selfHealTool 写法：空串=无原始名可用，
// 消费方不得据此放行）。
func (l *limitedTool) RawToolName() string {
	if rn, ok := l.inner.(mcp.RawNamer); ok {
		return rn.RawToolName()
	}
	return ""
}

// OutputSchema 出参 schema 透传（v0.12.5 OutputSchemer 可选能力面，mcp.OutputSchemaOf）。
func (l *limitedTool) OutputSchema() json.RawMessage {
	if o, ok := l.inner.(mcp.OutputSchemer); ok {
		return o.OutputSchema()
	}
	return nil
}

func (l *limitedTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	if l.calls.Add(1) > l.max {
		return fmt.Sprintf(
			"LIMIT_REACHED: 本工具调用已达上限（%d 次）。不要再调用本工具；请基于已获取的信息直接给出结论。",
			l.max), nil
	}
	return l.inner.InvokableRun(ctx, args, opts...)
}
