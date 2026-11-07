// Package invoke 是已搬迁包的 Deprecated 转发壳：只保编译兼容，不含任何第二份实现。
//
// # 为什么有这个壳
//
// toolgate v0.12.6 起，原 invoke 子包整体搬至 github.com/oagentkit/guardx/invoke
// （guardx@v0.19.0 起独立存在），旧 import 路径直接断裂且零转发——升级即
// undefined，且错误信息只说旧路径消失、不说新家在哪（见 oagentkit/agent#68）。
// 本壳逐符号别名转发 guardx/invoke 的全部导出符号，让旧调用方的
// `github.com/oagentkit/toolgate/invoke` 恢复编译；guardx 本就是本仓的
// 矩阵内依赖，转发零架构成本。
//
// # 只转发，不改语义
//
// 本壳不复制任何逻辑：类型是别名（同型），哨兵错误与函数是同一实体。
// guardx v0.20.0 起 Record.Allowed 的语义变更为「到达执行体即置位，与执行成败
// 无关」（guardx#73）——壳原样转发该语义，不做任何兼容性改写。
//
// # 搬迁公告
//
// 本仓 CHANGELOG 由生成器从 git 历史生成、不可手改，故 issue #68 建议的三行
// 固定格式记于此处（并随本提交的 commit body 一并收录进生成产物）：
//
//	MovedFrom: github.com/oagentkit/toolgate/invoke
//	MovedTo: github.com/oagentkit/guardx/invoke
//	兼容期截止版本: v0.16.0（壳随 v0.15.0 提供，保一个完整 minor）
//
// Deprecated: 已迁至 github.com/oagentkit/guardx/invoke，本壳只保编译兼容，
// 下个 minor（v0.16.0）删除。新代码请直接 import 新路径。
package invoke
