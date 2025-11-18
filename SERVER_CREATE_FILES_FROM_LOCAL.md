# 📋 服务器创建文件命令（使用本地实际配置）

## 🚀 根据本地实际配置文件创建服务器文件

---

## 步骤1：创建 config.json（使用本地实际内容）

```bash
cd ~/nofx

cat > config.json << 'JSONEOF'
{
  "beta_mode": false,
  "leverage": {
    "btc_eth_leverage": 5,
    "altcoin_leverage": 5
  },
  "use_default_coins": true,
  "default_coins": [
    "BTCUSDT",
    "ETHUSDT",
    "SOLUSDT",
    "BNBUSDT",
    "XRPUSDT",
    "DOGEUSDT",
    "ADAUSDT",
    "HYPEUSDT"
  ],
  "api_server_port": 8080,
  "max_daily_loss": 10.0,
  "max_drawdown": 20.0,
  "stop_trading_minutes": 60,
  "jwt_secret": "Qk0kAa+d0iIEzXVHXbNbm+UaN3RNabmWtH8rDWZ5OPf+4GX8pBflAHodfpbipVMyrw1fsDanHsNBjhgbDeK9Jg==",
  "log": {
    "level": "info"
  }
}
JSONEOF
```

---

## 步骤2：创建 prompts/default.txt（使用本地实际内容）

```bash
cd ~/nofx
mkdir -p prompts

cat > prompts/default.txt << 'PROMPTEOF'
你是专业的加密货币交易AI，在合约市场进行自主交易。

# 核心目标

最大化夏普比率（Sharpe Ratio）

夏普比率 = 平均收益 / 收益波动率

这意味着：
- 高质量交易（高胜率、大盈亏比）→ 提升夏普
- 稳定收益、控制回撤 → 提升夏普
- 耐心持仓、让利润奔跑 → 提升夏普
- 频繁交易、小盈小亏 → 增加波动，严重降低夏普
- 过度交易、手续费损耗 → 直接亏损
- 过早平仓、频繁进出 → 错失大行情

关键认知: 系统每3分钟扫描一次，但不意味着每次都要交易！
大多数时候应该是 `wait` 或 `hold`，只在极佳机会时才开仓。

# 交易哲学 & 最佳实践

## 核心原则：

资金保全第一：保护资本比追求收益更重要

纪律胜于情绪：执行你的退出方案，不随意移动止损或目标

质量优于数量：少量高信念交易胜过大量低信念交易

适应波动性：根据市场条件调整仓位

尊重趋势：不要与强趋势作对

## 常见误区避免：

过度交易：频繁交易导致费用侵蚀利润

复仇式交易：亏损后立即加码试图"翻本"

分析瘫痪：过度等待完美信号，导致失机

忽视相关性：BTC常引领山寨币，须优先观察BTC

过度杠杆：放大收益同时放大亏损

#交易频率认知

量化标准:
- 优秀交易员：每天2-4笔 = 每小时0.1-0.2笔
- 过度交易：每小时>2笔 = 严重问题
- 最佳节奏：开仓后持有至少30-60分钟

自查:
如果你发现自己每个周期都在交易 → 说明标准太低
如果你发现持仓<30分钟就平仓 → 说明太急躁

# 开仓标准（严格）

只在强信号时开仓，不确定就观望。

你拥有的完整数据：
- 原始序列：3分钟价格序列(MidPrices数组) + 4小时K线序列
- 技术序列：EMA20序列、MACD序列、RSI7序列、RSI14序列
- 资金序列：成交量序列、持仓量(OI)序列、资金费率
- 筛选标记：AI500评分 / OI_Top排名（如果有标注）

分析方法（完全由你自主决定）：
- 自由运用序列数据，你可以做但不限于趋势分析、形态识别、支撑阻力、技术阻力位、斐波那契、波动带计算
- 多维度交叉验证（价格+量+OI+指标+序列形态）
- 用你认为最有效的方法发现高确定性机会
- 综合信心度 ≥ 75 才开仓

避免低质量信号：
- 单一维度（只看一个指标）
- 相互矛盾（涨但量萎缩）
- 横盘震荡
- 刚平仓不久（<15分钟）

# 夏普比率自我进化

每次你会收到夏普比率作为绩效反馈（周期级别）：

夏普比率 < -0.5 (持续亏损):
  → 停止交易，连续观望至少6个周期（18分钟）
  → 深度反思：
     • 交易频率过高？（每小时>2次就是过度）
     • 持仓时间过短？（<30分钟就是过早平仓）
     • 信号强度不足？（信心度<75）
夏普比率 -0.5 ~ 0 (轻微亏损):
  → 严格控制：只做信心度>80的交易
  → 减少交易频率：每小时最多1笔新开仓
  → 耐心持仓：至少持有30分钟以上

夏普比率 0 ~ 0.7 (正收益):
  → 维持当前策略

夏普比率 > 0.7 (优异表现):
  → 可适度扩大仓位

关键: 夏普比率是唯一指标，它会自然惩罚频繁交易和过度进出。

#决策流程

1. 分析夏普比率: 当前策略是否有效？需要调整吗？
2. 评估持仓: 趋势是否改变？是否该止盈/止损？
3. 寻找新机会: 有强信号吗？多空机会？
4. 输出决策: 思维链分析 + JSON

# 仓位大小计算

**重要**：`position_size_usd` 是**名义价值**（包含杠杆），非保证金需求。

**计算步骤**：
1. **可用保证金** = Available Cash × 0.88（预留12%给手续费、滑点与清算保证金缓冲）
2. **名义价值** = 可用保证金 × Leverage
3. **position_size_usd** = 名义价值（JSON中填写此值）
4. **实际币数** = position_size_usd / Current Price

**示例**：可用资金 $500，杠杆 5x
- 可用保证金 = $500 × 0.88 = $440
- position_size_usd = $440 × 5 = **$2,200** ← JSON填此值
- 实际占用保证金 = $440，剩余 $60 用于手续费、滑点与清算保护

---

记住:
- 目标是夏普比率，不是交易频率
- 宁可错过，不做低质量交易
- 风险回报比1:3是底线
PROMPTEOF
```

---

## 步骤3：创建其他提示词文件（可选）

如果需要其他提示词文件，可以从本地上传：

```bash
# 在本地执行，上传其他提示词文件
scp prompts/nof1.txt user@server:~/nofx/prompts/
scp prompts/Hansen.txt user@server:~/nofx/prompts/
```

---

## 步骤4：创建 .env 文件（需要从本地获取密钥）

**重要：** `.env` 文件包含敏感信息，需要从本地获取。

### 在本地执行（获取密钥值）：

```bash
cd ~/code/nofx/nofx
cat .env
```

### 在服务器上创建 .env（替换为实际值）：

```bash
cd ~/nofx

cat > .env << 'ENVEOF'
DOCKERHUB_USERNAME=baimastryke
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080
NOFX_TIMEZONE=Asia/Shanghai
DATA_ENCRYPTION_KEY=从本地.env文件复制这个值
JWT_SECRET=从本地.env文件复制这个值
ENVEOF

chmod 600 .env
```

---

## 步骤5：创建其他必要文件和目录

```bash
cd ~/nofx

# 创建目录
mkdir -p secrets decision_logs

# 创建空文件
touch config.db beta_codes.txt

# 设置权限
chmod 700 secrets decision_logs
chmod 600 config.db
```

---

## 步骤6：上传 RSA 密钥（必须从本地上传）

**在本地执行：**

```bash
cd ~/code/nofx/nofx
scp secrets/rsa_key user@server:~/nofx/secrets/
scp secrets/rsa_key.pub user@server:~/nofx/secrets/
```

**在服务器上设置权限：**

```bash
cd ~/nofx
chmod 600 secrets/rsa_key
chmod 644 secrets/rsa_key.pub
chmod 700 secrets
```

---

## 步骤7：创建 docker-compose.prod.yml

```bash
cd ~/nofx

cat > docker-compose.prod.yml << 'EOF'
services:
  nofx:
    image: ${DOCKERHUB_USERNAME:-baimastryke}/nofx-backend:${IMAGE_TAG:-latest}
    container_name: nofx-trading
    restart: unless-stopped
    stop_grace_period: 30s
    ports:
      - "${NOFX_BACKEND_PORT:-8080}:8080"
    volumes:
      - ./config.json:/app/config.json:ro
      - ./config.db:/app/config.db
      - ./beta_codes.txt:/app/beta_codes.txt:ro
      - ./decision_logs:/app/decision_logs
      - ./prompts:/app/prompts
      - ./secrets:/app/secrets:ro
      - /etc/localtime:/etc/localtime:ro
    environment:
      - TZ=${NOFX_TIMEZONE:-Asia/Shanghai}
      - AI_MAX_TOKENS=4000
      - DATA_ENCRYPTION_KEY=${DATA_ENCRYPTION_KEY}
      - JWT_SECRET=${JWT_SECRET}
    networks:
      - nofx-network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
  nofx-frontend:
    image: ${DOCKERHUB_USERNAME:-baimastryke}/nofx-frontend:${IMAGE_TAG:-latest}
    container_name: nofx-frontend
    restart: unless-stopped
    ports:
      - "${NOFX_FRONTEND_PORT:-3000}:80"
    networks:
      - nofx-network
    depends_on:
      - nofx
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 5s
networks:
  nofx-network:
    driver: bridge
EOF
```

---

## ✅ 验证所有文件

```bash
cd ~/nofx

# 检查所有文件是否存在
ls -la config.json .env docker-compose.prod.yml
ls -la secrets/rsa_key secrets/rsa_key.pub
ls -la prompts/default.txt

# 验证 JSON 格式
cat config.json | python3 -m json.tool > /dev/null && echo "✓ config.json 格式正确" || echo "✗ config.json 格式错误"

# 检查文件权限
ls -la | grep -E "config.json|\.env|config.db|secrets"
```

---

## 🚀 然后拉取镜像并启动

```bash
cd ~/nofx
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a
source .env
set +a

docker login
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml logs -f
```

---

## ⚠️ 重要提示

1. **config.json** 中的 `jwt_secret` 应该与 `.env` 中的 `JWT_SECRET` 一致
2. **.env** 文件中的 `DATA_ENCRYPTION_KEY` 和 `JWT_SECRET` 必须与本地完全一致
3. **secrets/** 目录的 RSA 密钥必须从本地上传
4. **prompts/** 目录现在有实际的提示词内容，不再是空的





