package policy

// toolgate#133 回归钉：LoadOverrides 此前只校验 verdict/id，对 op_types 拼错
// 零校验——OpType 是封闭词表，拼错即 slices.Contains 永假、规则对所有操作
// 永失配；deny 规则因此变静默死字（fail-open，#86 点名的「deny 最坏失效形态」
// 换了个字段复现）。修后与 verdict 同款 fail-fast：启动即败，宁拒不启动
// 不带错跑。标题带 issue 号——回归时按号检索。

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestLoadOverridesBadOpTypesFailsFast_Issue133 攻击样本：op_types 拼错
// （toolcall，漏下划线）的 deny 规则必须在加载期被拒，而非静默永失配。
func TestLoadOverridesBadOpTypesFailsFast_Issue133(t *testing.T) {
	const path = "p.yaml"
	fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
		"operation_policy:\n  rules:\n    - id: deny-bad\n      op_types: [toolcall]\n      verdict: deny\n      reason: 拼错即死字\n")}}
	_, err := LoadOverrides(fsys, path)
	if err == nil {
		t.Fatal("op_types 拼错应 fail-fast——deny 规则静默死字（toolgate#133）")
	}
	if !strings.Contains(err.Error(), "op_types") || !strings.Contains(err.Error(), "tool_call") {
		t.Fatalf("报错应点名 op_types 与合法词表: %v", err)
	}
}

// TestLoadOverridesOpTypesDomain_Issue133 对照：合法词表（含空=不限）原样载入。
func TestLoadOverridesOpTypesDomain_Issue133(t *testing.T) {
	const path = "p.yaml"
	fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
		"operation_policy:\n  rules:\n    - id: r-all\n      verdict: need_human\n    - id: r-tool\n      op_types: [tool_call, execute_step, plan_submit]\n      verdict: need_human\n")}}
	p, err := LoadOverrides(fsys, path)
	if err != nil {
		t.Fatalf("合法 op_types 应放行: %v", err)
	}
	if len(p.Rules) != 2 {
		t.Fatalf("规则应原样载入: %+v", p.Rules)
	}
	if len(p.Rules[1].OpTypes) != 3 {
		t.Fatalf("op_types 应原样保留: %+v", p.Rules[1].OpTypes)
	}
}
