#!/bin/bash
# 本地构建镜像并推送到Docker Hub

set -e

echo "=========================================="
echo "本地构建NOFX镜像并推送到Docker Hub"
echo "=========================================="

# 检查是否设置了Docker Hub用户名
if [ -z "$DOCKERHUB_USERNAME" ]; then
    echo "⚠️  请先设置Docker Hub用户名"
    read -p "请输入您的Docker Hub用户名: " DOCKERHUB_USERNAME
    export DOCKERHUB_USERNAME
fi

# 检查Docker是否已登录
if ! docker info | grep -q "Username"; then
    echo "🔐 请先登录Docker Hub"
    docker login
fi

# 启用 BuildKit 加速构建
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

echo ""
echo "🔨 步骤1: 构建后端镜像..."
docker compose build --progress=plain nofx

echo ""
echo "🔨 步骤1.5: 构建前端镜像..."
docker compose build --progress=plain nofx-frontend

echo ""
echo "📝 步骤2: 打标签..."
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:latest

echo ""
echo "📤 步骤3: 推送镜像到Docker Hub..."
docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest

echo ""
echo "=========================================="
echo "✅ 构建和推送完成！"
echo "=========================================="
echo ""
echo "镜像地址："
echo "  - ${DOCKERHUB_USERNAME}/nofx-backend:latest"
echo "  - ${DOCKERHUB_USERNAME}/nofx-frontend:latest"
echo ""
echo "在服务器上使用时，修改docker-compose.yml中的镜像地址即可"


