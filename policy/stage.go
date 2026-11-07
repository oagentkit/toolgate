package policy

import (
	"context"

	"github.com/oagentkit/guardx/invoke"
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
// 放行/拒绝结论统一取自 verdictDisposition（唯一映射单源，#53）——装饰器
// AuditGate 与本适配器共用同一张表，两条判定路径不再各判各的：
//
//	VerdictAuto   放行
//	其余合法裁决   拒绝，不可重试（deny=策略否定，重试不会让策略变松；
//	              need_human/plan_only 的下一步是带外人工动作）
//	未识别裁决     拒绝且不可重试，理由如实标注「未识别」（新增 Verdict 值
//	              忘了登记映射时，别让人去修策略，该修的是守卫代码）
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

			allow, retryable, known := verdictDisposition(d.Verdict)
			if allow {
				return invoke.Verdict{Allow: true, Reason: d.Reason}, nil
			}
			reason := d.Reason
			if !known {
				reason = "审计门返回未识别的裁决：" + string(d.Verdict)
			}
			return invoke.Verdict{Allow: false, Reason: reason, Retryable: retryable}, nil
		},
	}
}
