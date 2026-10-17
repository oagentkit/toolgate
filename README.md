# toolgate

工具治理——优先级三层约束/注解驱动并发调度/操作审计门 —— oagentkit 模块矩阵 **tier 2**（集成层，依赖 tier0/tier1 + 重第三方地板）。

```
go get github.com/oagentkit/toolgate
```

| 包 | 职责 |
|---|---|
| `toolprior` | 工具优先级三层约束（收紧语义，下层已设的限制不得放宽） |
| `toolsched` | 注解驱动的工具并发调度与去重 |
| `policy` | 操作审计门：敏感工具调用的策略判定 |

## 定位

**tier 2** — 集成层，依赖 tier0/tier1 + 重第三方地板。

- 本仓依赖的矩阵内仓：`mcpkit`。

## 外部依赖地板

- `github.com/cloudwego/eino`
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
- 并发调度的去重键必须包含全部影响结果的参数，否则会错误合并不等价调用。