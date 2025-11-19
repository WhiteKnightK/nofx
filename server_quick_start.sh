#!/bin/bash
# 服务器端快速启动脚本（使用远程镜像）

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "🚀 NOFX 服务器端快速启动"
echo "=========================================="

# 检查 Docker Hub 用户名
if [ -z "$DOCKERHUB_USERNAME" ]; then
    echo -e "${YELLOW}⚠️  未设置 DOCKERHUB_USERNAME 环境变量${NC}"
    read -p "请输入您的 Docker Hub 用户名: " DOCKERHUB_USERNAME
    export DOCKERHUB_USERNAME
fi

echo -e "${GREEN}✓ 使用 Docker Hub 用户名: ${DOCKERHUB_USERNAME}${NC}"

# 检查配置文件
if [ ! -f "docker-compose.prod.yml" ]; then
    echo -e "${RED}❌ 未找到 docker-compose.prod.yml${NC}"
    echo "请先下载配置文件或使用 git clone 获取项目"
    exit 1
fi

if [ ! -f "config.json" ]; then
    echo -e "${YELLOW}⚠️  未找到 config.json${NC}"
    if [ -f "config.json.example" ]; then
        echo "从模板创建 config.json..."
        cp config.json.example config.json
        echo -e "${GREEN}✓ 已创建 config.json${NC}"
        echo -e "${YELLOW}⚠️  请编辑 config.json 后重新运行此脚本${NC}"
        exit 0
    else
        echo -e "${RED}❌ 未找到 config.json.example${NC}"
        exit 1
    fi
fi

# 创建必要的目录和文件
echo ""
echo "📁 创建必要的目录和文件..."
mkdir -p decision_logs
touch config.db 2>/dev/null || true
touch beta_codes.txt 2>/dev/null || true
echo -e "${GREEN}✓ 目录和文件准备完成${NC}"

# 拉取镜像
echo ""
echo "📥 拉取最新镜像..."
docker pull ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker pull ${DOCKERHUB_USERNAME}/nofx-frontend:latest
echo -e "${GREEN}✓ 镜像拉取完成${NC}"

# 停止旧容器（如果存在）
echo ""
echo "🛑 停止旧容器（如果存在）..."
docker compose -f docker-compose.prod.yml down 2>/dev/null || true

# 启动服务
echo ""
echo "🚀 启动服务..."
docker compose -f docker-compose.prod.yml up -d

# 等待服务启动
echo ""
echo "⏳ 等待服务启动..."
sleep 5

# 检查服务状态
echo ""
echo "📊 服务状态："
docker compose -f docker-compose.prod.yml ps

# 获取服务器 IP
SERVER_IP=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "localhost")

echo ""
echo "=========================================="
echo -e "${GREEN}✅ 服务启动完成！${NC}"
echo "=========================================="
echo ""
echo "🌐 访问地址："
echo "   Web 界面: http://${SERVER_IP}:3000"
echo "   API 端点: http://${SERVER_IP}:8080"
echo "   健康检查: http://${SERVER_IP}:8080/api/health"
echo ""
echo "📝 常用命令："
echo "   查看日志: docker compose -f docker-compose.prod.yml logs -f"
echo "   停止服务: docker compose -f docker-compose.prod.yml down"
echo "   重启服务: docker compose -f docker-compose.prod.yml restart"
echo ""







