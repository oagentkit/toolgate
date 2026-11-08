package policy

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/oagentkit/guardx/invoke"
)

// fakeTool 记录式工具替身：记录是否被真实调用。
type fakeTool struct {
	name   string
	called bool
}

func (f *fakeTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: f.name}, nil
}

func (f *fakeTool) InvokableRun(_ context.Context, _ string, _ ...tool.Option) (string, error) {
	f.called = true
	return "ok", nil
}

// 编译期确认实现 InvokableTool（Info+InvokableRun）。
var _ tool.InvokableTool = (*fakeTool)(nil)

// assertRejectedObservation 断言一次装饰器面拒绝的完整形态（#72 契约）：
// nil error（不炸宿主 ReAct 步骤）+ 以 PolicyRejectedPrefix 开头的结构化文本
// 观察（模型可见裁决与理由、可换路）。
func assertRejectedObservation(t *testing.T, out string, err error, wantRule string) {
	t.Helper()
	if err != nil {
		t.Fatalf("拒绝应降级为文本观察而非 error（eino ToolsNode 会把 error 上抛炸掉整个步骤，#72）: %v", err)
	}
	if !strings.HasPrefix(out, PolicyRejectedPrefix) {
		t.Fatalf("拒绝应返回以 %s 开头的结构化观察: %q", PolicyRejectedPrefix, out)
	}
	if !strings.Contains(out, "verdict=") || !strings.Contains(out, "rule=") {
		t.Fatalf("观察应含 verdict/rule 结构化字段: %q", out)
	}
	if wantRule != "" && !strings.Contains(out, wantRule) {
		t.Fatalf("观察应能定位到规则/红线 %s: %q", wantRule, out)
	}
	if !strings.Contains(out, "不要换参数重试") {
		t.Fatalf("观察应带换路指引（deny 重试不会变松；need_human 走带外人工）: %q", out)
	}
}

func TestWithAuditGateDeniesMutatingCall(t *testing.T) {
	inner := &fakeTool{name: "shell_execute"}
	var got Decision
	wrapped := AuditGate(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "shell_execute", Mutating: true},
		func(_ Op, dec Decision) { got = dec }, inner)
	out, err := wrapped.InvokableRun(context.Background(), "{}")
	assertRejectedObservation(t, out, err, "mutating_tool_guard")
	if inner.called {
		t.Fatal("被拒绝的调用不应触达底层工具")
	}
	if got.Verdict != VerdictDeny || got.Decider != DeciderRule {
		t.Fatalf("留痕回调应收到 deny 裁决，got %+v", got)
	}
}

func TestWithAuditGatePassesReadonlyCall(t *testing.T) {
	inner := &fakeTool{name: "get_load"}
	audits := 0
	wrapped := AuditGate(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "get_load"},
		func(_ Op, _ Decision) { audits++ }, inner)
	if out, err := wrapped.InvokableRun(context.Background(), "{}"); err != nil || out != "ok" {
		t.Fatalf("只读调用应透传: out=%q err=%v", out, err)
	}
	if !inner.called || audits != 1 {
		t.Fatalf("底层应被调用且留痕一次: called=%v audits=%d", inner.called, audits)
	}
}

func TestWithAuditGateNilGatePassthrough(t *testing.T) {
	inner := &fakeTool{name: "shell_execute"}
	wrapped := AuditGate(nil,
		Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "shell_execute", Mutating: true},
		nil, inner)
	if _, err := wrapped.InvokableRun(context.Background(), "{}"); err != nil {
		t.Fatalf("未装配审计门应保持存量行为: %v", err)
	}
	if !inner.called {
		t.Fatal("底层应被调用")
	}
}

// TestAuditGateFailClosed（toolgate#40/#53/#72）：非 VerdictAuto 一律拒绝且执行体
// 零触达——放行/拒绝结论取自唯一映射单源 verdictDisposition，与 Stage 同表。拒绝
// 形态为结构化文本观察（nil error，#72「降级不中断」）：eino ToolsNode 对工具
// error 直接上抛、整个 ReAct 步骤以 NodeRunError 收场，模型永远看不到拒绝理由；
// 观察文本则模型可见裁决与规则定位、可换路。裁决注入走宿主真实表达路径：规则面
// （deny/need_human/plan_only，#54 起 plan_only 可由规则直产）与矩阵面（plan 模式
// 提审恒 plan_only）。
func TestAuditGateFailClosed(t *testing.T) {
	type row struct {
		name string
		g    *Gate
		op   Op
		id   string // 拒绝观察应能定位到的规则 id
	}
	// 规则面注入。Mutating 故意置 false：若规则注入失效，裁决会落回只读白名单
	// 放行、本测当场红——置 true 则红线前置兜底 deny（#71），测的其实是红线不是
	// 规则注入。
	mkRule := func(v Verdict) row {
		id := "rule-" + string(v)
		p := DefaultPolicy()
		p.Rules = []Rule{{ID: id, Tools: []string{"restart_service"}, Verdict: v}}
		return row{name: string(v) + " 规则", g: NewGate(p, nil),
			op: Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "restart_service"}, id: id}
	}
	for _, r := range []row{
		mkRule(VerdictDeny),
		mkRule(VerdictNeedHuman),
		mkRule(VerdictPlanOnly), // plan_only=降级仅提审 ≠ need_human，语义不得被改写
		mkRule(VerdictUnknown),  // 灰区无仲裁器：Decide fail-safe 升人审，装饰器仍拒绝
		{"plan_only 矩阵（plan 模式提审）", NewGate(nil, nil),
			Op{Type: OpPlanSubmit, Mode: ModePlan, Risk: 2}, "plan_mode"},
	} {
		t.Run(r.name, func(t *testing.T) {
			inner := &fakeTool{name: r.op.Tool}
			out, err := AuditGate(r.g, r.op, nil, inner).InvokableRun(context.Background(), "{}")
			assertRejectedObservation(t, out, err, r.id)
			if inner.called {
				t.Fatal("被拒调用不得触达执行体")
			}
		})
	}
	// auto 放行且真实执行（对照组：拒绝不是因为门坏了）。
	inner := &fakeTool{name: "get_status"}
	if _, err := AuditGate(NewGate(nil, nil), Op{Type: OpToolCall, Mode: ModeAuto, Tool: "get_load"}, nil, inner).InvokableRun(context.Background(), "{}"); err != nil {
		t.Fatalf("auto 应放行: %v", err)
	}
	if !inner.called {
		t.Fatal("auto 应真实执行")
	}
}

// TestVerdictDispositionSingleSource（#53）：唯一映射单源对裁决全集的逐点
// 断言——auto 是唯一放行值；四个非 auto 合法值（含 plan_only）拒绝且不可
// 重试；认不出的值 fail-closed 并标记未登记。新增 Verdict 值忘了登记时这里
// 先红，而不是等两条执行路径里的某一面悄悄放行。
func TestVerdictDispositionSingleSource(t *testing.T) {
	for _, v := range []Verdict{VerdictAuto, VerdictNeedHuman, VerdictDeny, VerdictPlanOnly, VerdictUnknown} {
		allow, retryable, known := verdictDisposition(v)
		if !known {
			t.Fatalf("%s 应为已登记裁决", v)
		}
		if v == VerdictAuto {
			if !allow || retryable {
				t.Fatalf("auto 应放行且不可重试语义不适用: allow=%v retryable=%v", allow, retryable)
			}
			continue
		}
		if allow || retryable {
			t.Fatalf("%s 应拒绝且不可重试: allow=%v retryable=%v", v, allow, retryable)
		}
	}
	// 未识别裁决（未来新增值/脏数据）：拒绝、不可重试、known=false——调用方
	// 据此把「未识别」如实写进审计理由，而不是谎报成「策略拒绝了」。
	allow, retryable, known := verdictDisposition(Verdict("weird"))
	if allow || retryable || known {
		t.Fatalf("未识别裁决应 fail-closed 且标记未登记: allow=%v retryable=%v known=%v", allow, retryable, known)
	}
}

// TestAuditFacesAgreeOnEveryProducibleVerdict（#53/#72）：装饰器与 Stage 是同一张
// 映射表（verdictDisposition）的两个出口——对 Gate 能产出的每一类裁决，两面
// 必须同放行/同拒绝、同触达执行体。拒绝形态按各自面语义收口（#72）：装饰器=
// 结构化文本观察（nil error，模型可见可换路），Stage 链=DenyError 硬中断且
// 一律不可重试。#40 的形态就是「一面修了另一面没修」：此测红 = 判定又漂移回
// 两张表了。
func TestAuditFacesAgreeOnEveryProducibleVerdict(t *testing.T) {
	ruleGate := func(v Verdict) *Gate {
		p := DefaultPolicy()
		p.Rules = []Rule{{ID: "rule-" + string(v), Tools: []string{"restart_service"}, Verdict: v}}
		return NewGate(p, nil)
	}
	for _, c := range []struct {
		name string
		g    *Gate
		op   Op
	}{
		{"auto 白名单放行", NewGate(nil, nil), Op{Type: OpToolCall, Mode: ModeAuto, Tool: "read_logs"}},
		{"deny 变异拒绝", NewGate(nil, nil), Op{Type: OpToolCall, Mode: ModeFull, Tool: "restart_service", Mutating: true}},
		{"need_human 规则", ruleGate(VerdictNeedHuman), Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "restart_service"}},
		{"plan_only 规则（#54）", ruleGate(VerdictPlanOnly), Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "restart_service"}},
		{"plan_only 矩阵", NewGate(nil, nil), Op{Type: OpPlanSubmit, Mode: ModePlan, Risk: 2, Tool: "restart_all"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			// 装饰器面：拒绝=文本观察（nil error）
			inner := &fakeTool{name: c.op.Tool}
			out, decErr := AuditGate(c.g, c.op, nil, inner).InvokableRun(context.Background(), "{}")
			if decErr != nil {
				t.Fatalf("装饰器拒绝不得返回 error（#72）: %v", decErr)
			}
			decRejected := strings.HasPrefix(out, PolicyRejectedPrefix)
			// Stage 面：拒绝=DenyError 硬中断
			hits := 0
			ch := chainWith(t, Stage(c.g, c.op, nil), &hits)
			_, stErr := ch.Do(context.Background(), invoke.Call{Name: c.op.Tool})
			if decRejected != (stErr != nil) {
				t.Fatalf("两面结论漂移: 装饰器拒绝=%v（out=%q）, Stage 拒绝=%v（err=%v）",
					decRejected, out, stErr != nil, stErr)
			}
			if inner.called != (hits > 0) {
				t.Fatalf("执行体触达漂移: 装饰器触达=%v, Stage 触达=%d 次", inner.called, hits)
			}
			if !decRejected && !inner.called {
				t.Fatal("放行面应真实触达执行体（对照组）")
			}
			if stErr != nil && invoke.Retryable(stErr) {
				t.Fatal("审计门拒绝不可重试")
			}
		})
	}
}
