#!/bin/bash
# 本地一键部署脚本：检查配置 → 构建镜像 → 推送镜像

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

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo ""
echo "=========================================="
echo "🚀 NOFX 本地部署流程"
echo "=========================================="
echo ""

# 1. 检查配置文件
print_info "步骤 1/4: 检查配置文件..."

if [ ! -f "config.json" ]; then
    print_error "config.json 不存在！"
    print_info "请先创建配置文件："
    print_info "  cp config.json.example config.json"
    print_info "  然后编辑 config.json"
    exit 1
fi

if [ ! -f ".env" ]; then
    print_error ".env 不存在！"
    print_info "请先创建 .env 文件并配置密钥"
    exit 1
fi

# 验证 JSON 格式
if ! cat config.json | python3 -m json.tool > /dev/null 2>&1; then
    print_error "config.json 格式错误！"
    print_info "请检查 JSON 格式是否正确"
    exit 1
fi

# 检查必需的环境变量
if ! grep -q "^DATA_ENCRYPTION_KEY=" .env 2>/dev/null; then
    print_error ".env 文件中缺少 DATA_ENCRYPTION_KEY"
    exit 1
fi

if ! grep -q "^JWT_SECRET=" .env 2>/dev/null; then
    print_error ".env 文件中缺少 JWT_SECRET"
    exit 1
fi

# 检查 RSA 密钥
if [ ! -f "secrets/rsa_key" ] || [ ! -f "secrets/rsa_key.pub" ]; then
    print_warning "RSA密钥不存在，但可以继续（首次部署需要）"
fi

print_success "配置文件检查通过"

# 2. 设置 Docker Hub 用户名
if [ -z "$DOCKERHUB_USERNAME" ]; then
    if grep -q "^DOCKERHUB_USERNAME=" .env 2>/dev/null; then
        export DOCKERHUB_USERNAME=$(grep "^DOCKERHUB_USERNAME=" .env | cut -d'=' -f2 | tr -d '"' | tr -d ' ')
    else
        read -p "请输入 Docker Hub 用户名: " DOCKERHUB_USERNAME
        export DOCKERHUB_USERNAME
    fi
fi

print_info "Docker Hub 用户名: $DOCKERHUB_USERNAME"

# 3. 检查 Docker 登录状态
print_info "步骤 2/4: 检查 Docker 登录状态..."

if ! docker info 2>/dev/null | grep -q "Username"; then
    print_warning "未登录 Docker Hub，正在登录..."
    docker login
else
    print_success "已登录 Docker Hub"
fi

# 4. 构建镜像
print_info "步骤 3/4: 构建镜像..."
print_info "这可能需要几分钟时间..."

if [ -f "start.sh" ]; then
    ./start.sh start --build
else
    docker compose build
fi

print_success "镜像构建完成"

# 5. 推送镜像
print_info "步骤 4/4: 推送镜像到 Docker Hub..."

if [ -f "push_images.sh" ]; then
    ./push_images.sh
else
    print_error "push_images.sh 不存在"
    print_info "手动推送镜像："
    print_info "  docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:latest"
    print_info "  docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:latest"
    print_info "  docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest"
    print_info "  docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest"
    exit 1
fi

echo ""
echo "=========================================="
print_success "✅ 本地部署完成！"
echo "=========================================="
echo ""
print_info "📦 镜像已推送到 Docker Hub："
echo "   - ${DOCKERHUB_USERNAME}/nofx-backend:latest"
echo "   - ${DOCKERHUB_USERNAME}/nofx-frontend:latest"
echo ""
print_info "📝 下一步：在服务器上执行更新命令"
echo ""
print_info "服务器更新命令："
echo "   cd ~/nofx"
echo "   export DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME}"
echo "   set -a && source .env && set +a"
echo "   docker compose -f docker-compose.prod.yml pull"
echo "   docker compose -f docker-compose.prod.yml up -d"
echo ""





