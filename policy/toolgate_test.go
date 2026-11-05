package policy

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
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

func TestWithAuditGateDeniesMutatingCall(t *testing.T) {
	inner := &fakeTool{name: "shell_execute"}
	var got Decision
	wrapped := AuditGate(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "shell_execute", Mutating: true},
		func(_ Op, dec Decision) { got = dec }, inner)
	if _, err := wrapped.InvokableRun(context.Background(), "{}"); err == nil {
		t.Fatal("采集面变异工具调用应被拒绝")
	}
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

// AuditGate fail-closed（toolgate#40）：非 VerdictAuto 一律拒绝且不可重试——
// 与 Stage 对同一裁决集的映射对齐。此前只拦 deny，need_human/unknown 静默
// 放行执行体（宿主配置「需人工审批」后装饰器形态下工具照跑）。
func TestAuditGateFailClosed(t *testing.T) {
	// 裁决注入走 policy 规则面（Issue5485 同款宿主用法：LoadOverrides 配
	// 「该工具需人工审批」）；未识别裁决用包内 stubArbiter 直造。
	mk := func(verdict Verdict, rule string) (tool.InvokableTool, *fakeTool) {
		inner := &fakeTool{name: "restart_service"}
		g := gateForVerdict(verdict, rule)
		return AuditGate(g, Op{Type: OpToolCall, Mode: ModeConfirm, Tool: "restart_service", Mutating: true}, nil, inner), inner
	}
	for _, c := range []struct {
		v    Verdict
		name string
	}{{VerdictDeny, "deny"}, {VerdictNeedHuman, "need_human"}, {VerdictUnknown, "unknown"}, {VerdictPlanOnly, "plan_only"}, {Verdict("weird"), "unrecognized"}} {
		t.Run(c.name, func(t *testing.T) {
			wrapped, inner := mk(c.v, "human-gate")
			_, err := wrapped.InvokableRun(context.Background(), "{}")
			if err == nil {
				t.Fatalf("%s 裁决应拒绝执行", c.v)
			}
			if inner.called {
				t.Fatalf("%s: 执行体不得被触达", c.v)
			}
			// 规则裁决带规则 id；arbiter 非法输出被 Decide fail-safe 升人审
			//（RuleID 换 fail-safe 归属）——只断言拒绝不落执行体。
			if c.v == VerdictDeny || c.v == VerdictNeedHuman || c.v == VerdictUnknown {
				if !strings.Contains(err.Error(), "human-gate") {
					t.Fatalf("错误应带规则 id: %v", err)
				}
			}
		})
	}
	// auto 放行
	g := gateForVerdict(VerdictAuto, "auto-rule")
	inner := &fakeTool{name: "get_status"}
	if _, err := AuditGate(g, Op{Type: OpToolCall, Mode: ModeAuto, Tool: "get_load"}, nil, inner).InvokableRun(context.Background(), "{}"); err != nil {
		t.Fatalf("auto 应放行: %v", err)
	}
	if !inner.called {
		t.Fatal("auto 应真实执行")
	}
}

// gateForVerdict 造一个裁决恒为 v 的 Gate：规则裁决可表达的值走 LoadRules
// （need_human/deny），表达不了的（plan_only/未识别）走 stubArbiter。
func gateForVerdict(v Verdict, ruleID string) *Gate {
	switch v {
	case VerdictDeny, VerdictNeedHuman, VerdictUnknown:
		p := DefaultPolicy()
		p.Rules = []Rule{{ID: ruleID, Tools: []string{"restart_service"}, Verdict: v}}
		return NewGate(p, nil)
	default:
		return NewGate(DefaultPolicy(), stubArbiter{v: v, reason: "stub-hit"})
	}
}
