package policy

// 2026-11-11 issue 清零战回归测试（#98 #99 #100）：钉住换名不信任模板标记与
// 未评级不代批。

import (
	"context"
	"testing"

	"github.com/oagentkit/guardx/invoke"
)

// TestStageRenamesToolWithoutClassifierFailClosed_Issue98_100 模板显式
// Mutating(false) + 未配分类器 + 实际换名 ⇒ 标记置 nil 走 fail-safe 升人审
// ——#85 残留旁路（delete_repository 经声明「非变更」的 stage 被红线放行）
// 修掉；同名调用与模板全量声明（Tool==""）形态不受影响。
func TestStageRenamesToolWithoutClassifierFailClosed_Issue98_100(t *testing.T) {
	g := NewGate(nil, nil)
	op := Op{Type: OpToolCall, Mode: ModeAuto, Tool: "search", Mutating: Mutating(false)}
	s := Stage(g, op, nil)
	// 换名：delete_repository 不得借模板 false 放行。
	d := g.Decide(context.Background(), Op{Type: OpToolCall, Mode: ModeAuto,
		Tool: "delete_repository", Mutating: Mutating(false)})
	_ = s
	if d.Verdict != VerdictDeny {
		// 红线输入是 Mutating=true 才 deny；这里验证的是 Stage 侧置 nil。
		_ = d
	}
	// 经 Stage 链验证换名场景：无分类器 + 换名 → need_human（fail-safe）。
	hits := 0
	c := chainWith(t, Stage(g, op, nil), &hits)
	_, err := c.Do(context.Background(), invoke.Call{Name: "delete_repository"})
	if err == nil {
		t.Fatal("换名 + 无分类器不得借模板 false 放行（toolgate#98/#100）")
	}
	if hits != 0 {
		t.Fatal("执行体不得被触达")
	}
	// 同名调用：模板标记描述的就是它，照常放行。
	hits = 0
	c2 := chainWith(t, Stage(g, op, nil), &hits)
	if _, err := c2.Do(context.Background(), invoke.Call{Name: "search"}); err != nil {
		t.Fatalf("同名调用应照常放行: %v", err)
	}
	// 模板全量声明（Tool==""）：既有装配形态保持放行。
	hits = 0
	c3 := chainWith(t, Stage(g, Op{Type: OpToolCall, Mode: ModeAuto, Mutating: Mutating(false)}, nil), &hits)
	if _, err := c3.Do(context.Background(), invoke.Call{Name: "read_logs"}); err != nil {
		t.Fatalf("模板全量声明形态应保持放行: %v", err)
	}
}

// TestUnratedPlanNotAutoApprovedByRules_Issue99 未评级（Risk=0）的方案提交
// 不得被 max_risk 代批规则命中——规则面与矩阵面「未评级不代批」一个口径。
func TestUnratedPlanNotAutoApprovedByRules_Issue99(t *testing.T) {
	pol := DefaultPolicy()
	pol.Rules = []Rule{{ID: "low", MaxRisk: 2, Verdict: VerdictAuto, Reason: "低风险代批"}}
	d := NewGate(pol, nil).Decide(context.Background(),
		Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: 0})
	if d.Verdict != VerdictNeedHuman {
		t.Fatalf("Risk=0 方案应升人审（规则面不得代批，toolgate#99），实得 %+v", d)
	}
	if d.RuleID == "low" {
		t.Fatal("裁决不得记成规则裁量")
	}
	// 已评级低风险方案：规则照常代批（不误伤规则本义）。
	d2 := NewGate(pol, nil).Decide(context.Background(),
		Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: 1})
	if d2.Verdict != VerdictAuto || d2.RuleID != "low" {
		t.Fatalf("已评级低风险方案应被规则代批，实得 %+v", d2)
	}
	// tool_call 面的 Risk 恒 0：规则不受影响（#99 修复不外溢到工具调用面）。
	pol2 := DefaultPolicy()
	pol2.Rules = []Rule{{ID: "fast", MaxRisk: 2, Verdict: VerdictAuto}}
	d3 := NewGate(pol2, nil).Decide(context.Background(),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "search_docs", Mutating: Mutating(false)})
	if d3.Verdict != VerdictAuto {
		t.Fatalf("tool_call 面规则行为不得受 #99 修复影响，实得 %+v", d3)
	}
}
