# 服务器端直接执行命令（无需脚本文件）

## 🚀 快速部署命令

### 方式一：使用 latest 标签（推荐，最简单）

```bash
# 1. 设置 Docker Hub 用户名
export DOCKERHUB_USERNAME=baimastryke

# 2. 登录 Docker Hub（首次需要）
docker login

# 3. 创建 docker-compose.prod.yml 文件
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

# 4. 确保必要的目录和文件存在
mkdir -p secrets decision_logs prompts
touch config.json config.db beta_codes.txt
chmod 700 secrets
chmod 600 .env config.db 2>/dev/null || true

# 5. 拉取镜像
docker compose -f docker-compose.prod.yml pull

# 6. 启动服务
docker compose -f docker-compose.prod.yml up -d

# 7. 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

### 方式二：使用特定日期标签（如 2024-12-15）

```bash
# 1. 设置 Docker Hub 用户名和镜像标签
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2024-12-15

# 2. 登录 Docker Hub（首次需要）
docker login

# 3. 创建 docker-compose.prod.yml 文件（同上）
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

# 4. 确保必要的目录和文件存在
mkdir -p secrets decision_logs prompts
touch config.json config.db beta_codes.txt
chmod 700 secrets
chmod 600 .env config.db 2>/dev/null || true

# 5. 拉取镜像
docker compose -f docker-compose.prod.yml pull

# 6. 启动服务
docker compose -f docker-compose.prod.yml up -d

# 7. 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

## 📋 必需文件说明

在执行上述命令前，确保以下文件已准备好：

### 必须上传的文件：
1. **`.env`** - 环境变量文件，必须包含：
   ```bash
   DOCKERHUB_USERNAME=baimastryke
   DATA_ENCRYPTION_KEY=你的加密密钥
   JWT_SECRET=你的JWT密钥
   NOFX_FRONTEND_PORT=3000
   NOFX_BACKEND_PORT=8080
   NOFX_TIMEZONE=Asia/Shanghai
   ```

2. **`config.json`** - 配置文件（如果不存在，会创建空文件，需要后续配置）

3. **`secrets/rsa_key`** - RSA 私钥文件

4. **`secrets/rsa_key.pub`** - RSA 公钥文件

### 可选文件：
- `config.db` - 数据库文件（如果已有）
- `beta_codes.txt` - Beta码文件
- `prompts/` - 提示词目录

## 🔧 常用管理命令

### 查看服务状态
```bash
docker compose -f docker-compose.prod.yml ps
```

### 查看日志
```bash
# 查看所有服务日志
docker compose -f docker-compose.prod.yml logs -f

# 查看特定服务日志
docker compose -f docker-compose.prod.yml logs -f nofx
docker compose -f docker-compose.prod.yml logs -f nofx-frontend
```

### 停止服务
```bash
docker compose -f docker-compose.prod.yml stop
```

### 重启服务
```bash
docker compose -f docker-compose.prod.yml restart
```

### 停止并删除容器
```bash
docker compose -f docker-compose.prod.yml down
```

### 更新镜像（拉取最新版本）
```bash
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=latest  # 或指定日期标签，如 2024-12-15
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### 检查健康状态
```bash
curl http://localhost:8080/api/health
```

## ⚠️ 注意事项

1. **文件权限**：确保 `secrets/` 目录权限为 700，`.env` 和 `config.db` 权限为 600
2. **端口占用**：确保 8080 和 3000 端口未被占用
3. **Docker 登录**：首次使用需要 `docker login`，之后可以保存凭据
4. **环境变量**：`.env` 文件中的 `DATA_ENCRYPTION_KEY` 和 `JWT_SECRET` 必须与本地一致

## 🐛 故障排查

### 镜像拉取失败
```bash
# 检查是否登录
docker info | grep Username

# 重新登录
docker logout
docker login
```

### 服务启动失败
```bash
# 查看详细错误信息
docker compose -f docker-compose.prod.yml logs

# 检查文件是否存在
ls -la config.json config.db secrets/rsa_key secrets/rsa_key.pub
```

### 端口被占用
```bash
# 检查端口占用
netstat -tulpn | grep -E '8080|3000'

# 或修改 .env 文件中的端口配置
```







