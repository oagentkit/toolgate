package policy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"github.com/oagentkit/mcpkit/mcp"
)

// auditTool 操作审计门工具装饰器：每次工具调用先过 Gate 裁决、回调留痕，deny 即
// 返回错误（沿工具结果通道如实降级，与派发拒绝同语义——agent 拿到工具错误自行降级，
// 不中断整个环节）。裁决是纯内存操作，无需超时保护，应装在宿主超时装饰器之外。
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
	// fail-closed 与 Stage 同口径（oagentkit/toolgate#40）：此前只拦 VerdictDeny，
	// need_human/unknown/plan_only 静默放行执行体——宿主配置「该工具需人工审批」
	// 后装饰器形态下工具照常执行，与 Stage 的「拒绝且不可重试」映射相反。
	// 认不出的裁决同样拒绝（猜错的代价不是少放行一次）。
	if dec.Verdict != VerdictAuto {
		return "", fmt.Errorf("policy: 操作审计门拒绝该工具调用（verdict=%s, %s: %s）",
			dec.Verdict, dec.RuleID, dec.Reason)
	}
	return t.InvokableTool.InvokableRun(ctx, args, opts...)
}

// AuditGate 给工具套操作审计门；gate 为 nil 原样返回（未装配审计门=存量行为）。
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
