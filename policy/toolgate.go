package policy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"github.com/oagentkit/mcpkit/mcp"
)

// PolicyRejectedPrefix 审计门拒绝观察文本的稳定前缀（oagentkit/toolgate#72）。
// AuditGate 对非 auto 裁决不再返回 Go error，而是返回以此前缀开头的结构化文本
// 观察；宿主/测试可按前缀机器识别「这次调用被策略拒绝」，模型按文本内容换路。
// 裁决的结构化通道始终是 onAudit 回调（完整 Decision），文本观察只面向模型。
const PolicyRejectedPrefix = "POLICY_REJECTED"

// auditTool 操作审计门工具装饰器：每次工具调用先过 Gate 裁决、回调留痕，非 auto
// 裁决沿工具结果通道返回结构化文本观察（oagentkit/toolgate#72「降级不中断」）。
// 此前拒绝以 Go error 返回——eino ToolsNode 对工具 error 直接上抛（节点级失败），
// 整个 ReAct 步骤以 NodeRunError 收场，模型永远看不到拒绝理由，与降级承诺相悖。
// 现改为 mcpkit errorAsObservation 同型：模型可见 verdict/rule/理由、可换路（如
// 改走方案审批面），宿主步骤不中断；需要硬中断的宿主走 invoke 链 Stage——那里
// 拒绝收口为 DenyError，由宿主显式装配，两个面共用 verdictDisposition 一张表。
// 裁决是纯内存操作，无需超时保护，应装在宿主超时装饰器之外。
type auditTool struct {
	gate    *Gate
	op      Op // 构造时固化（工具名/模式/变异标记按次静态可判）
	onAudit func(Op, Decision)
	tool.InvokableTool
}

func (t auditTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	dec := t.gate.Decide(ctx, t.op)
	if t.onAudit != nil {
		t.onAudit(t.op, dec)
	}
	// 非 auto 一律拒绝（fail-closed，oagentkit/toolgate#40）：此前只拦 VerdictDeny，
	// need_human/unknown/plan_only 静默放行执行体——宿主配置「该工具需人工审批」
	// 后装饰器形态下工具照常执行。放行/拒绝结论与 Stage 同取唯一映射单源
	// verdictDisposition（#53）：两个面共用一张表，不再各判各的——#40 的根因
	// 正是两条判定路径漂移，一面修了另一面没修。
	if allow, _, _ := verdictDisposition(dec.Verdict); !allow {
		return rejectionObservation(dec), nil
	}
	return t.InvokableTool.InvokableRun(ctx, args, opts...)
}

// rejectionObservation 把非 auto 裁决渲染成模型可见的结构化文本观察
// （oagentkit/toolgate#72）。文本含 verdict/rule/reason 三要素与换路指引：
// deny 是策略否定（换参数重试不会变松），need_human/plan_only 的下一步是
// 带外人工动作（如提交方案审批）。
func rejectionObservation(dec Decision) string {
	return fmt.Sprintf(
		"%s: 操作审计门拒绝本次工具调用，未执行（verdict=%s, rule=%s, reason: %s）。"+
			"不要换参数重试同一调用：deny 是策略否定，重试不会让策略变松；"+
			"need_human/plan_only 需要带外人工动作（如提交方案审批），请改走相应路径，或基于已获取的信息继续。",
		PolicyRejectedPrefix, dec.Verdict, dec.RuleID, dec.Reason)
}

// AuditGate 给工具套操作审计门；gate 为 nil 原样返回（未装配审计门=存量行为）。
//
// 拒绝语义（oagentkit/toolgate#72）：非 auto 裁决返回以 PolicyRejectedPrefix 开头
// 的文本观察（nil error），模型可见裁决与理由、可换路，宿主 ReAct 步骤不中断；
// 裁决全量经 onAudit 留痕。需要把拒绝变成硬中断的宿主不要用本装饰器，走 invoke
// 链 Stage（见 stage.go）。
func AuditGate(g *Gate, op Op, onAudit func(Op, Decision), t tool.InvokableTool) tool.InvokableTool {
	if g == nil {
		return t
	}
	return auditTool{gate: g, op: op, onAudit: onAudit, InvokableTool: t}
}

// OutputSchema 出参 schema 透传（v0.12.5 OutputSchemer 可选能力面）。
func (t auditTool) OutputSchema() json.RawMessage {
	if o, ok := t.InvokableTool.(mcp.OutputSchemer); ok {
		return o.OutputSchema()
	}
	return nil
}
