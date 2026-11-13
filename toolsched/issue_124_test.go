package toolsched

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// waitDone 带超时地等 Execute 返回（编排失准时防挂死到 go test 全局超时）。
func waitDone(t *testing.T, done <-chan []Result) []Result {
	t.Helper()
	select {
	case res := <-done:
		return res
	case <-time.After(5 * time.Second):
		t.Fatal("Execute 未在时限内返回（调度编排失准）")
		return nil
	}
}

func findResult(t *testing.T, res []Result, id string) Result {
	t.Helper()
	for _, r := range res {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("结果缺任务 %s: %+v", id, res)
	return Result{}
}

// TestExecuteCtxCancelWhileWaitingOnGateSkips（toolgate#124）：skip 判定与执行
// 之间隔着 gate 这道**不可取消的 sync 锁**——#121 只修了 sem 路径，gate 等待期
// 取消此前照跑（与「等待期取消 = Skipped」口径分裂）。
// 编排：B 先持读锁（a 等 bStarted 才完成，消除「写锁先到」竞态）→ A 完成放行
// C（独占），C 抢写锁时 B 仍持读锁 → C 卡在 gate 等待且已过首道 skip 判定；
// 此刻 cancel → 放 B → C 拿到锁后复检 ctx → Skipped 而非执行。
func TestExecuteCtxCancelWhileWaitingOnGateSkips(t *testing.T) {
	sched := &Scheduler{MaxConcurrency: 4}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bStarted := make(chan struct{})
	aDone := make(chan struct{})
	releaseB := make(chan struct{})
	var executedC atomic.Bool

	// B 先跑起来（Run 开始即已持读锁——RLock 在 runGuarded 之前取得），
	// A 等它就绪才完成，保证 C 放行时写锁必然被读锁挡住。
	a := Task{Call: Call{ID: "A", Name: "a", Hints: Hints{Idempotent: b(true)}},
		Run: func(context.Context) (string, error) {
			<-bStarted
			close(aDone)
			return "a", nil
		}}
	taskB := Task{Call: Call{ID: "B", Name: "b", Hints: Hints{Idempotent: b(true)}},
		Run: func(context.Context) (string, error) {
			close(bStarted)
			<-releaseB
			return "b", nil
		}}
	// C 独占（Idempotent 显式 false → 非并行）且依赖 A：A 完成即就绪并被放行，
	// 抢 gate 写锁时 B 仍持读锁 → 卡在 gate 等待窗口。
	c := Task{
		Call: Call{ID: "C", Name: "c", Hints: Hints{Idempotent: b(false)}, DependsOn: []string{"A"}},
		Run:  func(context.Context) (string, error) { executedC.Store(true); return "c", nil },
	}

	done := make(chan []Result, 1)
	go func() { done <- sched.Execute(ctx, []Task{a, taskB, c}) }()

	<-aDone // A 完成 → C 就绪放行，进入 gate 等待（B 持读锁）
	time.Sleep(200 * time.Millisecond)
	if executedC.Load() {
		t.Fatal("B 持读锁时 C 不应已执行")
	}
	cancel()
	close(releaseB)

	res := waitDone(t, done)
	cRes := findResult(t, res, "C")
	if executedC.Load() {
		t.Fatalf("gate 等待期取消后任务不得照跑: %+v", cRes)
	}
	if !cRes.Skipped || cRes.Err != nil || cRes.Output != "" {
		t.Fatalf("gate 等待期取消且未执行的任务应记 Skipped 而非 Err: %+v", cRes)
	}
	// 已执行任务照常落账（A 正常、B 被放行后正常返回）。
	if aRes := findResult(t, res, "A"); aRes.Err != nil || aRes.Output != "a" {
		t.Fatalf("A 结果异常: %+v", aRes)
	}
	if bRes := findResult(t, res, "B"); bRes.Err != nil || bRes.Output != "b" {
		t.Fatalf("B 结果异常: %+v", bRes)
	}
}

// TestExecuteStopAfterWhileWaitingOnGateSkips（toolgate#124，StopAfter 面）：
// 同一道 gate 复检也要接住「等待期停轮落地」——独占任务卡在 gate 期间
// StopAfter 任务成功置 stopped，拿到写锁后必须记 Skipped 而非照跑，否则
// StopAfter 的截断语义被 gate 等待窗口击穿。
//
// 关键约束：StopAfter 任务本身也是独占（canRunInParallel 对 StopAfter 恒
// false），要让它在「读者持锁窗口」内先于 C 拿到写锁，必须**早于 C 入队**：
// S 与 C 同挂 A 的后继（dependents 按任务序先 S 后 C，launchMore 每次只放行
// 一个独占任务）→ A 完成先放 S（入队写锁），B1 完成再放 C（排在 S 之后）。
// 编排：B1/B2 先持读锁 → A 完成 → S 入队等锁 → 放 B1 → C 入队（仍在 S 后）
// → 放 B2 → S 拿锁跑完置 stopped → 放行 C → C 拿锁复检 stopped → Skipped。
func TestExecuteStopAfterWhileWaitingOnGateSkips(t *testing.T) {
	sched := &Scheduler{MaxConcurrency: 4}
	ctx := context.Background()

	b1Started := make(chan struct{})
	b2Started := make(chan struct{})
	aDone := make(chan struct{})
	releaseB1 := make(chan struct{})
	releaseB2 := make(chan struct{})
	sStarted := make(chan struct{})
	releaseS := make(chan struct{})
	var executedC atomic.Bool

	// A 等两个读者都持上读锁才完成（否则独占任务可能抢在读者前拿到写锁）。
	a := Task{Call: Call{ID: "A", Name: "a", Hints: Hints{Idempotent: b(true)}},
		Run: func(context.Context) (string, error) {
			<-b1Started
			<-b2Started
			close(aDone)
			return "a", nil
		}}
	b1 := Task{Call: Call{ID: "B1", Name: "b1", Hints: Hints{Idempotent: b(true)}},
		Run: func(context.Context) (string, error) {
			close(b1Started)
			<-releaseB1
			return "b1", nil
		}}
	b2 := Task{Call: Call{ID: "B2", Name: "b2", Hints: Hints{Idempotent: b(true)}},
		Run: func(context.Context) (string, error) {
			close(b2Started)
			<-releaseB2
			return "b2", nil
		}}
	// S 与 C 同挂 A 后继：dependents 按任务序 = [S, C]，A 完成的 launchMore
	// 只放行先找到的独占任务 S（入队写锁），C 等下一次放行（B1 完成）。
	stopAfter := Task{
		Call: Call{ID: "S", Name: "s", Hints: Hints{Idempotent: b(true)},
			StopAfter: true, DependsOn: []string{"A"}},
		Run: func(context.Context) (string, error) {
			close(sStarted)
			<-releaseS
			return "s", nil
		},
	}
	c := Task{
		Call: Call{ID: "C", Name: "c", Hints: Hints{Idempotent: b(false)}, DependsOn: []string{"A"}},
		Run:  func(context.Context) (string, error) { executedC.Store(true); return "c", nil },
	}

	done := make(chan []Result, 1)
	go func() { done <- sched.Execute(ctx, []Task{a, b1, b2, stopAfter, c}) }()

	<-aDone // A 完成 → S 放行、入队写锁（B1/B2 持读锁，S 起跑不了）
	time.Sleep(200 * time.Millisecond)
	close(releaseB1) // B1 完成 → C 放行、入队写锁（排在 S 之后）
	time.Sleep(200 * time.Millisecond)
	if executedC.Load() {
		t.Fatal("读者持锁且 S 未跑完时 C 不应已执行")
	}
	close(releaseB2) // 读锁全放 → S 先拿写锁起跑
	<-sStarted
	close(releaseS) // S 成功 → stopped=true → 放写锁 → C 拿锁复检
	// C 的复检发生在 S 放锁之后：stopped 已置 → Skipped（不执行）。

	res := waitDone(t, done)
	cRes := findResult(t, res, "C")
	if executedC.Load() {
		t.Fatalf("停轮落地后 gate 等待期的任务不得照跑: %+v", cRes)
	}
	if !cRes.Skipped || cRes.Err != nil || cRes.Output != "" {
		t.Fatalf("停轮落地且未执行的任务应记 Skipped 而非 Err: %+v", cRes)
	}
	if sRes := findResult(t, res, "S"); sRes.Err != nil || sRes.Output != "s" {
		t.Fatalf("S（StopAfter）结果异常: %+v", sRes)
	}
}
