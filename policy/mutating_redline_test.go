package policy

import (
	"context"
	"testing"

	"github.com/oagentkit/guardx/invoke"
)

// 变更类红线的输入可信度（oagentkit/toolgate#85）。
//
// 缺陷形态：Op.Mutating 是调用方填写的布尔位，零值 false 恒等于「声明非变更」
// ——漏填一个字段，delete_repository 这类变更工具就从 deny（红线）变成
// auto_proceed（放行），且审计留痕谎报「白名单内只读采集工具」（代码里根本
// 不存在白名单结构，真实依据只是「没人说它是变更类」）。门自身不知道工具名
// 是否变更类（跨模块依赖由调用方承担），所以唯一正确的兜底是：**未标记
// ≠ 非变更**——nil（未标记）必须 fail-safe 升人审，而不是当成 false 放行。

// TestUnclassifiedToolCallIsNotAutoProceed 同一工具名，漏填 Mutating 不得
// 放行——修复前这里是 auto_proceed + readonly_allowlist（红线被静默绕过）。
func TestUnclassifiedToolCallIsNotAutoProceed(t *testing.T) {
	g := NewGate(nil, nil)
	got := g.Decide(context.Background(), Op{
		Type: OpToolCall, Mode: ModeConfirm, Tool: "delete_repository",
	})
	if got.Verdict == VerdictAuto {
		t.Fatalf("未标记 Mutating 的 tool_call 不得 auto_proceed 放行（漏填布尔位=声明非变更=红线绕过，oagentkit/toolgate#85），实得 %+v", got)
	}
	// fail-safe 方向：升人审，而不是 deny 或静默放行——deny 会把「忘了填」
	// 误伤成硬拒绝（宿主可能确有只读工具未接入动词表），need_human 让人工
	// 裁决，与未知操作类型的既有口径一致。
	if got.Verdict != VerdictNeedHuman {
		t.Fatalf("未标记 Mutating 应 fail-safe 升人审，实得 %+v", got)
	}
	if got.RuleID == "readonly_allowlist" {
		t.Fatalf("审计留痕不得谎报白名单（代码中不存在白名单结构）: %+v", got)
	}
}

// TestExplicitMutatingFlagTriad 三态全谱：nil=未标记 fail-safe 升人审；
// true=红线 deny（任何模式、规则不可豁免——#71 语义保持）；false=显式声明
// 非变更，放行但留痕如实（不得再写 readonly_allowlist）。
func TestExplicitMutatingFlagTriad(t *testing.T) {
	g := NewGate(nil, nil)
	yes, no := Mutating(true), Mutating(false)

	deny := g.Decide(context.Background(), Op{Type: OpToolCall, Mode: ModeFull, Tool: "shell_execute", Mutating: yes})
	if deny.Verdict != VerdictDeny || deny.RuleID != "mutating_tool_guard" {
		t.Fatalf("显式 true 应红线 deny，实得 %+v", deny)
	}

	human := g.Decide(context.Background(), Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "mystery_tool"})
	if human.Verdict != VerdictNeedHuman || human.RuleID != "mutating_unclassified" {
		t.Fatalf("未标记（nil）应 fail-safe 升人审 mutating_unclassified，实得 %+v", human)
	}

	auto := g.Decide(context.Background(), Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "get_load", Mutating: no})
	if auto.Verdict != VerdictAuto {
		t.Fatalf("显式 false 应正常裁决放行，实得 %+v", auto)
	}
	if auto.RuleID == "readonly_allowlist" || auto.RuleID == "" {
		t.Fatalf("放行留痕必须如实标注依据（不得谎报白名单），实得 %+v", auto)
	}
}

// TestUnclassifiedToolCallNotBypassableByRules 宽松规则面前，未标记的
// tool_call 也不得被代批放行——fail-safe 段在规则链之前（与 #71 红线同位）。
func TestUnclassifiedToolCallNotBypassableByRules(t *testing.T) {
	pol := DefaultPolicy()
	pol.Rules = []Rule{{ID: "fast-track", MaxRisk: 2, Verdict: VerdictAuto, Reason: "低风险代批"}}
	got := NewGate(pol, nil).Decide(context.Background(),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "search_docs"})
	if got.Verdict != VerdictNeedHuman {
		t.Fatalf("宽松规则不得放行未标记变更性的 tool_call，实得 %+v", got)
	}
}

// TestStageRecomputesMutatingPerToolCall 模板期标记与实际工具脱节的修复：
// Stage 在用实际调用名覆盖 Tool 时按分类器重算变更标记——模板 Mutating=false
// 遇上变异工具不得放行（此前换工具名不重算，标记与实际调用脱节）。
func TestStageRecomputesMutatingPerToolCall(t *testing.T) {
	hits := 0
	classifier := func(tool string) bool {
		return tool == "delete_repository" || tool == "restart_service"
	}
	c := chainWith(t, Stage(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeConfirm, Mutating: Mutating(false)},
		nil, WithMutatingClassifier(classifier)), &hits)

	// 实际调用换成变异工具：分类器重算 ⇒ deny，执行体不得触达。
	if _, err := c.Do(context.Background(), invoke.Call{Name: "delete_repository"}); err == nil {
		t.Fatal("模板声明非变更、实际调用命中变异动词段——应拒绝（标记与工具脱节，oagentkit/toolgate#85）")
	}
	if hits != 0 {
		t.Fatalf("被拒调用不得触达执行体，实得 %d", hits)
	}
	// 实际调用是只读工具：照常放行。
	if _, err := c.Do(context.Background(), invoke.Call{Name: "read_logs"}); err != nil {
		t.Fatalf("只读工具应放行，实得 %v", err)
	}
	if hits != 1 {
		t.Fatalf("只应放行那一次调用，实得触达 %d 次", hits)
	}
}

// TestStageUnclassifiedTemplateFailsSafe 未接分类器的 Stage、模板又没填
// Mutating：每个 tool_call 都 fail-safe 升人审——「宿主忘了填」的默认结果
// 必须是人审而不是放行。
func TestStageUnclassifiedTemplateFailsSafe(t *testing.T) {
	hits := 0
	c := chainWith(t, Stage(NewGate(nil, nil), Op{Type: OpToolCall, Mode: ModeConfirm}, nil), &hits)
	if _, err := c.Do(context.Background(), invoke.Call{Name: "read_logs"}); err == nil {
		t.Fatal("未标记变更性的模板不得放行 tool_call（fail-safe 升人审）")
	}
	if hits != 0 {
		t.Fatalf("fail-safe 拒绝不得触达执行体，实得 %d", hits)
	}
}

// TestStageClassifierOverridesTemplateMark 分类器优先于模板标记：模板写着
// true、实际调用经分类器判为非变更——按实际工具裁决（重算即以实际为准），
// 与 TestStageRecomputesMutatingPerToolCall 的方向相反、结论对称。
func TestStageClassifierOverridesTemplateMark(t *testing.T) {
	hits := 0
	c := chainWith(t, Stage(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeConfirm, Mutating: Mutating(true)},
		nil, WithMutatingClassifier(func(tool string) bool { return false })), &hits)
	if _, err := c.Do(context.Background(), invoke.Call{Name: "read_logs"}); err != nil {
		t.Fatalf("分类器判非变更的工具应放行（按实际调用裁决）: %v", err)
	}
	if hits != 1 {
		t.Fatalf("放行后应触达执行体一次，实得 %d", hits)
	}
}
