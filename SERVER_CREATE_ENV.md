# 🔧 服务器创建 .env 文件命令

## ⚠️ 问题：环境变量未设置

看到警告：
- `DATA_ENCRYPTION_KEY` variable is not set
- `JWT_SECRET` variable is not set

需要创建 `.env` 文件并设置这些变量。

---

## 📝 创建 .env 文件（复制执行）

```bash
cd ~/nofx

# 创建 .env 文件
cat > .env << 'EOF'
# Docker Hub 用户名
DOCKERHUB_USERNAME=baimastryke

# 端口配置
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080

# 时区
NOFX_TIMEZONE=Asia/Shanghai

# 数据加密密钥（必须，至少32字符，从本地获取）
DATA_ENCRYPTION_KEY=你的DATA_ENCRYPTION_KEY

# JWT认证密钥（必须，至少64字符，从本地获取）
JWT_SECRET=你的JWT_SECRET
EOF

# 设置文件权限
chmod 600 .env
```

---

## 🔑 获取密钥值（在本地执行）

在**本地项目目录**执行：

```bash
cd ~/code/nofx/nofx

# 查看 DATA_ENCRYPTION_KEY
grep DATA_ENCRYPTION_KEY .env

# 查看 JWT_SECRET
grep JWT_SECRET .env

# 或者查看 config.json 中的 jwt_secret
grep jwt_secret config.json
```

---

## 📋 完整步骤（服务器上执行）

### 步骤1：创建 .env 文件

```bash
cd ~/nofx
nano .env
```

然后粘贴以下内容（**记得替换密钥值**）：

```bash
DOCKERHUB_USERNAME=baimastryke
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080
NOFX_TIMEZONE=Asia/Shanghai
DATA_ENCRYPTION_KEY=你的DATA_ENCRYPTION_KEY
JWT_SECRET=你的JWT_SECRET
```

保存退出（`Ctrl+X`，然后 `Y`，然后 `Enter`）

### 步骤2：设置文件权限

```bash
chmod 600 .env
```

### 步骤3：创建提示词文件（解决 prompts 目录为空）

```bash
cd ~/nofx
cat > prompts/default.txt << 'EOF'
你是一个专业的加密货币交易AI助手。
EOF
```

### 步骤4：重启服务

```bash
cd ~/nofx
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2025-11-10
set -a
source .env
set +a
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml logs -f
```

---

## 🔍 验证 .env 文件

```bash
# 检查文件是否存在
ls -la .env

# 检查文件内容（不显示敏感信息）
cat .env | grep -v "KEY\|SECRET"

# 验证变量是否设置
set -a && source .env && set +a && echo $DATA_ENCRYPTION_KEY | head -c 10 && echo "..." && echo $JWT_SECRET | head -c 10 && echo "..."
```

---

## ⚠️ 重要提示

1. **密钥必须与本地一致**：`DATA_ENCRYPTION_KEY` 和 `JWT_SECRET` 必须与本地 `.env` 文件中的值完全一致
2. **文件权限**：`.env` 文件权限必须是 600（只有所有者可读写）
3. **不要提交到 Git**：`.env` 文件包含敏感信息，不要提交到版本控制

---

## 🐛 如果密钥不匹配

如果服务器上的密钥与本地不一致，会导致：
- 无法解密数据库
- 无法验证 JWT token
- 数据访问失败

**解决方法：**
1. 确保服务器上的密钥与本地完全一致
2. 或者服务器上使用新的密钥（但会丢失之前的数据）






