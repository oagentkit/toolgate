package policy

// toolgate#134 回归钉：外置阈值此前只做「非负、不超基座」两道单键校验，
// 不校验两档之间的有序关系——auto_max_risk > full_max_risk 合法通过，ModeAuto
// 比 ModeFull 更宽，模式档位静默倒挂。修后加载末尾校验 auto ≤ full，违例
// fail-fast（auto == full 合法：语义塌缩但方向保守）。标题带 issue 号——回归时按号检索。

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestLoadOverridesInvertedTiersFailsFast_Issue134 倒挂配置启动即败。
func TestLoadOverridesInvertedTiersFailsFast_Issue134(t *testing.T) {
	const path = "p.yaml"
	// 外置文件只能调低不能调高（基座 auto=2/full=3）：倒挂只能经「auto 顶满
	// 基座 + full 调低」或「auto 缺省继承 + full 调低」构造
	fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
		"operation_policy:\n  auto_max_risk: 2\n  full_max_risk: 1\n")}}
	_, err := LoadOverrides(fsys, path)
	if err == nil {
		t.Fatal("auto > full 应 fail-fast——模式档位倒挂（toolgate#134）")
	}
	if !strings.Contains(err.Error(), "倒挂") || !strings.Contains(err.Error(), "auto_max_risk=2") {
		t.Fatalf("报错应点名模式档位倒挂: %v", err)
	}
}

// TestLoadOverridesInvertedTiersViaPartialKeys_Issue134 只写一档也能倒挂：
// full_max_risk: 1 + auto 缺省继承基座 2 → 倒挂 fail-fast。
func TestLoadOverridesInvertedTiersViaPartialKeys_Issue134(t *testing.T) {
	const path = "p.yaml"
	fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
		"operation_policy:\n  full_max_risk: 1\n")}}
	if _, err := LoadOverrides(fsys, path); err == nil {
		t.Fatal("full 调低到低于基座 auto 应 fail-fast（toolgate#134）")
	}
	// auto ≡ full（auto 显式 2 = full 显式 2）：合法，语义塌缩但方向保守
	fsysEq := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
		"operation_policy:\n  auto_max_risk: 2\n  full_max_risk: 2\n")}}
	p, err := LoadOverrides(fsysEq, path)
	if err != nil {
		t.Fatalf("auto == full 不应拦截: %v", err)
	}
	if p.AutoMaxRisk != 2 || p.FullMaxRisk != 2 {
		t.Fatalf("阈值应原样生效: %+v", p)
	}
}

// TestLoadOverridesOrderedTiersOK_Issue134 对照：正常有序档位照常载入。
func TestLoadOverridesOrderedTiersOK_Issue134(t *testing.T) {
	const path = "p.yaml"
	fsys := fstest.MapFS{path: &fstest.MapFile{Data: []byte(
		"operation_policy:\n  auto_max_risk: 1\n  full_max_risk: 2\n")}}
	p, err := LoadOverrides(fsys, path)
	if err != nil {
		t.Fatalf("有序档位应放行: %v", err)
	}
	if p.AutoMaxRisk != 1 || p.FullMaxRisk != 2 {
		t.Fatalf("阈值应原样生效: %+v", p)
	}
}
