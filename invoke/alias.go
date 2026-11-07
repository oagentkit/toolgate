// 本文件是已迁移包的别名转发表，手工维护（参照 agentkit 过渡门面的形态）。
//
// 上游 guardx/invoke 新增导出符号时必须在此补上对应别名，否则按旧路径 import
// 的下游会编译失败；上游删除导出符号时本壳随之下个 minor 一并移除（见 doc.go
// 的搬迁公告）。语义变更不在此处做任何改写（含 guardx#73 的 Record.Allowed）。
package invoke

import upstream "github.com/oagentkit/guardx/invoke"

type (
	Call        = upstream.Call
	Chain       = upstream.Chain
	ChainOption = upstream.ChainOption
	DenyError   = upstream.DenyError
	Handler     = upstream.Handler
	Outcome     = upstream.Outcome
	Record      = upstream.Record
	Result      = upstream.Result
	Stage       = upstream.Stage
	StageFunc   = upstream.StageFunc
	StageResult = upstream.StageResult
	Verdict     = upstream.Verdict
)

var (
	ErrEmptyName   = upstream.ErrEmptyName
	ErrNoHandler   = upstream.ErrNoHandler
	ErrPanic       = upstream.ErrPanic
	ErrStageFailed = upstream.ErrStageFailed
	EvaluateStages = upstream.EvaluateStages
	NewChain       = upstream.NewChain
	Retryable      = upstream.Retryable
	WithRecorder   = upstream.WithRecorder
)
