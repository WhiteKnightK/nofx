# Bitget 交易所接入需求分析

## 📋 目标
接入 Bitget 交易所，沿用现有架构，在交易所选择中增加 Bitget 选项。用户选择 Bitget 并填入 API Key、Secret Key、Passphrase 后，即可正常使用，功能与 Binance 一致。

---

## 🔍 当前架构分析

### 1. 交易所接入模式
系统采用 **策略模式**，通过 `Trader` 接口统一抽象所有交易所：

```go
type Trader interface {
    GetBalance() (map[string]interface{}, error)
    GetPositions() ([]map[string]interface{}, error)
    OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)
    OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)
    CloseLong(symbol string, quantity float64) (map[string]interface{}, error)
    CloseShort(symbol string, quantity float64) (map[string]interface{}, error)
    SetLeverage(symbol string, leverage int) error
    SetMarginMode(symbol string, isCrossMargin bool) error
    GetMarketPrice(symbol string) (float64, error)
    SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error
    SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error
    CancelStopLossOrders(symbol string) error
    CancelTakeProfitOrders(symbol string) error
    CancelAllOrders(symbol string) error
    CancelStopOrders(symbol string) error
    FormatQuantity(symbol string, quantity float64) (string, error)
}
```

### 2. 现有交易所实现
- **Binance**: 使用官方 Go SDK (`github.com/adshao/go-binance/v2`)
- **Hyperliquid**: 使用第三方 SDK (`github.com/sonirico/go-hyperliquid`)
- **Aster**: 自定义 HTTP API 实现

### 3. 集成点
- `trader/auto_trader.go`: 根据 `config.Exchange` 创建对应 Trader
- `api/server.go`: 处理交易所配置和交易员创建
- `config/database.go`: 存储交易所配置（API Key、Secret Key 等）

---

## 📊 Binance 使用的接口清单

### 账户相关
- `GET /fapi/v2/account` - 获取账户信息（余额、未实现盈亏）
- `GET /fapi/v2/balance` - 获取账户余额（备用）

### 持仓相关
- `GET /fapi/v2/positionRisk` - 获取持仓信息（数量、开仓价、标记价、盈亏、杠杆）

### 交易相关
- `POST /fapi/v1/order` - 下单（开仓/平仓）
- `POST /fapi/v1/leverage` - 设置杠杆
- `POST /fapi/v1/marginType` - 设置仓位模式（全仓/逐仓）
- `POST /fapi/v1/positionSide/dual` - 设置双向持仓模式

### 订单管理
- `GET /fapi/v1/openOrders` - 查询未完成订单
- `DELETE /fapi/v1/order` - 取消单个订单
- `DELETE /fapi/v1/allOpenOrders` - 取消所有订单

### 市场数据
- `GET /fapi/v1/ticker/price` - 获取市场价格
- `GET /fapi/v1/exchangeInfo` - 获取交易规则（精度、最小数量等）

### 系统
- `GET /fapi/v1/time` - 获取服务器时间（用于时间同步）

---

## 🔗 Bitget API 接口对比

### ✅ 用户提供的接口（参考文档）

| 功能 | Bitget API | 文档链接 |
|------|-----------|---------|
| **下单** | `POST /api/v2/mix/order/place-order` | [下单接口](https://www.bitget.com/zh-CN/api-doc/contract/trade/Place-Order) |
| **获取全部交易对行情** | `GET /api/v2/mix/market/tickers` | [行情接口](https://www.bitget.com/zh-CN/api-doc/contract/market/Get-All-Symbol-Ticker) |
| **WebSocket 行情** | WebSocket Tickers Channel | [WebSocket](https://www.bitget.com/zh-CN/api-doc/contract/websocket/public/Tickers-Channel) |

### ✅ Bitget API 结构确认

根据 [Bitget 合约交易API介绍页面](https://www.bitget.com/zh-CN/api-doc/contract/intro)，Bitget 提供了完整的API分类：

| API 分类 | 功能说明 | 包含的接口 |
|---------|---------|-----------|
| **行情 (Market)** | 市场数据 | ✅ 已提供：获取全部交易对行情 |
| **账户 (Account)** | 账户管理 | 账户余额查询、设置杠杆、设置仓位模式等 |
| **仓位 (Position)** | 持仓管理 | 持仓查询、持仓设置等 |
| **交易 (Trade)** | 订单管理 | ✅ 已提供：下单；还需：订单查询、取消订单等 |
| **策略交易** | 高级交易 | 止盈止损、条件单等 |
| **Websocket** | 实时数据 | ✅ 已提供：Tickers Channel |

**重要参数说明**:
- `productType`: 产品类型参数
  - `USDT-FUTURES`: U本位合约（以USDT结算）✅ **我们使用这个**
  - `COIN-FUTURES`: 币本位合约（以加密货币结算）
  - `USDC-FUTURES`: USDC合约（以USDC结算）

### ⚠️ 需要查找的具体接口

**说明**: 用户已提供了3个接口作为参考。要实现完整的交易功能，还需要在以下分类中查找具体接口：

| 功能 | API 分类 | 可能的接口路径 | 查找位置 |
|------|---------|--------------|---------|
| **获取账户余额** | 账户 Account | `/api/v2/mix/account/accounts` 或 `/api/v2/mix/account/account` | 合约交易API → 账户 |
| **获取持仓列表** | 仓位 Position | `/api/v2/mix/position/allPosition` 或 `/api/v2/mix/position/positions` | 合约交易API → 仓位 |
| **设置杠杆** | 账户 Account | `/api/v2/mix/account/setLeverage` | 合约交易API → 账户 |
| **设置仓位模式** | 账户 Account | `/api/v2/mix/account/setMarginMode` | 合约交易API → 账户 |
| **查询未完成订单** | 交易 Trade | `/api/v2/mix/order/current` 或 `/api/v2/mix/order/openOrders` | 合约交易API → 交易 |
| **取消订单** | 交易 Trade | `/api/v2/mix/order/cancel-order` | 合约交易API → 交易 |
| **取消所有订单** | 交易 Trade | `/api/v2/mix/order/cancel-all` | 合约交易API → 交易 |
| **获取单个交易对价格** | 行情 Market | `/api/v2/mix/market/ticker` | 合约交易API → 行情 |
| **获取交易规则** | 行情 Market | `/api/v2/mix/market/contracts` 或 `/api/v2/mix/market/symbols` | 合约交易API → 行情 |

**查找建议**:
1. 访问 Bitget 官方 API 文档: https://www.bitget.com/zh-CN/api-doc/contract
2. 查看左侧导航栏的以下分类:
   - **账户 (Account)**: 账户余额、设置杠杆、设置仓位模式
   - **仓位 (Position)**: 持仓查询
   - **交易 (Trade)**: 订单查询、取消订单
   - **行情 (Market)**: 单个交易对价格、交易规则

---

## ✅ 可行性评估

### 1. 核心功能支持度

| 功能 | Binance | Bitget | 评估 |
|------|---------|--------|------|
| **开多/开空** | ✅ | ✅ | 下单接口支持 `side: open_long/open_short` |
| **平多/平空** | ✅ | ✅ | 下单接口支持 `side: close_long/close_short` |
| **双向持仓** | ✅ | ✅ | Bitget 默认支持双向持仓 |
| **全仓/逐仓** | ✅ | ✅ | 需确认 `marginMode` 参数支持 |
| **设置杠杆** | ✅ | ✅ | 需确认独立接口或下单时指定 |
| **止盈止损** | ✅ | ✅ | 下单接口支持预设参数 |
| **获取余额** | ✅ | ✅ | 需确认账户接口 |
| **获取持仓** | ✅ | ✅ | 需确认持仓接口 |
| **获取市价** | ✅ | ✅ | 行情接口支持 |
| **格式化数量** | ✅ | ✅ | 需确认交易规则接口 |

### 2. 认证方式

| 项目 | Binance | Bitget |
|------|---------|--------|
| **API Key** | ✅ | ✅ |
| **Secret Key** | ✅ | ✅ |
| **Passphrase** | ❌ | ✅ |
| **签名算法** | HMAC-SHA256 | HMAC-SHA256 |
| **时间戳格式** | 毫秒 | 秒 |

**差异处理**: 
- Passphrase 字段前端已有支持（OKX），后端只需添加字段
- 时间戳格式差异需在签名函数中处理

### 3. 参数映射差异

#### Binance → Bitget 参数转换

| Binance | Bitget | 说明 |
|---------|--------|------|
| `side: BUY/SELL` + `positionSide: LONG/SHORT` | `side: open_long/open_short/close_long/close_short` | 需要组合转换 |
| `type: MARKET` | `orderType: market` | 直接映射 |
| `quantity` | `size` | 字段名不同 |
| `leverage` (独立设置) | `leverage` (下单时指定) | 可能需要在每次下单时指定 |
| `marginType: ISOLATED/CROSSED` | `marginMode: isolated/crossed` | 字段名和值略有不同 |

---

## 🎯 实现方案

### 1. 后端实现（Go）

#### 1.1 创建 `bitget_trader.go`
- 实现 `Trader` 接口的所有方法
- 实现 HMAC-SHA256 签名（注意时间戳为秒）
- 实现 HTTP 请求封装
- 实现参数格式转换（Binance → Bitget）

#### 1.2 核心方法映射

```go
// GetBalance
GET /api/v2/mix/account/accounts?productType=USDT-FUTURES&marginCoin=USDT

// GetPositions  
GET /api/v2/mix/position/allPosition?productType=USDT-FUTURES

// OpenLong
POST /api/v2/mix/order/place-order
{
  "symbol": "BTCUSDT",
  "productType": "USDT-FUTURES",
  "marginMode": "isolated",
  "marginCoin": "USDT",
  "side": "open_long",
  "orderType": "market",
  "size": "0.001",
  "leverage": "10"
}

// SetLeverage
POST /api/v2/mix/account/setLeverage (需确认)
// 或在下单时指定 leverage 参数

// SetMarginMode
POST /api/v2/mix/account/setMarginMode (需确认)

// GetMarketPrice
GET /api/v2/mix/market/ticker?symbol=BTCUSDT&productType=USDT-FUTURES

// FormatQuantity
GET /api/v2/mix/market/contracts (获取交易规则)
```

#### 1.3 数据库支持
- 在 `exchanges` 表中添加 `passphrase` 字段（如果还没有）
- 在 `initDefaultData` 中添加 bitget 默认配置

#### 1.4 集成点修改
- `trader/auto_trader.go`: 添加 `case "bitget"`
- `api/server.go`: 添加 bitget 配置处理
- `config/database.go`: 添加 passphrase 字段支持

### 2. 前端实现（TypeScript/React）

#### 2.1 交易所配置界面
- 添加 Bitget 选项
- 添加 Passphrase 输入框（复用 OKX 逻辑）
- 添加 Bitget 图标

#### 2.2 交易所列表
- 在支持的交易所列表中添加 Bitget

---

## ⚠️ 注意事项

### 1. API 文档完整性
**用户提供的接口**（仅3个）：
- ✅ 下单接口: `POST /api/v2/mix/order/place-order`
- ✅ 获取全部交易对行情: `GET /api/v2/mix/market/tickers`
- ✅ WebSocket 行情: Tickers Channel

**缺失的关键接口**（需查阅 Bitget 官方文档）：
- ❓ 账户余额查询（Account 分类）
- ❓ 持仓列表查询（Position 分类）
- ❓ 设置杠杆（Account 分类）
- ❓ 设置仓位模式（Account 分类）
- ❓ 订单管理（Trade 分类：查询、取消）
- ❓ 单个交易对价格（Market 分类）
- ❓ 交易规则（Market 分类：精度、最小数量）

**查找方法**:
1. 访问: https://www.bitget.com/zh-CN/api-doc/contract
2. 查看左侧导航栏的各个分类（账户、仓位、交易、行情）
3. 根据功能需求查找对应的接口文档

### 2. 实现建议
1. **先查阅完整 API 文档**: 访问 Bitget 官方 API 文档，查找所有必需接口
2. **参考现有实现**: 参考 `binance_futures.go` 和 `aster_trader.go` 的实现方式
3. **测试环境验证**: 使用 Bitget 测试网 (`https://testnet.bitget.com`) 验证所有功能
4. **错误处理**: Bitget 的错误码可能与 Binance 不同，需要适配
5. **接口路径推断**: 基于已知的 `/api/v2/mix/` 路径结构，可以推断其他接口的可能路径

### 3. 市场数据
- **当前系统**: 所有交易所共享 Binance WebSocket 市场数据
- **Bitget**: 可以使用 Binance 数据（兼容），也可以使用 Bitget WebSocket（可选）

---

## 📝 结论

### ✅ **可以接入 Bitget**

**理由**:
1. ✅ Bitget 提供完整的合约交易 API
2. ✅ 支持双向持仓、全仓/逐仓、杠杆、止盈止损等核心功能
3. ✅ 认证方式标准（HMAC-SHA256），Go 原生支持
4. ✅ 前端已有 Passphrase 支持，可复用
5. ✅ 市场数据可复用 Binance，不影响交易功能

**前提条件**:
1. ⚠️ **需要查阅 Bitget 官方 API 文档**，查找账户、持仓、订单管理等必需接口
2. ⚠️ 用户仅提供了3个接口作为参考，其他接口需在官方文档中查找
3. ⚠️ 需要确认所有必需接口的端点路径和参数格式

**下一步**:
1. **查阅 Bitget 官方 API 文档**: 
   - 访问: https://www.bitget.com/zh-CN/api-doc/contract
   - 查找: 账户(Account)、仓位(Position)、交易(Trade)、行情(Market) 分类下的所有必需接口
2. **创建接口映射表**: 将找到的接口与 Binance 接口进行映射
3. **创建 `bitget_trader.go`**: 实现 Trader 接口的所有方法
4. **添加数据库和前端支持**: 添加 passphrase 字段和 Bitget 选项
5. **测试环境验证**: 使用 Bitget 测试网验证所有功能

---

## 📚 参考文档

### 用户提供的接口文档
- [Bitget 合约交易 API - 下单](https://www.bitget.com/zh-CN/api-doc/contract/trade/Place-Order)
- [Bitget 合约行情 API - 获取全部交易对行情](https://www.bitget.com/zh-CN/api-doc/contract/market/Get-All-Symbol-Ticker)
- [Bitget WebSocket - Tickers Channel](https://www.bitget.com/zh-CN/api-doc/contract/websocket/public/Tickers-Channel)

### API 结构参考
- [Bitget 合约交易API介绍](https://www.bitget.com/zh-CN/api-doc/contract/intro) - 确认了API分类和productType参数用法

### 需要查阅的分类文档
- [账户 (Account)](https://www.bitget.com/zh-CN/api-doc/contract/account) - 账户余额、设置杠杆、设置仓位模式
- [仓位 (Position)](https://www.bitget.com/zh-CN/api-doc/contract/position) - 持仓查询
- [交易 (Trade)](https://www.bitget.com/zh-CN/api-doc/contract/trade) - 订单查询、取消订单
- [行情 (Market)](https://www.bitget.com/zh-CN/api-doc/contract/market) - 单个交易对价格、交易规则



