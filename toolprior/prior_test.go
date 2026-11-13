package toolprior

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/oagentkit/mcpkit/mcp"
)

// rawNamedTool 带 RawToolName 的工具替身（模拟 mcp.Pool 出口工具：暴露名带
// server 命名空间，原始名才是按名判定消费方要的，oagentkit/mcpkit#46）。
type rawNamedTool struct {
	*fakeTool
	raw string
}

func (f *rawNamedTool) RawToolName() string { return f.raw }

var _ mcp.RawNamer = (*rawNamedTool)(nil)

// fakeTool 假工具。
type fakeTool struct {
	name string
	desc string
	err  error
}

func (f *fakeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &schema.ToolInfo{Name: f.name, Desc: f.desc}, nil
}

func (f *fakeTool) InvokableRun(_ context.Context, _ string, _ ...tool.Option) (string, error) {
	return "ran:" + f.name, nil
}

func TestOrderedPrioritySort(t *testing.T) {
	ctx := context.Background()
	table := NewTable().
		Add(Entry{Tool: &fakeTool{name: "mcp"}, Priority: PriorityExternal}).
		Add(Entry{Tool: &fakeTool{name: "get_file"}, Priority: PriorityCore}).
		Add(Entry{Tool: &fakeTool{name: "knowledge"}, Priority: PrioritySupport})

	ordered := table.Ordered(ctx)
	want := []string{"get_file", "knowledge", "mcp"}
	for i, w := range want {
		info, _ := ordered[i].Info(ctx)
		if info.Name != w {
			t.Fatalf("位置 %d = %s, want %s", i, info.Name, w)
		}
	}
}

func TestOrderedStableSamePriority(t *testing.T) {
	ctx := context.Background()
	table := NewTable().
		Add(Entry{Tool: &fakeTool{name: "a"}, Priority: PriorityCore}).
		Add(Entry{Tool: &fakeTool{name: "b"}, Priority: PriorityCore})

	ordered := table.Ordered(ctx)
	// 同优先级保持注册序
	names := make([]string, 0, 2)
	for _, t2 := range ordered {
		info, _ := t2.Info(ctx)
		names = append(names, info.Name)
	}
	if names[0] != "a" || names[1] != "b" {
		t.Fatalf("同优先级应保持注册序: %v", names)
	}
}

func TestStrategyPrompt(t *testing.T) {
	ctx := context.Background()
	table := NewTable().
		Add(Entry{Tool: &fakeTool{name: "get_file"}, Priority: PriorityCore,
			When: "核实证据必须先用", Cost: "low"}).
		Add(Entry{Tool: &fakeTool{name: "mcp"}, Priority: PriorityExternal,
			When: "外部查询", Cost: "high"})

	prompt := table.StrategyPrompt(ctx)
	for _, want := range []string{
		"工具使用策略", "priority=0", "priority=2",
		"get_file", "核实证据必须先用", "cost=high",
		"避免不必要的外部开销",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("策略提示缺 %q", want)
		}
	}
	// 策略提示按优先级排序：get_file 在 mcp 之前
	if strings.Index(prompt, "get_file") > strings.Index(prompt, "mcp") {
		t.Fatal("策略提示顺序应为优先级序")
	}
}

func TestStrategyPromptEmpty(t *testing.T) {
	if s := NewTable().StrategyPrompt(context.Background()); s != "" {
		t.Fatalf("空表策略应为空串，得到 %q", s)
	}
}

func TestWithCallLimit(t *testing.T) {
	ft := &fakeTool{name: "mcp", desc: "external"}
	limited := LimitCalls(ft, 2)

	// 前两次正常（LimitCalls 返回 BaseTool，调用经 InvokableTool 断言）
	lt, ok := limited.(tool.InvokableTool)
	if !ok {
		t.Fatal("应实现 tool.InvokableTool")
	}
	for i := 0; i < 2; i++ {
		out, err := lt.InvokableRun(context.Background(), "{}")
		if err != nil {
			t.Fatalf("第 %d 次不应受限: %v", i+1, err)
		}
		if out != "ran:mcp" {
			t.Fatalf("透传失败: %q", out)
		}
	}
	// 第三次软止损：返回固定提示文本（nil error）——error 会被 eino ToolsNode
	// 上抛中止整个 agent 运行，模型永远看不到；文本则模型可见可收尾
	out, err := lt.InvokableRun(context.Background(), "{}")
	if err != nil {
		t.Fatalf("超限应软止损（nil error）: %v", err)
	}
	if !strings.Contains(out, "LIMIT_REACHED") || !strings.Contains(out, "上限") {
		t.Fatalf("超限应返回模型可见的提示文本: %q", out)
	}
	// 第四次同样拒绝（连续拒绝不透传内层）
	out, err = lt.InvokableRun(context.Background(), "{}")
	if err != nil || !strings.Contains(out, "LIMIT_REACHED") {
		t.Fatalf("持续超限应持续拒绝: %q %v", out, err)
	}
	// Info 透传不受限
	info, err := limited.Info(context.Background())
	if err != nil || info.Name != "mcp" {
		t.Fatalf("Info 应透传: %v %v", info, err)
	}
}

func TestWithCallLimitZero(t *testing.T) {
	ft := &fakeTool{name: "x"}
	limited := LimitCalls(ft, 0) // 0 = 不限（原样返回）
	lt := limited.(tool.InvokableTool)
	for i := 0; i < 10; i++ {
		if _, err := lt.InvokableRun(context.Background(), "{}"); err != nil {
			t.Fatalf("不限制时不应报错: %v", err)
		}
	}
}

// TestLimitCallsRawToolNamePassthrough（toolgate#73）：limitedTool 包装层透传
// 原始工具名——宿主自然堆叠 invokableTool → AuditGate → LimitCalls → 模型时
// 最外层是本包装，不透传则 RawToolNameOf 的类型断言断到外层即断链，按名判定
// 的消费方（只读白名单/动词启发式/审计归类）静默拿到带命名空间的暴露名。
// 跨包装层组合断言见 policy 包 TestRawToolNameAcrossWrappedStack。
func TestLimitCallsRawToolNamePassthrough(t *testing.T) {
	inner := &rawNamedTool{fakeTool: &fakeTool{name: "srv.search_docs"}, raw: "search_docs"}
	limited := LimitCalls(inner, 2)
	if got := mcp.RawToolNameOf(limited); got != "search_docs" {
		t.Fatalf("包装后最外层应取到原始名，got %q（want search_docs）", got)
	}
	// 内层非 RawNamer：空串=无原始名可用（同 mcpkit selfHealTool 契约；不得
	// 谎报暴露名为原始名，消费方对空串不得放行）。
	plain := LimitCalls(&fakeTool{name: "get_x"}, 2)
	if got := mcp.RawToolNameOf(plain); got != "" {
		t.Fatalf("内层非 RawNamer 应返回空串，got %q", got)
	}
}

func TestOrderedInfoErrorStillReturned(t *testing.T) {
	// Info 失败的工具不应被静默丢弃
	ctx := context.Background()
	table := NewTable().Add(Entry{Tool: &fakeTool{name: "broken", err: errors.New("info boom")}, Priority: PriorityCore})
	ordered := table.Ordered(ctx)
	if len(ordered) != 1 {
		t.Fatalf("Info 失败的工具仍应出现在表中，得到 %d", len(ordered))
	}
}

func TestStrategyPromptInfoErrorFallback(t *testing.T) {
	ctx := context.Background()
	table := NewTable().Add(Entry{Tool: &fakeTool{name: "", err: errors.New("boom")}, Priority: PriorityCore, When: "测试"})
	prompt := table.StrategyPrompt(ctx)
	if !strings.Contains(prompt, "tool#0") {
		t.Fatalf("Info 失败应有回退名: %s", prompt)
	}
}

// flakyTool 首次 Info 失败、之后成功——模拟 MCP tools/list 瞬时失败（toolgate#122）。
type flakyTool struct {
	*fakeTool
	calls atomic.Int32
}

func (f *flakyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	if f.calls.Add(1) == 1 {
		return nil, errors.New("tools/list 瞬时失败")
	}
	return f.fakeTool.Info(ctx)
}

// promptToolLines 提示词逐行显示名（"- <名>（…）"）。
func promptToolLines(prompt string) []string {
	var out []string
	for _, line := range strings.Split(prompt, "\n") {
		rest, ok := strings.CutPrefix(line, "- ")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, "（")
		out = append(out, name)
	}
	return out
}

// countingTool 带 Info 调用计数的工具（快照「解析一次、不重放」的观测面）。
type countingTool struct {
	*fakeTool
	calls atomic.Int32
}

func (c *countingTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	c.calls.Add(1)
	return c.fakeTool.Info(ctx)
}

// orderedNamesOf Ordered 逐条显示名。mark 里的工具按指针直取显示名——
// 测试自身调 bt.Info 会打乱 Info 调用计数（那是工具自己的 Info，不是快照
// 解析），带计数的工具必须进 mark。
func orderedNamesOf(tb *Table, mark map[any]string) []string {
	ctx := context.Background()
	var out []string
	for _, bt := range tb.Ordered(ctx) {
		if n, ok := mark[any(bt)]; ok {
			out = append(out, n)
			continue
		}
		info, _ := bt.Info(ctx)
		out = append(out, info.Name)
	}
	return out
}

// TestOrderedAndStrategyPromptShareOneSnapshot（toolgate#122，#125 后的成功
// 路径口径）：「共用同一视图」= Info 只解析一次、两入口读同一份快照——
// 提示词声明顺序与真实工具表顺序不得分裂（各算一次 ⇒ 两次解析间成败翻转即
// 分裂，正是 sortedEntries godoc 声明要避免的事）。Info 全部成功的构建落快照
// 后不再重放；#125 的重试只作用于 Info 报错的构建（见下一测试）。
// 两个读取顺序各钉一遍（Ordered 先 / StrategyPrompt 先）。
func TestOrderedAndStrategyPromptShareOneSnapshot(t *testing.T) {
	ctx := context.Background()
	build := func() (*Table, *countingTool) {
		mid := &countingTool{fakeTool: &fakeTool{name: "mid"}}
		tb := NewTable().
			Add(Entry{Tool: &fakeTool{name: "alpha"}, Priority: PriorityCore}).
			Add(Entry{Tool: mid, Priority: PriorityCore}).
			Add(Entry{Tool: &fakeTool{name: "beta"}, Priority: PriorityCore})
		return tb, mid
	}
	// 快照序（按显示名取序，与注册序区分：alpha/mid/beta 注册序 == 优先级序，
	// 用 mid 的计数证明「只解析一次」）。
	promptNames := func(tb *Table) []string {
		return promptToolLines(tb.StrategyPrompt(ctx))
	}

	t.Run("Ordered先读", func(t *testing.T) {
		tb, mid := build()
		ordered := orderedNamesOf(tb, map[any]string{any(mid): "mid"})
		if !slices.Equal(ordered, []string{"alpha", "mid", "beta"}) {
			t.Fatalf("首读 Ordered 序: %v", ordered)
		}
		if got := mid.calls.Load(); got != 1 {
			t.Fatalf("首读应恰解析 Info 一次: %d", got)
		}
		lines := promptNames(tb) // 次读：同视图，不得重放 Info
		if got := mid.calls.Load(); got != 1 {
			t.Fatalf("次读不应重放 Info（非快照）: %d", got)
		}
		if !slices.Equal(lines, []string{"alpha", "mid", "beta"}) {
			t.Fatalf("提示词与 Ordered 视图分裂: 提示词=%v Ordered=%v", lines, ordered)
		}
	})

	t.Run("StrategyPrompt先读", func(t *testing.T) {
		tb, mid := build()
		lines := promptNames(tb)
		if !slices.Equal(lines, []string{"alpha", "mid", "beta"}) {
			t.Fatalf("首读提示词行序: %v", lines)
		}
		if got := mid.calls.Load(); got != 1 {
			t.Fatalf("首读应恰解析 Info 一次: %d", got)
		}
		ordered := orderedNamesOf(tb, map[any]string{any(mid): "mid"}) // 次读：必须仍是同一份视图
		if got := mid.calls.Load(); got != 1 {
			t.Fatalf("次读不应重放 Info（非快照）: %d", got)
		}
		if !slices.Equal(ordered, []string{"alpha", "mid", "beta"}) {
			t.Fatalf("Ordered 与提示词视图分裂: Ordered=%v 提示词=%v", ordered, lines)
		}
	})
}

// TestTransientInfoFailureRecoversOnNextRead（toolgate#125）：瞬时失败不锁死。
// sync.Once 时代首读的一次 Info 抖动会把降级快照冻到进程重启；现在失败构建
// 不落盘——首读照常返回降级视图，次读重试成功后恢复，且恢复后的快照按 #122
// 共享、不再重放。
func TestTransientInfoFailureRecoversOnNextRead(t *testing.T) {
	ctx := context.Background()
	flaky := &flakyTool{fakeTool: &fakeTool{name: "flaky"}}
	tb := NewTable().
		Add(Entry{Tool: &fakeTool{name: "alpha"}, Priority: PriorityCore}).
		Add(Entry{Tool: flaky, Priority: PriorityCore}).
		Add(Entry{Tool: &fakeTool{name: "beta"}, Priority: PriorityCore})

	// 首读：flaky Info 第 1 次失败 → 降级视图（缺名排末尾），但**不落快照**。
	ordered1 := orderedNamesOf(tb, map[any]string{any(flaky): "flaky"})
	if !slices.Equal(ordered1, []string{"alpha", "beta", "flaky"}) {
		t.Fatalf("首读降级视图: %v", ordered1)
	}
	if got := flaky.calls.Load(); got != 1 {
		t.Fatalf("首读应恰解析 Info 一次: %d", got)
	}

	// 次读：重试 Info（第 2 次成功）→ 恢复正常视图并落快照。
	prompt := tb.StrategyPrompt(ctx)
	if got := flaky.calls.Load(); got != 2 {
		t.Fatalf("失败快照应允许次读重试（不再被 Once 冻死）: calls=%d", got)
	}
	if lines := promptToolLines(prompt); !slices.Equal(lines, []string{"alpha", "flaky", "beta"}) {
		t.Fatalf("次读应恢复完整视图: %v", lines)
	}

	// 第三读：成功快照已落 → 共享、不重放（#122 保证在恢复后继续成立）。
	ordered2 := orderedNamesOf(tb, map[any]string{any(flaky): "flaky"})
	if got := flaky.calls.Load(); got != 2 {
		t.Fatalf("恢复后不应再重放 Info: %d", got)
	}
	if !slices.Equal(ordered2, []string{"alpha", "flaky", "beta"}) {
		t.Fatalf("恢复后 Ordered 应与提示词同视图: %v", ordered2)
	}
}

// ctxAwareTool Info 尊重 ctx：ctx 取消/过期即报错（模拟真实 MCP tools/list
// 等带 deadline 的远程调用）。
type ctxAwareTool struct{}

func (c *ctxAwareTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &schema.ToolInfo{Name: "aware"}, nil
}

func (c *ctxAwareTool) InvokableRun(_ context.Context, _ string, _ ...tool.Option) (string, error) {
	return "ran:aware", nil
}

// TestSnapshotBuildDecoupledFromCallerCtx（toolgate#125）：快照构建不得跑在
// 首读者 ctx 上——短命请求（已取消/带 deadline）的取消语义若传进 Info 解析，
// 首读即失败、且失败态（Once 时代）被后续全部读者继承。WithoutCancel 后：
// 首读者 ctx 只影响他自己，快照照常建成。
func TestSnapshotBuildDecoupledFromCallerCtx(t *testing.T) {
	tb := NewTable().
		Add(Entry{Tool: &fakeTool{name: "alpha"}, Priority: PriorityCore}).
		Add(Entry{Tool: &ctxAwareTool{}, Priority: PrioritySupport})

	dead, cancel := context.WithCancel(context.Background())
	cancel() // 首读者 ctx 已取消

	ordered := tb.Ordered(dead)
	info, err := ordered[1].Info(context.Background())
	if err != nil || info.Name != "aware" {
		t.Fatalf("已取消 ctx 的首读者不应污染快照构建（Info 应在解耦 ctx 上成功）: %v %v", info, err)
	}
	// 恢复路径也正常（快照已按成功落盘）。
	if lines := promptToolLines(tb.StrategyPrompt(context.Background())); !slices.Equal(lines, []string{"alpha", "aware"}) {
		t.Fatalf("健康读者视图: %v", lines)
	}
}
