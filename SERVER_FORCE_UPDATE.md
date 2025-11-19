# 🔄 服务器强制更新镜像命令

## ⚠️ 问题：拉取的还是旧镜像

如果服务器上拉取的还是旧镜像，使用以下命令强制拉取最新版本。

---

## 🚀 方法1：使用日期标签（最可靠）

```bash
cd ~/nofx
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10  # 改为你推送时的日期
set -a
source .env
set +a

# 停止容器
docker compose -f docker-compose.prod.yml down

# 删除旧镜像（强制重新拉取）
docker rmi baimastryke/nofx-backend:latest baimastryke/nofx-frontend:latest 2>/dev/null || true
docker rmi baimastryke/nofx-backend:${IMAGE_TAG} baimastryke/nofx-frontend:${IMAGE_TAG} 2>/dev/null || true

# 拉取新镜像
docker compose -f docker-compose.prod.yml pull

# 启动服务
docker compose -f docker-compose.prod.yml up -d

# 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

---

## 🚀 方法2：强制拉取 latest 标签

```bash
cd ~/nofx
export DOCKERHUB_USERNAME=baimastryke
set -a
source .env
set +a

# 停止容器
docker compose -f docker-compose.prod.yml down

# 删除所有相关镜像（强制重新拉取）
docker images | grep -E "baimastryke/nofx" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true

# 强制拉取（不使用缓存）
docker pull baimastryke/nofx-backend:latest
docker pull baimastryke/nofx-frontend:latest

# 启动服务
docker compose -f docker-compose.prod.yml up -d

# 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

---

## 🔍 检查镜像版本

```bash
# 查看镜像创建时间
docker images baimastryke/nofx-backend --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedAt}}"

# 查看镜像详细信息
docker inspect baimastryke/nofx-backend:latest | grep -E "Created|Id"

# 查看所有标签
docker images | grep baimastryke/nofx
```

---

## 📋 一键强制更新（复制全部）

```bash
cd ~/nofx && \
export DOCKERHUB_USERNAME=baimastryke && \
export IMAGE_TAG=2025-11-10 && \
set -a && source .env && set +a && \
docker compose -f docker-compose.prod.yml down && \
docker images | grep -E "baimastryke/nofx" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true && \
docker compose -f docker-compose.prod.yml pull && \
docker compose -f docker-compose.prod.yml up -d && \
docker compose -f docker-compose.prod.yml logs -f
```

**记得将 `IMAGE_TAG=2025-11-10` 改为你实际推送的日期！**

---

## 🐛 如果还是旧镜像

### 检查1：确认本地已推送最新镜像

在**本地**执行：
```bash
docker images | grep baimastryke/nofx
```

查看镜像创建时间，确认是最新的。

### 检查2：确认推送成功

访问 Docker Hub 网站：
- https://hub.docker.com/r/baimastryke/nofx-backend/tags
- https://hub.docker.com/r/baimastryke/nofx-frontend/tags

查看 `latest` 标签的更新时间。

### 检查3：清除 Docker 缓存

在**服务器**上执行：
```bash
# 清除所有未使用的镜像
docker image prune -a -f

# 清除构建缓存
docker builder prune -a -f

# 然后重新拉取
docker compose -f docker-compose.prod.yml pull
```

---

## 💡 推荐做法

**使用日期标签而不是 latest**，这样可以确保拉取到正确的版本：

```bash
# 在服务器上设置日期标签
export IMAGE_TAG=2025-11-10  # 你推送时的日期

# 然后拉取和启动
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

这样就不会有版本混淆的问题了！







