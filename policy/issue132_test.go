package policy

// toolgate#132 回归钉：「未评级不代批」此前只堵了规则面（#99 的
// Rule.matches 守卫）——unknown 规则走仲裁器的返回值没有同款守卫，仲裁器
// 回 auto_proceed 时 Risk=0 的 plan_submit 直接代批（不变量在规则面/矩阵面
// 之外的第三面被绕过）。修后仲裁返回 VerdictAuto 前套与 #99 相同的守卫，
// 未评级方案一律转 need_human（三面一个口径）。标题带 issue 号——回归时按号检索。

import (
	"context"
	"testing"
)

// autoArbiter 恒回 auto 的仲裁器（复现形态：LLM 仲裁插件）。
type autoArbiter struct{}

func (autoArbiter) Arbitrate(context.Context, Op) (Verdict, string, error) {
	return VerdictAuto, "仲裁器认为可以继续", nil
}

// fixedArbiter 恒回 v 的仲裁器（对照面）。
type fixedArbiter struct{ v Verdict }

func (a fixedArbiter) Arbitrate(context.Context, Op) (Verdict, string, error) {
	return a.v, "fixed", nil
}

func grayGate(a Arbiter) *Gate {
	return NewGate(&Policy{AutoMaxRisk: 2, FullMaxRisk: 3, Rules: []Rule{
		{ID: "gray", OpTypes: []string{string(OpPlanSubmit)}, Verdict: VerdictUnknown},
	}}, a)
}

// TestArbiterAutoUndersignedPlan_Issue132 攻击样本：unknown 规则 + 返回 auto
// 的仲裁器 + Risk=0 的 plan_submit → 修前 auto_proceed（decider=llm）代批；
// 修后必须 need_human（未评级不代批，仲裁面同口径）。
func TestArbiterAutoUndersignedPlan_Issue132(t *testing.T) {
	d := grayGate(autoArbiter{}).Decide(context.Background(),
		Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: 0})
	if d.Verdict != VerdictNeedHuman {
		t.Fatalf("Risk=0 经仲裁回 auto 判 %s——未评级方案被代批（toolgate#132）：%+v", d.Verdict, d)
	}
	if d.Decider == DeciderLLM {
		t.Fatalf("守卫接管后裁决者不得记成 LLM： %+v", d)
	}
}

// TestArbiterAutoRatedPlanStillAuto_Issue132 对照：已评级（Risk>=1）方案经
// 仲裁回 auto 照常代批——守卫只针对未评级，不得收窄合法灰区代批面。
func TestArbiterAutoRatedPlanStillAuto_Issue132(t *testing.T) {
	for _, risk := range []int{1, 2, 3} {
		d := grayGate(autoArbiter{}).Decide(context.Background(),
			Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: risk})
		if d.Verdict != VerdictAuto || d.Decider != DeciderLLM {
			t.Fatalf("Risk=%d 经仲裁回 auto 应照常代批（decider=llm），得 %+v", risk, d)
		}
	}
}

// TestArbiterNonAutoUnchanged_Issue132 对照：仲裁回 need_human/deny 不受守卫
// 影响（守卫只在 auto 时接管）。
func TestArbiterNonAutoUnchanged_Issue132(t *testing.T) {
	cases := []Verdict{VerdictNeedHuman, VerdictDeny}
	for _, v := range cases {
		d := grayGate(fixedArbiter{v}).Decide(context.Background(),
			Op{Type: OpPlanSubmit, Mode: ModeAuto, Risk: 0})
		if d.Verdict != v {
			t.Fatalf("仲裁回 %s 应原样透传，得 %s（toolgate#132）", v, d.Verdict)
		}
	}
}
