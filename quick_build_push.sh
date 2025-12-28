#!/bin/bash
# NOFX 镜像构建并推送脚本（简单原版，无任何代理/加速配置）

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║              NOFX 镜像构建和推送脚本                       ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# 确保在脚本所在目录执行
cd "$(dirname "$0")"

# 固定 Docker Hub 用户名（你的账号）
export DOCKERHUB_USERNAME="baimastryke"

# 设置镜像标签（默认使用日期）
if [ -z "$IMAGE_TAG" ]; then
    IMAGE_TAG=$(date +%Y-%m-%d)
    echo -e "${BLUE}📅 使用日期标签: ${IMAGE_TAG}${NC}"
else
    echo -e "${BLUE}📅 使用指定标签: ${IMAGE_TAG}${NC}"
fi

export IMAGE_TAG
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

echo -e "${BLUE}🎯 目标仓库: Docker Hub (${DOCKERHUB_USERNAME})${NC}"
echo ""

# 检查Docker登录状态
echo -e "${YELLOW}🔐 检查 Docker Hub 登录状态...${NC}"
if ! docker info 2>/dev/null | grep -q "Username"; then
    echo -e "${YELLOW}需要登录 Docker Hub${NC}"
    docker login
else
    echo -e "${GREEN}✅ 已登录 Docker Hub${NC}"
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}步骤1: 构建后端镜像（强制重新构建，不使用缓存）${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
docker compose build --no-cache --progress=plain nofx

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}步骤2: 构建前端镜像（强制重新构建，不使用缓存）${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
docker compose build --no-cache --progress=plain nofx-frontend

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}步骤3: 打标签${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG}
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG}
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:latest
echo -e "${GREEN}✅ 打标签完成${NC}"

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}步骤4: 推送镜像到 Docker Hub${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📦 推送后端镜像...${NC}"
docker push ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG}
docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest
echo -e "${BLUE}📦 推送前端镜像...${NC}"
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG}
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                    ✅ 构建和推送完成！                    ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}📦 镜像地址:${NC}"
echo -e "  后端: ${DOCKERHUB_USERNAME}/nofx-backend:${IMAGE_TAG}"
echo -e "  后端: ${DOCKERHUB_USERNAME}/nofx-backend:latest"
echo -e "  前端: ${DOCKERHUB_USERNAME}/nofx-frontend:${IMAGE_TAG}"
echo -e "  前端: ${DOCKERHUB_USERNAME}/nofx-frontend:latest"
echo ""
echo -e "${BLUE}🚀 在服务器上更新:${NC}"
echo -e "  ssh -i A.pem ubuntu@43.202.115.56"
echo -e "  cd /home/ubuntu/nofx"
echo -e "  docker compose -f docker-compose.prod.yml pull"
echo -e "  docker compose -f docker-compose.prod.yml up -d"
echo ""
