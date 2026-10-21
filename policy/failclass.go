package policy

// 本文件是**协议层失败分类 → 处置层类别**的机械映射。
//
// # 两套分类各答什么
//
//	|                        | `mcpkit/mcp.ErrorKind` | `guardx/toolfail.Class` |
//	|------------------------|------------------------|-------------------------|
//	| 判什么                 | 协议层**事实**          | 处置层**语义**           |
//	| 取值                   | unreachable / protocol / tool_missing / invalid_args / unknown | transient / permanent / authentication / not_found / permission_denied / invalid_argument / rate_limited / self_reported / unknown |
//	| 回答                   | 「这次调用在传输层出了什么事」 | 「该不该重试、要不要人工、能不能原样重发」 |
//
// 二者不是重复实现，而是**同一件事的两个问题**。本文件给出的是两者之间那一步
// **机械映射**：它必须只有一份，散在各调用点手写必然漂移——同一类失败在 A 处
// 判成「可重试」、在 B 处判成「永久拒绝」，重试策略随之瞎掉。
//
// # 为什么映射表落在本包（而不是 mcpkit 或 guardx）
//
// - **不能落 guardx**：tier0 不能反向依赖 tier2 的 mcpkit（分层是硬规则）。
// - **不落 mcpkit**：那是协议层，协议层只回答「出了什么事」，不该替宿主决定
//   「怎么办」；处置口径归 `toolfail` 的策略表单源持有。
// - 本仓是矩阵里**唯一同时依赖 mcpkit 与 guardx 的仓**，两边的原语在此会合。
//   本包也是本仓唯一同时 import 这两侧的包——映射表与它所映射的两组常量同处一室，
//   改一侧就要看另一侧。
//
// # 映射的取舍：两层结论不一致时取不可重试的一侧
//
// `mcp.ErrorKind.Retryable()` 对 KindProtocol 返回 **true**（server 短暂异常时重试可能
// 恢复），而映射后是 `toolfail.ClassPermanent`（不可重试）。两者问的不是同一个问题
// （协议层问「重试有没有可能恢复」，处置层问「原样重试有没有意义」），但映射只能给
// 一个答案，故取**保守的一侧**：误判可重试是把一次失败放大成 N 次，还可能重复施加
// 副作用；误判不可重试的代价只是少重试一次。
//
// # 已知的表达力缺口
//
// 协议层的分类**表达不出**处置层的三类：`rate_limited`（可重试且要退避）、
// `authentication` 与 `permission_denied`（需人工介入）。它们在协议层都表现为
// 「服务端拒绝」，只能落到 protocol 或 unknown，映射后即按不可重试处理。
// 宿主若知道得更细（例如服务端回了 429），应当**显式归类**（见 Disposition），
// 而不是指望这张表替它猜。

import (
	"errors"

	"github.com/oagentkit/guardx/toolfail"
	"github.com/oagentkit/mcpkit/mcp"
)

// failureClassTable 协议层分类 → 处置层类别（单源）。
//
// 改这里必然改到契约测试 TestFailureClassCoversEveryErrorKind。
var failureClassTable = map[mcp.ErrorKind]toolfail.Class{
	// 连不上（网络/进程层）：稍后多半恢复 → 可重试 + 退避。
	// 对上 toolfail 的 ClassTransient：它的 SafeToResubmit 为 false——超时的定义
	// 就是「不知道对方执行到哪」，把它当成安全重发是把一次调用放大成两次。
	mcp.KindUnreachable: toolfail.ClassTransient,

	// 协议错：server 实现不合规或版本不匹配，重试多半无效 → 不可重试。
	mcp.KindProtocol: toolfail.ClassPermanent,

	// 工具不存在：配置/契约问题，重试不创造对象 → 不可重试。
	mcp.KindToolMissing: toolfail.ClassNotFound,

	// 参数错：改调用方才有用，原样重发必然复现 → 不可重试。
	mcp.KindInvalidArgs: toolfail.ClassInvalidArgument,

	// 未归类：不可分类 = 不可重试（与 toolfail 的保守缺省一致）。
	mcp.KindUnknown: toolfail.ClassUnknown,
}

// FailureClass 协议层分类 → 处置层类别。
//
// 这是**唯一**的映射入口：下游不要各写各的映射表。
//
// 未登记的分类（含零值与越界值）一律回落 ClassUnknown——不可分类 = 不可重试，
// 不给「查不到就乐观」留口子。
func FailureClass(kind mcp.ErrorKind) toolfail.Class {
	if c, ok := failureClassTable[kind]; ok {
		return c
	}
	return toolfail.ClassUnknown
}

// Disposition 一次失败的处置类别——调用点该调的那一个。
//
// 判定顺序：
//
//  1. 宿主已用 toolfail 显式归类（*Failure / 哨兵 / 自报可重试）→ 以那份为准：
//     那是产出它的那一层已经做完的判断，比协议层事实更具体。
//  2. 宿主自报「不可重试」→ ClassUnknown：不得回落到协议层映射，否则会把宿主
//     已经否决的失败重新判成可重试。
//  3. 否则按协议层事实机械映射（FailureClass）。
//
// 第 2 步单独存在的原因：toolfail 的自报分支把「自报不可重试」也归成 ClassUnknown，
// 从返回值上与「什么都没判出来」不可分。此处补一次判别，把这两种情形分开。
func Disposition(err error) toolfail.Class {
	if err == nil {
		return toolfail.ClassUnknown
	}
	if c := toolfail.Classify(err); c != toolfail.ClassUnknown {
		return c
	}
	var reporter toolfail.RetryableReporter
	if errors.As(err, &reporter) && !reporter.Retryable() {
		return toolfail.ClassUnknown
	}
	return FailureClass(mcp.ClassifyError(err))
}
