# toolgate CHANGELOG

> 本文件由 `matrix-tools/scripts/gen_changelogs.py` 从 git 历史生成，按 tag 切段、取每段内的 conventional commit 渲染。

## [v0.15.0] — 2026-11-07

### 新增(invoke)：补搬迁兼容壳——Deprecated 转发 guardx/invoke,旧 import 路径恢复编译（agent#68）

toolgate v0.12.6 起 invoke 子包整体搬至 guardx/invoke,旧 import 路径直接断裂
且零转发(oagentkit/agent#68)。本壳逐符号别名转发 guardx/invoke 全部导出符号
(12 type 别名 + 4 哨兵 var + 4 函数),只转发不改语义——含 guardx v0.20.0 起
Record.Allowed「到达执行体即置位,与执行成败无关」语义(guardx#73),不做任何
兼容性改写。guardx 本就是本仓矩阵内依赖(v0.20.0),零架构成本。
搬迁公告(本仓 CHANGELOG 由生成器生成不可手改,三行固定格式记于 invoke/doc.go
并随本 commit body 收录):
MovedFrom: github.com/oagentkit/toolgate/invoke
MovedTo: github.com/oagentkit/guardx/invoke
兼容期截止版本: v0.16.0(壳随 v0.15.0 提供,保一个完整 minor)
验证:gofmt -l . 空、go vet ./...、go build ./...、go test ./... 全绿;
新增 invoke/compat_test.go 以旧 import 路径消费代表性符号并断言与上游同实体。
未触碰 CHANGELOG.md / go.mod / AGENTS.md / matrix-tools,未打 tag。
Refs oagentkit/agent#68

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/guardx v0.21.0

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oagentkit/guardx` | `v0.20.0` | `v0.21.0` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/runtime v0.12.0

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oagentkit/runtime` | `v0.11.0` | `v0.12.0` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>


## [v0.14.0] — 2026-11-07

### 修复(policy)：裁决→放行映射收敛唯一单源，规则合法 verdict 域补 plan_only（#53 #54）

close #53
close #54
## #53 AuditGate 装饰器与 Stage 收敛同一张裁决→放行映射表
**事实核对**：issue 正文描述的「装饰器只拦 `VerdictDeny`」在当前 main 已不存在——`fix(policy,docs): AuditGate fail-closed 与 Stage 对齐`（#42，merge `989c918`，随 v0.13.0 发布）已把装饰器改为「非 `VerdictAuto` 一律拒绝」，并关掉了同型的 #40。#53 报文引用的 `toolgate.go:23-31` 是修复前代码。
**残留的真问题**（issue「建议修复」后半句）：装饰器与 Stage 仍是**两份各判各的判断**（一个 `if`、一个五分支 `switch`），今天结论一致只是碰巧——#40 的形态正是「两条路径漂移、一面修了另一面没修」，没有任何护栏阻止复发。
**修法**：
- 抽 `verdictDisposition(v) (allow, retryable, known)` 作为裁决→执行处置的**唯一映射单源**（`policy/policy.go`）：auto 唯一放行；其余四个合法值拒绝且不可重试；认不出的值 fail-closed 且 `known=false` 供调用方把「未识别」如实写进审计理由。新增 Verdict 值时只改这一处。
- 装饰器（`toolgate.go`）与 Stage（`stage.go`）全部改为经该单源取结论，删掉各自的分支判断。
- 新增 `TestAuditFacesAgreeOnEveryProducibleVerdict`：对 Gate 能产出的每一类裁决（auto 白名单 / deny 变异 / need_human 规则 / plan_only 规则 / plan_only 矩阵），**同一 Gate 同时驱动装饰器与 Stage 两面**，断言同放行、同触达执行体、拒绝不可重试——两面漂移当场红。
- 新增 `TestVerdictDispositionSingleSource`：对裁决全集（含未识别值）逐点断言映射单源。
- 修正既有 `TestAuditGateFailClosed` 的**假阳注入**：原 plan_only/未识别两行走 `DefaultPolicy`（无规则）+ `stubArbiter` 造门，但仲裁器只在「规则 verdict=unknown 命中」时才被触发，实际裁决走的是矩阵 deny——测的不是注入的裁决。现改走宿主真实表达路径（规则面直产 + plan 模式提审矩阵面），并把 `Mutating` 置 false 防矩阵兜底掩盖规则注入失效。
## #54 规则合法 verdict 域纳入 plan_only
**根因**：`isValidVerdict` 白名单不含 `VerdictPlanOnly`，而 `Verdict` 注释称 plan_only 是「例外规则可产出」的过程值——yaml 规则写 `verdict: plan_only` 会被 fail-safe 改写成 `need_human`，审计理由谎报「规则 verdict 非法（plan_only）」。plan_only=降级为仅提审 ≠ need_human=升人审，fail-safe 改写了配置意图。
**修法**：
- `VerdictPlanOnly` 入 `isValidVerdict` 合法域：规则可直产 plan_only，经 #53 的映射单源在两个执行面统一按「拒绝且不可重试」收口（fail-closed 不变，语义不再被改写）。
- `Rule.Verdict` 字段注释、`LoadOverrides` 报错文案把合法集列全（`auto_proceed|need_human|deny|plan_only|unknown`），`Verdict` 类型注释口径改准（plan_only 矩阵与规则均可产出；unknown 仅规则可写且裁决面从不原样返回）。
- 测试：`TestRuleVerdictPlanOnlyIsLegal`（规则原样产出 plan_only；拼错值仍 fail-safe 升人审）+ `TestLoadOverridesRuleVerdictDomain`（加载期放行 plan_only 规则、拼错值 fail-fast 且文案含合法集）。
## 验证
- `gofmt -l .` 空、`go vet ./...`、`go build ./...`、`go test ./...` 本地全绿。
- 未触碰 CHANGELOG.md / go.mod / AGENTS.md / README / matrix-tools，未打 tag。

- 杂务(deps)：依赖地板追到最新——github.com/oekit/pkg v0.37.1

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oekit/pkg` | `v0.37.0` | `v0.37.1` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/guardx v0.20.0

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oagentkit/guardx` | `v0.19.0` | `v0.20.0` |
| `github.com/oagentkit/mcpkit` | `v0.11.0` | `v0.12.0` |
| `github.com/oagentkit/runtime` | `v0.8.0` | `v0.11.0` |
| `github.com/oekit/concurrency` | `v0.25.1` | `v0.28.0` |
| `github.com/oekit/pkg` | `v0.35.0` | `v0.37.0` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。


## [v0.13.0] — 2026-11-06

### 修复(policy)：overrides 阈值三档语义——0=禁用代批，负数/超基座 fail-fast（#44）

auto_max_risk: 0 的自然语义「auto 模式禁止任何代批」此前被 <=0 钳回基座
——clamp 把「调低」与「禁用」一并吞掉，配置面无禁用表达（只能绕道写 deny
规则）。
- *int 影子解码区分「未写键」（继承基座）与「显式 0」（禁用）——yaml 缺键
  零值也是 0，裸 int 分不出
- 超基座从静默 clamp 改为 fail-fast（调高=扩权须改基座走评审；静默回基座
  把配置错误藏进启动成功里）
- 负数 fail-fast（无语义；缺省请删键）
TestLoadOverridesZeroDisables 四档钉子；既有 clamp 测试同步更新为
fail-fast 断言。全仓 -race 绿。
关联 oagentkit/toolgate#44
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

### 修复(policy,docs)：AuditGate fail-closed 与 Stage 对齐 + 调度去重承诺纠偏（#40 #41）

policy#40——AuditGate 此前只拦 VerdictDeny，need_human/unknown/plan_only
静默放行执行体：宿主 LoadOverrides 配「该工具需人工审批」后，装饰器形态
下工具照常执行、只多一条审计回调，与 Stage 的「非 auto 一律拒绝且不可
重试」映射相反（AGENTS 的 fail-closed 纪律在装饰器形态失守）。拒绝条件
改为 Verdict != VerdictAuto，错误带 verdict/rule/reason 三元组。
toolsched#41——README/AGENTS 宣称「并发调度与去重」，但 Call 结构无参数
字段（ID/Name/DependsOn/Hints/StopAfter），执行体参数活在 Task.Run 闭包
里，调度层没有「name+归一化参数」去重键可算——去重语义在对标落地时丢失，
文档是残留承诺。文档三方（README/AGENTS/包注释）纠偏为「不做语义去重，
宿主装配前自做」；诚实文档优于幽灵承诺。
TestAuditGateFailClosed：deny/need_human/unknown/plan_only/未识别五裁决
逐点拒绝 + 执行体零触达 + auto 真实放行。
关联 oagentkit/toolgate#40 oagentkit/toolgate#41
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——github.com/oekit/pkg v0.35.0

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oekit/pkg` | `v0.34.1` | `v0.35.0` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/mcpkit v0.11.0

mcpkit v0.11.0（2026-11-06 发布）落地后本仓地板落后；verify_tag_floor 要求
打 tag 时的地板必须当时最新。提交前 go build / go vet / go test -race 全绿，
go mod tidy -diff 无漂移；零行为差异。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/guardx v0.19.0

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oagentkit/guardx` | `v0.18.0` | `v0.19.0` |
| `github.com/oagentkit/runtime` | `v0.7.3` | `v0.8.0` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——github.com/oekit/concurrency v0.25.1

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oekit/concurrency` | `v0.24.0` | `v0.25.1` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——本轮新 tag（首轮 tidy 撞上新发版 proxy 延迟，重跑补完）

Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——github.com/oekit/pkg v0.34.1

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oekit/pkg` | `v0.34.0` | `v0.34.1` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/guardx v0.17.1

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oagentkit/guardx` | `v0.17.0` | `v0.17.1` |
| `github.com/oagentkit/mcpkit` | `v0.9.0` | `v0.9.1` |
| `github.com/oagentkit/runtime` | `v0.7.2` | `v0.7.3` |
| `github.com/oekit/concurrency` | `v0.23.3` | `v0.24.0` |
| `github.com/oekit/pkg` | `v0.31.1` | `v0.34.0` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。
Co-authored-by: Murphy-Yi <zhangyi@Murphy-YideMac-Mini.local>


## [v0.12.6] — 2026-11-04

### 重构(toolsched)：并行在飞闸门收编 syncx.Gate、runGuarded 收口 RunSafeRetWith

闸门从裸 channel 信号量改 oekit syncx.Gate（取名额感知 ctx——取消后不再无限期
等名额，归还幂等）；任务执行体 panic 隔离改 async.Runner+*PanicError（Result.Err
文案逐字一致，errors.As 可取栈；本包无组件 logger，Handler 置空抑制全局 stderr
兜底）。deps：oekit/concurrency v0.23.3

- 杂务(deps)：依赖地板追到最新——github.com/oekit/concurrency v0.23.2

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oekit/concurrency` | `v0.23.1` | `v0.23.2` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/guardx v0.17.0 / github.com/oagentkit/mcpkit v0.9.0

两条：
- guardx v0.17.0（2026-10-28 发布）⇒ 地板落后；
- mcpkit v0.9.0（2026-10-28 发布，**含两处破坏式变更**：Allow 缺省反转、工具暴露名带 server 命名空间）⇒ 本仓是矩阵内 mcpkit 的消费方，抬地板即**真的踩上**那个破坏 ⇒ 实测 build / vet / test -race 全绿（本仓只用 mcpkit 的 `OutputSchemer` 与 `ErrorKind`，未触碰 `ToolSpec.Allow` 与工具名 ⇒ 无需改代码）。
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`，宽口径）要求：**各仓 HEAD**
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
兼容性实测：本仓只用这些依赖的少量 API ⇒ go build / go vet / go test -race 全绿，
**零行为差异**；go 指令未被顶起。
只改 go.mod / go.sum ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。

- 杂务(deps)：依赖地板追到最新——github.com/oekit/concurrency v0.23.1

oekit 基座又发了一版（`concurrency` v0.23.1，2026-10-29 02:19 CST）⇒ 上一轮抬到
v0.23.0 的地板已经落后 ⇒ 再抬一格。
本仓只用到 `concurrency` 的少量 API ⇒ 实测 go build / go vet / go test -race 全绿，
**零行为差异**；go 指令未被顶起。
只改 go.mod / go.sum ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。

- 杂务(deps)：依赖地板追到最新——github.com/oekit/concurrency v0.23.0

oekit 基座发了新版本（pkg v0.31.0 / concurrency v0.23.0 / logx v0.8.0）⇒ 本仓的
依赖地板落后 ⇒ 抬到最新，让**下一次发版**不再带出落后地板（打 tag 时刻的地板
必须是当时的最新——`verify_tag_floor` / `release.py doctor` 都在查这条）。
兼容性实测：本仓只用这些基座包的少量 API ⇒ go build / go vet / go test -race
全绿，**零行为差异**；go 指令未被顶起（本仓的 go 地板不动）。

- 杂务(deps)：依赖地板追平上游本轮新版本——github.com/oagentkit/guardx@v0.15.0、github.com/oekit/concurrency@v0.22.0、github.com/oekit/pkg@v0.28.0

本仓 HEAD 的依赖地板已落后于各依赖的最新版本（`verify_tag_floor.py --org oagentkit --ref HEAD` 判红）。
本仓一旦发版，就会把落后地板发出去——而 tag 受 protect-tags ruleset 保护、打完不能重打，
只能靠后续版本补齐。
本版只抬依赖地板，不改任何行为：
- github.com/oagentkit/guardx → v0.15.0
- github.com/oekit/concurrency → v0.22.0
- github.com/oekit/pkg → v0.28.0
对应 issue matrix-tools#143。


- 文档(toolgate)：刷正依赖清单——补上 invoke 迁出后新增的 guardx

本仓 go.mod 的矩阵内**直接**依赖是 guardx 与 mcpkit 两条，而 README.md 与
AGENTS.md 的「本仓依赖的矩阵内仓」清单都只列了 mcpkit，漏了 guardx。
guardx 依赖是 5c4922918524e9f446bf593fd04b25c2185b1d97
（refactor!: invoke 迁出本仓 —— 框架无关的原语不该被 eino 重仓的地板拖住）
带进来的：invoke 迁到 github.com/oagentkit/guardx/invoke，本仓直接调用它，
go.mod 里那行 `require github.com/oagentkit/guardx v0.14.0` **没有** // indirect 标记
⇒ 是直接依赖。
这行漏列尤其危险：本仓 AGENTS.md 的核心不变量「新增门禁一律加在 invoke 链上」
依赖的正是这条 guardx 链路，而清单里没有 guardx，读的人会以为本仓不依赖它。
清单口径 = go.mod 的「直接」矩阵内依赖（// indirect 不列）——
本仓 go.mod 里 `github.com/oagentkit/runtime v0.7.2 // indirect` 是被 mcpkit
拉进来的，不列，与 modules.json 的 toolgate.dependsOn = ["guardx","mcpkit"] 同口径。
关联 oagentkit/matrix-tools#127

- 文档(toolgate)：刷正 README 的矩阵内依赖清单 —— 与 go.mod 实测对账

本仓 README 的「矩阵内依赖」清单与**各仓 go.mod 的事实**对不上。
矩阵的单一数据源 `matrix-tools/modules.json` 也有同样漂移，已在
[matrix-tools#126](https://github.com/oagentkit/matrix-tools/pull/126) 同步刷正。
## 现状核实
```bash
$ grep -rn "oagentkit/guardx" toolgate/ --include=*.go | grep -v _test.go
toolgate/policy/failclass.go:45:	"github.com/oagentkit/guardx/toolfail"
toolgate/policy/stage.go:6:	"github.com/oagentkit/guardx/invoke"
```
⇒ `guardx` 是**生产代码**的直接依赖（不是测试依赖），清单里漏了它。
`modules.json` 的 `dependsOn` 本来就写了 `["guardx", "mcpkit"]` ⇒ **数据源是对的，是 README 漏了**。
## 影响
`guardx` 提供 `invoke`（调用链构造）与 `toolfail`（失败分类）—— 本仓的「工具调用链」纪律就建在它上面。读者按 README 只看到 mcpkit ⇒ 看不到这条依赖方向。
## 改动
**只改清单那一行（`- 本仓依赖的矩阵内仓：`mcpkit`。` ⇒ `- 本仓依赖的矩阵内仓：`guardx`、`mcpkit`。`）**，其余段落逐字未动。
## 验收
- [x] 清单与 `go.mod` 实测一致
- [x] 与 `matrix-tools/modules.json` 的登记一致（同期在 matrix-tools#126 刷正）
- [x] **纯 .md 改动** ⇒ 不需要 CHANGELOG 条目、不打 tag、不发版（无发版步骤）
关联 oagentkit/matrix-tools#125


## [v0.12.5] — 2026-10-23

- 杂务(deps)：依赖地板跟到矩阵本轮新版本——guardx v0.14.0 / mcpkit v0.8.6 / concurrency v0.20.2 / pkg v0.22.0

## 下游要做什么
**`go mod tidy`**。地板抬高对下游是**隐式破坏性变更**：不跑就报
`updates to go.mod needed`，且报错不提真正原因（**不是本仓的 API 变了**）。
本仓**自身代码零改动**——只看本仓的 diff 看不出这是一次地板抬高。
## 为什么抬
本仓 HEAD（`1991f00`，2026-10-21T12:01:45Z）**早于**下面四个版本的发布时刻，
所以宽口径判据（`verify_tag_floor.py --repo toolgate --ref HEAD`）此刻仍判绿；
但用门禁自己的 `check_ref()` 把 ref 时刻推进到现在 ⇒ **4 处红**：
- github.com/oagentkit/guardx: v0.13.1 → v0.14.0（2026-10-22T09:46:52Z 发布）
- github.com/oagentkit/mcpkit: v0.8.5 → v0.8.6（2026-10-23T05:54:01Z 发布）
- github.com/oekit/concurrency: v0.19.0 → v0.20.2（2026-10-23T03:09:14Z 发布）
- github.com/oekit/pkg: v0.20.0 → v0.22.0（2026-10-22T09:18:33Z 发布）
⇒ **下一次发版会把「打 tag 那一刻就已落后」的地板发出去**，而 tag 受 protect-tags
ruleset 保护不可重打 ⇒ 坏 tag 流出后只能补发新 tag，且 `--latest-only` 口径的 CI
届时会当场判红（它审的就是「最新 tag 当时的地板」）。
门禁给的修法是「抬 go.mod 地板 → go mod tidy → 重新发 tag」。本 PR 就是那条修法。
## 为什么无行为差异
逐条 diff 两版的实测差异，**四条对本仓都是行为中性**：
- `guardx v0.14.0`：只改 `permgate/permgate.go`（新增「只读拦截」段）——本仓只 import
  `guardx/invoke` + `guardx/toolfail`，未命中；
- `mcpkit v0.8.6`：零 `.go` 改动；
- `oekit/concurrency v0.20.2`：只改 `syncx/gate.go` + `syncx/syncx.go`
  （`Gate.TryAcquire` 新增、`KeyedGate` 幂等标记改 `atomic.Bool`+CAS）——本仓只 import
  `concurrency/async`，未命中；
- `oekit/pkg v0.22.0`：只新增 `pathx/*` 与 `stringx/csv.go`（`status: added`），
  既有函数逐字未改——本仓不 import `pathx` / `stringx`。
本仓用到的转发面与符号集一字未动。
## 门禁
```
go build ./...                 ✅
go test -race ./...            ✅ (policy / toolprior / toolsched)
go mod tidy -diff              ✅ 空
gofmt -l .                     ✅ 空
```


## [v0.12.4] — 2026-10-21

- 杂务(deps)：依赖地板跟到矩阵本轮新版本——mcpkit v0.8.5

## 下游要做什么
**`go mod tidy`**。地板抬高对下游是**隐式破坏性变更**：不跑就报
`updates to go.mod needed`，且报错不提真正原因（**不是本仓的 API 变了**）。
本仓**自身代码零改动**——只看本仓的 diff 看不出这是一次地板抬高。
## 为什么抬
上一版 tag 打出去时，它依赖的模块的**新版已经先发出**了——
即「打 tag 那一刻地板就落后」。矩阵的「发版时刻地板时效」门禁
（`verify_tag_floor.py --org oagentkit --latest-only`）当场报红：
- github.com/oagentkit/mcpkit@v0.8.5
门禁给的修法是「抬 go.mod 地板 → go mod tidy → 重新发 tag」。
本 PR 就是那条修法。
## 为什么无行为差异
被抬的这几个版本**自身也都是纯依赖地板追平**（各仓代码零改动，只抬
`oekit/pkg` / `oekit/concurrency` / `guardx` / `llm` 四类地板之一），
本仓用到的转发面与符号集一字未动。
## 门禁
```
go build / vet / test -race   ✅
gofmt -l .                    ✅ 空
```


## [v0.12.3] — 2026-10-21

### 新增(policy)：协议层失败分类 → 处置层类别的机械映射

mcpkit/mcp.ErrorKind 判的是协议层事实，guardx/toolfail.Class 判的是处置层语义，
两者是同一件事的两个问题。本仓是矩阵里唯一同时依赖 mcpkit 与 guardx 的仓，
两边的原语在此会合，故映射表落在这里——散在各调用点手写必然漂移。
- FailureClass(kind)：ErrorKind → toolfail.Class 的单源映射表；未登记的分类
  一律回落 ClassUnknown（不可分类 = 不可重试）。
- Disposition(err)：调用点该调的那一个。宿主已显式归类时以那份为准；宿主自报
  「不可重试」时不得回落到协议层映射，否则会把宿主已否决的失败翻成可重试。
- 两层结论不一致时取不可重试的一侧：mcp.KindProtocol.Retryable() 为 true，
  映射后却是 ClassPermanent——协议层问「重试有没有可能恢复」，处置层问
  「原样重试有没有意义」；误判可重试会把一次失败放大成 N 次。
- TestFailureClassCoversEveryErrorKind 枚举 ErrorKind 的全部取值，缺映射当场红
  （新增协议层分类漏改映射表时兜住）。
Closes oagentkit/mcpkit#17
* docs(changelog): 预写 v0.12.3 待发版段

## [v0.12.2] — 2026-10-21

- 杂务(deps)：依赖地板跟到矩阵本轮新版本（第二轮：消除打 tag 时刻的地板落后）

* chore(deps): 依赖地板跟到矩阵本轮新版本——mcpkit v0.8.4 runtime v0.7.2
## 下游要做什么
**`go mod tidy`**。地板抬高对下游是**隐式破坏性变更**：不跑就报
`updates to go.mod needed`，且报错不提真正原因（**不是本仓的 API 变了**）。
本仓**自身代码零改动**——只看本仓的 diff 看不出这是一次地板抬高。
## 为什么抬
上一版 tag 打出去时，它依赖的模块的**新版已经先发出**了——
即「打 tag 那一刻地板就落后」。矩阵的「发版时刻地板时效」门禁
（`verify_tag_floor.py --org oagentkit --latest-only`）当场报红：
- github.com/oagentkit/mcpkit@v0.8.4
- github.com/oagentkit/runtime@v0.7.2
门禁给的修法是「抬 go.mod 地板 → go mod tidy → 重新发 tag」。
本 PR 就是那条修法。
## 为什么无行为差异
被抬的这几个版本**自身也都是纯依赖地板追平**（各仓代码零改动，只抬
`oekit/pkg` / `oekit/concurrency` / `guardx` / `llm` 四类地板之一），
本仓用到的转发面与符号集一字未动。
## 门禁
```
go build / vet / test -race   ✅
gofmt -l .                    ✅ 空
```
* docs(changelog): 重生成 CHANGELOG.md——补回浅克隆期间漏掉的条目
上一版 tag（v0.12.1）的 CHANGELOG 段**漏了 5 条提交**：本地克隆是浅克隆
（depth 限制），生成器遍历 tag 区间时看不到更早的提交，于是那几条被漏掉。
本次重生成（本地已补齐完整历史）把 v0.12.1 段补回，并预写 v0.12.2 待发版段
（依赖地板跟到 mcpkit v0.8.4、runtime v0.7.2）。
**为什么必须修**：下游 `go get` 拿到的是 tag 快照——快照里的 CHANGELOG 若
缺条目，发布说明就是残缺的（矩阵的 CHANGELOG 覆盖门禁正是查这个）。


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

- 杂务(deps)：oagentkit/guardx 依赖地板抬到 v0.9.0

guardx v0.9.0 新增了 toolfail 与 sandbox 两个 tier0 原语，
是**新增能力**而非 API 变更——本仓无需改代码，只抬地板。
为什么现在补：guardx v0.9.0 是 2026-10-21 发布的，本仓上一个 tag 早于它，
于是这一格欠账一直挂着（依赖地板报告器把「子仓发版让下游落后」显式化，
但报告器恒不失败——真正的硬约束是「打 tag 时刻地板必须是当时的最新」）。
本地门禁：go mod tidy 无差异 / go build / go vet 通过。

- 杂务(deps)：oagentkit/mcpkit 依赖地板抬到 v0.8.3

* docs(AGENTS): 补「发版顺序」小节，指向 org CONTRIBUTING 单一副本
13 个业务仓的 AGENTS.md 此前对发版顺序零覆盖（只有 textx 有个
「### 版本与发版」标题，但里面只讲 SemVer 与 module 路径），
发版顺序只写在 org 规则仓的 .github/CONTRIBUTING.md 里，单仓读者看不到。
本次只加**指针 + 本仓相关提醒**，runbook 本体保持**单一副本**
（CONTRIBUTING §发版），避免 13 份副本各自漂移。
覆盖四条：
- CHANGELOG.md 由 matrix-tools/scripts/gen_changelogs.py 从 git 历史生成，不要手改
- tag 必须落在 CHANGELOG 提交之后（下游 go get 拿到的是 tag 快照）
- 警告：无参重生成会吞掉待发版段，--next-version 是唯一能产出待发版段的开关
- 本仓相关提醒（见正文）
* chore(deps): oagentkit/mcpkit 依赖地板抬到 v0.8.3
mcpkit v0.8.2 → v0.8.3 是**纯依赖地板抬升**（零 .go 改动，只动了 go.mod/go.sum
与 CHANGELOG），本仓无需改代码。
为什么现在补：矩阵的依赖地板是「会漂」的常态——任何一次子仓发版都会让下游落后。
本仓的 mcpkit v0.8.3 是 2026-10-20 发布的，而本仓 v0.12.0 打 tag 在那之前，
于是这一格欠账一直挂着。补齐它，本次全矩阵的第一方依赖地板达成零落后
（此前唯一剩下的两处是 agent→llm 与 toolgate→mcpkit）。
mcpkit v0.8.2 → v0.8.3 的内容：oekit 三个基座抬到 v0.18.0 +
矩阵内跨仓依赖抬到各仓最新 tag，无 API 变更。
本地门禁：go mod tidy 无差异 / go build / go vet 通过。

- 杂务(deps)：oekit 依赖地板抬到 v0.18.0

oekit 姊妹矩阵的依赖地板对齐到最新：
- github.com/oekit/pkg        v0.17.0 → v0.18.0
- github.com/oekit/concurrency v0.17.1 → v0.18.0
- github.com/oekit/logx       已是最新 v0.7.1，不动
oekit v0.18.0 的两个发布都是**纯增量**，无破坏性变更：
- pkg：jsonrepair 补抽数组原语 ExtractArray；
- concurrency：新增 `DoValue[T]`（带返回值的重试），`Do` 改为委托给它——
  退避序列、抖动、MaxDelay 封顶、DelayHint 优先、Retryable 门控、ctx 取消
  **只有一份实现**（此前这类调用方只能自己重写一遍退避，且常丢掉 DelayHint 语义）。
依据 oagentkit/matrix-tools#44 的还债流程：依赖地板对齐到发版时刻的最新。
本仓本轮不发 tag（不在发版集内），地板在 main 上就位，下次发版自然带上。


- 文档(AGENTS)：补「发版顺序」小节，指向 org CONTRIBUTING 单一副本

13 个业务仓的 AGENTS.md 此前对发版顺序零覆盖（只有 textx 有个
「### 版本与发版」标题，但里面只讲 SemVer 与 module 路径），
发版顺序只写在 org 规则仓的 .github/CONTRIBUTING.md 里，单仓读者看不到。
本次只加**指针 + 本仓相关提醒**，runbook 本体保持**单一副本**
（CONTRIBUTING §发版），避免 13 份副本各自漂移。
覆盖四条：
- CHANGELOG.md 由 matrix-tools/scripts/gen_changelogs.py 从 git 历史生成，不要手改
- tag 必须落在 CHANGELOG 提交之后（下游 go get 拿到的是 tag 快照）
- 警告：无参重生成会吞掉待发版段，--next-version 是唯一能产出待发版段的开关
- 本仓相关提醒（见正文）

- 文档(changelog)：按新生成器规则重生成 CHANGELOG.md

生成器修复（oagentkit/matrix-tools#43）落地：
① `[round-N] ` 前缀被剥离 → conventional commit 解析不再失效，
   **commit body 保留**（此前整段被丢弃，发布说明只剩标题）；
② 非 conventional 提交也保留 body；
③ `--check` 识别「待发版段」（预写了但还没打 tag 的那段）。
本 PR 恢复的是**本就在 commit body 里**的正文（迁移步骤、代码块、
判据锚点）——此前被 `[round-N] ` 前缀导致的解析失败丢弃了。
**没有任何发布说明内容被删除**：删除的行都能在新增行里找到对应
（规范化掉 `(#N)` 与 `[round-N] ` 前缀后逐行比对）。
关联：oagentkit/matrix-tools#43、oagentkit/matrix-tools#40


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

- 与 v0.9.1 相比无代码差异（版本号/打包变更）

## [v0.9.1] — 2026-11-04

### 新增(policy)：协议层失败分类 → 处置层类别的机械映射

mcpkit/mcp.ErrorKind 判的是协议层事实，guardx/toolfail.Class 判的是处置层语义，
两者是同一件事的两个问题。本仓是矩阵里唯一同时依赖 mcpkit 与 guardx 的仓，
两边的原语在此会合，故映射表落在这里——散在各调用点手写必然漂移。
- FailureClass(kind)：ErrorKind → toolfail.Class 的单源映射表；未登记的分类
  一律回落 ClassUnknown（不可分类 = 不可重试）。
- Disposition(err)：调用点该调的那一个。宿主已显式归类时以那份为准；宿主自报
  「不可重试」时不得回落到协议层映射，否则会把宿主已否决的失败翻成可重试。
- 两层结论不一致时取不可重试的一侧：mcp.KindProtocol.Retryable() 为 true，
  映射后却是 ClassPermanent——协议层问「重试有没有可能恢复」，处置层问
  「原样重试有没有意义」；误判可重试会把一次失败放大成 N 次。
- TestFailureClassCoversEveryErrorKind 枚举 ErrorKind 的全部取值，缺映射当场红
  （新增协议层分类漏改映射表时兜住）。
Closes oagentkit/mcpkit#17
* docs(changelog): 预写 v0.12.3 待发版段

### 重构(toolsched)：并行在飞闸门收编 syncx.Gate、runGuarded 收口 RunSafeRetWith

闸门从裸 channel 信号量改 oekit syncx.Gate（取名额感知 ctx——取消后不再无限期
等名额，归还幂等）；任务执行体 panic 隔离改 async.Runner+*PanicError（Result.Err
文案逐字一致，errors.As 可取栈；本包无组件 logger，Handler 置空抑制全局 stderr
兜底）。deps：oekit/concurrency v0.23.3

### 重构：invoke 迁出本仓 —— 框架无关的原语不该被 eino 重仓的地板拖住

BREAKING CHANGE: github.com/oagentkit/toolgate/invoke → github.com/oagentkit/guardx/invoke
invoke 的包文档自证「刻意不携带任何框架类型：本包与 policy/toolegate 无耦合」——
它是框架无关的原语，自身 import 面只有 oekit/pkg/errors，却在带 eino 家族的重仓里
让消费方白吃 34 个第三方模块。
policy 改为依赖 guardx/invoke（tier2 → tier0，方向合法，无环）。
关联 oagentkit/guardx#3

- 杂务(deps)：依赖地板追到最新——github.com/oekit/concurrency v0.23.2

| 依赖 | 现地板 | 抬到 |
|---|---|---|
| `github.com/oekit/concurrency` | `v0.23.1` | `v0.23.2` |
## 为什么
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`）要求：各仓 HEAD
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
oekit 基座与矩阵模块都在高频发版 ⇒ 地板是**跑步机**，落后一格就会判红。
## 兼容性实测
本工具在提交前跑完 `go build` / `go vet` / `go test -race -count=1 ./...`（全绿才提交）
与 `go mod tidy -diff`（无漂移）⇒ 改动**只影响依赖版本**，零行为差异。
只改 `go.mod` / `go.sum` ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。

- 杂务(deps)：依赖地板追到最新——github.com/oagentkit/guardx v0.17.0 / github.com/oagentkit/mcpkit v0.9.0

两条：
- guardx v0.17.0（2026-10-28 发布）⇒ 地板落后；
- mcpkit v0.9.0（2026-10-28 发布，**含两处破坏式变更**：Allow 缺省反转、工具暴露名带 server 命名空间）⇒ 本仓是矩阵内 mcpkit 的消费方，抬地板即**真的踩上**那个破坏 ⇒ 实测 build / vet / test -race 全绿（本仓只用 mcpkit 的 `OutputSchemer` 与 `ErrorKind`，未触碰 `ToolSpec.Allow` 与工具名 ⇒ 无需改代码）。
矩阵的**发版时刻地板时效**门禁（`verify_tag_floor --ref HEAD`，宽口径）要求：**各仓 HEAD**
（= 下次发版会发出去的内容）的依赖地板，相对那一刻的依赖最新版本，不能已落后。
兼容性实测：本仓只用这些依赖的少量 API ⇒ go build / go vet / go test -race 全绿，
**零行为差异**；go 指令未被顶起。
只改 go.mod / go.sum ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。

- 杂务(deps)：依赖地板追到最新——github.com/oekit/concurrency v0.23.1

oekit 基座又发了一版（`concurrency` v0.23.1，2026-10-29 02:19 CST）⇒ 上一轮抬到
v0.23.0 的地板已经落后 ⇒ 再抬一格。
本仓只用到 `concurrency` 的少量 API ⇒ 实测 go build / go vet / go test -race 全绿，
**零行为差异**；go 指令未被顶起。
只改 go.mod / go.sum ⇒ 不需要 CHANGELOG 条目（版本段由下次发版的 `prewrite` 生成）。

- 杂务(deps)：依赖地板追到最新——github.com/oekit/concurrency v0.23.0

oekit 基座发了新版本（pkg v0.31.0 / concurrency v0.23.0 / logx v0.8.0）⇒ 本仓的
依赖地板落后 ⇒ 抬到最新，让**下一次发版**不再带出落后地板（打 tag 时刻的地板
必须是当时的最新——`verify_tag_floor` / `release.py doctor` 都在查这条）。
兼容性实测：本仓只用这些基座包的少量 API ⇒ go build / go vet / go test -race
全绿，**零行为差异**；go 指令未被顶起（本仓的 go 地板不动）。

- 杂务(deps)：依赖地板追平上游本轮新版本——github.com/oagentkit/guardx@v0.15.0、github.com/oekit/concurrency@v0.22.0、github.com/oekit/pkg@v0.28.0

本仓 HEAD 的依赖地板已落后于各依赖的最新版本（`verify_tag_floor.py --org oagentkit --ref HEAD` 判红）。
本仓一旦发版，就会把落后地板发出去——而 tag 受 protect-tags ruleset 保护、打完不能重打，
只能靠后续版本补齐。
本版只抬依赖地板，不改任何行为：
- github.com/oagentkit/guardx → v0.15.0
- github.com/oekit/concurrency → v0.22.0
- github.com/oekit/pkg → v0.28.0
对应 issue matrix-tools#143。

- 杂务(deps)：依赖地板跟到矩阵本轮新版本——guardx v0.14.0 / mcpkit v0.8.6 / concurrency v0.20.2 / pkg v0.22.0

## 下游要做什么
**`go mod tidy`**。地板抬高对下游是**隐式破坏性变更**：不跑就报
`updates to go.mod needed`，且报错不提真正原因（**不是本仓的 API 变了**）。
本仓**自身代码零改动**——只看本仓的 diff 看不出这是一次地板抬高。
## 为什么抬
本仓 HEAD（`1991f00`，2026-10-21T12:01:45Z）**早于**下面四个版本的发布时刻，
所以宽口径判据（`verify_tag_floor.py --repo toolgate --ref HEAD`）此刻仍判绿；
但用门禁自己的 `check_ref()` 把 ref 时刻推进到现在 ⇒ **4 处红**：
- github.com/oagentkit/guardx: v0.13.1 → v0.14.0（2026-10-22T09:46:52Z 发布）
- github.com/oagentkit/mcpkit: v0.8.5 → v0.8.6（2026-10-23T05:54:01Z 发布）
- github.com/oekit/concurrency: v0.19.0 → v0.20.2（2026-10-23T03:09:14Z 发布）
- github.com/oekit/pkg: v0.20.0 → v0.22.0（2026-10-22T09:18:33Z 发布）
⇒ **下一次发版会把「打 tag 那一刻就已落后」的地板发出去**，而 tag 受 protect-tags
ruleset 保护不可重打 ⇒ 坏 tag 流出后只能补发新 tag，且 `--latest-only` 口径的 CI
届时会当场判红（它审的就是「最新 tag 当时的地板」）。
门禁给的修法是「抬 go.mod 地板 → go mod tidy → 重新发 tag」。本 PR 就是那条修法。
## 为什么无行为差异
逐条 diff 两版的实测差异，**四条对本仓都是行为中性**：
- `guardx v0.14.0`：只改 `permgate/permgate.go`（新增「只读拦截」段）——本仓只 import
  `guardx/invoke` + `guardx/toolfail`，未命中；
- `mcpkit v0.8.6`：零 `.go` 改动；
- `oekit/concurrency v0.20.2`：只改 `syncx/gate.go` + `syncx/syncx.go`
  （`Gate.TryAcquire` 新增、`KeyedGate` 幂等标记改 `atomic.Bool`+CAS）——本仓只 import
  `concurrency/async`，未命中；
- `oekit/pkg v0.22.0`：只新增 `pathx/*` 与 `stringx/csv.go`（`status: added`），
  既有函数逐字未改——本仓不 import `pathx` / `stringx`。
本仓用到的转发面与符号集一字未动。
## 门禁
```
go build ./...                 ✅
go test -race ./...            ✅ (policy / toolprior / toolsched)
go mod tidy -diff              ✅ 空
gofmt -l .                     ✅ 空
```

- 杂务(deps)：依赖地板跟到矩阵本轮新版本——mcpkit v0.8.5

## 下游要做什么
**`go mod tidy`**。地板抬高对下游是**隐式破坏性变更**：不跑就报
`updates to go.mod needed`，且报错不提真正原因（**不是本仓的 API 变了**）。
本仓**自身代码零改动**——只看本仓的 diff 看不出这是一次地板抬高。
## 为什么抬
上一版 tag 打出去时，它依赖的模块的**新版已经先发出**了——
即「打 tag 那一刻地板就落后」。矩阵的「发版时刻地板时效」门禁
（`verify_tag_floor.py --org oagentkit --latest-only`）当场报红：
- github.com/oagentkit/mcpkit@v0.8.5
门禁给的修法是「抬 go.mod 地板 → go mod tidy → 重新发 tag」。
本 PR 就是那条修法。
## 为什么无行为差异
被抬的这几个版本**自身也都是纯依赖地板追平**（各仓代码零改动，只抬
`oekit/pkg` / `oekit/concurrency` / `guardx` / `llm` 四类地板之一），
本仓用到的转发面与符号集一字未动。
## 门禁
```
go build / vet / test -race   ✅
gofmt -l .                    ✅ 空
```

- 杂务(deps)：依赖地板跟到矩阵本轮新版本（第二轮：消除打 tag 时刻的地板落后）

* chore(deps): 依赖地板跟到矩阵本轮新版本——mcpkit v0.8.4 runtime v0.7.2
## 下游要做什么
**`go mod tidy`**。地板抬高对下游是**隐式破坏性变更**：不跑就报
`updates to go.mod needed`，且报错不提真正原因（**不是本仓的 API 变了**）。
本仓**自身代码零改动**——只看本仓的 diff 看不出这是一次地板抬高。
## 为什么抬
上一版 tag 打出去时，它依赖的模块的**新版已经先发出**了——
即「打 tag 那一刻地板就落后」。矩阵的「发版时刻地板时效」门禁
（`verify_tag_floor.py --org oagentkit --latest-only`）当场报红：
- github.com/oagentkit/mcpkit@v0.8.4
- github.com/oagentkit/runtime@v0.7.2
门禁给的修法是「抬 go.mod 地板 → go mod tidy → 重新发 tag」。
本 PR 就是那条修法。
## 为什么无行为差异
被抬的这几个版本**自身也都是纯依赖地板追平**（各仓代码零改动，只抬
`oekit/pkg` / `oekit/concurrency` / `guardx` / `llm` 四类地板之一），
本仓用到的转发面与符号集一字未动。
## 门禁
```
go build / vet / test -race   ✅
gofmt -l .                    ✅ 空
```
* docs(changelog): 重生成 CHANGELOG.md——补回浅克隆期间漏掉的条目
上一版 tag（v0.12.1）的 CHANGELOG 段**漏了 5 条提交**：本地克隆是浅克隆
（depth 限制），生成器遍历 tag 区间时看不到更早的提交，于是那几条被漏掉。
本次重生成（本地已补齐完整历史）把 v0.12.1 段补回，并预写 v0.12.2 待发版段
（依赖地板跟到 mcpkit v0.8.4、runtime v0.7.2）。
**为什么必须修**：下游 `go get` 拿到的是 tag 快照——快照里的 CHANGELOG 若
缺条目，发布说明就是残缺的（矩阵的 CHANGELOG 覆盖门禁正是查这个）。

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

- 杂务(deps)：oagentkit/guardx 依赖地板抬到 v0.9.0

guardx v0.9.0 新增了 toolfail 与 sandbox 两个 tier0 原语，
是**新增能力**而非 API 变更——本仓无需改代码，只抬地板。
为什么现在补：guardx v0.9.0 是 2026-10-21 发布的，本仓上一个 tag 早于它，
于是这一格欠账一直挂着（依赖地板报告器把「子仓发版让下游落后」显式化，
但报告器恒不失败——真正的硬约束是「打 tag 时刻地板必须是当时的最新」）。
本地门禁：go mod tidy 无差异 / go build / go vet 通过。

- 杂务(deps)：oagentkit/mcpkit 依赖地板抬到 v0.8.3

* docs(AGENTS): 补「发版顺序」小节，指向 org CONTRIBUTING 单一副本
13 个业务仓的 AGENTS.md 此前对发版顺序零覆盖（只有 textx 有个
「### 版本与发版」标题，但里面只讲 SemVer 与 module 路径），
发版顺序只写在 org 规则仓的 .github/CONTRIBUTING.md 里，单仓读者看不到。
本次只加**指针 + 本仓相关提醒**，runbook 本体保持**单一副本**
（CONTRIBUTING §发版），避免 13 份副本各自漂移。
覆盖四条：
- CHANGELOG.md 由 matrix-tools/scripts/gen_changelogs.py 从 git 历史生成，不要手改
- tag 必须落在 CHANGELOG 提交之后（下游 go get 拿到的是 tag 快照）
- 警告：无参重生成会吞掉待发版段，--next-version 是唯一能产出待发版段的开关
- 本仓相关提醒（见正文）
* chore(deps): oagentkit/mcpkit 依赖地板抬到 v0.8.3
mcpkit v0.8.2 → v0.8.3 是**纯依赖地板抬升**（零 .go 改动，只动了 go.mod/go.sum
与 CHANGELOG），本仓无需改代码。
为什么现在补：矩阵的依赖地板是「会漂」的常态——任何一次子仓发版都会让下游落后。
本仓的 mcpkit v0.8.3 是 2026-10-20 发布的，而本仓 v0.12.0 打 tag 在那之前，
于是这一格欠账一直挂着。补齐它，本次全矩阵的第一方依赖地板达成零落后
（此前唯一剩下的两处是 agent→llm 与 toolgate→mcpkit）。
mcpkit v0.8.2 → v0.8.3 的内容：oekit 三个基座抬到 v0.18.0 +
矩阵内跨仓依赖抬到各仓最新 tag，无 API 变更。
本地门禁：go mod tidy 无差异 / go build / go vet 通过。

- 杂务(deps)：oekit 依赖地板抬到 v0.18.0

oekit 姊妹矩阵的依赖地板对齐到最新：
- github.com/oekit/pkg        v0.17.0 → v0.18.0
- github.com/oekit/concurrency v0.17.1 → v0.18.0
- github.com/oekit/logx       已是最新 v0.7.1，不动
oekit v0.18.0 的两个发布都是**纯增量**，无破坏性变更：
- pkg：jsonrepair 补抽数组原语 ExtractArray；
- concurrency：新增 `DoValue[T]`（带返回值的重试），`Do` 改为委托给它——
  退避序列、抖动、MaxDelay 封顶、DelayHint 优先、Retryable 门控、ctx 取消
  **只有一份实现**（此前这类调用方只能自己重写一遍退避，且常丢掉 DelayHint 语义）。
依据 oagentkit/matrix-tools#44 的还债流程：依赖地板对齐到发版时刻的最新。
本仓本轮不发 tag（不在发版集内），地板在 main 上就位，下次发版自然带上。

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

- 杂务(deps)：依赖地板对齐远端最新（mcpkit v0.6.0 / runtime v0.6.0 / pkg v0.15.1 / concurrency v0.17.1）

四处漂移与本仓当前钉住的版本相比源码零改动，逐版实测确认：
- mcpkit v0.5.1 → v0.6.0       全树逐字节相同（diff -rq 退出码 0）
- runtime v0.5.0 → v0.6.0      全树逐字节相同（退出码 0）
- oekit/pkg v0.14.0 → v0.15.1  仅新增 urlguard 包（纯标准库，pkg 的 go.mod
                              连 require 块都没有，零新增三方依赖）
- oekit/concurrency v0.17.0 → v0.17.1 仅三个文档文件，无 .go 改动
下游 agent-harness 不必再靠手写 require 顶 mcpkit 地板。
本地门禁：go mod tidy -diff 无输出 / go build / go vet / go test -race 全通过。


- 文档(toolgate)：刷正依赖清单——补上 invoke 迁出后新增的 guardx

本仓 go.mod 的矩阵内**直接**依赖是 guardx 与 mcpkit 两条，而 README.md 与
AGENTS.md 的「本仓依赖的矩阵内仓」清单都只列了 mcpkit，漏了 guardx。
guardx 依赖是 5c4922918524e9f446bf593fd04b25c2185b1d97
（refactor!: invoke 迁出本仓 —— 框架无关的原语不该被 eino 重仓的地板拖住）
带进来的：invoke 迁到 github.com/oagentkit/guardx/invoke，本仓直接调用它，
go.mod 里那行 `require github.com/oagentkit/guardx v0.14.0` **没有** // indirect 标记
⇒ 是直接依赖。
这行漏列尤其危险：本仓 AGENTS.md 的核心不变量「新增门禁一律加在 invoke 链上」
依赖的正是这条 guardx 链路，而清单里没有 guardx，读的人会以为本仓不依赖它。
清单口径 = go.mod 的「直接」矩阵内依赖（// indirect 不列）——
本仓 go.mod 里 `github.com/oagentkit/runtime v0.7.2 // indirect` 是被 mcpkit
拉进来的，不列，与 modules.json 的 toolgate.dependsOn = ["guardx","mcpkit"] 同口径。
关联 oagentkit/matrix-tools#127

- 文档(toolgate)：刷正 README 的矩阵内依赖清单 —— 与 go.mod 实测对账

本仓 README 的「矩阵内依赖」清单与**各仓 go.mod 的事实**对不上。
矩阵的单一数据源 `matrix-tools/modules.json` 也有同样漂移，已在
[matrix-tools#126](https://github.com/oagentkit/matrix-tools/pull/126) 同步刷正。
## 现状核实
```bash
$ grep -rn "oagentkit/guardx" toolgate/ --include=*.go | grep -v _test.go
toolgate/policy/failclass.go:45:	"github.com/oagentkit/guardx/toolfail"
toolgate/policy/stage.go:6:	"github.com/oagentkit/guardx/invoke"
```
⇒ `guardx` 是**生产代码**的直接依赖（不是测试依赖），清单里漏了它。
`modules.json` 的 `dependsOn` 本来就写了 `["guardx", "mcpkit"]` ⇒ **数据源是对的，是 README 漏了**。
## 影响
`guardx` 提供 `invoke`（调用链构造）与 `toolfail`（失败分类）—— 本仓的「工具调用链」纪律就建在它上面。读者按 README 只看到 mcpkit ⇒ 看不到这条依赖方向。
## 改动
**只改清单那一行（`- 本仓依赖的矩阵内仓：`mcpkit`。` ⇒ `- 本仓依赖的矩阵内仓：`guardx`、`mcpkit`。`）**，其余段落逐字未动。
## 验收
- [x] 清单与 `go.mod` 实测一致
- [x] 与 `matrix-tools/modules.json` 的登记一致（同期在 matrix-tools#126 刷正）
- [x] **纯 .md 改动** ⇒ 不需要 CHANGELOG 条目、不打 tag、不发版（无发版步骤）
关联 oagentkit/matrix-tools#125

- 文档(AGENTS)：补「发版顺序」小节，指向 org CONTRIBUTING 单一副本

13 个业务仓的 AGENTS.md 此前对发版顺序零覆盖（只有 textx 有个
「### 版本与发版」标题，但里面只讲 SemVer 与 module 路径），
发版顺序只写在 org 规则仓的 .github/CONTRIBUTING.md 里，单仓读者看不到。
本次只加**指针 + 本仓相关提醒**，runbook 本体保持**单一副本**
（CONTRIBUTING §发版），避免 13 份副本各自漂移。
覆盖四条：
- CHANGELOG.md 由 matrix-tools/scripts/gen_changelogs.py 从 git 历史生成，不要手改
- tag 必须落在 CHANGELOG 提交之后（下游 go get 拿到的是 tag 快照）
- 警告：无参重生成会吞掉待发版段，--next-version 是唯一能产出待发版段的开关
- 本仓相关提醒（见正文）

- 文档(changelog)：按新生成器规则重生成 CHANGELOG.md

生成器修复（oagentkit/matrix-tools#43）落地：
① `[round-N] ` 前缀被剥离 → conventional commit 解析不再失效，
   **commit body 保留**（此前整段被丢弃，发布说明只剩标题）；
② 非 conventional 提交也保留 body；
③ `--check` 识别「待发版段」（预写了但还没打 tag 的那段）。
本 PR 恢复的是**本就在 commit body 里**的正文（迁移步骤、代码块、
判据锚点）——此前被 `[round-N] ` 前缀导致的解析失败丢弃了。
**没有任何发布说明内容被删除**：删除的行都能在新增行里找到对应
（规范化掉 `(#N)` 与 `[round-N] ` 前缀后逐行比对）。
关联：oagentkit/matrix-tools#43、oagentkit/matrix-tools#40


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
