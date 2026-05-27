# USDT (TRC20 / ERC20) 测试网联调验收指南

本功能为**自托管 HD 钱包收款**：每个用户分配一个由主助记词派生的专属充值地址，
后端轮询链上对账自动入账，管理员可一键归集到冷地址。

> ⚠️ **上线前必做**：链上签名/广播/归集逻辑无法在本地模拟，必须先在测试网
> （TRC20→Shasta，ERC20→Sepolia）端到端验收通过，再切主网。本文是验收清单。

---

## 0. 前置条件

- `TOTP_ENCRYPTION_KEY` 环境变量已配（钱包初始化/归集要 TOTP）。
- **管理员已开启并绑定 2FA**（钱包"动钱"操作强制二次验证）。
- 一个能发测试币的钱包（TronLink / MetaMask）。

钱包的两类地址（均由主助记词在派生序号 0 派生，互不相同）：
- **燃料(gas)钱包**：TRON 用 `fee_address`（持测试 TRX），ETH 用 `eth_fee_address`（持测试 ETH）。归集时给充值地址打 gas。
- **归集地址**：你掌控的冷地址，归集目标。TRON 填 `T...`，ETH 填 `0x...`。

---

## 1. TRC20 (Shasta 测试网)

### 1.1 创建收款服务商
管理端 → 设置 → 支付 → 新增服务商 → 类型 **USDT (TRC20)**，config：

| 键 | 测试网取值 |
|---|---|
| `cnyPerUsdt` | 汇率，如 `7.2`（必填） |
| `minRechargeCny` | 最低充值人民币，默认 `100` |
| `usdtContract` | **Shasta 上的测试 TRC20 USDT 合约地址**（见 1.2） |
| `trongridApiBase` | `https://api.shasta.trongrid.io` |
| `trongridGrpcNode` | `grpc.shasta.trongrid.io:50051` |
| `trongridApiKey` | TronGrid 申请的 API Key（强烈建议，免限流） |
| `confirmSeconds` | 默认 `60` |
| `gasTopUpSun` | 默认 `30000000`（30 TRX） |
| `feeLimitSun` | 默认 `100000000`（100 TRX） |
| `sweepMinUsdt` | 默认 `5` |

保存后在「启用的服务商」勾选 **USDT (TRC20)**。

### 1.2 测试币与合约
- **测试 TRX**：Shasta 水龙头（如 https://shasta.tronex.io 或 TronLink 内置 faucet）。
- **测试 USDT 合约**：Shasta 没有官方 USDT，需自备一个 TRC20 测试代币：
  - 自己部署一个 6 位精度的 TRC20 代币，或用社区测试代币；
  - 把合约地址填入 `usdtContract`，并给自己的测试钱包铸一些用于付款。

### 1.3 验收步骤
1. 钱包页 → **初始化钱包**（输 TOTP）→ **离线抄写助记词** → 记下 `fee_address`。
2. 给 `fee_address` 打若干测试 TRX（够多笔归集 gas）。
3. 钱包页 → **设归集地址**（TOTP），填你的 Shasta `T...` 地址。
4. 用户端充值 → 选 USDT(TRC20) → 输金额(≥`minRechargeCny`) → 得到**收款地址** + 应付 USDT（= 人民币 ÷ 汇率）。
5. 用测试钱包给该收款地址转**精确金额**的测试 USDT。
6. 等待 `confirmSeconds`(~60s) + 对账 ticker(≤15s) → 订单变 **COMPLETED**、用户余额到账。
7. **一键归集**(TOTP) → 观察 sweep 任务：`gas_funding → gas_confirmed → sweeping → confirmed`；归集地址 USDT 余额增加。
8. 在 https://shasta.tronscan.org 核对：gas 充值 tx + USDT 归集 tx 均成功。

---

## 2. ERC20 (Sepolia 测试网)

### 2.1 创建收款服务商
新增服务商 → 类型 **USDT (ERC20)**，config：

| 键 | 测试网取值 |
|---|---|
| `cnyPerUsdt` | 汇率，如 `6.8`（必填） |
| `minRechargeCny` | 默认 `500`（ETH gas 贵，最低额设高些） |
| `usdtContract` | **Sepolia 上的测试 ERC20 USDT 合约**（见 2.2） |
| `etherscanApiBase` | `https://api-sepolia.etherscan.io/api` |
| `etherscanApiKey` | Etherscan API Key（必配，否则限流严重） |
| `ethRpcUrl` | Sepolia JSON-RPC（Infura/Alchemy，**仅归集用**） |
| `confirmSeconds` | 默认 `180`（ETH 出块慢） |
| `gasTopUpWei` | 默认 `3000000000000000`（0.003 ETH） |
| `sweepMinUsdt` | 默认 `50` |

启用「USDT (ERC20)」。

### 2.2 测试币与合约
- **测试 ETH**：Sepolia 水龙头（sepoliafaucet.com / Alchemy / Google Cloud faucet）。
- **测试 USDT(ERC20) 合约**：部署一个 6 位精度的 ERC20 测试代币（如 OpenZeppelin ERC20 mock），把合约填入 `usdtContract`，给测试钱包铸币。
- `gasTopUpWei` 要够付一笔 ERC20 transfer 的 gas（按当时 Sepolia gas price 估，0.003 ETH 通常足够，拥堵时调大）。

### 2.3 验收步骤
与 TRC20 相同，差异点：
- 初始化后用 `eth_fee_address`，给它打测试 **ETH**（不是 TRX）。
- 归集地址填 `0x...`（你掌控的 Sepolia 地址）。
- 充值收款地址是 `0x...`；用测试钱包转**精确**测试 USDT。
- 等待 `confirmSeconds`(~180s) + ticker → COMPLETED。
- 一键归集后在 https://sepolia.etherscan.io 核对：ETH gas 充值 tx + ERC20 transfer tx 成功。

---

## 3. 验收清单

- [ ] 创建并启用 TRC20 / ERC20 服务商，config 完整
- [ ] 钱包初始化成功，助记词已离线备份
- [ ] 燃料地址已充值测试 gas（TRX / ETH）
- [ ] 归集地址已设置（T... / 0x...）
- [ ] 下单：金额按汇率正确折算成 USDT；< `minRechargeCny` 被拒（`USDT_MIN_AMOUNT`）
- [ ] 收款二维码是**纯地址**（可被 OKX/币安/钱包扫码识别）
- [ ] 转测试 USDT 后，订单在 ~确认时间内自动 **COMPLETED**，余额到账
- [ ] `trc20_consumed_txs` 有该 txHash 记录（防重复入账）
- [ ] 一键归集：两阶段 tx 均上链确认，归集地址余额增加
- [ ] 重复点归集被拒（`SWEEP_IN_PROGRESS`）

---

## 4. 排查

| 现象 | 排查 |
|---|---|
| 订单不入账 | TronGrid/Etherscan 是否限流（配 API Key）；`confirmSeconds` 是否还没到；金额是否与应付**完全一致**；收款地址网络是否对 |
| 归集 `NO_ETH_RPC` / RPC 报错 | ERC20 必须配 `ethRpcUrl`；检查 RPC 可达、chainId 正确 |
| 归集卡在 gas_funding | 燃料地址 gas 不足；或确认超时（任务可重试，下次归集会续跑未完成任务） |
| 钱包操作报 `TOTP_NOT_SETUP` | 管理员未绑定 2FA，先开启 |
| 充值页弹出别人/旧订单 | 已修复：恢复前校验订单归属当前用户且 PENDING |

> 主网切换：把 `usdtContract` 换成主网 USDT（TRC20 `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t` / ERC20 `0xdAC17F958D2ee523a2206206994597C13D831ec7`），`trongridApiBase`/`etherscanApiBase`/`ethRpcUrl`/gRPC 换主网，确认 `cnyPerUsdt` 为真实汇率。
