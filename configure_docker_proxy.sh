#!/bin/bash

echo "=========================================="
echo "🔧 为 WSL2 中的 Docker 配置代理"
echo "=========================================="

# 获取宿主机 IP（从 resolv.conf）
HOST_IP=$(cat /etc/resolv.conf | grep -oP '(?<=nameserver\ ).*' | head -1)

if [ -z "$HOST_IP" ]; then
    echo "❌ 无法获取宿主机 IP"
    exit 1
fi

echo "✓ 检测到宿主机 IP: $HOST_IP"

# 询问代理端口
read -p "请输入代理端口（默认 7890）: " PROXY_PORT
PROXY_PORT=${PROXY_PORT:-7890}

echo ""
echo "📝 配置信息："
echo "   宿主机 IP: $HOST_IP"
echo "   代理端口: $PROXY_PORT"
echo "   代理地址: http://$HOST_IP:$PROXY_PORT"
echo ""

read -p "确认配置？(y/n): " CONFIRM
if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
    echo "已取消"
    exit 0
fi

# 创建配置目录
echo ""
echo "📁 创建配置目录..."
sudo mkdir -p /etc/systemd/system/docker.service.d

# 备份现有配置（如果存在）
if [ -f "/etc/systemd/system/docker.service.d/proxy.conf" ]; then
    sudo cp /etc/systemd/system/docker.service.d/proxy.conf \
        /etc/systemd/system/docker.service.d/proxy.conf.backup.$(date +%Y%m%d_%H%M%S)
    echo "✓ 已备份现有配置"
fi

# 创建代理配置文件
echo "📝 创建代理配置文件..."
sudo tee /etc/systemd/system/docker.service.d/proxy.conf > /dev/null <<EOF
# proxy.conf
[Service]
ExecStartPre=/bin/bash -c "echo http_proxy=http://$(cat /etc/resolv.conf | grep -oP '(?<=nameserver\\ ).*' | head -1):${PROXY_PORT} > /tmp/docker_env"
ExecStartPre=/bin/bash -c "echo https_proxy=http://$(cat /etc/resolv.conf | grep -oP '(?<=nameserver\\ ).*' | head -1):${PROXY_PORT} >> /tmp/docker_env"

[Service]
EnvironmentFile=-/tmp/docker_env
Environment=no_proxy="127.0.0.1,localhost"
EOF

echo "✓ 配置文件已创建"

# 重新加载 systemd 配置
echo ""
echo "🔄 重新加载 systemd 配置..."
sudo systemctl daemon-reload

# 重启 Docker
echo "🔄 重启 Docker..."
sudo systemctl restart docker

# 等待 Docker 启动
sleep 3

# 检查 Docker 状态
echo ""
echo "📊 检查 Docker 状态..."
if systemctl is-active --quiet docker; then
    echo "✅ Docker 已成功启动"
else
    echo "❌ Docker 启动失败，请检查配置"
    sudo systemctl status docker
    exit 1
fi

# 显示环境变量
echo ""
echo "📋 Docker 代理环境变量："
sudo cat /tmp/docker_env 2>/dev/null || echo "环境变量文件不存在"

echo ""
echo "=========================================="
echo "✅ 配置完成！"
echo "=========================================="
echo ""
echo "🧪 测试代理是否生效："
echo "   docker pull hello-world"
echo ""
echo "📝 如果测试失败，请检查："
echo "   1. 宿主机代理是否运行在端口 $PROXY_PORT"
echo "   2. 代理是否允许来自 WSL2 的连接"
echo "   3. 查看 Docker 日志: sudo journalctl -u docker.service -n 50"
echo ""





