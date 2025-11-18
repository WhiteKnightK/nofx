# 🔧 修复数据库初始化失败问题

## 问题：`unable to open database file: out of memory (14)`

这个错误通常是文件系统权限或路径问题，不是真的内存不足。

---

## 🔍 诊断步骤

### 1. 检查目录权限和磁盘空间

```bash
cd ~/nofx

# 检查磁盘空间
df -h .

# 检查目录权限
ls -la
pwd

# 检查当前用户
whoami

# 检查目录是否可写
touch test_write.txt && rm test_write.txt && echo "✓ 目录可写" || echo "✗ 目录不可写"
```

### 2. 检查 Docker volume 挂载

```bash
# 查看完整的 volume 配置
docker compose -f docker-compose.prod.yml config | grep -A 10 volumes

# 检查容器内目录权限
docker compose -f docker-compose.prod.yml run --rm nofx ls -la /app/
```

---

## 🚀 解决方案

### 方案1：修复目录权限（最常见）

```bash
cd ~/nofx

# 停止容器
docker compose -f docker-compose.prod.yml down

# 删除所有数据库相关文件
rm -f config.db config.db-wal config.db-shm

# 修复目录权限
chmod 755 ~/nofx
chmod 755 ~/nofx/decision_logs
chmod 755 ~/nofx/prompts

# 确保当前用户拥有目录
sudo chown -R $USER:$USER ~/nofx

# 重新启动
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a && source .env && set +a
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml logs -f nofx
```

### 方案2：使用绝对路径（如果相对路径有问题）

```bash
cd ~/nofx

# 获取绝对路径
FULL_PATH=$(pwd)
echo "绝对路径: $FULL_PATH"

# 停止容器
docker compose -f docker-compose.prod.yml down

# 备份现有配置
cp docker-compose.prod.yml docker-compose.prod.yml.bak

# 修改为绝对路径（手动编辑或使用 sed）
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
      - /home/ubuntu/nofx/config.json:/app/config.json:ro
      - /home/ubuntu/nofx/config.db:/app/config.db
      - /home/ubuntu/nofx/beta_codes.txt:/app/beta_codes.txt:ro
      - /home/ubuntu/nofx/decision_logs:/app/decision_logs
      - /home/ubuntu/nofx/prompts:/app/prompts
      - /home/ubuntu/nofx/secrets:/app/secrets:ro
      - /etc/localtime:/etc/localtime:ro
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

### 方案3：检查是否是 SELinux 问题（如果使用 SELinux）

```bash
# 检查 SELinux 状态
getenforce

# 如果是 Enforcing，临时设置为 Permissive 测试
sudo setenforce 0

# 然后重启容器测试
docker compose -f docker-compose.prod.yml restart nofx

# 如果解决了，永久设置（需要重启系统）
# sudo sed -i 's/SELINUX=enforcing/SELINUX=permissive/' /etc/selinux/config
```

### 方案4：在容器内手动创建数据库测试

```bash
# 进入容器
docker compose -f docker-compose.prod.yml run --rm nofx /bin/sh

# 在容器内执行
cd /app
touch test_db.db
sqlite3 test_db.db "CREATE TABLE test (id INTEGER);"
ls -la test_db.db
rm test_db.db
exit
```

如果容器内可以创建文件，说明问题在宿主机权限。

---

## 📋 完整修复命令（推荐）

```bash
cd ~/nofx

# 1. 停止所有容器
docker compose -f docker-compose.prod.yml down

# 2. 清理数据库文件
rm -f config.db config.db-wal config.db-shm

# 3. 修复权限
sudo chown -R $USER:$USER ~/nofx
chmod 755 ~/nofx
chmod 755 ~/nofx/decision_logs
chmod 755 ~/nofx/prompts
chmod 700 ~/nofx/secrets

# 4. 检查磁盘空间
df -h . | head -2

# 5. 确保提示词文件存在
ls -la prompts/*.txt || echo "提示词文件不存在，需要创建"

# 6. 使用绝对路径重新创建 docker-compose.prod.yml
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
      - /home/ubuntu/nofx/config.json:/app/config.json:ro
      - /home/ubuntu/nofx/config.db:/app/config.db
      - /home/ubuntu/nofx/beta_codes.txt:/app/beta_codes.txt:ro
      - /home/ubuntu/nofx/decision_logs:/app/decision_logs
      - /home/ubuntu/nofx/prompts:/app/prompts
      - /home/ubuntu/nofx/secrets:/app/secrets:ro
      - /etc/localtime:/etc/localtime:ro
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

# 7. 重新启动
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a && source .env && set +a
docker compose -f docker-compose.prod.yml up -d

# 8. 查看日志
docker compose -f docker-compose.prod.yml logs -f nofx
```

---

## 🔍 关于日志中的 "ET \"/api/my-traders\""

这是日志输出被截断了，完整应该是：
```
[GIN] 2025/11/10 - 01:27:02 | 200 | 373.48µs | 82.26.72.133 | GET "/api/my-traders"
```

这是正常的 API 请求日志，表示：
- 时间：2025/11/10 01:27:02
- 状态码：200（成功）
- 响应时间：373.48 微秒
- 客户端IP：82.26.72.133
- 请求方法：GET
- 请求路径：/api/my-traders

这个不是错误，是正常的 API 访问日志。

---

## ⚠️ 如果还是失败

检查以下几点：

1. **磁盘空间**：`df -h` 确保有足够空间
2. **文件系统类型**：某些网络文件系统可能不支持 SQLite WAL 模式
3. **Docker 版本**：确保 Docker 版本不是太旧
4. **容器用户权限**：检查容器内运行的用户是否有权限写入

