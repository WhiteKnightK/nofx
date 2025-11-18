# 📄 服务器创建 docker-compose.prod.yml 文件

## ⚠️ 问题：缺少 docker-compose.prod.yml 文件

服务器上需要先创建这个文件才能拉取和运行镜像。

---

## 🚀 创建文件命令（复制全部执行）

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

# 验证文件创建成功
ls -la docker-compose.prod.yml
```

---

## 📋 完整首次部署流程

### 步骤1：创建项目目录和必要文件

```bash
# 创建目录
mkdir -p ~/nofx
cd ~/nofx

# 创建必要的目录
mkdir -p secrets prompts decision_logs

# 创建空文件
touch config.json config.db beta_codes.txt

# 设置权限
chmod 700 secrets decision_logs
chmod 600 config.db
```

### 步骤2：创建 docker-compose.prod.yml（使用上面的命令）

### 步骤3：创建 .env 文件

```bash
cat > .env << 'ENVEOF'
DOCKERHUB_USERNAME=baimastryke
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080
NOFX_TIMEZONE=Asia/Shanghai
DATA_ENCRYPTION_KEY=你的DATA_ENCRYPTION_KEY
JWT_SECRET=你的JWT_SECRET
ENVEOF

chmod 600 .env
```

### 步骤4：创建 config.json

```bash
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
    "BNBUSDT"
  ],
  "api_server_port": 8080,
  "max_daily_loss": 10.0,
  "max_drawdown": 20.0,
  "stop_trading_minutes": 60,
  "jwt_secret": "你的JWT_SECRET（与.env中一致）",
  "log": {
    "level": "info"
  }
}
JSONEOF
```

### 步骤5：创建提示词文件

```bash
cat > prompts/default.txt << 'EOF'
你是一个专业的加密货币交易AI助手。
EOF
```

### 步骤6：上传密钥文件（从本地上传）

在**本地**执行：
```bash
scp secrets/rsa_key user@server:~/nofx/secrets/
scp secrets/rsa_key.pub user@server:~/nofx/secrets/
```

在**服务器**上设置权限：
```bash
chmod 600 ~/nofx/secrets/rsa_key
chmod 644 ~/nofx/secrets/rsa_key.pub
```

### 步骤7：拉取镜像并启动

```bash
cd ~/nofx
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a
source .env
set +a

# 登录 Docker Hub（首次需要）
docker login

# 拉取镜像
docker compose -f docker-compose.prod.yml pull

# 启动服务
docker compose -f docker-compose.prod.yml up -d

# 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

---

## 🔍 验证文件

```bash
cd ~/nofx

# 检查所有必需文件
ls -la docker-compose.prod.yml .env config.json secrets/rsa_key secrets/rsa_key.pub

# 验证 docker-compose 文件格式
docker compose -f docker-compose.prod.yml config
```

---

## ⚠️ 重要提示

1. **docker-compose.prod.yml** 必须存在才能使用 `docker compose` 命令
2. **.env** 文件必须包含 `DATA_ENCRYPTION_KEY` 和 `JWT_SECRET`
3. **config.json** 中的 `jwt_secret` 应该与 `.env` 中的 `JWT_SECRET` 一致
4. **secrets/** 目录必须包含 RSA 密钥对

