package policy

// RawNamer 透传回归钉（toolgate#73）：auditTool 包装层透传原始工具名，跨包装层
// 堆叠组合在此钉住（mcpkit 侧 selfHealTool/errorAsObservation 各自的透传测试只测
// 自家层，跨仓堆叠后最外层取 RawToolName 的组合此前无人测，断链正是从这里漏掉）。

import (
	"testing"

	"github.com/cloudwego/eino/components/tool"

	"github.com/oagentkit/mcpkit/mcp"
	"github.com/oagentkit/toolgate/toolprior"
)

// rawNamedTool 带 RawToolName 的工具替身（模拟 mcp.Pool 出口工具：暴露名带
// server 命名空间，原始名才是按名判定消费方要的，oagentkit/mcpkit#46）。
type rawNamedTool struct {
	fakeTool
	raw string
}

func (f *rawNamedTool) RawToolName() string { return f.raw }

var _ mcp.RawNamer = (*rawNamedTool)(nil)

// TestAuditGateRawToolNamePassthrough（toolgate#73）：auditTool 包装层透传原始
// 工具名——宿主自然堆叠 invokableTool → AuditGate → 模型时最外层是本包装，此前
// 类型断言断到外层即断链，RawToolNameOf 回落 Info().Name 拿到带命名空间的暴露名，
// 按名判定的消费方（只读白名单/动词启发式/审计归类）静默漏判。
func TestAuditGateRawToolNamePassthrough(t *testing.T) {
	inner := &rawNamedTool{fakeTool: fakeTool{name: "srv.search_docs"}, raw: "search_docs"}
	wrapped := AuditGate(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "search_docs"}, nil, inner)
	if got := mcp.RawToolNameOf(wrapped); got != "search_docs" {
		t.Fatalf("包装后最外层应取到原始名，got %q（want search_docs）", got)
	}
	// 内层非 RawNamer：空串=无原始名可用（同 mcpkit selfHealTool 契约；不得谎报
	// 暴露名为原始名，消费方对空串不得放行）。
	plain := AuditGate(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "get_x"}, nil, &fakeTool{name: "get_x"})
	if got := mcp.RawToolNameOf(plain); got != "" {
		t.Fatalf("内层非 RawNamer 应返回空串，got %q", got)
	}
}

// TestRawToolNameAcrossWrappedStack（toolgate#73）：跨包装层堆叠组合钉——宿主
// 自然形态 invokableTool → AuditGate → LimitCalls → 模型（及反向装配序），最外层
// 取原始名必须拿到 search_docs 而非 srv.search_docs。
func TestRawToolNameAcrossWrappedStack(t *testing.T) {
	inner := &rawNamedTool{fakeTool: fakeTool{name: "srv.search_docs"}, raw: "search_docs"}
	audit := AuditGate(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "search_docs"}, nil, inner)
	// 堆叠序一：AuditGate 在内、LimitCalls 在外（issue 描述的自然形态）。
	outer := toolprior.LimitCalls(audit, 3)
	if got := mcp.RawToolNameOf(outer); got != "search_docs" {
		t.Fatalf("AuditGate→LimitCalls 堆叠后最外层应取到原始名，got %q", got)
	}
	// 堆叠序二：LimitCalls 在内、AuditGate 在外。
	outer2 := AuditGate(NewGate(nil, nil),
		Op{Type: OpToolCall, Mode: ModeAuto, Tool: "search_docs"}, nil,
		toolprior.LimitCalls(inner, 3).(tool.InvokableTool))
	if got := mcp.RawToolNameOf(outer2); got != "search_docs" {
		t.Fatalf("LimitCalls→AuditGate 堆叠后最外层应取到原始名，got %q", got)
	}
}
