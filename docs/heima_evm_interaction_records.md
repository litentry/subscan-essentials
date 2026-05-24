# Heima EVM 智能合约交互记录调查

调查日期：2026-05-25

关联任务：GitHub #27

## 结论

当前状态：**部分支持，但当前可访问的线上浏览器不能完成公开验证**。

代码层面已经具备 EVM 合约、地址、交易列表、交易详情、ERC-20/721 transfer 和 Etherscan-style logs API 的基础能力；但是当前产品 UI 只能直接展示普通交易列表、token transfer 和交易 raw input。它还没有在合约/交易详情页展示完整的 contract interaction 视图，例如 decoded method call、event/log tab、internal transaction、trace/call tree 或基于 ABI 的参数解析。

线上层面，`https://test-explorer.heima.network/` 当前在未登录浏览器和同源 API 请求中返回 Vercel Authentication，不是 Heima explorer 页面。因此本次无法用公开线上页面证明“页面可以看到该合约已有真实交互记录”。但 Heima RPC 上可以确认同一合约确实存在真实链上交互和 logs。

## 最佳线上证据

目标合约：

```text
0x63c4545ac01c77cc74044f25b8edea3880224577
```

目标页面：

```text
https://test-explorer.heima.network/contract/0x63c4545ac01c77cc74044f25b8edea3880224577
```

当前公开浏览器访问结果：

```text
HTTP 401
Vercel Authentication
```

Heima RPC 真实交互证据：

```text
RPC: https://rpc.heima-parachain.heima.network
method: eth_getLogs
address: 0x63c4545ac01c77cc74044f25b8edea3880224577
topic0: 0x4e21321d01571fa35038552651b1fd51fdb2935e1c8566378607aecf7fa70919
result_count: 14
```

代表交易：

```text
tx: 0xcc5dfeac4a35f3850a5684003e4449508333d7cd8ffa200dcf747d39748f8725
block: 0x92dea9
from: 0xde644936d5b7d5d42032fd08bba42fbbfd6663bc
to: 0x63c4545ac01c77cc74044f25b8edea3880224577
status: 0x1
log_count_in_receipt: 1
```

该交易的 `eth_getTransactionReceipt` 返回一条 log，地址为上述合约，topic0 为 `AuditAppended(bytes32,bytes32,bytes32,uint8,uint256,bytes32)` 对应事件 topic。

## 代码/产品能力确认

前端已有入口：

- 合约详情页：`ui-react/src/pages/contract/[id].tsx`
  - 读取 `/api/plugin/evm/contract`
  - 展示 Contract/Transactions/ERC-20 Transfers/ERC-721 Transfers tabs
- 地址详情页：`ui-react/src/pages/address/[id].tsx`
  - 展示 ERC-20 Tokens/ERC-721 Tokens/Transactions/Transfers tabs
- 交易详情页：`ui-react/src/pages/tx/[id].tsx`
  - 展示 EVM transaction 基础字段、raw input data、gas、signature

后端已有入口：

- `/api/plugin/evm/contract`
- `/api/plugin/evm/contracts`
- `/api/plugin/evm/transaction`
- `/api/plugin/evm/transactions`
- `/api/plugin/evm/token/transfer`
- `/api/plugin/evm/etherscan?module=logs&action=getLogs`

扫描/索引层已有部分能力：

- `CreateTransactionByExecuted` 会调用 `eth_getTransactionReceipt`。
- receipt logs 会写入 `evm_transaction_receipts`。
- receipt 记录保存 `address`、`topics`、`method_hash`、`topic1`、`topic2`、`topic3`、`data`、`transaction_hash`、`block_num`。
- verified ABI 会写入 `evm_abi_mappings`，可为后续 method/event decode 提供基础。

## 缺口定位

前端缺口：

- 合约页没有 Event/Logs tab。
- 交易详情页没有展示 receipt logs。
- 交易列表没有 method name/function name。
- 没有 internal transaction/trace 视图。
- 没有 ABI decode 后的 method call 参数和 event 参数展示。

API 缺口：

- native `/api/plugin/evm/transaction` 只返回交易本体，不返回 receipt logs、decoded input、decoded events 或 internal transactions。
- native `/api/plugin/evm/transactions` 支持按 `from_address`/`to_address` 过滤，但不按 receipt log address 回查“和该合约相关”的所有事件型交互。
- Etherscan-style `logs-getLogs` 存在，但当前 UI 没有消费它。
- `account-txlistinternal` 在 etherscan handler 中仍是 TODO。

数据库缺口：

- 已有 `evm_transactions` 和 `evm_transaction_receipts`，可以保存普通交易和 logs。
- 未看到 internal transaction/trace 表。
- method/event decode 依赖 ABI；未验证合约或缺 ABI 时只能展示 method id、topics 和 raw data。

扫描解析缺口：

- 普通交易和 receipt logs 的扫描路径存在。
- 目前没有看到 trace/internal tx 采集。
- 事件参数 decode 没有成为面向 UI 的稳定查询结果。

线上环境缺口：

- 未认证访问 `test-explorer.heima.network` 返回 Vercel Authentication，无法从公开浏览器路径证明 UI 可查看真实交互记录。
- 同源 `/api/...` 也被同一 Vercel auth 层拦截，无法直接验证 Heima explorer API 的数据库返回。
- Heima RPC 证明链上数据存在，因此“没有交互”的判断不成立；缺口在 explorer 可访问性、API/DB 可验证性和 UI 展示完整度。

## 最小补齐路径

前端：

1. 在合约详情页新增 `Events / Logs` tab，按 contract address 调用 logs API。
2. 在交易详情页展示 receipt logs。
3. 对已验证 ABI 的合约展示 decoded method name、input 参数、event name 和 event 参数；无 ABI 时展示 raw method id/topics/data。
4. 后续再增加 internal transactions tab。

API：

1. 扩展 `/api/plugin/evm/transaction`，返回 `logs`、`method_id`、可选 `decoded_input`。
2. 增加 native `/api/plugin/evm/logs` 或在现有 transaction/contract API 中聚合 receipt logs，避免前端直接依赖 Etherscan-compatible query shape。
3. 实现 `account-txlistinternal` 或新增 trace/internal transaction native endpoint。

数据库：

1. 继续使用 `evm_transaction_receipts` 保存 logs。
2. 增加 decoded event/method 缓存列或派生表，避免每次查询实时 ABI decode。
3. 如果需要 internal transaction，新增 trace/internal tx 表，至少包含 parent tx hash、from、to、value、input、type、trace address、status/error。

扫描解析：

1. 确认线上 Heima worker 已持续执行 `eth_getTransactionReceipt` 并写入 `evm_transaction_receipts`。
2. 增加 ABI decode job：合约验证后回填历史 logs/method input decode。
3. 如需 internal transaction，增加 `debug_traceTransaction` 或等价 trace RPC 的采集与重试策略。

## 本次未直接修改代码行为

本次最小交付是调查报告。原因是当前问题首先需要明确“线上能否查看”和“缺口在哪一层”；在未能公开访问 Heima explorer API/DB 的情况下，直接修改 UI 或 API 容易掩盖真正的线上访问和数据验证问题。
