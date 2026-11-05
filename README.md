# toolgate

工具治理——优先级三层约束/注解驱动并发调度/操作审计门 —— oagentkit 模块矩阵 **tier 2**（集成层，依赖 tier0/tier1 + 重第三方地板）。

```
go get github.com/oagentkit/toolgate
```

| 包 | 职责 |
|---|---|
| `toolprior` | 工具优先级三层约束（收紧语义，下层已设的限制不得放宽） |
| `toolsched` | 注解驱动的工具并发调度（依赖分层 + 并行切组；**不做语义去重**——见 toolsched 包注释） |
| `policy` | 操作审计门：敏感工具调用的策略判定；协议层失败分类 → 处置层类别的机械映射（`mcpkit` 的 `ErrorKind` → `guardx/toolfail` 的 `Class`） |

## 定位

**tier 2** — 集成层，依赖 tier0/tier1 + 重第三方地板。

- 本仓依赖的矩阵内仓：`guardx`、`mcpkit`。

## 外部依赖地板

- `github.com/cloudwego/eino`
- `github.com/oekit/pkg`
- `gopkg.in/yaml.v3`

## 与 oekit 的关系

本矩阵（oagentkit，Agent/LLM 编排工具链）**依赖** oekit（Open Easy Kit，Go 微服务工具链）：
`oekit/pkg` / `oekit/concurrency` / `oekit/logx` 等。oekit 是另一套独立矩阵，不是子品牌。

## 开发

```bash
go test ./... -race
go vet ./...
gofmt -l .
```

规范见 [AGENTS.md](AGENTS.md)。

## License

MIT
- 优先级三层约束是**收紧**语义：上层只能进一步限制，**不得放宽**下层已设的限制。
- 并发调度**不做语义去重**（oagentkit/toolgate#41）：`Call` 只有 ID/Name/DependsOn/Hints，
  执行体参数活在 `Task.Run` 闭包里，调度层没有「name+归一化参数」去重键可算——
  README/AGENTS 此前宣称「与去重」是对标源语义在落地时丢失后的残留承诺。
  语义重复调用的合并由宿主在装配前自做（按 Call.ID 去重是调用方职责）。
- `invoke` 链（已迁至 `github.com/oagentkit/guardx/invoke`）拿到就绕不过去：`NewChain` 把 Handler 按值收进私有字段且不提供取回
  方法，因此宿主无法拿到裸执行体再绕过门禁。**这条是类型保证的，不是约定**——
  新增能交出执行体的出口即破此不变量。
- `Stage` 返回 error 按拒绝处理；构造期与执行期一律 fail-closed。
- 要复用判定环（例如程序化编排里代码内的 `host.<tool>()` 调用）调
  `guardx/invoke.EvaluateStages`，**不要自己再排一遍环**：环排两份会静默漂移，panic 归口只应
  存在于本仓一处。`Chain.Do` 自己也走这个入口。