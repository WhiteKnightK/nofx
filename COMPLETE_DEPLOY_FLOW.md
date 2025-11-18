# 🚀 完整部署流程：本地开发 → 构建 → 推送 → 服务器部署

## 📋 流程概览

```
本地开发 → 配置检查 → 构建镜像 → 推送镜像 → 服务器拉取 → 服务器运行
```

---

## 阶段一：本地准备和配置

### 1.1 确保项目文件完整

```bash
cd ~/code/nofx/nofx

# 检查必需文件是否存在
ls -la config.json .env secrets/rsa_key secrets/rsa_key.pub
```

### 1.2 配置 `config.json`（如果还没有）

```bash
# 如果 config.json 不存在或需要重置
cp config.json.example config.json

# 编辑配置文件（使用你喜欢的编辑器）
nano config.json
# 或
vim config.json
```

**最小配置示例：**
```json
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
  "jwt_secret": "你的JWT密钥（至少64字符）",
  "log": {
    "level": "info"
  }
}
```

### 1.3 配置 `.env` 文件

```bash
# 检查 .env 文件
cat .env

# 确保包含以下变量：
# - DATA_ENCRYPTION_KEY（数据加密密钥）
# - JWT_SECRET（JWT认证密钥）
# - NOFX_FRONTEND_PORT（前端端口，默认3000）
# - NOFX_BACKEND_PORT（后端端口，默认8080）
# - NOFX_TIMEZONE（时区，默认Asia/Shanghai）
```

**`.env` 文件示例：**
```bash
# Docker Hub 用户名（用于推送和拉取镜像）
DOCKERHUB_USERNAME=baimastryke

# 端口配置
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080

# 时区
NOFX_TIMEZONE=Asia/Shanghai

# 数据加密密钥（必须，至少32字符）
DATA_ENCRYPTION_KEY=你的数据加密密钥

# JWT认证密钥（必须，至少64字符）
JWT_SECRET=你的JWT密钥
```

### 1.4 确保 RSA 密钥存在

```bash
# 检查密钥文件
ls -la secrets/rsa_key secrets/rsa_key.pub

# 如果不存在，生成新的密钥对
mkdir -p secrets
chmod 700 secrets
# 使用项目提供的脚本生成（如果有）
# 或手动生成：
openssl genrsa -out secrets/rsa_key 2048
openssl rsa -in secrets/rsa_key -pubout -out secrets/rsa_key.pub
chmod 600 secrets/rsa_key
chmod 644 secrets/rsa_key.pub
```

### 1.5 确保提示词文件存在（可选但推荐）

```bash
# 检查提示词目录
ls -la prompts/

# 如果为空，创建默认提示词
mkdir -p prompts
cat > prompts/default.txt << 'EOF'
你是一个专业的加密货币交易AI助手。
EOF
```

### 1.6 验证配置完整性

```bash
# 验证 JSON 格式
cat config.json | python3 -m json.tool > /dev/null && echo "✓ config.json 格式正确" || echo "✗ config.json 格式错误"

# 检查必需文件
[ -f config.json ] && echo "✓ config.json 存在" || echo "✗ config.json 不存在"
[ -f .env ] && echo "✓ .env 存在" || echo "✗ .env 不存在"
[ -f secrets/rsa_key ] && echo "✓ RSA私钥存在" || echo "✗ RSA私钥不存在"
[ -f secrets/rsa_key.pub ] && echo "✓ RSA公钥存在" || echo "✗ RSA公钥不存在"
```

---

## 阶段二：本地构建镜像

### 2.1 登录 Docker Hub

```bash
docker login
# 输入你的 Docker Hub 用户名和密码
```

### 2.2 构建镜像

```bash
# 设置用户名（如果还没设置）
export DOCKERHUB_USERNAME=baimastryke

# 构建镜像（这会构建后端和前端）
./start.sh start --build

# 或者使用 docker compose 直接构建
docker compose build
```

### 2.3 验证镜像构建成功

```bash
# 检查镜像是否存在
docker images | grep nofx

# 应该看到：
# nofx-nofx:latest
# nofx-nofx-frontend:latest
```

---

## 阶段三：推送镜像到 Docker Hub

### 3.1 推送镜像（自动添加日期标签）

```bash
# 设置用户名
export DOCKERHUB_USERNAME=baimastryke

# 运行推送脚本（会自动添加日期标签）
./push_images.sh
```

**推送脚本会：**
- 给镜像打两个标签：`latest` 和 `YYYY-MM-DD`（如 `2024-12-15`）
- 推送到 Docker Hub
- 显示推送的镜像地址

### 3.2 验证推送成功

访问 Docker Hub 网站确认：
- `https://hub.docker.com/r/baimastryke/nofx-backend`
- `https://hub.docker.com/r/baimastryke/nofx-frontend`

---

## 阶段四：服务器首次部署

### 4.1 准备服务器环境

```bash
# SSH 连接到服务器
ssh user@your-server

# 创建项目目录
mkdir -p ~/nofx
cd ~/nofx
```

### 4.2 上传配置文件到服务器

**方式一：使用 scp（推荐）**

在**本地**执行：

```bash
# 从本地上传必需文件到服务器
scp config.json user@your-server:~/nofx/
scp .env user@your-server:~/nofx/
scp -r secrets user@your-server:~/nofx/
scp -r prompts user@your-server:~/nofx/  # 可选

# 设置服务器上的文件权限
ssh user@your-server "cd ~/nofx && chmod 600 .env config.json && chmod 700 secrets && chmod 600 secrets/rsa_key"
```

**方式二：手动创建（如果无法上传文件）**

在**服务器**上执行：

```bash
# 创建 config.json（需要手动输入内容）
nano config.json
# 粘贴本地 config.json 的内容

# 创建 .env（需要手动输入内容）
nano .env
# 粘贴本地 .env 的内容

# 创建 secrets 目录
mkdir -p secrets
chmod 700 secrets

# 然后需要手动创建或上传密钥文件
# 可以通过 cat > secrets/rsa_key 然后粘贴内容
```

### 4.3 在服务器上创建必要目录和文件

```bash
# 创建目录
mkdir -p decision_logs

# 创建空数据库文件
touch config.db beta_codes.txt

# 设置权限
chmod 600 config.db
```

### 4.4 创建 docker-compose.prod.yml

```bash
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

### 4.5 拉取镜像并启动

```bash
# 设置环境变量
export DOCKERHUB_USERNAME=baimastryke

# 加载 .env 文件中的变量
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

## 阶段五：更新部署（代码更新后）

### 5.1 本地更新代码

```bash
cd ~/code/nofx/nofx

# 拉取最新代码
git pull

# 检查配置是否有变化
git diff config.json.example  # 如果有新的配置项，需要更新 config.json
```

### 5.2 更新配置（如果需要）

```bash
# 如果 config.json.example 有更新，检查是否需要同步
diff config.json.example config.json

# 如果有新字段，手动添加到 config.json
```

### 5.3 重新构建和推送

```bash
# 构建新镜像
./start.sh start --build

# 推送镜像（会自动添加新的日期标签）
export DOCKERHUB_USERNAME=baimastryke
./push_images.sh
```

### 5.4 服务器上更新

```bash
# SSH 到服务器
ssh user@your-server
cd ~/nofx

# 设置环境变量
export DOCKERHUB_USERNAME=baimastryke
set -a
source .env
set +a

# 拉取最新镜像
docker compose -f docker-compose.prod.yml pull

# 重启服务（使用新镜像）
docker compose -f docker-compose.prod.yml up -d

# 查看日志确认更新成功
docker compose -f docker-compose.prod.yml logs -f
```

---

## 📝 完整命令清单（快速参考）

### 本地操作

```bash
# 1. 准备配置
cd ~/code/nofx/nofx
# 确保 config.json, .env, secrets/ 都存在且正确

# 2. 构建镜像
./start.sh start --build

# 3. 推送镜像
export DOCKERHUB_USERNAME=baimastryke
./push_images.sh
```

### 服务器操作（首次）

```bash
# 1. 上传文件（在本地执行）
scp config.json .env user@server:~/nofx/
scp -r secrets prompts user@server:~/nofx/

# 2. 在服务器上准备
ssh user@server
cd ~/nofx
mkdir -p decision_logs
touch config.db beta_codes.txt
chmod 600 config.db .env
chmod 700 secrets

# 3. 创建 docker-compose.prod.yml（使用上面的内容）

# 4. 拉取并启动
export DOCKERHUB_USERNAME=baimastryke
set -a && source .env && set +a
docker login
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### 服务器操作（更新）

```bash
# 在服务器上执行
cd ~/nofx
export DOCKERHUB_USERNAME=baimastryke
set -a && source .env && set +a
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml logs -f
```

---

## 🔄 自动化脚本（可选）

### 本地一键推送脚本

创建 `local_deploy.sh`：

```bash
#!/bin/bash
set -e

echo "🚀 开始本地部署流程..."

# 检查配置
echo "📋 检查配置文件..."
[ -f config.json ] || { echo "❌ config.json 不存在"; exit 1; }
[ -f .env ] || { echo "❌ .env 不存在"; exit 1; }
[ -f secrets/rsa_key ] || { echo "❌ RSA密钥不存在"; exit 1; }

# 构建镜像
echo "🔨 构建镜像..."
./start.sh start --build

# 推送镜像
echo "📤 推送镜像..."
export DOCKERHUB_USERNAME=baimastryke
./push_images.sh

echo "✅ 本地部署完成！"
echo "📝 下一步：在服务器上执行更新命令"
```

### 服务器一键更新脚本

在服务器上创建 `server_update.sh`：

```bash
#!/bin/bash
set -e

cd ~/nofx

export DOCKERHUB_USERNAME=baimastryke
set -a
source .env
set +a

echo "📥 拉取最新镜像..."
docker compose -f docker-compose.prod.yml pull

echo "🔄 重启服务..."
docker compose -f docker-compose.prod.yml up -d

echo "✅ 更新完成！"
docker compose -f docker-compose.prod.yml ps
```

---

## ⚠️ 重要注意事项

1. **配置文件一致性**：服务器上的 `config.json` 和 `.env` 必须与本地一致（特别是密钥）
2. **密钥安全**：不要将包含真实密钥的文件提交到 Git
3. **文件权限**：确保服务器上文件权限正确（secrets 700，.env 和 config.db 600）
4. **首次部署**：首次部署需要上传配置文件，之后更新只需要拉取新镜像
5. **数据库备份**：更新前建议备份 `config.db` 文件

---

## 🐛 故障排查

### 问题：服务器上配置文件丢失

**解决：** 从本地上传：
```bash
scp config.json .env user@server:~/nofx/
```

### 问题：镜像拉取失败

**解决：** 
```bash
docker logout
docker login
docker compose -f docker-compose.prod.yml pull
```

### 问题：服务启动失败

**解决：** 查看日志：
```bash
docker compose -f docker-compose.prod.yml logs nofx
```

检查配置文件格式：
```bash
cat config.json | python3 -m json.tool
```

---

## 📚 相关文档

- [SERVER_COMMANDS.md](./SERVER_COMMANDS.md) - 服务器直接执行命令
- [DEPLOY_TO_SERVER.md](./DEPLOY_TO_SERVER.md) - 详细部署文档
- [SERVER_SETUP.md](./SERVER_SETUP.md) - 服务器准备清单

