package invoke

import (
	"context"
	"encoding/json"
	"time"

	"github.com/oekit/pkg/errors"
)

// Call 一次工具调用的统一契约。
//
// 刻意**不**携带任何框架类型（如 policy.Op）：本包与 policy/toolegate 无耦合，
// 审计语义由各 Stage 自己从 Name/Server/Args 读，跨包的判定口径不互相污染。
type Call struct {
	Name   string // 工具名，必填；空名按拒绝处理
	Server string // 来源标识（如 MCP server 名），单工具场景可空
	Caller string // 调用者标识（agent 名 / 环节名），供审计定位
	Args   string // 参数原文（通常是 JSON 文本），按次静态可判
}

// Result 统一出参。Text 与 Structured 至多其一有值，出参类型由 Handler 自决。
type Result struct {
	Text       string
	Structured json.RawMessage
}

// Handler 真正的执行体——链的终点，也是链唯一持有的执行入口。
//
// 契约：Handler 不得把 ctx 之外的可变状态当成请求无关的；跨调用共享状态由调用方
// 自己负责同步。本包不替 Handler 加锁（那是执行体的事，不是链的事）。
type Handler func(ctx context.Context, c Call) (Result, error)

// Verdict 一环的判定结论。
type Verdict struct {
	Allow  bool   // 放行进下一环 / 拒绝收口
	Reason string // 结论依据，供审计留痕；拒绝时必填
	// Retryable 该拒绝是否值得重试。默认 false —— 认不出的否定一律不可重试：
	// 不存在的东西不会因为重试而出现，退避只会把一次策略否定变成无穷重试。
	Retryable bool
}

// Stage 链上的一环。
//
// 返回约定（两条都重要）：
//   - (Verdict, nil)：按 Verdict 判定；
//   - (Verdict, error)：**按拒绝处理**，链立刻收口。原因为空时用错误本身兜底。
//
// error 不被当作「这环管不着，跳过」——那会让一次故障的门禁静默变成不设防。
type Stage interface {
	Name() string
	Check(ctx context.Context, c Call) (Verdict, error)
}

// StageFunc 把普通函数适配成 Stage，便于宿主快速拼一条链而不必为每环声明类型。
type StageFunc struct {
	// StageName 该环的名字，进审计留痕；空串按 "unnamed" 记录（不留空是因为
	// 「哪一环拒的」正是审计要回答的问题，空名等于这个问题没有答案）。
	StageName string
	Fn        func(ctx context.Context, c Call) (Verdict, error)
}

// Name 实现 Stage。
func (s StageFunc) Name() string {
	if s.StageName == "" {
		return "unnamed"
	}
	return s.StageName
}

// Check 实现 Stage；Fn 为 nil 视为拒绝（配漏一环应当拒绝，不该放行）。
func (s StageFunc) Check(ctx context.Context, c Call) (Verdict, error) {
	if s.Fn == nil {
		return Verdict{}, errors.New("invoke: 门禁环未装配判定函数")
	}
	return s.Fn(ctx, c)
}

// 哨兵错误。用 oekit/pkg/errors（与全家族一致），调用方用 errors.Is 判定。
var (
	// ErrNoHandler 构造链时未给执行体。
	ErrNoHandler = errors.New("invoke: 链必须持有执行体")
	// ErrEmptyName 空工具名。空名无法定位、无法审计，按拒绝而非放行。
	ErrEmptyName = errors.New("invoke: 工具名为空")
	// ErrStageFailed 门禁环自身报错，按拒绝收口。
	ErrStageFailed = errors.New("invoke: 门禁环判定失败")
	// ErrPanic 门禁环或执行体 panic，已被 recover 并转成错误。
	ErrPanic = errors.New("invoke: 调用过程中 panic")
)

// DenyError 一次被门禁拒绝的调用。带是哪一环拒的、为什么、值不值得重试，
// 让调用方不必靠解析错误字符串做重试决策。
type DenyError struct {
	Stage     string // 拒绝的环节名，"precondition" 表示链前校验拒的
	Tool      string
	Reason    string
	Retryable bool
	// Cause 阶段报错/panic 的原始错误。为 nil 表示这是一次正常判定得出的拒绝，
	// 而不是「这一环自己出问题了」——两者对调用方的处置完全不同。
	Cause error
}

// Error 实现 error。
func (e *DenyError) Error() string {
	return "invoke: 拒绝调用 " + e.Tool + "（" + e.Stage + "：" + e.Reason + "）"
}

// Unwrap 让 errors.Is 同时能命中「归口错误」与「原始错误」。
//
// 归口错误（ErrStageFailed）与原始错误都要能命中：调用方一要分清
// 「策略拒绝」与「门禁自己坏了」（处置不同），二要能顺着链路挖到根因。
func (e *DenyError) Unwrap() []error {
	if e.Stage == stageInternal {
		if e.Cause != nil {
			return []error{ErrStageFailed, e.Cause}
		}
		return []error{ErrStageFailed}
	}
	if e.Cause != nil {
		return []error{e.Cause}
	}
	return nil
}

const (
	// stagePrecondition 链前校验（空名）所属的虚拟环节名。
	stagePrecondition = "precondition"
	// stageInternal 阶段自身报错 / panic 的归口环节名。
	stageInternal = "internal"
)

// Retryable 判断一个错误是否值得重试。
//
// 认不出的错误一律 false（fail-closed）：门禁两类结局的差别只在「是否可恢复」上，
// 猜错的代价不是少重试一次，而是不停重复一个永远不会成功的调用。
func Retryable(err error) bool {
	var de *DenyError
	if errors.As(err, &de) {
		return de.Retryable
	}
	return false
}

// StageResult 单环的判定留痕。
type StageResult struct {
	Stage    string
	Allow    bool
	Reason   string
	Duration time.Duration
}

// Record 一次调用的完整留痕。
//
// 不变式：**任何结局都恰好一条**——放行、拒绝、阶段报错、执行体报错、panic。
// 「有一条」和「只有一条」同样重要：多一条会让审计把同一次调用算成两次，
// 少一条则等于这次调用没发生过。
type Record struct {
	Call     Call
	Stages   []StageResult
	Allowed  bool          // 是否真的到达了执行体
	Err      error         // 收口原因，nil 即放行成功
	Duration time.Duration // 整链耗时
	OutBytes int           // 出参字节数（Text + Structured），留痕用
}

// ChainOption 链构造选项。
type ChainOption func(*Chain)

// WithRecorder 挂统一调用留痕。f 为 nil 时不挂（不留痕是显式选择，不是默认）。
func WithRecorder(f func(Record)) ChainOption {
	return func(c *Chain) { c.onRecord = f }
}

// Chain 不可绕过的调用链。
//
// 构造后不可变：stages 只读、handler 私有、没有取回裸 handler 的方法。
// 因此并发下多调用共享一条链是安全的，不需要额外同步。
type Chain struct {
	handler  Handler
	stages   []Stage
	onRecord func(Record)
}

// NewChain 构造调用链。handler 按值收进私有字段，此后不再对外暴露——
// 这是「不可绕过」的全部机制：绕过代码写不出来，而不是靠文档说不要这么写。
//
// handler 为 nil 直接构造失败：返回一个「空转的链」比构造失败危险得多，
// 因为它看起来能用。
func NewChain(handler Handler, stages []Stage, opts ...ChainOption) (*Chain, error) {
	if handler == nil {
		return nil, ErrNoHandler
	}
	c := &Chain{
		handler: handler,
		stages:  make([]Stage, len(stages)),
	}
	copy(c.stages, stages)
	for _, o := range opts {
		if o != nil {
			o(c)
		}
	}
	return c, nil
}

// Stages 返回已装配的环节名（只读快照，调试与自检用）。
//
// 刻意**不**返回 Stage 本身或 handler：任何能把执行体交出去的出口，
// 都是给绕过留的门。
func (c *Chain) Stages() []string {
	out := make([]string, len(c.stages))
	for i, s := range c.stages {
		out[i] = s.Name()
	}
	return out
}

// Do 执行一次调用：逐环判定，全部通过才到达执行体。
//
// 无论结局如何都恰好产出一次留痕。返回的 error 在拒绝时是 *DenyError，
// 调用方用 Retryable(err) 决定退避与否，不必解析错误文本。
func (c *Chain) Do(ctx context.Context, call Call) (res Result, err error) {
	start := time.Now()
	rec := Record{Call: call}
	// 用命名返回值捕获收口原因：留痕里少了 Err 的失败记录，读起来和成功记录没区别，
	// 而「为什么停」正是审计要回答的第一个问题。
	defer func() {
		rec.Err = err
		rec.Duration = time.Since(start)
		if c.onRecord != nil {
			c.onRecord(rec)
		}
	}()

	// 链前校验：空名无法定位与审计，门禁不对空名乐观放行。
	if call.Name == "" {
		rec.Stages = append(rec.Stages, StageResult{
			Stage:  stagePrecondition,
			Reason: ErrEmptyName.Error(),
		})
		return Result{}, &DenyError{
			Stage: stagePrecondition, Tool: call.Name, Reason: ErrEmptyName.Error(),
		}
	}

	for _, s := range c.stages {
		st := time.Now()
		v, err := runStage(ctx, s, call)
		elapsed := time.Since(st)
		if err != nil {
			rec.Stages = append(rec.Stages, StageResult{
				Stage: s.Name(), Reason: err.Error(), Duration: elapsed,
			})
			return Result{}, err
		}
		rec.Stages = append(rec.Stages, StageResult{
			Stage: s.Name(), Allow: v.Allow, Reason: v.Reason, Duration: elapsed,
		})
		if !v.Allow {
			return Result{}, &DenyError{
				Stage: s.Name(), Tool: call.Name, Reason: v.Reason, Retryable: v.Retryable,
			}
		}
	}

	res, err = runHandler(ctx, c.handler, call)
	if err != nil {
		return Result{}, err
	}
	rec.Allowed = true
	rec.OutBytes = len(res.Text) + len(res.Structured)
	return res, nil
}

// runStage 跑一环，把 panic 转成错误。
//
// recover 在这里而不是散落各处，是本包的核心价值之一：工具执行一旦被搬出主流程
// （后台服务、worker、子进程），原本包在宿主进程里的 recover 就不在了。
// 链是唯一还在的进程内边界，所以它必须兜住。
func runStage(ctx context.Context, s Stage, call Call) (v Verdict, err error) {
	defer func() {
		if r := recover(); r != nil {
			v = Verdict{}
			err = &DenyError{
				Stage:  stageInternal,
				Tool:   call.Name,
				Reason: "门禁环 panic：" + panicReason(r),
				Cause:  asError(r),
			}
		}
	}()
	v, err = s.Check(ctx, call)
	if err != nil {
		// 阶段报错必须**归口成 DenyError** 再抛出去：裸透传会让调用方拿到一个
		// 无归属的错误，既不知道是哪一环拒的，也不知道该不该重试——两样都要靠猜。
		return Verdict{}, &DenyError{
			Stage:  stageInternal,
			Tool:   call.Name,
			Reason: err.Error(),
			Cause:  err,
		}
	}
	return v, nil
}

// runHandler 跑执行体，把 panic 转成错误（理由同 runStage）。
func runHandler(ctx context.Context, h Handler, call Call) (res Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			res = Result{}
			err = errors.Wrap(ErrPanic, "执行体 panic："+panicReason(r))
		}
	}()
	return h(ctx, call)
}

// stageDuration 占位已移除：分环耗时在 Do 的循环里按真实墙钟测量。

func panicReason(r any) string {
	switch v := r.(type) {
	case error:
		return v.Error()
	case string:
		return v
	default:
		return "未知"
	}
}

// asError 把 panic 值转成 error，供 errors.Is/As 顺着链路挖到根因。
// panic 值未必是 error（可能是裸字符串甚至任意值），不能直接塞进 Cause。
func asError(r any) error {
	if e, ok := r.(error); ok {
		return e
	}
	return errors.New(panicReason(r))
}
