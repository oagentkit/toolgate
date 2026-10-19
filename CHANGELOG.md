# toolgate CHANGELOG

> 本文件由 `matrix-tools/scripts/gen_changelogs.py` 从 git 历史生成，按 tag 切段、取每段内的 conventional commit 渲染。

## [v0.11.0] — 2026-10-19

- 杂务(deps)：依赖地板对齐远端最新（mcpkit v0.8.0 / runtime v0.7.0） (13)

## [v0.10.0] — 2026-10-19

- 杂务(deps)：依赖地板对齐远端最新（mcpkit v0.6.0 / runtime v0.6.0 / pkg v0.15.1 / concurrency v0.17.1） (#10)

## [v0.9.0] — 2026-10-19

- 杂务(deps)：oekit 依赖升级（mcpkit v0.5.0→v0.5.1、concurrency v0.16.0→v0.17.0）

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

- 杂务(deps)：依赖地板升到「历史最新 + 0.1.0」 (#5)
- 杂务(deps)：跟进最新 tag (#4)
- 杂务(deps)：oekit 依赖升级 (#3)
- 杂务(deps)：mcpkit v0.2.1 → v0.3.0（本轮新发 tag 的连锁升级） (#2)
- 杂务(deps)：跨仓依赖升级 (#1)
- 杂务(deps)：oekit/concurrency v0.12.0 -> v0.13.1
- 杂务(deps)：oekit 三模块升至最新（pkg v0.13.0 / concurrency v0.12.0 / logx v0.7.0）
- 杂务(deps)：oekit/concurrency v0.6.0 -> v0.9.0

- CI：接入 org 共享门禁（规则本体在 oagentkit/.github）
