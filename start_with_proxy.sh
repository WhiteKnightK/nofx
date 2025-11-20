#!/bin/bash

cd /home/master/code/nofx/nofx

# 代理配置（根据你的实际代理地址修改）
PROXY_HOST="127.0.0.1"
PROXY_PORT="7890"

# 设置代理
export HTTP_PROXY="http://${PROXY_HOST}:${PROXY_PORT}"
export HTTPS_PROXY="http://${PROXY_HOST}:${PROXY_PORT}"

# 排除不需要代理的地址（MySQL、本地服务等）
export NO_PROXY="localhost,127.0.0.1,*.rds.amazonaws.com,*.mysql.rds.aliyuncs.com,*.aliyuncs.com"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔧 代理配置:"
echo "   HTTP_PROXY: $HTTP_PROXY"
echo "   HTTPS_PROXY: $HTTPS_PROXY"
echo "   NO_PROXY: $NO_PROXY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 启动后端
go run main.go
