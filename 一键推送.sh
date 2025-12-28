#!/bin/bash
# NOFX 一键构建推送脚本 - 国内版本
# 使用国内镜像加速器，直接推送到 Docker Hub (baimastryke)

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║            NOFX 一键构建推送（国内优化版）                 ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# 切换到脚本所在目录
cd "$(dirname "$0")"

# 检查 Docker daemon 配置
echo -e "${BLUE}🔍 检查 Docker 镜像加速器配置...${NC}"
if docker info 2>/dev/null | grep -q "Registry Mirrors"; then
    echo -e "${GREEN}✅ 已配置镜像加速器${NC}"
    docker info | grep -A 3 "Registry Mirrors"
else
    echo -e "${YELLOW}⚠️  未检测到镜像加速器配置${NC}"
    echo ""
    read -p "是否现在配置 Docker 镜像加速器？[Y/n] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
        echo -e "${BLUE}🔧 开始配置...${NC}"
        ./setup_docker_china.sh
    else
        echo -e "${YELLOW}⚠️  跳过配置，但可能会遇到网络问题${NC}"
        echo "如果推送失败，请运行: ./setup_docker_china.sh"
    fi
fi

echo ""
echo -e "${BLUE}🔐 检查 Docker Hub 登录状态...${NC}"
if ! docker info 2>/dev/null | grep -q "Username.*baimastryke"; then
    echo -e "${YELLOW}需要登录 Docker Hub (baimastryke)${NC}"
    docker login
else
    echo -e "${GREEN}✅ 已登录 Docker Hub (baimastryke)${NC}"
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}开始构建和推送流程...${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# 运行构建推送脚本
./quick_build_push.sh

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                 🎉 一键推送完成！                         ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"



