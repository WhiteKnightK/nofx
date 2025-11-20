# 🚀 本地构建镜像并部署到服务器

## 📋 完整操作流程

### 步骤1: 设置环境变量

```bash
cd /home/master/code/nofx/nofx

# 设置Docker Hub用户名（你的用户名）
export DOCKERHUB_USERNAME=baimastryke

# 设置镜像标签（建议使用日期，如：2025-11-20）
export IMAGE_TAG=$(date +%Y-%m-%d)
# 或者手动指定：
# export IMAGE_TAG=2025-11-20
```

### 步骤2: 登录Docker Hub

```bash
# 登录Docker Hub（需要输入用户名和密码）
docker login
```

### 步骤3: 构建镜像

**方式1: 使用docker compose构建（推荐）**

```bash
# 启用BuildKit加速构建
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

# 构建后端镜像
docker compose build --progress=plain nofx

# 构建前端镜像
docker compose build --progress=plain nofx-frontend
```

**方式2: 使用docker build直接构建**

```bash
# 构建后端镜像
docker build -f docker/Dockerfile.backend -t ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG} .

# 构建前端镜像
docker build -f docker/Dockerfile.frontend -t ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG} .
```

### 步骤4: 打标签（如果需要latest标签）

```bash
# 为后端镜像打latest标签
docker tag ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG} ${DOCKERHUB_USERNAME}/nofx-backend:latest

# 为前端镜像打latest标签
docker tag ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG} ${DOCKERHUB_USERNAME}/nofx-frontend:latest
```

### 步骤5: 推送镜像到Docker Hub

```bash
# 推送后端镜像（带版本标签）
docker push ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG}

# 推送后端镜像（latest标签）
docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest

# 推送前端镜像（带版本标签）
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG}

# 推送前端镜像（latest标签）
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest
```

### 步骤6: 在服务器上更新镜像

```bash
# SSH连接到服务器
ssh -i /home/master/code/nofx/A.pem ubuntu@43.202.115.56

# 进入项目目录
cd /home/ubuntu/nofx

# 更新.env文件中的IMAGE_TAG（如果需要使用特定版本）
# 或者直接使用latest标签

# 拉取最新镜像
docker compose -f docker-compose.prod.yml pull

# 重启服务
docker compose -f docker-compose.prod.yml up -d

# 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

---

## 🎯 一键脚本（推荐）

### 使用现有脚本

```bash
cd /home/master/code/nofx/nofx

# 设置环境变量
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=$(date +%Y-%m-%d)

# 运行构建脚本（需要先修改脚本以支持IMAGE_TAG）
./build_and_push.sh
```

### 完整一键命令

```bash
cd /home/master/code/nofx/nofx

# 设置变量
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=$(date +%Y-%m-%d)
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

# 登录（如果未登录）
docker login

# 构建并推送
echo "🔨 构建后端镜像..."
docker compose build --progress=plain nofx
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG}
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:latest

echo "🔨 构建前端镜像..."
docker compose build --progress=plain nofx-frontend
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG}
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:latest

echo "📤 推送镜像..."
docker push ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG}
docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG}
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest

echo "✅ 完成！镜像标签: ${IMAGE_TAG}"
```

---

## 📝 服务器端操作

### 更新服务器上的.env文件（如果需要特定版本）

```bash
ssh -i /home/master/code/nofx/A.pem ubuntu@43.202.115.56 "cd /home/ubuntu/nofx && sed -i 's/IMAGE_TAG=.*/IMAGE_TAG=2025-11-20/' .env"
```

### 或者直接使用latest标签（推荐）

服务器上的`.env`文件已经设置了`IMAGE_TAG=latest`，所以直接拉取即可：

```bash
ssh -i /home/master/code/nofx/A.pem ubuntu@43.202.115.56 "cd /home/ubuntu/nofx && docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d"
```

---

## ⚠️ 注意事项

1. **构建时间**: 后端镜像构建可能需要10-20分钟（需要编译TA-Lib和Go代码）
2. **网络**: 确保网络连接稳定，构建过程中需要下载依赖
3. **磁盘空间**: 确保有足够的磁盘空间（至少5GB）
4. **Docker Hub配额**: 免费账户有推送限制，注意不要超过

---

## 🔍 验证镜像

### 本地验证

```bash
# 查看本地镜像
docker images | grep nofx

# 测试运行（可选）
docker run --rm ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG} --version
```

### Docker Hub验证

访问: https://hub.docker.com/r/baimastryke/nofx-backend/tags
访问: https://hub.docker.com/r/baimastryke/nofx-frontend/tags

---

## 🆘 常见问题

### Q: 构建失败怎么办？
**A**: 检查：
1. Docker是否正常运行: `docker info`
2. 网络连接是否正常
3. 磁盘空间是否充足: `df -h`
4. 查看详细错误: `docker compose build --progress=plain --no-cache nofx`

### Q: 推送失败怎么办？
**A**: 检查：
1. 是否已登录: `docker login`
2. Docker Hub用户名是否正确
3. 镜像名称是否正确
4. 是否有推送权限

### Q: 如何只构建一个镜像？
**A**: 
```bash
# 只构建后端
docker compose build nofx

# 只构建前端
docker compose build nofx-frontend
```

### Q: 如何清理构建缓存？
**A**:
```bash
# 清理所有未使用的构建缓存
docker builder prune -a

# 清理所有未使用的镜像
docker image prune -a
```

