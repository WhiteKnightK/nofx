# 🚀 NOFX 部署检查清单

## 📋 部署前必查文件

在将代码打包上传到服务器之前，请确保以下文件都已准备好：

### 1. 数据库文件 ⚠️ **最重要**
```bash
config.db          # SQLite数据库文件，包含所有交易历史和决策数据
config.db-shm      # SQLite共享内存文件（如果存在）
config.db-wal      # SQLite预写日志文件（如果存在）
```

> **注意**：如果使用MySQL，则不需要这些文件，但需要配置 `DATABASE_URL` 环境变量

### 2. 配置文件
```bash
config.json        # 主配置文件
.env               # 环境变量文件（包含敏感信息，不要提交到Git）
```

### 3. 目录结构
```bash
decision_logs/     # AI决策日志目录（必须存在，否则日志无法保存）
prompts/           # 提示词模板目录
secrets/           # RSA密钥目录
  ├── private.pem  # RSA私钥
  └── public.pem   # RSA公钥
ssl/               # SSL证书目录（生产环境）
  ├── cert.pem
  └── key.pem
```

### 4. 可选文件
```bash
beta_codes.txt     # 内测码文件（如果启用内测模式）
nginx/nginx.conf   # 自定义Nginx配置（如果需要）
```

---

## 🔧 部署步骤

### 方案A：使用SQLite（推荐用于单机部署）

1. **打包文件**
```bash
# 在本地执行
tar -czf nofx-deploy.tar.gz \
  docker/ \
  web/ \
  config.json \
  config.db \
  config.db-wal \
  config.db-shm \
  decision_logs/ \
  prompts/ \
  secrets/ \
  beta_codes.txt \
  docker-compose.prod.yml \
  .env
```

2. **上传到服务器**
```bash
scp nofx-deploy.tar.gz user@server:/path/to/nofx/
```

3. **解压并启动**
```bash
# 在服务器上执行
cd /path/to/nofx/
tar -xzf nofx-deploy.tar.gz

# 确保 config.db 有正确的权限
chmod 644 config.db

# 确保目录存在
mkdir -p decision_logs

# 启动服务
docker-compose -f docker-compose.prod.yml up -d
```

### 方案B：使用MySQL（推荐用于多实例部署）

1. **配置环境变量**
```bash
# 在服务器的 .env 文件中添加
DATABASE_URL=user:password@tcp(mysql-host:3306)/nofx?charset=utf8mb4&parseTime=True&loc=Local
DB_HOST=mysql-host
DB_PORT=3306
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=nofx
```

2. **注释掉 config.db 挂载**
```yaml
# 在 docker-compose.prod.yml 中
volumes:
  - ./config.json:/app/config.json:ro
  # - ./config.db:/app/config.db  # 使用MySQL时注释掉
```

3. **数据迁移**（如果从SQLite迁移到MySQL）
```bash
# 使用工具导出SQLite数据
sqlite3 config.db .dump > dump.sql

# 导入到MySQL（需要手动调整SQL语法）
mysql -u user -p nofx < dump_converted.sql
```

---

## 🔍 部署后检查

### 1. 检查容器状态
```bash
docker-compose -f docker-compose.prod.yml ps
```

### 2. 检查日志
```bash
# 查看后端日志
docker logs nofx-trading

# 查看前端日志
docker logs nofx-frontend
```

### 3. 检查数据库连接
```bash
# 进入容器
docker exec -it nofx-trading sh

# 检查数据库文件
ls -la /app/config.db

# 测试数据库查询
# （如果安装了sqlite3）
sqlite3 /app/config.db "SELECT COUNT(*) FROM strategy_decision_history;"
```

### 4. 检查API健康状态
```bash
curl http://localhost:8080/api/health
```

### 5. 检查前端访问
```bash
curl http://localhost:3000
```

---

## ❌ 常见问题排查

### 问题1：分析看板没有数据/线图消失

**原因**：`config.db` 文件缺失或未正确挂载

**解决方案**：
```bash
# 1. 检查服务器上是否有 config.db
ls -la /path/to/nofx/config.db

# 2. 检查 docker-compose.prod.yml 中是否挂载了 config.db
grep "config.db" docker-compose.prod.yml

# 3. 如果文件缺失，从本地复制
scp config.db* user@server:/path/to/nofx/

# 4. 重启容器
docker-compose -f docker-compose.prod.yml restart nofx
```

### 问题2：决策日志无法保存

**原因**：`decision_logs` 目录不存在或权限不足

**解决方案**：
```bash
mkdir -p decision_logs
chmod 755 decision_logs
docker-compose -f docker-compose.prod.yml restart nofx
```

### 问题3：数据库被重置为空

**原因**：容器内创建了新的数据库文件，而不是使用挂载的文件

**解决方案**：
```bash
# 1. 停止容器
docker-compose -f docker-compose.prod.yml down

# 2. 确保 config.db 在正确位置
ls -la config.db

# 3. 检查文件权限
chmod 644 config.db

# 4. 重新启动
docker-compose -f docker-compose.prod.yml up -d

# 5. 验证挂载
docker exec nofx-trading ls -la /app/config.db
```

### 问题4：MySQL连接失败

**原因**：环境变量配置错误或MySQL服务不可达

**解决方案**：
```bash
# 1. 检查环境变量
docker exec nofx-trading env | grep DB_

# 2. 测试MySQL连接
docker exec nofx-trading sh -c "nc -zv $DB_HOST $DB_PORT"

# 3. 检查MySQL用户权限
mysql -u $DB_USER -p -h $DB_HOST -e "SHOW GRANTS;"
```

---

## 📊 数据备份建议

### SQLite备份
```bash
# 每日备份脚本
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
cp config.db backups/config_${DATE}.db
# 保留最近7天的备份
find backups/ -name "config_*.db" -mtime +7 -delete
```

### MySQL备份
```bash
# 每日备份脚本
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
mysqldump -u $DB_USER -p$DB_PASSWORD $DB_NAME > backups/nofx_${DATE}.sql
# 保留最近7天的备份
find backups/ -name "nofx_*.sql" -mtime +7 -delete
```

---

## 🔐 安全检查清单

- [ ] `.env` 文件权限设置为 600
- [ ] `secrets/` 目录权限设置为 700
- [ ] RSA私钥权限设置为 600
- [ ] 数据库密码使用强密码
- [ ] JWT_SECRET 使用随机生成的长字符串
- [ ] DATA_ENCRYPTION_KEY 使用随机生成的32字节密钥
- [ ] 生产环境启用HTTPS
- [ ] 防火墙只开放必要端口（80, 443, 8080）

---

## 📝 环境变量模板

创建 `.env.example` 文件作为模板：

```bash
# 时区设置
NOFX_TIMEZONE=Asia/Shanghai

# 端口配置
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000
NOFX_FRONTEND_HTTPS_PORT=3443

# Docker镜像配置
DOCKERHUB_USERNAME=yourusername
IMAGE_TAG=latest

# 安全密钥（请修改为随机生成的值）
DATA_ENCRYPTION_KEY=your-32-byte-encryption-key-here
JWT_SECRET=your-random-jwt-secret-here

# 2FA登录开关
ENABLE_2FA_LOGIN=true

# MySQL配置（可选，不配置则使用SQLite）
DATABASE_URL=
DB_HOST=
DB_PORT=3306
DB_USER=
DB_PASSWORD=
DB_NAME=nofx
```

---

## 🎯 快速修复命令

如果部署后发现数据丢失，立即执行：

```bash
# 1. 停止容器
docker-compose -f docker-compose.prod.yml down

# 2. 从本地复制数据库文件
scp config.db* user@server:/path/to/nofx/

# 3. 确保 docker-compose.prod.yml 中挂载了 config.db
sed -i 's/# - \.\/config\.db/- \.\/config\.db/' docker-compose.prod.yml

# 4. 重新启动
docker-compose -f docker-compose.prod.yml up -d

# 5. 验证数据
docker exec nofx-trading sh -c "ls -la /app/config.db && echo 'Database file found!'"
```
