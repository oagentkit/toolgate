# AGENTS.md — oagentkit/toolgate

> 工具治理——优先级三层约束/注解驱动并发调度/操作审计门
> module：`github.com/oagentkit/toolgate` · org：[oagentkit = Open Agent Kit](https://github.com/oagentkit)
> 矩阵分层：**tier 2**（集成层，依赖 tier0/tier1 + 重第三方地板）

## 本仓包含

```
`toolprior/`
`toolsched/`
`policy/`
```

## 硬规则

### 分层与依赖

- 依赖严格自底向上，**禁止反向依赖**。本仓是 tier 2，只允许依赖 tier ≤ 2 的仓。
- 本仓依赖的矩阵内仓：`guardx`、`mcpkit`。
- 改 module 路径必须同步：全部 import、go.mod、README 安装行。

### 错误处理

- 一律使用 `github.com/oekit/pkg/errors`；业务错误用 BizError。
- 所有 error 禁止 `_` 忽略。

### 并发安全

- 禁止裸 `go func`：用 `github.com/oekit/concurrency/async` 的 GoSafe 系列（panic 被 recover）。
- 共享状态用 mutex / atomic 保护；包级全局可变状态用 `atomic.Pointer[T]`。

### API 设计

- 函数选项模式命名 `XxxOption`（禁止裸 `Option`，多包 co-import 会冲突）。
- 导出符号 PascalCase + godoc 注释；包文档放 `doc.go`。

### 日志

- 日志走 `oekit/logx`，组件 logger 用 `logx.Delegating("oagentkit-toolgate")`。
- 被拒的工具调用属审计事件，字段需带工具名、调用者、拒绝原因。

### 发版顺序

本仓的 `CHANGELOG.md` 由 `matrix-tools/scripts/gen_changelogs.py` 从 git 历史生成，
**不要手改**。发版顺序的**单一副本**见 org 规则仓
[CONTRIBUTING §发版](https://github.com/oagentkit/.github/blob/main/CONTRIBUTING.md#发版)：
**预写待发版段 → 提交 CHANGELOG.md → 在那个提交上打 annotated tag**。
tag 必须落在 CHANGELOG 提交**之后** —— 下游 `go get` 拿到的是 tag 快照，
快照里的 CHANGELOG.md 若缺自己那一段，发布说明就是残缺的。

⚠️ 无参重生成 CHANGELOG 会**吞掉待发版段**：生成器只按已存在的 tag 切段，
`--next-version` 是唯一能产出待发版段的开关。**预写之后、打 tag 之前，
不要跑无参重生成。**

打 tag 前自检依赖地板：内部 `require` 要钉在**打 tag 那一刻**各依赖仓的最新 tag
（`matrix-tools/scripts/verify_tag_floor.py`）。本仓已有 1 个 CHANGELOG-at-tag 欠账
（`toolgate@v0.12.0`）—— **反面例子，不是范例**。

## 仓特有纪律

- **失败分类的映射只有一份，在 `policy` 包里**（`FailureClass` / `Disposition`）：
  协议层 `mcpkit/mcp.ErrorKind` → 处置层 `guardx/toolfail.Class`。**下游不得各写各的映射表**——
  同一类失败在 A 处判成「可重试」、在 B 处判成「永久拒绝」，重试策略随之瞎掉。
  两层结论不一致时取**不可重试**的一侧（误判可重试会把一次失败放大成 N 次）。
  新增 `ErrorKind` 必须同时补映射表与用例——`TestFailureClassCoversEveryErrorKind` 会当场红。
- 优先级三层约束是**收紧**语义：上层只能进一步限制，**不得放宽**下层已设的限制。
- 并发调度的去重键必须包含全部影响结果的参数，否则会错误合并不等价调用。
- **新增门禁一律加在 `invoke` 链上，不要再加装饰器。**（`invoke` 已迁至
  `github.com/oagentkit/guardx/invoke` —— 它是框架无关的原语，不该被 eino 重仓的地板拖住。）
  装饰器形态下「宿主忘了包」
  是个不报错、不告警、事后也查不出来的失效形态：`policy.AuditGate` 保留是为了
  兼容既有调用点，但新代码的审计门应挂进 `invoke` 链——那里绕不过去。
- **门禁链的不可绕过性靠类型保证，不靠约定**：`NewChain` 把 Handler 按值收进私有
  字段且不提供取回方法。改动本包时若加了任何能把执行体或 Stage 交出去的出口
  （`Handler()`、`Stage()` 之类），这条不变量即破，须同步改测试并重新论证。
- `Stage` 返回 error 时按**拒绝**处理，不得当作「这环管不着、跳过」——后者等于
  把一次故障静默降级成不设防。
- **要复用判定环就调 `guardx/invoke.EvaluateStages`，不要自己再排一遍环。** 环有两份排法就会
  静默漂移：panic 归口用的 `stageInternal` 是包内未导出常量，下游既不能引用也不能可靠
  复制（硬编码字符串等于与实现耦合），漏掉的后果是「这环自己崩了」被谎报成「这环正常
  拒绝了」——调用方于是去修策略，而真正该修的是守卫代码。最容易踩的场景是程序化编排
  （模型写的程序在沙箱里跑、代码内 `host.<tool>()` 调宿主工具）：执行体必须跑在宿主绑定
  闭包里，链用不上，于是「顺手再排一遍」就成了默认选项。`Chain.Do` 自己也走这个入口。

## 明确不属于本仓的

| 能力 | 归属 |
|---|---|
| 文本/差异/会话/报告 | `github.com/oagentkit/textx` |
| 事件总线/流控/子进程/热插拔/钩子 | `github.com/oagentkit/runtime` |
| 脱敏/出口围栏/权限/沙箱解析/审计 | `github.com/oagentkit/guardx` |
| DB 队列与选主 | `github.com/oagentkit/queue` |
| 通用 errors/stringx/encoding/mathx | `github.com/oekit/pkg` |
| goroutine 池/单飞/重试/令牌桶 | `github.com/oekit/concurrency` |
