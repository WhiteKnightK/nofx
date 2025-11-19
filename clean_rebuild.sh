#!/bin/bash
# 清理旧镜像并重新构建（确保使用最新代码）

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

echo ""
echo "=========================================="
echo "🧹 清理并重新构建 NOFX 镜像"
echo "=========================================="
echo ""

# 1. 停止并删除现有容器（如果正在运行）
print_info "步骤 1/5: 停止现有容器..."
if docker compose ps -q 2>/dev/null | grep -q .; then
    docker compose down 2>/dev/null || true
    print_success "容器已停止"
else
    print_info "没有运行中的容器"
fi

# 2. 删除本地构建的镜像（保留已推送的）
print_info "步骤 2/5: 清理本地镜像..."

# 删除本地构建的镜像（nofx-nofx 和 nofx-nofx-frontend）
if docker images | grep -q "nofx-nofx.*latest"; then
    docker rmi nofx-nofx:latest 2>/dev/null || true
    print_success "已删除 nofx-nofx:latest"
fi

if docker images | grep -q "nofx-nofx-frontend.*latest"; then
    docker rmi nofx-nofx-frontend:latest 2>/dev/null || true
    print_success "已删除 nofx-nofx-frontend:latest"
fi

# 可选：删除所有 nofx 相关镜像（包括已推送的标签）
read -p "是否删除所有 nofx 镜像（包括已推送的标签）？[y/N]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_warning "删除所有 nofx 相关镜像..."
    docker images | grep -E "nofx|baimastryke/nofx" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true
    print_success "已删除所有 nofx 镜像"
else
    print_info "保留已推送的镜像标签"
fi

# 3. 清理构建缓存（可选，但推荐）
print_info "步骤 3/5: 清理构建缓存..."
read -p "是否清理 Docker 构建缓存？[y/N]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    docker builder prune -f
    print_success "构建缓存已清理"
else
    print_info "跳过缓存清理"
fi

# 4. 设置 Docker Hub 用户名
if [ -z "$DOCKERHUB_USERNAME" ]; then
    if [ -f ".env" ] && grep -q "^DOCKERHUB_USERNAME=" .env 2>/dev/null; then
        export DOCKERHUB_USERNAME=$(grep "^DOCKERHUB_USERNAME=" .env | cut -d'=' -f2 | tr -d '"' | tr -d ' ')
    else
        read -p "请输入 Docker Hub 用户名: " DOCKERHUB_USERNAME
        export DOCKERHUB_USERNAME
    fi
fi

print_info "Docker Hub 用户名: $DOCKERHUB_USERNAME"

# 5. 强制重新构建（不使用缓存）
print_info "步骤 4/5: 强制重新构建镜像（不使用缓存）..."
print_warning "这可能需要几分钟时间..."

docker compose build --no-cache

print_success "镜像重新构建完成"

# 6. 验证镜像
print_info "步骤 5/5: 验证镜像..."
docker images | grep -E "nofx-nofx|REPOSITORY" | head -3

echo ""
echo "=========================================="
print_success "✅ 清理和重建完成！"
echo "=========================================="
echo ""
print_info "下一步：推送镜像到 Docker Hub"
print_info "运行: ./push_images.sh"
echo ""






