package policy

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/oagentkit/guardx/toolfail"
	"github.com/oagentkit/mcpkit/mcp"
)

// TestFailureClassMapping 映射表的逐条断言。
//
// 这张表是「协议层事实 → 处置层语义」的唯一转换点，改它必须改这里。
func TestFailureClassMapping(t *testing.T) {
	cases := []struct {
		kind mcp.ErrorKind
		want toolfail.Class
	}{
		{mcp.KindUnreachable, toolfail.ClassTransient},
		{mcp.KindProtocol, toolfail.ClassPermanent},
		{mcp.KindToolMissing, toolfail.ClassNotFound},
		{mcp.KindInvalidArgs, toolfail.ClassInvalidArgument},
		{mcp.KindUnknown, toolfail.ClassUnknown},
	}
	for _, c := range cases {
		if got := FailureClass(c.kind); got != c.want {
			t.Errorf("FailureClass(%v)=%q，期望 %q", c.kind, got, c.want)
		}
	}
}

// TestFailureClassCoversEveryErrorKind 枚举 ErrorKind 的全部取值，逐一断言有显式映射。
//
// 为什么用探针而不是手写清单：手写清单与枚举各自会漂，探针不会——枚举里多一个
// 类别而映射表没跟上，这里当场红。
//
// 探针的边界：ErrorKind 用 iota 连续赋值，故新增类别会落在 0..63 区间内；其
// String() 不会是 "unknown"（"unknown" 只属于 KindUnknown 与越界值），故能被发现。
// 若将来改成非连续赋值，须同步扩大本区间并更新本注释。
func TestFailureClassCoversEveryErrorKind(t *testing.T) {
	for i := 0; i < 64; i++ {
		kind := mcp.ErrorKind(i)
		if kind.String() == "unknown" {
			continue // KindUnknown 与越界值都归到 "unknown"
		}
		if _, ok := failureClassTable[kind]; !ok {
			t.Errorf("ErrorKind(%d)=%q 没有映射：新增协议层分类必须同时补映射表与用例", i, kind.String())
		}
	}
}

// TestFailureClassUnmappedIsNotRetryable 未登记的分类按不可重试处理。
//
// 不可分类 = 不可重试：误判可重试是把一次失败放大成 N 次，还可能重复施加副作用。
func TestFailureClassUnmappedIsNotRetryable(t *testing.T) {
	got := FailureClass(mcp.ErrorKind(99))
	if got != toolfail.ClassUnknown {
		t.Fatalf("越界分类映射成 %q，期望 %q", got, toolfail.ClassUnknown)
	}
	if toolfail.PolicyOf(got).Retryable {
		t.Fatal("未登记的分类必须不可重试")
	}
}

// TestFailureClassTakesConservativeSide 两层结论不一致时取不可重试的一侧。
//
// 协议层说 KindProtocol 可重试，处置层映射成永久失败。两层问的不是同一个问题，
// 但映射只能给一个答案——取保守的一侧。这条把取舍钉住：若上游哪天改了
// ErrorKind.Retryable 的口径，本测试会先红，逼着重新论证而不是悄悄漂移。
func TestFailureClassTakesConservativeSide(t *testing.T) {
	if !mcp.KindProtocol.Retryable() {
		t.Fatal("前提变了：协议层不再认为 protocol 可重试，取舍依据需重新论证")
	}
	if toolfail.PolicyOf(FailureClass(mcp.KindProtocol)).Retryable {
		t.Fatal("protocol 必须映射成不可重试的类别（保守侧）")
	}
}

// TestFailureClassUnreachableIsRetryableButNotResubmittable 连不上：可重试、但不可盲目重发。
//
// 这是超时类失败最要紧的一条：可重试却可能已经生效，原样重发即重复施加副作用。
func TestFailureClassUnreachableIsRetryableButNotResubmittable(t *testing.T) {
	p := toolfail.PolicyOf(FailureClass(mcp.KindUnreachable))
	if !p.Retryable {
		t.Fatal("连不上必须可重试")
	}
	if !p.Backoff {
		t.Fatal("连不上必须建议退避后重试")
	}
	if p.SafeToResubmit {
		t.Fatal("连不上不得判为可原样重发：超时的定义就是不知道对方执行到哪")
	}
}

// Disposition 的用例：调用点只该调这一个函数。

func TestDispositionPrefersExplicitClassification(t *testing.T) {
	// toolfail 哨兵 = 宿主显式表态，应压过协议层事实。
	if got := Disposition(toolfail.ErrRateLimited); got != toolfail.ClassRateLimited {
		t.Errorf("Disposition(toolfail 哨兵)=%q，期望 %q", got, toolfail.ClassRateLimited)
	}
	if got := Disposition(toolfail.ErrNotFound); got != toolfail.ClassNotFound {
		t.Errorf("Disposition(toolfail 哨兵)=%q，期望 %q", got, toolfail.ClassNotFound)
	}
}

func TestDispositionFallsBackToProtocolFact(t *testing.T) {
	// 连接被拒：协议层判 KindUnreachable → ClassTransient。
	err := fmt.Errorf("dial server: %w", syscall.ECONNREFUSED)
	if got := Disposition(err); got != toolfail.ClassTransient {
		t.Errorf("Disposition(连接被拒)=%q，期望 %q", got, toolfail.ClassTransient)
	}
}

func TestDispositionSelfReported(t *testing.T) {
	if got := Disposition(selfReported{retryable: true}); got != toolfail.ClassSelfReported {
		t.Errorf("Disposition(自报可重试)=%q，期望 %q", got, toolfail.ClassSelfReported)
	}
	// 自报不可重试不得回落到协议层映射（那会把宿主已否决的失败翻成可重试）。
	if got := Disposition(selfReported{retryable: false}); got != toolfail.ClassUnknown {
		t.Errorf("Disposition(自报不可重试)=%q，期望 %q", got, toolfail.ClassUnknown)
	}
}

func TestDispositionNilError(t *testing.T) {
	if got := Disposition(nil); got != toolfail.ClassUnknown {
		t.Errorf("Disposition(nil)=%q，期望 %q", got, toolfail.ClassUnknown)
	}
}

// selfReported 宿主自报可重试性的错误（toolfail.RetryableReporter 的最小实现）。
type selfReported struct {
	retryable bool
}

func (e selfReported) Error() string { return "self reported failure" }

func (e selfReported) Retryable() bool { return e.retryable }
