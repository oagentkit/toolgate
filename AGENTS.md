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
- 本仓依赖的矩阵内仓：`mcpkit`。
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

## 仓特有纪律

- 优先级三层约束是**收紧**语义：上层只能进一步限制，**不得放宽**下层已设的限制。
- 并发调度的去重键必须包含全部影响结果的参数，否则会错误合并不等价调用。

## 明确不属于本仓的

| 能力 | 归属 |
|---|---|
| 文本/差异/会话/报告 | `github.com/oagentkit/textx` |
| 事件总线/流控/子进程/热插拔/钩子 | `github.com/oagentkit/runtime` |
| 脱敏/出口围栏/权限/沙箱解析/审计 | `github.com/oagentkit/guardx` |
| DB 队列与选主 | `github.com/oagentkit/queue` |
| 通用 errors/stringx/encoding/mathx | `github.com/oekit/pkg` |
| goroutine 池/单飞/重试/令牌桶 | `github.com/oekit/concurrency` |
