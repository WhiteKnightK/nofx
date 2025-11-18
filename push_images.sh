#!/bin/bash
# 推送已构建的镜像到 Docker Hub（不重新构建）
# 会自动添加日期标签，避免覆盖旧版本

set -e

echo "=========================================="
echo "📤 推送 NOFX 镜像到 Docker Hub"
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

# 检查镜像是否存在
if ! docker images | grep -q "nofx-nofx.*latest"; then
    echo "❌ 错误: 找不到 nofx-nofx:latest 镜像"
    echo "请先运行: ./start.sh start --build"
    exit 1
fi

if ! docker images | grep -q "nofx-nofx-frontend.*latest"; then
    echo "❌ 错误: 找不到 nofx-nofx-frontend:latest 镜像"
    echo "请先运行: ./start.sh start --build"
    exit 1
fi

# 生成日期标签（格式：YYYY-MM-DD）
DATE_TAG=$(date +%Y-%m-%d)
echo ""
echo "📅 日期标签: ${DATE_TAG}"

echo ""
echo "📝 步骤1: 给镜像打标签..."
# 打 latest 标签
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:latest

# 打日期标签
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:${DATE_TAG}
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:${DATE_TAG}

echo ""
echo "📤 步骤2: 推送后端镜像到 Docker Hub..."
echo "   推送 latest 标签..."
docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest
echo "   推送日期标签 ${DATE_TAG}..."
docker push ${DOCKERHUB_USERNAME}/nofx-backend:${DATE_TAG}

echo ""
echo "📤 步骤3: 推送前端镜像到 Docker Hub..."
echo "   推送 latest 标签..."
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest
echo "   推送日期标签 ${DATE_TAG}..."
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:${DATE_TAG}

echo ""
echo "=========================================="
echo "✅ 推送完成！"
echo "=========================================="
echo ""
echo "📦 镜像地址："
echo "   - ${DOCKERHUB_USERNAME}/nofx-backend:latest"
echo "   - ${DOCKERHUB_USERNAME}/nofx-backend:${DATE_TAG}"
echo "   - ${DOCKERHUB_USERNAME}/nofx-frontend:latest"
echo "   - ${DOCKERHUB_USERNAME}/nofx-frontend:${DATE_TAG}"
echo ""
echo "🌐 在服务器上使用以下命令部署："
echo ""
echo "   # 使用 latest 标签（默认）"
echo "   export DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME}"
echo "   docker compose -f docker-compose.prod.yml pull"
echo "   docker compose -f docker-compose.prod.yml up -d"
echo ""
echo "   # 或使用特定日期标签"
echo "   export DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME}"
echo "   export IMAGE_TAG=${DATE_TAG}"
echo "   docker compose -f docker-compose.prod.yml pull"
echo "   docker compose -f docker-compose.prod.yml up -d"
echo ""

