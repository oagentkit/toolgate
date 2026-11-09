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
// 两面拒绝语义分工（oagentkit/toolgate#72）：装饰器对非 auto 裁决返回结构化
// 文本观察（nil error）——模型可见裁决与理由、可换路，宿主 ReAct 步骤不中断
// （工具 error 会被 eino ToolsNode 直接上抛炸掉整个步骤）；本适配器（invoke 链）
// 才是硬中断档——拒绝收口为 DenyError，由宿主显式装配。放行/拒绝结论两面共用
// verdictDisposition 一张表（#53）。装配可选项见 StageOption / WithMutatingClassifier。

// StageOption Stage 装配可选项。
type StageOption func(*stageConfig)

// stageConfig Stage 装配配置（零值 = 全部缺省）。
type stageConfig struct {
	classifyMutating func(tool string) *bool
}

// WithMutatingClassifier 按实际工具名重算变更标记的分类器（oagentkit/toolgate#85）。
// 通常传 agentrun.IsMutatingTool（变异动词段表在调用方侧，policy 不跨模块
// 依赖）。未提供时模板的 Mutating 原样生效——模板也未标记（nil）的 tool_call
// 由 Decide fail-safe 升人审。
func WithMutatingClassifier(classify func(tool string) bool) StageOption {
	return func(c *stageConfig) {
		c.classifyMutating = func(tool string) *bool {
			b := classify(tool)
			return &b
		}
	}
}

// Stage 把操作审计门接成 invoke 链上的一环（为什么需要链上适配、两面拒绝
// 语义分工见文件头部注释）。
//
// # Op 的取法
//
// op 是**模板**：Type / Mode / Risk 在装配期固化（它们按调用点是静态可判的），
// Tool 每次由链上的实际调用覆盖。这样一个 stage 能服务同一调用点下的多个工具，
// 不必为每个工具各装配一次。
//
// Mutating 同为模板字段，但**换工具名必须重算**（oagentkit/toolgate#85）：
// 模板期标记描述的是装配时的工具，链上实际调用的工具可以不同——此前覆盖
// Tool 时不重算，模板声明「非变更」的 stage 换成 delete_repository 照样放行，
// 红线被模板期标记与实际工具的脱节绕过。经 WithMutatingClassifier 提供分类器
// 后，每次调用按实际工具名重算；未提供分类器且模板未标记（nil）时，Decide
// 的 fail-safe 段把 tool_call 升人审——默认方向 fail-closed。
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
func Stage(g *Gate, op Op, onDecision func(Decision), opts ...StageOption) invoke.Stage {
	var cfg stageConfig
	for _, o := range opts {
		o(&cfg)
	}
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
				// 换工具名就重算变更标记（oagentkit/toolgate#85）：模板期
				// 标记描述的是装配时的工具，与实际调用脱节时红线被绕过。
				if cfg.classifyMutating != nil {
					effective.Mutating = cfg.classifyMutating(c.Name)
				}
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
