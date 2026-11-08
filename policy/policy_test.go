package policy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDecideMatrix(t *testing.T) {
	g := NewGate(nil, nil)
	cases := []struct {
		name string
		op   Op
		want Verdict
		rule string
	}{
		{"confirm L1 提审→人审", Op{Type: OpPlanSubmit, Mode: ModeConfirm, Risk: 1}, VerdictNeedHuman, "confirm_default"},
		{"confirm L4 提审→人审", Op{Type: OpPlanSubmit, Mode: ModeConfirm, Risk: 4}, VerdictNeedHuman, "confirm_default"},
		{"auto L2→代批", Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: 2}, VerdictAuto, "auto_matrix"},
		{"auto L3→人审", Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: 3}, VerdictNeedHuman, "auto_matrix"},
		{"auto L4→人审", Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: 4}, VerdictNeedHuman, "auto_matrix"},
		{"full L3→代批", Op{Type: OpPlanSubmit, Mode: ModeFull, Risk: 3}, VerdictAuto, "full_matrix"},
		{"full L4→人审（双确认不豁免）", Op{Type: OpPlanSubmit, Mode: ModeFull, Risk: 4}, VerdictNeedHuman, "full_matrix"},
		{"plan L1→只审不执行", Op{Type: OpPlanSubmit, Mode: ModePlan, Risk: 1}, VerdictPlanOnly, "plan_mode"},
		{"采集面只读→放行", Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "get_load"}, VerdictAuto, "readonly_allowlist"},
		{"采集面变异→拒绝（任何模式）", Op{Type: OpToolCall, Mode: ModeFull, Tool: "shell_execute", Mutating: true}, VerdictDeny, "mutating_tool_guard"},
		{"执行面已批准步骤→放行", Op{Type: OpExecuteStep, Mode: ModeAuto, Tool: "shell_execute", Mutating: true}, VerdictAuto, "approved_plan"},
		{"未知操作类型→fail-safe 人审", Op{Type: "weird", Mode: ModeFull}, VerdictNeedHuman, "unknown_op"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := g.Decide(context.Background(), tc.op)
			if got.Verdict != tc.want || got.RuleID != tc.rule {
				t.Fatalf("got %+v, want verdict=%s rule=%s", got, tc.want, tc.rule)
			}
			if got.Decider != DeciderRule {
				t.Fatalf("矩阵/规则裁决 decider 应为 rule，got %s", got.Decider)
			}
		})
	}
}

func TestLoadOverrides(t *testing.T) {
	const path = "_shared/operation_policy.yaml"
	t.Run("缺文件回退基座", func(t *testing.T) {
		p, err := LoadOverrides(fstest.MapFS{}, path)
		if err != nil {
			t.Fatal(err)
		}
		if p.AutoMaxRisk != 2 || p.FullMaxRisk != 3 {
			t.Fatalf("基座阈值被改动: %+v", p)
		}
	})
	t.Run("阈值只能调低不能调高", func(t *testing.T) {
		// 超基座从静默 clamp 改为 fail-fast（toolgate#44：调高=扩权须走评审，
		// 静默回基座把配置错误藏进启动成功里）
		fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
			"operation_policy:\n  auto_max_risk: 4\n  full_max_risk: 3\n")}}
		_, err := LoadOverrides(fsys, path)
		if err == nil {
			t.Fatal("auto_max_risk=4 超基座应 fail-fast")
		}
		fsys = fstest.MapFS{path: &fstest.MapFile{Data: []byte(
			"operation_policy:\n  auto_max_risk: 1\n  full_max_risk: 2\n")}}
		p, err := LoadOverrides(fsys, path)
		if err != nil {
			t.Fatal(err)
		}
		if p.AutoMaxRisk != 1 || p.FullMaxRisk != 2 {
			t.Fatalf("调低应生效，got %+v", p)
		}
	})
	t.Run("解析失败 fail-fast", func(t *testing.T) {
		fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
			"operation_policy: [broken")}}
		if _, err := LoadOverrides(fsys, path); err == nil {
			t.Fatal("解析失败应返回 error")
		}
	})
	t.Run("例外规则优先于矩阵层", func(t *testing.T) {
		fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
			"operation_policy:\n  rules:\n    - id: prod-host-guard\n      op_types: [plan_submit]\n      min_risk: 1\n      verdict: need_human\n      reason: 生产机变更恒人审\n")}}
		p, err := LoadOverrides(fsys, path)
		if err != nil {
			t.Fatal(err)
		}
		g := NewGate(p, nil)
		got := g.Decide(context.Background(), Op{Type: OpPlanSubmit, Mode: ModeFull, Risk: 2})
		if got.Verdict != VerdictNeedHuman || got.RuleID != "prod-host-guard" {
			t.Fatalf("例外规则应优先于矩阵层，got %+v", got)
		}
	})
}

// TestRuleVerdictPlanOnlyIsLegal（toolgate#54）：plan_only 是 Verdict 注释承诺
// 「例外规则可产出」的合法过程值——规则写 plan_only 应原样产出「降级为仅提审」，
// 而不是被合法域白名单 fail-safe 改写成 need_human（语义不同：前者=只审不执行，
// 后者=升人审），更不该给出「规则 verdict 非法（plan_only）」的误导审计理由。
func TestRuleVerdictPlanOnlyIsLegal(t *testing.T) {
	pol := &Policy{AutoMaxRisk: 2, FullMaxRisk: 3, Rules: []Rule{
		{ID: "prod-change-plan-only", Tools: []string{"restart_service"}, Verdict: VerdictPlanOnly, Reason: "生产变更只审不执行"},
	}}
	got := NewGate(pol, nil).Decide(context.Background(),
		Op{Type: OpToolCall, Mode: ModeFull, Tool: "restart_service"})
	if got.Verdict != VerdictPlanOnly || got.Decider != DeciderRule || got.RuleID != "prod-change-plan-only" {
		t.Fatalf("plan_only 规则应原样产出降级裁决, got %+v", got)
	}
	// 拼错的 verdict 仍走 fail-safe 升人审（合法域扩员不等于放弃校验）
	pol.Rules = []Rule{{ID: "typo", Tools: []string{"restart_service"}, Verdict: Verdict("plan_onli")}}
	got = NewGate(pol, nil).Decide(context.Background(),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "restart_service"})
	if got.Verdict != VerdictNeedHuman || !strings.Contains(got.Reason, "非法") {
		t.Fatalf("拼错的 verdict 应 fail-safe 升人审, got %+v", got)
	}
}

type stubArbiter struct {
	v      Verdict
	reason string
	err    error
}

func (s stubArbiter) Arbitrate(_ context.Context, _ Op) (Verdict, string, error) {
	return s.v, s.reason, s.err
}

func TestUnknownRuleArbiter(t *testing.T) {
	pol := &Policy{AutoMaxRisk: 2, FullMaxRisk: 3, Rules: []Rule{
		{ID: "gray", Verdict: VerdictUnknown, Reason: "灰区"},
	}}
	op := Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "weird_tool"}
	t.Run("无仲裁器 fail-safe 升人审", func(t *testing.T) {
		got := NewGate(pol, nil).Decide(context.Background(), op)
		if got.Verdict != VerdictNeedHuman || got.Decider != DeciderRule {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("仲裁成功", func(t *testing.T) {
		got := NewGate(pol, stubArbiter{v: VerdictAuto, reason: "语义只读"}).Decide(context.Background(), op)
		if got.Verdict != VerdictAuto || got.Decider != DeciderLLM || got.Reason != "语义只读" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("仲裁失败 fail-safe", func(t *testing.T) {
		got := NewGate(pol, stubArbiter{err: errors.New("boom")}).Decide(context.Background(), op)
		if got.Verdict != VerdictNeedHuman {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("仲裁非法输出 fail-safe", func(t *testing.T) {
		got := NewGate(pol, stubArbiter{v: VerdictPlanOnly}).Decide(context.Background(), op)
		if got.Verdict != VerdictNeedHuman {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestModeNormalize(t *testing.T) {
	if Normalize("") != ModeConfirm || Normalize("hack") != ModeConfirm {
		t.Fatal("非法/空模式应回落 confirm")
	}
	for _, m := range []string{"confirm", "auto", "plan", "full"} {
		if !ValidMode(m) {
			t.Fatalf("%s 应合法", m)
		}
	}
	if ValidMode("") || ValidMode("Auto") {
		t.Fatal("空串/大小写不一致应非法（严格校验）")
	}
}

// TestRedLineNotBypassableByRules（toolgate#71）：变异工具红线独立于规则裁决链
// ——此前裁决序「例外规则首个命中即胜 → 矩阵层」下，矩阵层的 mutating_tool_guard
// 只在零规则命中时可达，一条缺省语义合法的宽松规则（如 {max_risk: 2,
// verdict: auto_proceed}，未写 op_types/mutating/min_risk）对 Risk=0 的变异工具
// 调用恒命中并放行，「任何模式不可绕过」的硬不变量被击穿，审计还把越权放行记成
// 规则裁决（Decider=rule、RuleID=规则 id）。红线面前无例外：例外规则无论怎么写
// （放行/人审/降级/灰区仲裁）都到不了变异调用。
func TestRedLineNotBypassableByRules(t *testing.T) {
	yes := true
	cases := []struct {
		name    string
		rules   []Rule
		arbiter Arbiter
	}{
		{"宽松规则 max_risk+auto（issue 复现形态）", []Rule{{ID: "fast-track", MaxRisk: 2, Verdict: VerdictAuto}}, nil},
		{"零约束 auto 规则吞掉一切未评级操作", []Rule{{ID: "allow-all", Verdict: VerdictAuto}}, nil},
		{"规则点名放行该变异工具", []Rule{{ID: "allow-shell", Tools: []string{"shell_execute"}, Verdict: VerdictAuto}}, nil},
		{"need_human 规则（红线语义不受规则软化）", []Rule{{ID: "soften", Tools: []string{"shell_execute"}, Verdict: VerdictNeedHuman}}, nil},
		{"plan_only 规则", []Rule{{ID: "downgrade", Tools: []string{"shell_execute"}, Verdict: VerdictPlanOnly}}, nil},
		{"灰区规则+仲裁插件判 auto", []Rule{{ID: "gray", Verdict: VerdictUnknown}}, stubArbiter{v: VerdictAuto, reason: "仲裁说语义只读"}},
		{"规则显式写 mutating:true 也不改红线出口", []Rule{{ID: "explicit", OpTypes: []string{string(OpToolCall)}, Mutating: &yes, Verdict: VerdictAuto}}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pol := DefaultPolicy()
			pol.Rules = c.rules
			got := NewGate(pol, c.arbiter).Decide(context.Background(),
				Op{Type: OpToolCall, Mode: ModeAuto, Tool: "shell_execute", Mutating: true})
			if got.Verdict != VerdictDeny {
				t.Fatalf("红线面前例外规则不可豁免，实得 %+v", got)
			}
			// 审计如实标注：RuleID 是红线而非命中规则——越权放行不得记成合规裁量。
			if got.RuleID != "mutating_tool_guard" {
				t.Fatalf("审计应如实标红线 mutating_tool_guard，实得 RuleID=%s", got.RuleID)
			}
		})
	}
	// 装饰器面同样如实标红线：宽松规则在场时变异调用仍被拦、执行体零触达，
	// 留痕指向红线而非命中规则；拒绝形态为结构化观察（#72 契约）。
	inner := &fakeTool{name: "shell_execute"}
	var got Decision
	pol := DefaultPolicy()
	pol.Rules = []Rule{{ID: "fast-track", MaxRisk: 2, Verdict: VerdictAuto}}
	out, err := AuditGate(NewGate(pol, nil),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "shell_execute", Mutating: true},
		func(_ Op, dec Decision) { got = dec }, inner).InvokableRun(context.Background(), "{}")
	assertRejectedObservation(t, out, err, "mutating_tool_guard")
	if inner.called || got.Verdict != VerdictDeny || got.RuleID != "mutating_tool_guard" {
		t.Fatalf("装饰器面红线拦截且审计如实标注，实得 called=%v %+v", inner.called, got)
	}
	// 对照组：同一宽松规则对非变异采集调用照常生效（修复不误伤规则本义）。
	pol2 := DefaultPolicy()
	pol2.Rules = []Rule{{ID: "fast-track", MaxRisk: 2, Verdict: VerdictAuto, Reason: "低风险代批"}}
	dec := NewGate(pol2, nil).Decide(context.Background(),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "search_docs"})
	if dec.Verdict != VerdictAuto || dec.RuleID != "fast-track" {
		t.Fatalf("宽松规则对非变异调用应照常生效，实得 %+v", dec)
	}
}
