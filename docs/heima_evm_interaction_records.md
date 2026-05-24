# Heima EVM 智能合约交互记录调查

调查日期：2026-05-25

## 结论

当前状态：部分支持。

Heima 线上 API 已经索引 EVM 合约、普通 EVM 交易和交易 receipt logs。现有产品也已经有合约详情页、地址详情页、交易列表页和交易详情页，所以“某个合约有哪些直接 EVM 交易交互”可以查看。

但这还不是完整的智能合约交互记录能力。当前前端没有 raw event/log 页签、没有 internal transaction/trace 入口，也没有在交易详情里展示 ABI 解码后的 method call、event name 或参数。对于未验证合约，线上 API 返回的 `abi`、`method_identifiers`、`event_identifiers` 也为空，因此页面只能展示交易哈希、from/to、value、input data 等低层字段。

## 最佳线上证据

线上页面入口：

- 合约页：`https://test-explorer.heima.network/contract/0xeb9c31afbe1bc3cfbb218f554148b456095def9b`
- 交易页：`https://test-explorer.heima.network/tx/0x2e2f768ca1ffc8a5d53fc6dcd7e4af36af02c29d762ecc0f1ecbf2825d70bb6d`

线上 API 样本：

- `POST https://explorer-api.heima.network/api/scan/metadata` 返回 `enable_evm: true`、`total_evm_contract: "22"`、`total_transaction: "2106"`。
- `POST https://explorer-api.heima.network/api/plugin/evm/contracts` 返回合约 `0xeb9c31afbe1bc3cfbb218f554148b456095def9b`，`transaction_count: 46`。
- `POST https://explorer-api.heima.network/api/plugin/evm/transactions` 加 `address: "0xeb9c31afbe1bc3cfbb218f554148b456095def9b"` 返回多条真实交互交易，例如 `0x2e2f768ca1ffc8a5d53fc6dcd7e4af36af02c29d762ecc0f1ecbf2825d70bb6d`。
- `POST https://explorer-api.heima.network/api/plugin/evm/transaction` 查询该 hash 返回 `to_address: "0xeb9c31afbe1bc3cfbb218f554148b456095def9b"`、`input_data: "0x28d3a294..."`、`success: true`、`extrinsic_index: "9649159-2"`。
- `GET https://explorer-api.heima.network/api/plugin/evm/etherscan?module=logs&action=getLogs&address=0xeb9c31afbe1bc3cfbb218f554148b456095def9b&page=1&offset=10` 返回多条 receipt log，说明数据库/API 层有 raw event/log 数据。

## 产品和代码层面

已存在入口：

- `ui-react/src/pages/contract/[id].tsx`：合约详情页，包含 `Contract`、`Transactions`、`ERC-20 Transfers`、`ERC-721 Transfers` 页签。
- `ui-react/src/pages/address/[id].tsx`：地址详情页，包含 token、交易和 transfer 页签。
- `ui-react/src/pages/tx/[id].tsx`：EVM 交易详情页，展示 hash、from、to、value、result、nonce、input data、fee 和 signature。
- `ui-react/src/components/tx/txTable.tsx`：EVM 交易表，按地址或区块过滤。

已存在 API/索引能力：

- `plugins/evm/http/http.go` 注册了 `transactions`、`transaction`、`contracts`、`contract`、`token/transfer`、`etherscan` 等 EVM API。
- `plugins/evm/dao/transaction.go` 在扫描 EVM block transaction 时调用 `eth_getTransactionReceipt`，写入 `evm_transactions` 和 `evm_transaction_receipts`。
- `plugins/evm/dao/api.go` 的 `API_GetLogs` 从 `evm_transaction_receipts` 输出 Etherscan-compatible logs。
- `plugins/evm/dao/event.go` 会对部分 receipt event 做 token/proxy 后处理，例如 ERC-20/ERC-721 Transfer 和 proxy Upgraded。

## 缺失层级和上游原因

前端层：

- 缺少合约详情页的 raw `Logs / Events` 页签。
- 缺少交易详情页的 receipt logs、decoded method call 和 decoded event 参数展示。
- 缺少 internal transaction/trace 页签或入口。

API 层：

- 已有 Etherscan-compatible `logs/getLogs` raw log API，但没有被当前 React 页面消费。
- `txlistinternal` 请求当前返回 HTTP 200 且 body 为空，未形成可展示的 internal transaction 数据契约。
- `API_Transactions` 的 `FunctionName` 仍是空字符串，当前 API 没有把 method selector 映射成可读函数名输出给页面。

数据库层：

- 已有 `evm_transactions` 和 `evm_transaction_receipts`，能够支撑直接交易列表和 raw event/log。
- 未看到 internal transaction/trace 专用表；当前 schema 不能支撑合约内部调用树展示。
- 示例合约 `0xeb9c31afbe1bc3cfbb218f554148b456095def9b` 未验证，线上 `abi`、`method_identifiers`、`event_identifiers` 均为空，限制了 method/event 解码。

扫描解析层：

- 扫描器已经解析 block transaction 和 receipt logs。
- 扫描器没有显式采集 `debug_traceTransaction`、`trace_*` 或等价 internal call 数据。
- receipt log 后处理目前聚焦 token/proxy 特定事件，不是通用 ABI event 解码流水线。

## 最小补齐路径

前端：

1. 在合约详情页增加 `Logs / Events` 页签，消费现有 `etherscan?module=logs&action=getLogs&address=...`。
2. 在交易详情页增加 receipt logs 区块，并展示 topic、data、log index、contract address。
3. 对已验证合约增加 method/event 名称和参数展示；未验证合约保留 raw input/topic/data。

API：

1. 提供 UI-native 的 `POST /api/plugin/evm/logs` 或在前端规范化消费现有 Etherscan GET logs。
2. 在 `transactions` 和 `transaction` 返回值中补充 method selector、已知 method name 和 decode 状态。
3. 明确定义 `internal transactions` API；在没有 trace 数据前返回结构化空结果，而不是空 body。

数据库：

1. 复用 `evm_transaction_receipts` 支撑 raw logs。
2. 增加 method/event ABI 映射缓存与按合约、topic、selector 的查询索引。
3. 如需 internal tx，新增 trace/internal call 表，至少记录 tx hash、trace path、from、to、value、input、output、call type、status/error。

扫描解析：

1. 继续保存 `eth_getTransactionReceipt` logs。
2. 对已验证合约按 ABI 解码 input 和 logs。
3. 如链节点支持 trace/debug RPC，新增 internal call 采集任务；如不支持，需要先确认 Heima 节点侧 RPC 能力。
