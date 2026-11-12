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

// TestOrderedAndStrategyPromptShareOneSnapshot（toolgate#122）：「共用同一视图」
// 必须是同一份首读快照——首读 Info 瞬时失败不得在次读翻转（各算一次 ⇒ 提示词
// 声明顺序与真实工具表顺序分裂，正是 sortedEntries godoc 声明要避免的事）。
// 两个读取顺序各钉一遍（Ordered 先 / StrategyPrompt 先）。
func TestOrderedAndStrategyPromptShareOneSnapshot(t *testing.T) {
	ctx := context.Background()
	// flaky 注册在中间——失败即排末尾才看得出顺序差（排本就末尾是空操作）。
	build := func() (*Table, *flakyTool) {
		flaky := &flakyTool{fakeTool: &fakeTool{name: "flaky"}}
		tb := NewTable().
			Add(Entry{Tool: &fakeTool{name: "alpha"}, Priority: PriorityCore}).
			Add(Entry{Tool: flaky, Priority: PriorityCore}).
			Add(Entry{Tool: &fakeTool{name: "beta"}, Priority: PriorityCore})
		return tb, flaky
	}
	// Ordered 逐条显示名（flaky 按指针识别，免 Info 计数被测试自身打乱）。
	orderedNames := func(tb *Table, flaky *flakyTool) []string {
		var out []string
		for _, bt := range tb.Ordered(ctx) {
			if any(bt) == any(flaky) {
				out = append(out, "flaky")
				continue
			}
			info, _ := bt.Info(ctx)
			out = append(out, info.Name)
		}
		return out
	}

	t.Run("Ordered先读", func(t *testing.T) {
		tb, flaky := build()
		ordered := orderedNames(tb, flaky) // 首读：flaky Info 第 1 次失败 → 排本档末尾
		if !slices.Equal(ordered, []string{"alpha", "beta", "flaky"}) {
			t.Fatalf("首读 Ordered 序: %v", ordered)
		}
		if got := flaky.calls.Load(); got != 1 {
			t.Fatalf("首读应恰解析 Info 一次: %d", got)
		}
		prompt := tb.StrategyPrompt(ctx) // 次读：同视图，不得重放 Info
		if got := flaky.calls.Load(); got != 1 {
			t.Fatalf("次读不应重放 Info（非快照）: %d", got)
		}
		if lines := promptToolLines(prompt); !slices.Equal(lines, []string{"alpha", "beta", "tool#1"}) {
			t.Fatalf("提示词与 Ordered 视图分裂（瞬时失败在两入口间翻转）: 提示词=%v Ordered=%v", lines, ordered)
		}
	})

	t.Run("StrategyPrompt先读", func(t *testing.T) {
		tb, flaky := build()
		prompt := tb.StrategyPrompt(ctx) // 首读：flaky Info 第 1 次失败 → 缺名回退 tool#1、排末尾
		lines := promptToolLines(prompt)
		if !slices.Equal(lines, []string{"alpha", "beta", "tool#1"}) {
			t.Fatalf("首读提示词行序: %v", lines)
		}
		if got := flaky.calls.Load(); got != 1 {
			t.Fatalf("首读应恰解析 Info 一次: %d", got)
		}
		ordered := orderedNames(tb, flaky) // 次读：必须仍是同一份视图
		if !slices.Equal(ordered, []string{"alpha", "beta", "flaky"}) {
			t.Fatalf("Ordered 与提示词视图分裂: Ordered=%v 提示词=%v", ordered, lines)
		}
		if got := flaky.calls.Load(); got != 1 {
			t.Fatalf("次读不应重放 Info（非快照）: %d", got)
		}
	})
}
