#!/bin/bash

# 检查MySQL数据库中的default用户数据
# 请根据实际情况修改MySQL连接参数

echo "=== 检查 default 用户 ===" 
mysql -h localhost -u root -p -e "SELECT * FROM nofx.users WHERE id='default';"

echo ""
echo "=== 检查 AI 模型 ==="
mysql -h localhost -u root -p -e "SELECT * FROM nofx.ai_models WHERE user_id='default';"

echo ""
echo "=== 检查交易所 ==="
mysql -h localhost -u root -p -e "SELECT * FROM nofx.exchanges WHERE user_id='default';"

