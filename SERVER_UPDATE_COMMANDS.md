# 🚀 服务器更新命令（直接复制执行）

## 步骤1：停止并删除现有容器

```bash
cd ~/nofx
docker compose -f docker-compose.prod.yml down
```

## 步骤2：删除旧镜像（可选，释放空间）

```bash
# 删除本地镜像（保留已推送的标签）
docker rmi baimastryke/nofx-backend:latest baimastryke/nofx-frontend:latest 2>/dev/null || true

# 或者删除所有 nofx 相关镜像（包括日期标签）
docker images | grep -E "baimastryke/nofx" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true
```

## 步骤3：设置环境变量并强制拉取新镜像（不使用缓存）

```bash
export DOCKERHUB_USERNAME=baimastryke
set -a
source .env
set +a

# 强制拉取最新镜像（不使用本地缓存）
docker compose -f docker-compose.prod.yml pull --ignore-pull-failures

# 或者使用日期标签确保拉取最新版本（推荐）
export IMAGE_TAG=2025-11-10
docker compose -f docker-compose.prod.yml pull
```

## 步骤4：启动服务

```bash
docker compose -f docker-compose.prod.yml up -d
```

## 步骤5：查看日志确认启动成功

```bash
docker compose -f docker-compose.prod.yml logs -f
```

---

## 📋 一键执行（复制全部，强制拉取最新）

```bash
cd ~/nofx && \
export DOCKERHUB_USERNAME=baimastryke && \
export IMAGE_TAG=2025-11-10 && \
set -a && source .env && set +a && \
docker compose -f docker-compose.prod.yml down && \
docker compose -f docker-compose.prod.yml pull && \
docker compose -f docker-compose.prod.yml up -d && \
docker compose -f docker-compose.prod.yml logs -f
```

**注意：** 将 `IMAGE_TAG=2025-11-10` 改为你推送镜像时的日期标签

---

## 🔍 检查状态命令

```bash
# 查看容器状态
docker compose -f docker-compose.prod.yml ps

# 查看镜像
docker images | grep baimastryke/nofx

# 查看日志（实时）
docker compose -f docker-compose.prod.yml logs -f

# 查看特定服务日志
docker compose -f docker-compose.prod.yml logs -f nofx
docker compose -f docker-compose.prod.yml logs -f nofx-frontend
```

---

## 🐛 如果出现问题

### 问题1：容器无法启动

```bash
# 查看详细错误
docker compose -f docker-compose.prod.yml logs nofx

# 检查配置文件
cat config.json | python3 -m json.tool

# 检查文件权限
ls -la config.json config.db .env secrets/
```

### 问题2：镜像拉取失败

```bash
# 重新登录 Docker Hub
docker logout
docker login

# 再次拉取
export DOCKERHUB_USERNAME=baimastryke
docker compose -f docker-compose.prod.yml pull
```

### 问题3：端口被占用

```bash
# 检查端口占用
netstat -tulpn | grep -E '8080|3000'

# 或者修改 .env 文件中的端口
nano .env
# 修改 NOFX_BACKEND_PORT 和 NOFX_FRONTEND_PORT
```

---

## ⚠️ 重要提示

1. **数据不会丢失**：`config.db` 和 `decision_logs/` 在 volume 中，删除容器不会影响数据
2. **配置文件需要存在**：确保 `config.json`、`.env`、`secrets/` 等文件存在
3. **首次部署**：如果是首次部署，需要先上传配置文件（参考 SERVER_SETUP.md）

