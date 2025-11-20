# 🚀 MySQL模式服务器部署文件清单

使用MySQL数据库时，需要上传到服务器的文件清单。

---

## 📋 必需文件清单

### 1. **Docker Compose 配置**
```bash
nofx/
├── docker-compose.prod.yml   # ✅ Docker编排配置（已经配置好MySQL环境变量）
```

### 2. **环境变量配置**
```bash
nofx/
├── .env                       # ⚠️ 重要！包含敏感信息（MySQL连接、加密密钥）
```

**创建方式**：
```bash
# 从示例文件创建
cp env.mysql.example .env

# 编辑填入你的配置
nano .env
```

**必需包含的环境变量**：
- `DB_HOST` - MySQL主机地址
- `DB_PORT` - MySQL端口（默认3306）
- `DB_USER` - MySQL用户名
- `DB_PASSWORD` - MySQL密码
- `DB_NAME` - 数据库名（默认nofx）
- `DATA_ENCRYPTION_KEY` - 数据加密密钥（32字节base64）
- `JWT_SECRET` - JWT认证密钥
- `DOCKERHUB_USERNAME` - Docker Hub用户名
- `IMAGE_TAG` - Docker镜像标签

### 3. **RSA加密密钥（前端/后端通信）**
```bash
nofx/
├── secrets/
│   ├── rsa_key           # ✅ RSA私钥
│   └── rsa_key.pub       # ✅ RSA公钥
```

### 4. **AI提示词模板**
```bash
nofx/
├── prompts/
│   ├── default.txt              # ✅ 默认提示词
│   ├── Hansen.txt              # ✅ Hansen策略
│   ├── nof1.txt                # ✅ NOF1策略
│   ├── taro_long_prompts.txt   # ✅ Taro Long策略
│   └── test_mode.txt           # ✅ 测试模式
```

### 5. **Beta邀请码（可选）**
```bash
nofx/
├── beta_codes.txt        # ⚠️ 如果启用beta模式需要
```

### 6. **系统配置**
```bash
nofx/
├── config.json           # ✅ 系统配置（杠杆、风控参数等）
```

### 7. **日志目录（自动创建）**
```bash
nofx/
├── decision_logs/        # 🔄 容器会自动创建
```

---

## ❌ 不需要的文件

使用MySQL后，以下文件**不再需要**：
- ❌ `config.db` - SQLite数据库文件（MySQL替代）
- ❌ `*.go` - Go源代码文件（已打包到镜像）
- ❌ `web/` - 前端源代码（已打包到镜像）
- ❌ `go.mod`, `go.sum` - Go依赖文件（已打包）
- ❌ 各种 `*.sh` 构建脚本（本地构建用）

---

## 📦 服务器目录结构

推荐在服务器上创建如下目录结构：

```bash
/home/ubuntu/nofx/
├── .env                        # 环境变量（敏感）
├── docker-compose.prod.yml     # Docker编排
├── config.json                 # 系统配置
├── beta_codes.txt             # Beta码（可选）
├── secrets/                    # RSA密钥目录
│   ├── rsa_key
│   └── rsa_key.pub
├── prompts/                    # AI提示词目录
│   ├── default.txt
│   ├── Hansen.txt
│   ├── nof1.txt
│   ├── taro_long_prompts.txt
│   └── test_mode.txt
└── decision_logs/             # 决策日志（自动创建）
```

---

## 🔐 安全注意事项

### 1. **保护敏感文件**
```bash
# 设置正确的文件权限
chmod 600 .env
chmod 600 secrets/rsa_key
chmod 644 secrets/rsa_key.pub
chmod 644 config.json
```

### 2. **不要将敏感文件提交到Git**
`.env` 和 `secrets/` 应该已经在 `.gitignore` 中。

### 3. **加密密钥要求**
- `DATA_ENCRYPTION_KEY`: 32字节随机数据的base64编码
- `JWT_SECRET`: 至少64个字符的随机字符串

**生成新密钥**：
```bash
# 生成DATA_ENCRYPTION_KEY
openssl rand -base64 32

# 生成JWT_SECRET
openssl rand -base64 64
```

---

## 📤 上传文件到服务器

### 方法1：使用SCP（推荐）

```bash
# 1. 创建服务器目录
ssh -i A.pem ubuntu@43.202.115.56 "mkdir -p /home/ubuntu/nofx/{secrets,prompts,decision_logs}"

# 2. 上传配置文件
scp -i A.pem docker-compose.prod.yml ubuntu@43.202.115.56:/home/ubuntu/nofx/
scp -i A.pem .env ubuntu@43.202.115.56:/home/ubuntu/nofx/
scp -i A.pem config.json ubuntu@43.202.115.56:/home/ubuntu/nofx/
scp -i A.pem beta_codes.txt ubuntu@43.202.115.56:/home/ubuntu/nofx/

# 3. 上传RSA密钥
scp -i A.pem secrets/* ubuntu@43.202.115.56:/home/ubuntu/nofx/secrets/

# 4. 上传提示词
scp -i A.pem prompts/* ubuntu@43.202.115.56:/home/ubuntu/nofx/prompts/

# 5. 设置权限
ssh -i A.pem ubuntu@43.202.115.56 "cd /home/ubuntu/nofx && chmod 600 .env secrets/rsa_key"
```

### 方法2：使用rsync（适合批量更新）

```bash
rsync -avz -e "ssh -i A.pem" \
  --exclude='*.go' \
  --exclude='web/' \
  --exclude='*.db' \
  --exclude='*.md' \
  --exclude='*.sh' \
  docker-compose.prod.yml \
  .env \
  config.json \
  beta_codes.txt \
  secrets/ \
  prompts/ \
  ubuntu@43.202.115.56:/home/ubuntu/nofx/
```

---

## ✅ 验证清单

在服务器上运行以下命令验证文件完整性：

```bash
cd /home/ubuntu/nofx

# 1. 检查必需文件
echo "=== 检查必需文件 ==="
ls -lh docker-compose.prod.yml .env config.json

# 2. 检查RSA密钥
echo -e "\n=== 检查RSA密钥 ==="
ls -lh secrets/

# 3. 检查提示词
echo -e "\n=== 检查提示词 ==="
ls -lh prompts/

# 4. 验证.env配置
echo -e "\n=== 验证环境变量 ==="
grep -E "^(DB_HOST|DATA_ENCRYPTION_KEY|JWT_SECRET)=" .env | sed 's/=.*/=***/'

# 5. 测试MySQL连接
echo -e "\n=== 测试MySQL连接 ==="
source .env
echo "quit" | mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p$DB_PASSWORD $DB_NAME 2>&1 | head -3
```

---

## 🚀 启动服务

文件上传完成后，在服务器上运行：

```bash
cd /home/ubuntu/nofx

# 1. 拉取最新镜像
docker-compose -f docker-compose.prod.yml pull

# 2. 启动服务
docker-compose -f docker-compose.prod.yml up -d

# 3. 查看日志
docker-compose -f docker-compose.prod.yml logs -f --tail=100

# 4. 检查健康状态
docker-compose -f docker-compose.prod.yml ps
```

---

## 🔄 更新流程

当代码更新后，只需：

1. **构建并推送新镜像**（本地）：
```bash
./build_and_push.sh
```

2. **在服务器上更新**：
```bash
cd /home/ubuntu/nofx
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d
```

3. **检查配置文件是否需要更新**：
   - 如果 `config.json.example` 有更新 → 更新 `config.json`
   - 如果 `env.mysql.example` 有更新 → 更新 `.env`
   - 如果 `prompts/` 有新文件 → 上传新提示词

---

## 🆘 常见问题

### Q1: 忘记上传某个文件怎么办？
**A**: 容器启动时会报错，根据错误信息补充上传即可。常见错误：
- `failed to load data encryption key` → 缺少 `.env` 或 `DATA_ENCRYPTION_KEY`
- `failed to load RSA key` → 缺少 `secrets/rsa_key`
- `config.json not found` → 缺少 `config.json`

### Q2: MySQL连接失败？
**A**: 检查以下项：
1. `.env` 中的MySQL配置是否正确
2. MySQL服务器防火墙是否允许Docker容器IP访问
3. 数据库用户是否有足够权限
4. 数据库是否已创建（需要先创建 `nofx` 数据库）

### Q3: 如何备份配置？
**A**: 定期备份以下文件：
```bash
# 在服务器上
cd /home/ubuntu/nofx
tar czf nofx-config-backup-$(date +%Y%m%d).tar.gz \
  .env config.json secrets/ prompts/ beta_codes.txt

# 下载到本地
scp -i A.pem ubuntu@43.202.115.56:/home/ubuntu/nofx/nofx-config-backup-*.tar.gz ./
```

---

## 📊 MySQL数据库初始化

首次使用MySQL时，系统会自动：
1. 创建所有必需的数据表
2. 初始化默认的AI模型配置
3. 初始化支持的交易所配置

**如果需要从SQLite迁移数据**：
1. 将本地 `config.db` 上传到服务器
2. 系统启动时会自动检测并迁移数据到MySQL
3. 迁移完成后可以删除 `config.db`

---

## 🎯 总结

**最小必需文件（7个）**：
1. `docker-compose.prod.yml`
2. `.env`
3. `config.json`
4. `secrets/rsa_key`
5. `secrets/rsa_key.pub`
6. `prompts/default.txt`（至少一个提示词）
7. `beta_codes.txt`（如果启用beta模式）

**其他都是源代码和临时文件，不需要上传！**

✅ 使用MySQL后，你的服务器部署变得**超级简单**：
- 不需要管理 SQLite 数据库文件
- 不需要担心数据库锁定问题
- 可以方便地使用数据库客户端查看数据
- 支持多节点部署（未来扩展）

