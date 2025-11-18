#!/bin/bash
# 服务器端部署脚本 - 从 Docker Hub 拉取镜像并运行

set -e

# ═══════════════════════════════════════════════════════════════
# NOFX AI Trading System - 服务器端部署脚本
# 用法: ./server_deploy.sh [DOCKERHUB_USERNAME] [IMAGE_TAG]
# 示例: ./server_deploy.sh baimastryke 2024-12-15
# ═══════════════════════════════════════════════════════════════

# ------------------------------------------------------------------------
# Color Definitions
# ------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ------------------------------------------------------------------------
# Utility Functions: Colored Output
# ------------------------------------------------------------------------
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

# ------------------------------------------------------------------------
# Detection: Docker Compose Command
# ------------------------------------------------------------------------
detect_compose_cmd() {
    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        print_error "Docker Compose 未安装！请先安装 Docker Compose"
        exit 1
    fi
    print_info "使用 Docker Compose 命令: $COMPOSE_CMD"
}

# ------------------------------------------------------------------------
# Validation: Docker Installation
# ------------------------------------------------------------------------
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装！请先安装 Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi

    detect_compose_cmd
    print_success "Docker 和 Docker Compose 已安装"
}

# ------------------------------------------------------------------------
# Get Docker Hub Username
# ------------------------------------------------------------------------
get_dockerhub_username() {
    if [ -n "$1" ]; then
        DOCKERHUB_USERNAME="$1"
    elif [ -n "$DOCKERHUB_USERNAME" ]; then
        # 使用环境变量
        print_info "使用环境变量 DOCKERHUB_USERNAME: $DOCKERHUB_USERNAME"
    else
        print_warning "未指定 Docker Hub 用户名"
        read -p "请输入您的 Docker Hub 用户名: " DOCKERHUB_USERNAME
    fi
    
    if [ -z "$DOCKERHUB_USERNAME" ]; then
        print_error "Docker Hub 用户名不能为空"
        exit 1
    fi
    
    export DOCKERHUB_USERNAME
    print_info "Docker Hub 用户名: $DOCKERHUB_USERNAME"
}

# ------------------------------------------------------------------------
# Check Docker Login
# ------------------------------------------------------------------------
check_docker_login() {
    if ! docker info 2>/dev/null | grep -q "Username"; then
        print_warning "未登录 Docker Hub，正在登录..."
        docker login
    else
        print_success "已登录 Docker Hub"
    fi
}

# ------------------------------------------------------------------------
# Validation: Environment File (.env)
# ------------------------------------------------------------------------
check_env() {
    if [ ! -f ".env" ]; then
        print_warning ".env 不存在，从模板复制..."
        if [ -f ".env.example" ]; then
            cp .env.example .env
            print_info "✓ 已使用默认环境变量创建 .env"
        else
            print_error ".env.example 不存在，请手动创建 .env 文件"
            exit 1
        fi
        print_info "💡 请编辑 .env 文件配置必要的环境变量"
    fi
    print_success "环境变量文件存在"
}

# ------------------------------------------------------------------------
# Validation: Configuration File
# ------------------------------------------------------------------------
check_config() {
    if [ ! -f "config.json" ]; then
        print_warning "config.json 不存在，从模板复制..."
        if [ -f "config.json.example" ]; then
            cp config.json.example config.json
            print_info "✓ 已使用默认配置创建 config.json"
        else
            print_error "config.json.example 不存在"
            exit 1
        fi
    fi
    print_success "配置文件存在"
}

# ------------------------------------------------------------------------
# Validation: Database File
# ------------------------------------------------------------------------
check_database() {
    if [ -d "config.db" ]; then
        print_warning "config.db 是目录而非文件，正在删除目录..."
        rm -rf config.db
        install -m 600 /dev/null config.db
        print_success "✓ 已创建空数据库文件"
    elif [ ! -f "config.db" ]; then
        print_info "创建数据库文件..."
        install -m 600 /dev/null config.db
        print_success "✓ 已创建空数据库文件"
    else
        print_success "数据库文件存在"
    fi
}

# ------------------------------------------------------------------------
# Validation: Secrets Directory
# ------------------------------------------------------------------------
check_secrets() {
    if [ ! -d "secrets" ]; then
        print_warning "secrets 目录不存在，正在创建..."
        mkdir -p secrets
        chmod 700 secrets
        print_info "💡 请确保 secrets/rsa_key 和 secrets/rsa_key.pub 存在"
    fi
    
    if [ ! -f "secrets/rsa_key" ] || [ ! -f "secrets/rsa_key.pub" ]; then
        print_warning "RSA密钥对不存在"
        print_info "💡 请从本地复制 secrets/ 目录到服务器，或运行加密设置脚本"
    else
        print_success "RSA密钥对存在"
    fi
}

# ------------------------------------------------------------------------
# Pull Images
# ------------------------------------------------------------------------
pull_images() {
    # 检查是否指定了镜像标签
    if [ -z "$IMAGE_TAG" ]; then
        IMAGE_TAG="latest"
        print_info "使用默认标签: latest"
    else
        print_info "使用指定标签: ${IMAGE_TAG}"
    fi
    export IMAGE_TAG
    
    print_info "从 Docker Hub 拉取镜像..."
    print_info "  后端镜像: ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG}"
    print_info "  前端镜像: ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG}"
    
    $COMPOSE_CMD -f docker-compose.prod.yml pull
    
    print_success "镜像拉取完成"
}

# ------------------------------------------------------------------------
# Start Services
# ------------------------------------------------------------------------
start_services() {
    print_info "启动服务..."
    
    # 确保必要的目录存在
    if [ ! -d "decision_logs" ]; then
        mkdir -p decision_logs
        chmod 700 decision_logs
    fi
    
    $COMPOSE_CMD -f docker-compose.prod.yml up -d
    
    print_success "服务已启动！"
    
    # 读取端口配置
    NOFX_FRONTEND_PORT=$(grep "^NOFX_FRONTEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "3000")
    NOFX_BACKEND_PORT=$(grep "^NOFX_BACKEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "8080")
    NOFX_FRONTEND_PORT=$(echo "$NOFX_FRONTEND_PORT" | tr -d '"'"'" | tr -d ' ')
    NOFX_BACKEND_PORT=$(echo "$NOFX_BACKEND_PORT" | tr -d '"'"'" | tr -d ' ')
    NOFX_FRONTEND_PORT=${NOFX_FRONTEND_PORT:-3000}
    NOFX_BACKEND_PORT=${NOFX_BACKEND_PORT:-8080}
    
    print_info "Web 界面: http://localhost:${NOFX_FRONTEND_PORT}"
    print_info "API 端点: http://localhost:${NOFX_BACKEND_PORT}"
    print_info ""
    print_info "查看日志: docker compose -f docker-compose.prod.yml logs -f"
    print_info "停止服务: docker compose -f docker-compose.prod.yml down"
}

# ------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------
main() {
    echo ""
    echo "=========================================="
    echo "🚀 NOFX 服务器端部署"
    echo "=========================================="
    echo ""
    
    check_docker
    get_dockerhub_username "$1"
    
    # 检查是否指定了镜像标签（第二个参数）
    if [ -n "$2" ]; then
        export IMAGE_TAG="$2"
        print_info "使用镜像标签: ${IMAGE_TAG}"
    elif [ -n "$IMAGE_TAG" ]; then
        print_info "使用环境变量 IMAGE_TAG: ${IMAGE_TAG}"
    else
        print_info "未指定镜像标签，将使用 latest"
    fi
    
    check_docker_login
    check_env
    check_config
    check_database
    check_secrets
    
    echo ""
    print_info "准备部署..."
    pull_images
    start_services
    
    echo ""
    echo "=========================================="
    echo "✅ 部署完成！"
    echo "=========================================="
    echo ""
}

# Execute Main
main "$@"
