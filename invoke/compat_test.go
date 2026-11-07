// 编译兼容测试（oagentkit/agent#68）：按旧 import 路径消费转发壳的代表性符号，
// 证明 v0.12.6 起断裂的 github.com/oagentkit/toolgate/invoke 恢复可用，
// 且与 guardx/invoke 是同一实体——壳只转发，不改语义。
package invoke_test

import (
	"context"
	"errors"
	"testing"

	upstream "github.com/oagentkit/guardx/invoke"

	"github.com/oagentkit/toolgate/invoke" // Deprecated: 已迁至 github.com/oagentkit/guardx/invoke，下个 minor 删除
)

// 壳的类型是别名而非新类型：与上游同型可直接互相赋值（编译期证明）。
var (
	_ upstream.Record  = invoke.Record{}
	_ upstream.Handler = invoke.Handler(nil)
	_ invoke.Stage     = upstream.StageFunc{}
)

func TestCompatShellForwardsGuardxInvoke(t *testing.T) {
	allow := invoke.StageFunc{
		StageName: "compat-allow",
		Fn: func(_ context.Context, c invoke.Call) (invoke.Verdict, error) {
			return invoke.Verdict{Allow: true, Reason: "测试放行"}, nil
		},
	}

	var records []invoke.Record
	chain, err := invoke.NewChain(
		func(_ context.Context, _ invoke.Call) (invoke.Result, error) {
			return invoke.Result{Text: "ok"}, nil
		},
		[]invoke.Stage{allow},
		invoke.WithRecorder(func(r invoke.Record) { records = append(records, r) }),
	)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}

	if _, err := chain.Do(context.Background(), invoke.Call{Name: "echo", Caller: "compat-test"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("留痕应恰好一条，得到 %d 条", len(records))
	}
	if !records[0].Allowed || records[0].Err != nil || !records[0].Acted() {
		t.Fatalf("放行留痕异常: %+v", records[0])
	}

	// 哨兵错误与上游是同一实体（var 别名），errors.Is 跨壳/上游判定不漂移。
	if !errors.Is(invoke.ErrNoHandler, upstream.ErrNoHandler) {
		t.Fatal("ErrNoHandler 哨兵与上游不同实体")
	}
	if _, err := invoke.NewChain(nil, nil); !errors.Is(err, invoke.ErrNoHandler) {
		t.Fatalf("nil handler 应返回 ErrNoHandler，得到 %v", err)
	}
}

func TestCompatShellKeepsGuardxSemantics(t *testing.T) {
	deny := invoke.StageFunc{
		StageName: "compat-deny",
		Fn: func(_ context.Context, _ invoke.Call) (invoke.Verdict, error) {
			return invoke.Verdict{Reason: "测试拒绝", Retryable: true}, nil
		},
	}

	err := invoke.EvaluateStages(context.Background(), []invoke.Stage{deny}, invoke.Call{Name: "rm"})
	var de *invoke.DenyError
	if !errors.As(err, &de) {
		t.Fatalf("拒绝应归口为 *DenyError，得到 %T", err)
	}
	if !invoke.Retryable(err) {
		t.Fatal("声明可重试的拒绝应被 Retryable 认出")
	}
	if de.Stage != "compat-deny" || de.Tool != "rm" {
		t.Fatalf("DenyError 携带环节与工具名异常: %+v", de)
	}
}
