package policy

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// 阈值三档语义（toolgate#44）：缺省继承基座 / 正数调低 / **0=禁用代批** /
// 负数与超基座 fail-fast——此前 <=0 一律钳回基座，0 的「禁用代批」被吞。
func TestLoadOverridesZeroDisables(t *testing.T) {
	write := func(t *testing.T, content string) fs.FS {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "p.yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return os.DirFS(dir)
	}
	// 0 = 禁用：auto 判定面不命中
	p, err := LoadOverrides(write(t, "operation_policy:\n  auto_max_risk: 0\n"), "p.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p.AutoMaxRisk != 0 {
		t.Fatalf("0 应保留为禁用语义: %d", p.AutoMaxRisk)
	}
	if p.FullMaxRisk != 3 {
		t.Fatalf("未写键应继承基座: %d", p.FullMaxRisk)
	}
	// 正数调低仍有效
	p, err = LoadOverrides(write(t, "operation_policy:\n  auto_max_risk: 1\n"), "p.yaml")
	if err != nil || p.AutoMaxRisk != 1 {
		t.Fatalf("正数调低应生效: %v %d", err, p.AutoMaxRisk)
	}
	// 负数 fail-fast
	if _, err := LoadOverrides(write(t, "operation_policy:\n  auto_max_risk: -1\n"), "p.yaml"); err == nil {
		t.Fatal("负数应 fail-fast")
	}
	// 超基座 fail-fast（扩权面）
	if _, err := LoadOverrides(write(t, "operation_policy:\n  full_max_risk: 4\n"), "p.yaml"); err == nil {
		t.Fatal("超基座应 fail-fast")
	}
}
