package policy

import (
	"context"
	"testing"

	"github.com/oagentkit/guardx/invoke"
)

// chainWith 把一个 stage 装进链，执行体恒成功且记录触达次数。
func chainWith(t *testing.T, s invoke.Stage, hits *int) *invoke.Chain {
	t.Helper()
	c, err := invoke.NewChain(func(context.Context, invoke.Call) (invoke.Result, error) {
		*hits++
		return invoke.Result{Text: "ok"}, nil
	}, []invoke.Stage{s})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestStageAllowsReadonlyToolCall 只读采集面走白名单放行并触达执行体。
// 用真实的内置策略判定，不构造桩 Arbiter——桩只能证明桩自己的行为。
func TestStageAllowsReadonlyToolCall(t *testing.T) {
	hits := 0
	c := chainWith(t, Stage(NewGate(nil, nil), Op{Type: OpToolCall, Mode: ModeAuto}, nil), &hits)
	if _, err := c.Do(context.Background(), invoke.Call{Name: "read_logs"}); err != nil {
		t.Fatalf("只读采集面应放行，实得 %v", err)
	}
	if hits != 1 {
		t.Fatalf("放行后应触达执行体一次，实得 %d", hits)
	}
}

// TestStageDeniesMutatingToolCall 变异工具一律拒绝，且不得触达执行体。
// 这是适配器的核心承诺：把审计门搬上链之后，拒绝是真的拦住了。
func TestStageDeniesMutatingToolCall(t *testing.T) {
	hits := 0
	c := chainWith(t, Stage(NewGate(nil, nil), Op{Type: OpToolCall, Mode: ModeFull, Mutating: true}, nil), &hits)

	_, err := c.Do(context.Background(), invoke.Call{Name: "restart_service"})
	if err == nil {
		t.Fatal("变异工具应拒绝")
	}
	if hits != 0 {
		t.Fatalf("被拒调用不得触达执行体，实得 %d 次", hits)
	}
	if invoke.Retryable(err) {
		t.Fatal("策略否定不可重试：重试不会让策略变松")
	}
}

// TestStageNeedHumanNotRetryable 需要人工的裁决**不得**被判成可重试。
//
// 本适配器最容易做错的一条：「需要人」很容易被顺手写成「重试一下」，于是退避
// 重试一直撞同一堵墙，真正的阻塞点被埋在重试风暴里。
func TestStageNeedHumanNotRetryable(t *testing.T) {
	hits := 0
	// plan 模式的 execute_step 必然无上游审批 → 升人审。
	c := chainWith(t, Stage(NewGate(nil, nil), Op{Type: OpExecuteStep, Mode: ModePlan}, nil), &hits)

	_, err := c.Do(context.Background(), invoke.Call{Name: "apply"})
	if err == nil {
		t.Fatal("需人工的裁决应拒绝")
	}
	if invoke.Retryable(err) {
		t.Fatal("需要人工审批属带外动作，不该被判成可重试")
	}
	if hits != 0 {
		t.Fatalf("被拒调用不得触达执行体，实得 %d 次", hits)
	}
}

// TestStagePlanOnlyBlocks plan 模式的方案提审被降级为「只审不执行」，不得放行。
func TestStagePlanOnlyBlocks(t *testing.T) {
	hits := 0
	c := chainWith(t, Stage(NewGate(nil, nil), Op{Type: OpPlanSubmit, Mode: ModePlan}, nil), &hits)

	_, err := c.Do(context.Background(), invoke.Call{Name: "restart_all"})
	if err == nil {
		t.Fatal("plan_only 不得放行执行")
	}
	if invoke.Retryable(err) {
		t.Fatal("plan_only 不可重试")
	}
	if hits != 0 {
		t.Fatalf("被拒调用不得触达执行体，实得 %d 次", hits)
	}
}

// TestStageRuleVerdictDrivesDecision 例外规则应逐工具生效。
//
// Op 是模板：Tool 由实际调用覆盖。若把装配期的 Tool 原样带下去，审计记的就是
// 「上一次装配时的工具名」——等于记错，而记错的审计比没有审计更危险。
func TestStageRuleVerdictDrivesDecision(t *testing.T) {
	yes, no := true, false
	g := NewGate(&Policy{
		AutoMaxRisk: 2,
		FullMaxRisk: 4,
		Rules: []Rule{
			{ID: "deny_danger", Tools: []string{"danger_tool"}, Verdict: VerdictDeny, Reason: "高危工具"},
			{ID: "pin_mutating", OpTypes: []string{string(OpToolCall)}, Mutating: &yes, Verdict: VerdictDeny, Reason: "变异段"},
		},
	}, nil)

	hits := 0
	c := chainWith(t, Stage(g, Op{Type: OpToolCall, Mode: ModeAuto, Mutating: no}, nil), &hits)

	// 目录内的普通工具：放行。
	if _, err := c.Do(context.Background(), invoke.Call{Name: "read_logs"}); err != nil {
		t.Fatalf("普通工具应放行，实得 %v", err)
	}
	// 被规则点名的高危工具：拒绝。
	if _, err := c.Do(context.Background(), invoke.Call{Name: "danger_tool"}); err == nil {
		t.Fatal("被规则点名的工具应拒绝")
	}
	if hits != 1 {
		t.Fatalf("只应放行那一次调用，实得触达 %d 次", hits)
	}
}

// TestStageReportsDecisionToCallback 裁决必须留痕，否则链上发生了什么无从复盘。
func TestStageReportsDecisionToCallback(t *testing.T) {
	var got []Decision
	hits := 0
	c := chainWith(t, Stage(NewGate(nil, nil), Op{Type: OpToolCall, Mode: ModeFull, Mutating: true},
		func(d Decision) { got = append(got, d) }), &hits)

	if _, err := c.Do(context.Background(), invoke.Call{Name: "restart_service"}); err == nil {
		t.Fatal("变异工具应拒绝")
	}
	if len(got) != 1 {
		t.Fatalf("应留一条裁决，实得 %d 条", len(got))
	}
	if got[0].Verdict != VerdictDeny {
		t.Fatalf("留痕的裁决应为 deny，实得 %q", got[0].Verdict)
	}
	if got[0].RuleID == "" {
		t.Fatal("留痕应带规则 ID（否则无法回答是哪条规则拒的）")
	}
}

// TestStageNilGateDoesNotBlock 未装配审计门 = 存量行为，放行但环仍在链上。
func TestStageNilGateDoesNotBlock(t *testing.T) {
	hits := 0
	c := chainWith(t, Stage(nil, Op{Type: OpToolCall, Mode: ModeAuto}, nil), &hits)

	if _, err := c.Do(context.Background(), invoke.Call{Name: "t"}); err != nil {
		t.Fatalf("未装配审计门应放行（存量行为），实得 %v", err)
	}
	if hits != 1 {
		t.Fatalf("放行后应触达执行体一次，实得 %d", hits)
	}
	if got := c.Stages(); len(got) != 1 || got[0] == "" {
		t.Fatalf("审计环仍应在链上可见，实得 %v", got)
	}
}
