#!/bin/bash

# ═══════════════════════════════════════════════════════════════
# 检查MySQL模式部署文件完整性
# 使用方法: ./check_deploy_files.sh
# ═══════════════════════════════════════════════════════════════

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║       NOFX MySQL模式 - 部署文件完整性检查                  ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 统计变量
MISSING_COUNT=0
REQUIRED_COUNT=0
OPTIONAL_COUNT=0

# 检查文件是否存在
check_file() {
    local file=$1
    local required=$2
    local description=$3
    
    if [ "$required" = "true" ]; then
        ((REQUIRED_COUNT++))
    else
        ((OPTIONAL_COUNT++))
    fi
    
    if [ -f "$file" ]; then
        echo -e "${GREEN}✅${NC} $description"
        echo -e "   ${BLUE}→${NC} $file"
        
        # 显示文件大小和修改时间
        ls -lh "$file" | awk '{print "   大小: " $5 ", 修改: " $6 " " $7 " " $8}'
        
        # 对于敏感文件，检查权限
        if [[ "$file" == *".env"* ]] || [[ "$file" == *"rsa_key"* ]]; then
            local perms=$(stat -c "%a" "$file" 2>/dev/null || stat -f "%OLp" "$file" 2>/dev/null)
            if [ "$perms" != "600" ] && [ "$perms" != "400" ]; then
                echo -e "   ${YELLOW}⚠️  建议修改权限: chmod 600 $file${NC}"
            fi
        fi
    else
        if [ "$required" = "true" ]; then
            echo -e "${RED}❌${NC} $description ${RED}(必需)${NC}"
            echo -e "   ${RED}→ 缺失: $file${NC}"
            ((MISSING_COUNT++))
        else
            echo -e "${YELLOW}⚠️${NC} $description ${YELLOW}(可选)${NC}"
            echo -e "   ${YELLOW}→ 未找到: $file${NC}"
        fi
    fi
    echo ""
}

# 检查目录是否存在
check_directory() {
    local dir=$1
    local required=$2
    local description=$3
    
    if [ -d "$dir" ]; then
        local file_count=$(ls -1 "$dir" 2>/dev/null | wc -l)
        echo -e "${GREEN}✅${NC} $description"
        echo -e "   ${BLUE}→${NC} $dir (包含 $file_count 个文件)"
        
        # 列出目录内容
        if [ $file_count -gt 0 ]; then
            ls -lh "$dir" | tail -n +2 | awk '{print "      - " $9 " (" $5 ")"}'
        fi
    else
        if [ "$required" = "true" ]; then
            echo -e "${RED}❌${NC} $description ${RED}(必需)${NC}"
            echo -e "   ${RED}→ 目录不存在: $dir${NC}"
            ((MISSING_COUNT++))
        else
            echo -e "${YELLOW}⚠️${NC} $description ${YELLOW}(可选)${NC}"
            echo -e "   ${YELLOW}→ 目录不存在: $dir${NC}"
        fi
    fi
    echo ""
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  1. Docker配置文件"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_file "docker-compose.prod.yml" "true" "Docker Compose生产配置"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  2. 环境变量配置"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_file ".env" "true" "环境变量配置（包含MySQL连接信息）"

if [ -f ".env" ]; then
    echo -e "${BLUE}🔍 检查.env配置项:${NC}"
    
    # 检查必需的环境变量
    env_vars=("DB_HOST" "DB_USER" "DB_PASSWORD" "DB_NAME" "DATA_ENCRYPTION_KEY" "JWT_SECRET")
    for var in "${env_vars[@]}"; do
        if grep -q "^${var}=" .env; then
            value=$(grep "^${var}=" .env | cut -d'=' -f2)
            if [ -n "$value" ]; then
                # 隐藏敏感信息
                if [[ "$var" == *"PASSWORD"* ]] || [[ "$var" == *"KEY"* ]] || [[ "$var" == *"SECRET"* ]]; then
                    echo -e "   ${GREEN}✓${NC} $var=***（已设置）"
                else
                    echo -e "   ${GREEN}✓${NC} $var=$value"
                fi
            else
                echo -e "   ${RED}✗${NC} $var （已定义但值为空）"
            fi
        else
            echo -e "   ${RED}✗${NC} $var （未定义）"
        fi
    done
    echo ""
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  3. 系统配置"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_file "config.json" "true" "系统配置（杠杆、风控参数等）"

if [ -f "config.json" ]; then
    echo -e "${BLUE}🔍 配置内容预览:${NC}"
    # 显示配置的关键信息
    if command -v jq &> /dev/null; then
        echo "   Beta模式: $(jq -r '.beta_mode' config.json)"
        echo "   API端口: $(jq -r '.api_server_port' config.json)"
        echo "   最大日损失: $(jq -r '.max_daily_loss' config.json)%"
        echo "   最大回撤: $(jq -r '.max_drawdown' config.json)%"
    else
        echo "   （安装jq可查看详细配置: sudo apt install jq）"
    fi
    echo ""
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  4. RSA加密密钥"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_file "secrets/rsa_key" "true" "RSA私钥（用于前后端加密通信）"
check_file "secrets/rsa_key.pub" "true" "RSA公钥（用于前后端加密通信）"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  5. AI提示词模板"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_directory "prompts" "true" "AI提示词目录"
check_file "prompts/default.txt" "true" "默认提示词"
check_file "prompts/Hansen.txt" "false" "Hansen策略提示词"
check_file "prompts/nof1.txt" "false" "NOF1策略提示词"
check_file "prompts/taro_long_prompts.txt" "false" "Taro Long策略提示词"
check_file "prompts/test_mode.txt" "false" "测试模式提示词"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  6. Beta邀请码（可选）"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_file "beta_codes.txt" "false" "Beta邀请码文件（仅beta模式需要）"

if [ -f "beta_codes.txt" ]; then
    code_count=$(wc -l < beta_codes.txt)
    echo -e "${BLUE}📊 包含 $code_count 个邀请码${NC}"
    echo ""
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  7. 日志目录"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ -d "decision_logs" ]; then
    check_directory "decision_logs" "false" "决策日志目录（容器启动时自动创建）"
else
    echo -e "${BLUE}ℹ️${NC}  决策日志目录 ${BLUE}(会在容器启动时自动创建)${NC}"
    echo -e "   ${BLUE}→${NC} decision_logs/"
    echo ""
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  8. 不需要的文件（已排除）"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✓${NC} 以下文件不需要上传到服务器:"
echo "   • config.db (SQLite数据库 - MySQL替代)"
echo "   • *.go (Go源代码 - 已打包到镜像)"
echo "   • web/ (前端源代码 - 已打包到镜像)"
echo "   • go.mod, go.sum (Go依赖 - 已打包)"
echo "   • *.sh (构建脚本 - 仅本地使用)"
echo ""

echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    检查结果汇总                             ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

if [ $MISSING_COUNT -eq 0 ]; then
    echo -e "${GREEN}🎉 所有必需文件都已就绪！${NC}"
    echo ""
    echo "📦 文件统计:"
    echo "   • 必需文件: $REQUIRED_COUNT 个"
    echo "   • 可选文件: $OPTIONAL_COUNT 个"
    echo "   • 缺失文件: $MISSING_COUNT 个"
    echo ""
    echo -e "${GREEN}✅ 可以开始部署了！${NC}"
    echo ""
    echo "📤 上传文件到服务器:"
    echo "   ./upload_to_server.sh"
    echo ""
    echo "🚀 或者在服务器上启动:"
    echo "   docker-compose -f docker-compose.prod.yml up -d"
else
    echo -e "${RED}⚠️  发现 $MISSING_COUNT 个必需文件缺失！${NC}"
    echo ""
    echo "📦 文件统计:"
    echo "   • 必需文件: $REQUIRED_COUNT 个"
    echo "   • 可选文件: $OPTIONAL_COUNT 个"
    echo "   • 缺失文件: $MISSING_COUNT 个"
    echo ""
    echo -e "${YELLOW}💡 请先创建缺失的文件:${NC}"
    echo ""
    echo "1. 创建环境变量文件:"
    echo "   cp env.mysql.example .env"
    echo "   nano .env  # 填入你的MySQL配置"
    echo ""
    echo "2. 检查RSA密钥是否存在:"
    echo "   ls -lh secrets/"
    echo ""
    echo "3. 检查提示词目录:"
    echo "   ls -lh prompts/"
    echo ""
    echo "详细说明请参考: MYSQL_DEPLOY_FILES.md"
    echo ""
    exit 1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  📚 相关文档"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "• 完整部署指南: MYSQL_DEPLOY_FILES.md"
echo "• 服务器设置: SERVER_SETUP.md"
echo "• 环境变量示例: env.mysql.example"
echo ""

