# toolgate CHANGELOG

> 本文件由 `matrix-tools/scripts/gen_changelogs.py` 从 git 历史生成，按 tag 切段、取每段内的 conventional commit 渲染。

## [v0.12.1] — 2026-10-21

- 杂务(deps)：依赖地板追平上游本轮新版本——oagentkit/guardx v0.13.1 oekit/concurrency v0.19.0 oekit/pkg v0.20.0

## 下游要做什么
**`go mod tidy`**。地板抬高对下游是隐式 breaking：不跑就报
`updates to go.mod needed`，且报错不提真正原因（**不是本仓的 API 变了**）。
本仓**自身代码零改动**——只看本仓的 diff 看不出这是一次地板抬高。
## 抬了哪几块、为什么无行为差异
**`github.com/oagentkit/guardx` v0.13.0 → v0.13.1**
上游 diff 只有 `go.mod` / `go.sum` / `CHANGELOG.md`——**没有 Go 源码变更**
（内容是它自己把 `oekit/pkg` 地板抬到 v0.20.0）。本仓用到的包与转发面一字未动。
**`github.com/oekit/concurrency` v0.18.0 → v0.19.0**
上游 diff 只动了 2 个包：
- `retry`：**抖动 panic 修复**——`rand.Int63n` 的契约是 n > 0，而「退避幅度为 0」
  是生产可达路径（cfg.Delay=0 / DelayHint 返回 0 / float64→int64 溢出成负数），
  旧实现在这些路径上直接 panic。改为钳到 [0, MaxInt64]。
- `syncx`：新增全局在飞闸门原语（`Gate`）+ 说明同步。
**`breaker` 与 `async` 两个包一字未动。**
**本仓实际 import 的 `oekit/concurrency` 子包：async **
与上游改动相交的：**（无）**
**`github.com/oekit/pkg` v0.18.0 → v0.20.0**
上游 diff 只动了 5 个包：
- `errors`：**纯增量**——新增 `Errorf`（格式化 + `%w` 包装，nil-safe）。
  `New`/`As`/`Is`/`Unwrap`/`Wrap` 是同型透传，一字未动。
- `httpjson`：**退避溢出修复**——旧写法 `base << (attempt-1)` 在 attempt 较大时
  int64 溢出成负数，负数绕过 `MaxDelay` 封顶，`time.After(负数)` 立刻返回
  ⇒ **该轮退避彻底消失**。改为乘法 + 饱和封顶。正常 attempt 数（1..20）下
  结果**逐字相同**，差别只在溢出的病态路径。
- `urlguard`：**纯增量**——新增 `WithExactHosts`，未设置时行为与既有完全一致。
- `stringx`：**仅注释**（补 `TruncEllipsis` 的 n 语义说明）。
- `pathx`：新增包（`SanitizeRel` 等）。
**本仓实际 import 的 `oekit/pkg` 子包：（本仓未直接 import oekit/pkg 的任何子包）**
与上游改动相交的：**（无）**
## 门禁
```
go build / vet / test -race   ✅
gofmt -l .                    ✅ 空
```

- 杂务(deps)：guardx 依赖地板抬到 v0.13.0

本仓只用 guardx 的既有包（invoke），而它在 v0.11.0 → v0.13.0 之间**未变**
（那几版只加了 sandbox 的 Capable 与 wire）——故这是纯地板追平，无行为差异。
矩阵的依赖地板时效门禁（verify_dep_floors.py）会把「地板落后远端最新」报出来。
门禁自检：go build / vet / test ✅，gofmt ✅，go mod tidy ✅

- 杂务(deps)：oagentkit/guardx 依赖地板抬到 v0.11.0

guardx v0.11.0 新增 sandbox/wire 跨进程调用协议，是**新增能力**而非 API 变更——
本仓无需改代码，只抬地板。
为什么现在补：guardx v0.11.0 是本轮刚发布的，本仓上一个 tag 早于它，
于是这一格欠账一直挂着。依赖地板报告器把「子仓发版让下游落后」显式化，
但报告器恒不失败——真正的硬约束是「打 tag 时刻地板必须是当时的最新」。
本地门禁：go mod tidy 无差异 / go build / go vet 通过。


## [v0.12.0] — 2026-10-19

### 重构：invoke 迁出本仓 —— 框架无关的原语不该被 eino 重仓的地板拖住

BREAKING CHANGE: github.com/oagentkit/toolgate/invoke → github.com/oagentkit/guardx/invoke
invoke 的包文档自证「刻意不携带任何框架类型：本包与 policy/toolegate 无耦合」——
它是框架无关的原语，自身 import 面只有 oekit/pkg/errors，却在带 eino 家族的重仓里
让消费方白吃 34 个第三方模块。
policy 改为依赖 guardx/invoke（tier2 → tier0，方向合法，无环）。
关联 oagentkit/guardx#3

## [v0.11.0] — 2026-10-19

- 杂务(deps)：依赖地板对齐远端最新（mcpkit v0.8.0 / runtime v0.7.0） (13)

matrix-tools 的 verify_dep_floors.py 首次全矩阵扫描查出的地板漂移，本仓是受影响仓之一。
本轮按 tier 拓扑顺序推进：tier0、tier1 先发，本仓再把地板跟到它们的新 tag——
否则就是「发了新的、旧的立刻变落后」，永远收敛不了。
所升版本与原钉版本相比源码零改动，逐版实测确认：
- textx v0.4.0→v0.5.0 / obs v0.5.0→v0.6.0 / guardx v0.4.1→v0.5.0 /
  runtime v0.5.0→v0.6.0 / mcpkit v0.6.0→v0.7.0  全树逐字节相同（diff -rq 退出码 0）
- oekit/pkg v0.14.0→v0.15.1  仅新增 urlguard 包（纯标准库，零新增三方依赖）
- oekit/concurrency v0.17.0→v0.17.1  仅三个文档文件
- oekit/logx v0.7.0→v0.7.1  仅 CI 配置与文档
无 API 变更、无兼容风险。
本地门禁：go mod tidy -diff 无输出 / go build / go vet / gofmt / go test -race 全通过。
关联 oagentkit/matrix-tools#13


## [v0.10.0] — 2026-10-19

- 杂务(deps)：依赖地板对齐远端最新（mcpkit v0.6.0 / runtime v0.6.0 / pkg v0.15.1 / concurrency v0.17.1）

四处漂移与本仓当前钉住的版本相比源码零改动，逐版实测确认：
- mcpkit v0.5.1 → v0.6.0       全树逐字节相同（diff -rq 退出码 0）
- runtime v0.5.0 → v0.6.0      全树逐字节相同（退出码 0）
- oekit/pkg v0.14.0 → v0.15.1  仅新增 urlguard 包（纯标准库，pkg 的 go.mod
                              连 require 块都没有，零新增三方依赖）
- oekit/concurrency v0.17.0 → v0.17.1 仅三个文档文件，无 .go 改动
下游 agent-harness 不必再靠手写 require 顶 mcpkit 地板。
本地门禁：go mod tidy -diff 无输出 / go build / go vet / go test -race 全通过。


## [v0.9.0] — 2026-10-19

- 杂务(deps)：oekit 依赖升级（mcpkit v0.5.0→v0.5.1、concurrency v0.16.0→v0.17.0）

本 PR 含两项改动。
## 1. oekit 依赖地板
姊妹矩阵 oekit 的 `concurrency` 出了 v0.17.0，依赖地板一并对齐。
升级前已核对 `oekit/concurrency` v0.16.0...v0.17.0 的全部 2 个提交：
- `feat(ratelimit): KeyedLimiter.Create 护栏内原子登记 (Fixes #33)` — `ratelimit/keyed.go` +28 行
- `docs: 矩阵总览表 DESC 口径同步` — 仅 AGENTS.md / README.md / CHANGELOG.md
无 breaking / remove 标记，未触及任何已有导出签名，故为纯补丁式升级。
## 2. 迁出被 force 移动过的 tag
本仓原先依赖的姊妹仓 tag（发布后被强推到了重写后的 main）内容已与
sum.golang.org 记录不一致，于是任何走 `proxy.golang.org` 或 `GOPROXY=direct`
的构建都会 SECURITY ERROR。本地往往不报错——模块缓存预热 + goproxy.cn
还留着移动前的字节，把问题掩盖了。
已为 8 个模块重打不可变的**附注 tag**（内容与被移动 tag 指向同一 commit，
只换版本号，让校验和数据库重新记录一次正确哈希），并把依赖迁过去。
`go.sum` 的 h1 全部取自 sum.golang.org，仅就地替换目标模块的两行
（zip h1 + go.mod h1），其余行不动。


## [v0.8.0] — 2026-10-19

### 新增(invoke)：导出只求值不执行的环入口，让链与程序内调用共用一份语义

下游 agent-harness/ptc 的执行体必须跑在 goja 绑定闭包里（写回 res.Calls /
res.Bytes 预算记账、追加 CallLog、触发 vm.Interrupt），没法交给 Chain.Do，
于是自己排了一遍环。那份副本已经与 Chain.Do 漂移，且下游修不了：
  - panic / 环报错归口用 st.Name() 而非包内未导出常量 stageInternal，且
    Cause 留空 → errors.Is(err, ErrStageFailed) 判否。于是经程序内
    host.<tool>() 触发的守卫 panic，与「守卫正常拒绝」在调用方眼里
    不可区分，而 DenyError.Cause 的文档定义的正是相反的方向：
    调用方会去修策略，真正该修的是守卫代码。
  - 没有空名前置校验（当前不可达，但与上游无任何一致性机制）。
stageInternal 未导出，下游既不能引用也不能可靠复制（硬编码字符串等于与
实现耦合），这处分歧只能在上游解决。故导出：
    func EvaluateStages(ctx context.Context, stages []Stage, call Call) error
只求值、不执行。Chain.Do 的环循环改调它，空名前置校验一并收进同一处，
于是 panic 归口只存在于本包一处。它不持有执行体（返回类型只有 error），
不构成绕过链的出口——绕过需要 Handler，而 Handler 只有 NewChain 内部拿得到。
本提交另有两处**既有行为变更**，都是把静默失效改成能被判定：
  1. 链里有 nil 环节时，Do 在 runStage 的 recover 之外又调了一次 s.Name()，
     panic 逃出去直接杀掉编排进程——违反本包 doc「panic 不杀进程」的承诺
     （实测复现：SIGSEGV 逃到 testing.tRunner）。取名改走 stageName，
     nil 环按 runStage 既有归口判为 internal 拒绝：fail-closed，不跳过。
  2. 空名拒绝的 DenyError 挂上 Cause: ErrEmptyName，使 errors.Is 能命中。
     哨兵区注释写着「调用方用 errors.Is 判定」，此前这一条命中不了，
     只能靠比对 Reason 文本。Stage 仍是 precondition，ErrStageFailed
     依旧判否（空名不是某一环坏了）。
测试：新增 invoke/evaluate_test.go。环自身出问题的三项归口
（ErrStageFailed / stageInternal / Cause 非 nil）是本文件最要紧的断言；
另有防漂移用例 TestChainDoAndEvaluateStagesAgree——同一条环走链与走求值入口
必须同归口、留痕同条数。五处变异逐个验证过会转红：去掉 panic 的 Cause、
改归口环节名、去掉 nil 兜底、空名不挂 ErrEmptyName、Do 重新自己排环。

### 修复(deps)：mcpkit v0.5.0 的 go.sum 记的是 tag 重写前的哈希

org 那次全量 tag/历史重写把同名 tag 指到了新提交，本仓 go.sum 里
mcpkit v0.5.0 的两条 h1 仍是重写**之前**算出来的：
    记的  23rGvFbb…（zip） / afdfmgAO…（go.mod）
    现网  VHjaIsxb…（zip） / yW8N7N58…（go.mod）
后果是硬失败，不是下载慢一点：
    verifying github.com/oagentkit/mcpkit@v0.5.0: checksum mismatch
    SECURITY ERROR … the bits may have been replaced on the origin server
本仓最后一次 CI 绿在 tag 重写之前，所以这条红是潜伏的，下次触发 CI 就会炸。
修法是只替换这两行，不整体重算 go.sum：全矩阵里被重写的 tag 有十几个版本，
逐行核对后**只有 mcpkit v0.5.0 这一个版本**的条目在本仓是陈旧的，
整体 tidy 会把无关条目一起搅动（且若沿用共享模块缓存，还会把重写前的副本
原样记回去——命令成功、依据是错的）。核对方式是用空 GOMODCACHE 直连 VCS
取现网真实哈希，与 go.sum 逐条比对。
三方模块条目一条未动，sumdb 校验面没被关掉。
验证（空模块缓存 + GOPRIVATE 直连，与 CI 同配置）：
  - go mod tidy -diff 退出码 0 —— 替换这两行就是完整的，不缺别的条目
  - go build ./... 通过、go test ./... -race 四包全绿、golangci-lint 0 issues

## [v0.7.0] — 2026-10-18

### 新增(policy)：审计门接成 invoke.Stage，装饰器之外多一条绕不过去的路

AuditGate 是装饰器：宿主忘了包一层，这次调用就没有审计门——不报错、不告警，事后从
日志里也看不出这次调用没走门。故新增 Stage(g, op, onDecision) 把同一套裁决接到
invoke 链上；两者并存，AuditGate 保留兼容既有调用点，新代码走这里。
Op 是模板：Type/Mode/Risk/Mutating 装配期固化（按调用点静态可判），Tool 每次由
链上的实际调用覆盖——一个 stage 能服务同一调用点下的多个工具。若把装配期的 Tool
原样带下去，审计记的就是上一次装配时的工具名，记错的审计比没有审计更危险。
裁决映射：auto→放行；deny/plan_only/need_human/unknown→拒绝且**一律不可重试**。
需要人工的情形绝不判成可重试：人工审批是带外动作，把它翻译成「多试几次」会让
退避重试一直撞同一堵墙，真正的阻塞点被埋在重试风暴里。default 分支是纵深
防御（当前 Decide 无路径产出未识别裁决），保持 fail-closed。
测试全部走真实内置策略判定（OpToolCall 变异/只读、plan 模式 execute_step、
plan 模式 plan_submit、逐工具例外规则），不构造桩 Arbiter——桩只能证明桩自己
的行为。反向验证：把拒绝分支翻成放行后 5 个用例变红。
逆序依赖检查：本包 import invoke，invoke 不 import policy，无环。

### 新增(invoke)：不可绕过的工具调用链，门禁由构造保证

本仓既有的三包都只能**加**约束，没有一个能**强制**约束：policy.AuditGate 是
装饰器（宿主忘了包一层，这次调用就没有审计门），toolsched 只做调度不参与判定，
toolprior 是数据不是闸。
「宿主忘了包」是个非常现实的失效形态：它不报错、不告警，工具照常执行，事后从
日志里也看不出这次调用没走审计门。OpenCode 2.0 上线一周内爆出的三个安全回归
（配置热重载窗口绕过 provider deny、系统级配置目录被静默丢弃、TUI 层 sandbox
在工具执行搬进后台服务后静默失效）全部属于同一形态：**信任边界移动了，沙箱
没跟着移动**，而且静默。
故新增 invoke 包，把门禁从「装饰」改成「结构」：NewChain 把 Handler 按值收进
私有字段且不提供取回方法，想执行一次调用唯一的办法就是 chain.Do，它必然过
完整条链。绕过代码写不出来——这是类型保证，不是约定。
一并解决的次级问题：审计面不对称。此前经 Code Mode 的调用有完整调用账目而普通
调用没有对等物；能力更强的路径审计更好，会让人在评审时高估整体覆盖。现在任何
结局（放行/拒绝/阶段报错/执行体报错/panic）都恰好产出一条 Record。
fail-closed 覆盖构造期与执行期：nil handler 构造失败、空名按拒绝、Stage 返回
error 按拒绝（不是「跳过该环」——那等于把一次故障静默降级成不设防）、panic 被
recover 转错误（工具执行一旦搬出主流程，宿主进程里的 recover 就够不着了，链是
唯一还在的进程内边界）。
顺带把 oekit/pkg 由间接 v0.11.0 提为直接依赖 v0.14.0（需要 errors.New，v0.12
起才有）。全量 go vet + go test -race 四包通过，既有三包未被升版影响。
反向验证（重新引入绕过 → 测试变红）：
  M1 取消「拒绝即收口」 → 4 个用例失败
  M2 阶段报错裸透传     → TestStageErrorDeniesInsteadOfPassing 失败
  M3 摘掉 runHandler 的 recover → panic 掀翻测试进程（证明该 recover 承重）
另：开发中由测试抓出两处真 bug——阶段报错未归口 DenyError（调用方无法判断该
不该重试）、以及本仓并发用例自身的无锁自增竞态。

### 新增：toolgate 仓建立——工具治理（3 包）

从 agentkit 单仓切出 tier2 集成层模块，module 路径 github.com/oagentkit/toolgate。
收编 3 个包：
- toolprior 工具优先级三层约束（收紧语义，下层已设限制不得放宽）
- toolsched 注解驱动的工具并发调度与去重
- policy     操作审计门：敏感工具调用的策略判定
矩阵内依赖：mcpkit。外部地板 eino + yaml.v3。
门禁：go build / go vet / go test -race 全绿，gofmt 干净。

- 杂务(deps)：依赖地板升到「历史最新 + 0.1.0」

本轮做了版本线重置：清空全部历史 tag，按「历史最新 + 0.1.0」重打新版本
（如 llm v0.6.0 → v0.7.0、toolgate v0.6.1 → v0.7.0、agentkit v1.3.0 → v1.4.0）。
各仓 go.mod 的矩阵内依赖地板同步跟进到新版本号。
**go.sum 的 oagentkit 行已整段删除**，由 CI 的 `go mod tidy` 重算：
新 tag 的内容与任何历史版本都不同，`h1` 必须现算；本地没有 Go 工具链，
且 `/go.mod` 的 dirhash 无法自行复现（zip hash 可复现，gomod hash 不可复现）。
重算依赖 `.github` 的 go-ci 配了 `GOPRIVATE=github.com/oagentkit/*` 直连 VCS：
goproxy.cn 缓存着旧版本，若走 proxy 会拉回**旧代码**并算出旧 hash，
门禁全绿但依赖是错的。
（第二个提交会补上重算好的 go.sum。）
* chore(deps): 依赖地板升到「历史最新 + 0.1.0」
chore(deps): 依赖地板升到「历史最新 + 0.1.0」
本轮做了版本线重置：清空全部历史 tag，按「历史最新 + 0.1.0」重打新版本
（如 llm v0.6.0 → v0.7.0、toolgate v0.6.1 → v0.7.0、agentkit v1.3.0 → v1.4.0）。
各仓 go.mod 的矩阵内依赖地板同步跟进到新版本号。
**go.sum 的 oagentkit 行已整段删除**，由 CI 的 `go mod tidy` 重算：
新 tag 的内容与任何历史版本都不同，`h1` 必须现算；本地没有 Go 工具链，
且 `/go.mod` 的 dirhash 无法自行复现（zip hash 可复现，gomod hash 不可复现）。
重算依赖 `.github` 的 go-ci 配了 `GOPRIVATE=github.com/oagentkit/*` 直连 VCS：
goproxy.cn 缓存着旧版本，若走 proxy 会拉回**旧代码**并算出旧 hash，
门禁全绿但依赖是错的。
（第二个提交会补上重算好的 go.sum。）
* chore(deps): 回填 CI 重算的 go.sum
`h1` 由 CI 里真实的 `go mod tidy` 算出后回填——本地没有 Go 工具链，
且 `/go.mod` 的 dirhash 无法自行复现（zip hash 可复现，gomod hash 不可复现），
所以这是唯一权威来源。
重算依赖 `.github` go-ci 里的 `GOPRIVATE=github.com/oagentkit/*` 直连 VCS：
goproxy.cn 仍缓存着版本线重置前的旧内容，走 proxy 会拉回**旧代码**并算出旧 hash，
门禁全绿但依赖是错的。
验证方式就是本 PR 的 `go mod tidy -diff`：若回填有任何偏差，它会照样报差异。
* chore(deps): 回填 CI 重算的 go.sum
`h1` 由 CI 里真实的 `go mod tidy` 算出后回填——本地没有 Go 工具链，
且 `/go.mod` 的 dirhash 无法自行复现（zip hash 可复现，gomod hash 不可复现），
所以这是唯一权威来源。
重算依赖 `.github` go-ci 里的 `GOPRIVATE=github.com/oagentkit/*` 直连 VCS：
goproxy.cn 仍缓存着版本线重置前的旧内容，走 proxy 会拉回**旧代码**并算出旧 hash，
门禁全绿但依赖是错的。
验证方式就是本 PR 的 `go mod tidy -diff`：若回填有任何偏差，它会照样报差异。
* chore(deps): 回填 CI 重算的 go.sum
`h1` 由 CI 里真实的 `go mod tidy` 算出后回填——本地没有 Go 工具链，
且 `/go.mod` 的 dirhash 无法自行复现（zip hash 可复现，gomod hash 不可复现），
所以这是唯一权威来源。
重算依赖 `.github` go-ci 里的 `GOPRIVATE=github.com/oagentkit/*` 直连 VCS：
goproxy.cn 仍缓存着版本线重置前的旧内容，走 proxy 会拉回**旧代码**并算出旧 hash，
门禁全绿但依赖是错的。
验证方式就是本 PR 的 `go mod tidy -diff`：若回填有任何偏差，它会照样报差异。

- 杂务(deps)：跟进最新 tag

oagentkit/mcpkit v0.3.0→v0.4.0、oagentkit/runtime v0.3.1→v0.4.0
go.sum 取 sumdb 权威 h1，仅就地替换目标 mod 两行。

- 杂务(deps)：oekit 依赖升级

concurrency v0.13.1→v0.16.0
姊妹矩阵 oekit 的依赖地板此前被漏掉（上一轮只扫了 oagentkit 内部依赖），本次一并对齐到最新。
升级前已确认 oekit/concurrency v0.13.1..v0.16.0 的 8 个提交全是 feat/docs/chore，
无 breaking/remove 标记。其中 #28 订正了 `WithCooldown(0)` 的语义——
即 llms.txt 里警告过的那个陷阱，升上去正好拿到修正后的行为。
go.sum 的 h1 取自 sumdb，仅就地替换目标 mod 的两行、其余行不动。

- 杂务(deps)：mcpkit v0.2.1 → v0.3.0（本轮新发 tag 的连锁升级）

mcpkit v0.3.0 已发布（本次依赖地板升级），本仓跟进。
go.sum 的 h1 取自 sumdb，仅就地替换该 mod 的两行。

- 杂务(deps)：跨仓依赖升级

mcpkit v0.1.0 → v0.2.1 / runtime v0.1.0 → v0.3.1
全 org 跨仓依赖地板统一升级。go.sum 的 h1 取自 sumdb（经 goproxy.cn 代理），
即 `go mod tidy` 会写入的同一个权威值；只就地替换该 mod 的两行、其余行一个字节不动，
不重排全文件（go.sum 行序由 cmd/go 自己维护，手写重排会引入漂移）。

- 杂务(deps)：oekit/concurrency v0.12.0 -> v0.13.1

本仓不使用上游 breaker（熔断只在 llm 仓），v0.13.0 的 #25 修复与
v0.13.1 的 #26（RetryableWithHeaders 头值 OWS 修剪）对本仓无直接影响，
本次为矩阵内版本对齐。
门禁：go mod tidy -diff / build / vet /
test -race / gofmt 全绿。

- 杂务(deps)：oekit 三模块升至最新（pkg v0.13.0 / concurrency v0.12.0 / logx v0.7.0）

拿到上游新增能力：
- concurrency #22 RetryableWithHeaders：403 头感知判定（次级限流 / 主限流耗尽）
- concurrency #23 FanInLimit：受限并发扇出
- concurrency #24 WithTokenClock、#21 Lockout（keyed lockout/penalty）
- logx #12 TraceHandler：给裸 slog.Handler 注入 trace_id
破坏性变更核对（均无影响）：
- logx v0.7.0 ParseLogLevel 签名改为 (Level, error) —— 我们未使用该函数
- concurrency v0.9.0 的 Slot.Panicked 废弃 —— rag 已迁到 Slot.Err + errors.As
门禁：go build / go vet / go test -race 全绿。

- 杂务(deps)：oekit/concurrency v0.6.0 -> v0.9.0

拿到上游显式三态状态机（#17）与 singleflight 可观测折叠（#18），并为本次
breaker 替换评估了语义差异。
用到的子包只有 async / singleflight / syncx —— 上游 breaker 包我们并未引用
（runtime/breaker 是本仓自己的三态实现），所以本次升级零 breaking。
关于 breaker 替换的现状：#17 已落地（closed/open/half-open + 代绑定 + 探测时限 +
开放期失败不延长冷却），但两侧的触发语义仍不对齐，已在上游补两个需求：
- #19 冷却期与探测失联时限共用 probeTimeout，无法独立配置
- #20 缺「连续失败 N 次即熔断」（trip）触发
因此本包暂不删除 —— llm.TestResilientBreakerTrips 钉的是 trip 语义，
上游 v0.9.0 只有速率型触发（dropRatio > 0），现在换会让该测试失败。


- CI：接入 org 共享门禁（规则本体在 oagentkit/.github）
