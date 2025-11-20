#!/bin/bash

echo "=========================================="
echo "🔧 配置 Docker 镜像加速器"
echo "=========================================="

# 检测操作系统
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    DOCKER_CONFIG="/etc/docker/daemon.json"
    if command -v systemctl &> /dev/null; then
        RESTART_CMD="sudo systemctl daemon-reload && sudo systemctl restart docker"
    else
        RESTART_CMD="sudo service docker restart"
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    echo "⚠️  macOS 请手动在 Docker Desktop 中配置："
    echo "   1. 打开 Docker Desktop"
    echo "   2. 点击设置（Settings）"
    echo "   3. 选择 Docker Engine"
    echo "   4. 添加镜像加速器配置"
    exit 0
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || -n "$WSL_DISTRO_NAME" ]]; then
    # Windows (WSL2)
    DOCKER_CONFIG="/etc/docker/daemon.json"
    RESTART_CMD="echo '⚠️  请手动重启 Docker Desktop'"
fi

# 创建配置目录
echo "📁 创建配置目录..."
sudo mkdir -p /etc/docker

# 备份现有配置
if [ -f "$DOCKER_CONFIG" ]; then
    sudo cp "$DOCKER_CONFIG" "$DOCKER_CONFIG.backup.$(date +%Y%m%d_%H%M%S)"
    echo "✓ 已备份现有配置"
fi

# 创建新配置
echo "📝 写入镜像加速器配置..."
sudo tee "$DOCKER_CONFIG" > /dev/null <<EOF
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com",
    "https://dockerproxy.com"
  ]
}
EOF

echo "✓ 配置已写入"

# 重启 Docker
echo ""
echo "🔄 重启 Docker..."
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || -n "$WSL_DISTRO_NAME" ]]; then
    echo "⚠️  请手动重启 Docker Desktop："
    echo "   1. 右键点击系统托盘中的 Docker 图标"
    echo "   2. 选择 'Restart Docker Desktop'"
    echo "   3. 等待重启完成"
else
    eval $RESTART_CMD
fi

echo ""
echo "=========================================="
echo "✅ 配置完成！"
echo "=========================================="
echo ""
echo "📋 验证配置："
echo "   docker info | grep -A 10 'Registry Mirrors'"
echo ""
echo "🚀 然后重新构建："
echo "   docker compose build --no-cache"
echo ""









