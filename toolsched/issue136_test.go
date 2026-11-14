package toolsched

// toolgate#136 回归钉：「未声明」此前两个方向——具名未声明查兜底表缺省保守
// 不并行、匿名未声明 return true 并行（激进）。匿名 + Hints 全 nil 的调用
//（内部编排直造 Call{ID:"a"}、模型侧未带注解的透传）恰是没走正规注册面的
// 调用，最可能缺注解也最可能破坏性——若实际是破坏性操作即与他人并发。
// 修后匿名未声明与具名同侧保守不并行；「匿名必并行」的宿主场景走显式
// 三态注解（Idempotent/ReadOnly）而不是缺省翻向。标题带 issue 号——回归时按号检索。

import "testing"

// TestAnonymousUndeclaredNotParallel_Issue136 攻击样本前/后对照：修前
// Call{ID:"a"}（匿名 + Hints 零值）CanRunParallel=true 与同批任务并发；
// 修后 false（独占组）。
func TestAnonymousUndeclaredNotParallel_Issue136(t *testing.T) {
	s := &Scheduler{}
	if s.canRunInParallel(Call{ID: "a"}) {
		t.Fatal("匿名未声明判并行——破坏性匿名调用可与他人并发（toolgate#136）")
	}
	sched, err := s.Schedule([]Call{
		{ID: "a"},
		{ID: "r", Hints: Hints{ReadOnly: b(true)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sched.Items[0].CanRunParallel {
		t.Fatal("Schedule 产物面匿名未声明仍并行（toolgate#136）")
	}
	// 分组面：匿名未声明独占单例组，不再与只读任务同组
	for _, g := range sched.Groups {
		if len(g) > 1 && containsID136(g, "a") {
			t.Fatalf("匿名未声明被并入并行组: %v", g)
		}
	}
}

func containsID136(g []string, id string) bool {
	for _, x := range g {
		if x == id {
			return true
		}
	}
	return false
}

// TestAnonymousDeclaredStillHonors_Issue136 对照：匿名但带显式注解的调用照常
// 按注解裁决——保守缺省不吃掉显式声明（宿主「匿名必并行」场景的正规出口）。
func TestAnonymousDeclaredStillHonors_Issue136(t *testing.T) {
	s := &Scheduler{}
	if !s.canRunInParallel(Call{ID: "a", Hints: Hints{Idempotent: b(true)}}) {
		t.Fatal("匿名 + 显式 idempotent 应并行（显式声明优先于缺省，toolgate#136）")
	}
	if !s.canRunInParallel(Call{ID: "a", Hints: Hints{ReadOnly: b(true)}}) {
		t.Fatal("匿名 + 显式 readOnly 应并行")
	}
	if s.canRunInParallel(Call{ID: "a", Hints: Hints{Destructive: b(true)}}) {
		t.Fatal("匿名 + destructive 一票否决不应并行")
	}
}
