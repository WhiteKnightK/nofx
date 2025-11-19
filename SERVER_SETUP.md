# 服务器部署前准备工作清单

## ⚠️ 问题分析

根据错误日志：
1. `config.json` 文件为空或格式错误（`unexpected end of JSON input`）
2. `prompts` 目录缺少 .txt 文件（警告，不影响启动）
3. 后端服务无法启动，导致前端502错误

## 📋 部署前必须准备的文件

### 1. 创建正确的 `config.json` 文件

在服务器上执行以下命令创建 `config.json`：

```bash
cat > config.json << 'EOF'
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
    "ADAUSDT"
  ],
  "api_server_port": 8080,
  "max_daily_loss": 10.0,
  "max_drawdown": 20.0,
  "stop_trading_minutes": 60,
  "jwt_secret": "CHANGE_THIS_TO_A_RANDOM_SECRET_KEY_AT_LEAST_64_CHARS_LONG",
  "log": {
    "level": "info"
  }
}
EOF
```

**重要：** 请将 `jwt_secret` 替换为一个至少64字符的随机字符串！

### 2. 创建 `.env` 文件

```bash
cat > .env << 'EOF'
# Docker Hub 用户名
DOCKERHUB_USERNAME=baimastryke

# 端口配置
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080

# 时区
NOFX_TIMEZONE=Asia/Shanghai

# 数据加密密钥（必须与本地一致，至少32字符）
DATA_ENCRYPTION_KEY=YOUR_DATA_ENCRYPTION_KEY_HERE

# JWT认证密钥（必须与本地一致，至少64字符）
JWT_SECRET=YOUR_JWT_SECRET_HERE
EOF
```

**重要：** 
- `DATA_ENCRYPTION_KEY` 和 `JWT_SECRET` 必须与本地 `.env` 文件中的值完全一致！
- 这些密钥用于加密数据库和认证，不一致会导致无法访问数据

### 3. 创建 `secrets` 目录和RSA密钥

```bash
# 创建目录
mkdir -p secrets
chmod 700 secrets

# 如果本地已有密钥，需要上传到服务器
# 或者生成新的密钥对（但这样会无法解密之前的数据）
```

**重要：** 必须从本地复制 `secrets/rsa_key` 和 `secrets/rsa_key.pub` 到服务器！

### 4. 创建其他必要目录和文件

```bash
# 创建目录
mkdir -p prompts decision_logs

# 创建空文件
touch config.db beta_codes.txt

# 设置权限
chmod 600 config.db .env
chmod 700 secrets decision_logs
```

### 5. 创建 `prompts` 目录的提示词文件（可选，但推荐）

```bash
# 创建默认提示词文件
cat > prompts/default.txt << 'EOF'
你是一个专业的加密货币交易AI助手。请根据市场数据做出交易决策。
EOF
```

## 🚀 完整部署流程

### 步骤1: 在服务器上准备目录

```bash
# 创建项目目录
mkdir -p ~/nofx
cd ~/nofx

# 创建所有必要目录
mkdir -p secrets prompts decision_logs
touch config.json config.db beta_codes.txt
chmod 700 secrets decision_logs
chmod 600 config.db
```

### 步骤2: 创建配置文件

```bash
# 创建 config.json（使用上面的内容）
cat > config.json << 'EOF'
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
    "BNBUSDT"
  ],
  "api_server_port": 8080,
  "max_daily_loss": 10.0,
  "max_drawdown": 20.0,
  "stop_trading_minutes": 60,
  "jwt_secret": "CHANGE_THIS_TO_A_RANDOM_SECRET_KEY_AT_LEAST_64_CHARS_LONG",
  "log": {
    "level": "info"
  }
}
EOF

# 创建 .env 文件（替换为你的实际密钥）
cat > .env << 'EOF'
DOCKERHUB_USERNAME=baimastryke
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080
NOFX_TIMEZONE=Asia/Shanghai
DATA_ENCRYPTION_KEY=YOUR_DATA_ENCRYPTION_KEY_HERE
JWT_SECRET=YOUR_JWT_SECRET_HERE
EOF

# 创建默认提示词
cat > prompts/default.txt << 'EOF'
你是一个专业的加密货币交易AI助手。
EOF
```

### 步骤3: 上传必需文件

使用 `scp` 或其他方式上传以下文件：

```bash
# 从本地上传到服务器
scp secrets/rsa_key user@server:~/nofx/secrets/
scp secrets/rsa_key.pub user@server:~/nofx/secrets/
scp .env user@server:~/nofx/.env  # 确保密钥正确
```

### 步骤4: 设置文件权限

```bash
chmod 600 .env config.db secrets/rsa_key
chmod 644 secrets/rsa_key.pub
chmod 700 secrets
```

### 步骤5: 创建 docker-compose.prod.yml 并启动

```bash
# 创建 docker-compose 文件（参考 SERVER_COMMANDS.md）
# 然后拉取镜像并启动
export DOCKERHUB_USERNAME=baimastryke
docker login
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

## ✅ 验证部署

```bash
# 检查容器状态
docker compose -f docker-compose.prod.yml ps

# 查看日志
docker compose -f docker-compose.prod.yml logs -f

# 检查健康状态
curl http://localhost:8080/api/health
```

## 🔑 密钥获取方法

### 从本地获取密钥

在本地项目目录执行：

```bash
# 查看 DATA_ENCRYPTION_KEY
grep DATA_ENCRYPTION_KEY .env

# 查看 JWT_SECRET
grep JWT_SECRET .env

# 查看 config.json 中的 jwt_secret
grep jwt_secret config.json
```

**重要：** 服务器上的 `.env` 和 `config.json` 中的密钥必须与本地完全一致！

## 🐛 常见问题

### 问题1: config.json 解析失败

**原因：** 文件为空或格式错误

**解决：** 确保 `config.json` 是有效的JSON格式，可以使用 `jq` 验证：
```bash
cat config.json | jq .
```

### 问题2: 提示词目录警告

**原因：** `prompts` 目录中没有 .txt 文件

**解决：** 创建至少一个提示词文件（可选，不影响启动）

### 问题3: 后端一直重启

**原因：** 通常是配置文件错误或缺少必需文件

**解决：** 
1. 检查 `config.json` 格式
2. 检查 `.env` 文件是否存在且包含必需变量
3. 检查 `secrets/rsa_key` 是否存在
4. 查看详细日志：`docker compose logs nofx`

### 问题4: 前端502错误

**原因：** 后端服务未正常启动

**解决：** 先修复后端问题，后端启动后前端会自动恢复

## 📝 注意事项

1. **密钥一致性**：服务器上的密钥必须与本地完全一致
2. **文件权限**：确保文件权限正确（secrets 700，.env 和 config.db 600）
3. **首次启动**：系统会自动初始化数据库，可能需要一些时间
4. **Web配置**：v3.0.0版本支持通过Web界面配置AI模型和交易所，无需编辑JSON






