#!/bin/bash
# 快速构建并推送脚本（优化版）

set -e

echo "=========================================="
echo "🚀 NOFX 快速构建并推送到 Docker Hub"
echo "=========================================="

# 检查 Docker Hub 用户名
if [ -z "$DOCKERHUB_USERNAME" ]; then
    read -p "请输入您的 Docker Hub 用户名: " DOCKERHUB_USERNAME
    export DOCKERHUB_USERNAME
fi

# 检查是否已登录
if ! docker info 2>/dev/null | grep -q "Username"; then
    echo "🔐 请先登录 Docker Hub"
    docker login
fi

# 启用 BuildKit 加速构建
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

echo ""
echo "🔨 开始构建镜像..."
echo "   - 后端镜像（Go + TA-Lib）"
echo "   - 前端镜像（React + Nginx）"
echo ""

# 构建镜像（并行构建）
docker compose build --parallel

echo ""
echo "📝 打标签..."
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:latest

echo ""
echo "📤 推送镜像到 Docker Hub..."
docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest

echo ""
echo "=========================================="
echo "✅ 构建和推送完成！"
echo "=========================================="
echo ""
echo "📦 镜像地址："
echo "   - ${DOCKERHUB_USERNAME}/nofx-backend:latest"
echo "   - ${DOCKERHUB_USERNAME}/nofx-frontend:latest"
echo ""
echo "🌐 在服务器上执行以下命令："
echo ""
echo "   export DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME}"
echo "   docker compose -f docker-compose.prod.yml up -d"
echo ""
echo "   或者使用 server_deploy.sh 脚本"
echo ""






