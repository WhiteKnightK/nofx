# 🔧 修复 SQLite WAL 模式错误

## 问题：`unable to open database file: out of memory (14)`

这个错误通常是因为：
1. 文件系统不支持 WAL 模式（如某些网络文件系统）
2. 目录权限问题
3. Docker volume 挂载问题

---

## 🔍 诊断步骤

### 1. 检查文件系统类型

```bash
df -T ~/nofx-new
mount | grep $(df ~/nofx-new | tail -1 | awk '{print $1}')
```

### 2. 检查容器内权限

```bash
cd ~/nofx-new

# 进入容器检查
docker compose -f docker-compose.prod.yml run --rm nofx ls -la /app/

# 尝试在容器内创建文件
docker compose -f docker-compose.prod.yml run --rm nofx touch /app/test_write.txt
docker compose -f docker-compose.prod.yml run --rm nofx ls -la /app/test_write.txt
docker compose -f docker-compose.prod.yml run --rm nofx rm /app/test_write.txt
```

### 3. 检查目录权限

```bash
cd ~/nofx-new
ls -la
pwd
whoami
```

---

## 🚀 解决方案

### 方案1：修复权限并确保目录可写

```bash
cd ~/nofx-new

# 停止容器
docker compose -f docker-compose.prod.yml down

# 确保目录权限正确
sudo chown -R $USER:$USER ~/nofx-new
chmod 755 ~/nofx-new
chmod 755 ~/nofx-new/decision_logs

# 确保目录可写
touch ~/nofx-new/test_write && rm ~/nofx-new/test_write && echo "✓ 目录可写" || echo "✗ 目录不可写"

# 重新启动
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a && source .env && set +a
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml logs -f nofx
```

### 方案2：使用 tmpfs 挂载数据库（临时解决）

如果文件系统不支持 WAL，可以临时使用内存文件系统：

```bash
cd ~/nofx-new

# 停止容器
docker compose -f docker-compose.prod.yml down

# 修改 docker-compose.prod.yml，将 config.db 改为 tmpfs
# 注意：这样数据不会持久化，重启会丢失
cat > docker-compose.prod.yml << 'COMPOSEEOF'
services:
  nofx:
    image: baimastryke/nofx-backend:${IMAGE_TAG:-latest}
    container_name: nofx-trading
    restart: unless-stopped
    stop_grace_period: 30s
    ports:
      - "8080:8080"
    volumes:
      - /home/ubuntu/nofx-new/config.json:/app/config.json:ro
      - /home/ubuntu/nofx-new/beta_codes.txt:/app/beta_codes.txt:ro
      - /home/ubuntu/nofx-new/decision_logs:/app/decision_logs
      - /home/ubuntu/nofx-new/prompts:/app/prompts
      - /home/ubuntu/nofx-new/secrets:/app/secrets:ro
      - /etc/localtime:/etc/localtime:ro
    tmpfs:
      - /app/config.db:rw,noexec,nosuid,size=100m
    environment:
      - TZ=Asia/Shanghai
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
    image: baimastryke/nofx-frontend:${IMAGE_TAG:-latest}
    container_name: nofx-frontend
    restart: unless-stopped
    ports:
      - "3000:80"
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
COMPOSEEOF

# 重新启动
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a && source .env && set +a
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml logs -f nofx
```

**注意**：方案2 使用 tmpfs，数据不会持久化，重启容器会丢失数据。仅用于测试。

### 方案3：检查是否是 Docker 版本或配置问题

```bash
# 检查 Docker 版本
docker --version

# 检查 Docker 信息
docker info | grep -i "storage\|filesystem"

# 检查是否有 AppArmor 或 SELinux 限制
getenforce 2>/dev/null || echo "SELinux not installed"
```

### 方案4：尝试在容器内手动创建数据库

```bash
cd ~/nofx-new

# 进入容器
docker compose -f docker-compose.prod.yml run --rm nofx /bin/sh

# 在容器内执行
cd /app
sqlite3 config.db "PRAGMA journal_mode=WAL;"
exit
```

如果容器内也失败，说明是文件系统或 Docker 配置问题。

---

## 🔧 最可能有效的解决方案

基于错误信息，最可能的原因是文件系统权限或 Docker volume 挂载问题。尝试以下步骤：

```bash
cd ~/nofx-new

# 1. 完全停止并清理
docker compose -f docker-compose.prod.yml down
docker system prune -f

# 2. 检查并修复所有权限
sudo chown -R ubuntu:ubuntu ~/nofx-new
chmod -R 755 ~/nofx-new
chmod 700 ~/nofx-new/secrets
chmod 600 ~/nofx-new/.env ~/nofx-new/config.json

# 3. 确保目录可写
sudo chmod 1777 ~/nofx-new/decision_logs 2>/dev/null || chmod 755 ~/nofx-new/decision_logs

# 4. 检查磁盘空间和 inode
df -h ~/nofx-new
df -i ~/nofx-new

# 5. 尝试不使用 volume，直接在容器内创建数据库（测试）
docker compose -f docker-compose.prod.yml run --rm nofx sqlite3 /tmp/test.db "PRAGMA journal_mode=WAL; SELECT 1;"

# 6. 如果测试成功，说明是 volume 挂载问题
# 重新启动服务
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a && source .env && set +a
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml logs -f nofx
```

---

## 💡 如果所有方案都失败

可能需要修改代码临时禁用 WAL 模式，但这需要重新构建镜像。或者联系我，我可以帮你修改代码。






