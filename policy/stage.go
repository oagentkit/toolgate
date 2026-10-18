package policy

import (
	"context"

	"github.com/oagentkit/toolgate/invoke"
)

// Stage 把操作审计门接成 invoke 链上的一环。
//
// # 为什么要这个适配器
//
// AuditGate 是**装饰器**：宿主忘了包一层，这次调用就没有审计门——而且不报错、
// 不告警，事后从日志里也看不出这次调用没走门。invoke 链不存在这个形态：
// 拿到链就只能走 Do，绕过代码写不出来（见 invoke 包头）。
//
// 故两者并存：AuditGate 保留以兼容既有调用点，**新代码的审计门走这里**。
//
// # Op 的取法
//
// op 是**模板**：Type / Mode / Risk / Mutating 在装配期固化（它们按调用点是静态可判的），
// Tool 每次由链上的实际调用覆盖。这样一个 stage 能服务同一调用点下的多个工具，
// 不必为每个工具各装配一次。
//
// # 裁决到放行/拒绝的映射
//
//	VerdictAuto      放行
//	VerdictDeny      拒绝，不可重试（策略否定；重试不会让策略变松）
//	VerdictNeedHuman 拒绝，**不可重试**：重试会立刻得到同样的结果，
//	                人工审批是带外动作，不该由退避重试去撞
//	VerdictPlanOnly  拒绝（执行被降级为仅提审），不可重试
//	VerdictUnknown   拒绝（fail-closed），不可重试
//
// 需要人工介入的情形一律不可重试：把「需要人」翻译成「多试几次」是退避语义
// 最常见的误用，重试风暴会一直撞同一堵墙，且把真正的阻塞点埋在日志里。
func Stage(g *Gate, op Op, onDecision func(Decision)) invoke.Stage {
	name := "audit_gate"
	return invoke.StageFunc{
		StageName: name,
		Fn: func(ctx context.Context, c invoke.Call) (invoke.Verdict, error) {
			// g 为 nil 视为不放行这道门（未装配审计门 = 存量行为），
			// 但**不静默**：留痕里仍会有这一环，名称可查。
			if g == nil {
				return invoke.Verdict{Allow: true, Reason: "未装配审计门"}, nil
			}

			effective := op
			if c.Name != "" {
				effective.Tool = c.Name
			}
			d := g.Decide(ctx, effective)
			if onDecision != nil {
				onDecision(d)
			}

			switch d.Verdict {
			case VerdictAuto:
				return invoke.Verdict{Allow: true, Reason: d.Reason}, nil
			case VerdictDeny, VerdictPlanOnly, VerdictNeedHuman, VerdictUnknown:
				return invoke.Verdict{
					Allow:     false,
					Reason:    d.Reason,
					Retryable: false,
				}, nil
			default:
				// 认不出的裁决一律按拒绝且不可重试：猜错的代价不是少放行一次，
				// 而是把一次策略否定放进无穷重试。
				return invoke.Verdict{
					Allow:     false,
					Reason:    "审计门返回未识别的裁决：" + string(d.Verdict),
					Retryable: false,
				}, nil
			}
		},
	}
}
