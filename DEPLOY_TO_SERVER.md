# 服务器部署指南

本指南说明如何将本地构建好的镜像推送到 Docker Hub，然后在服务器上直接拉取运行，避免在服务器上重新构建。

## 📋 前提条件

1. **本地和服务器使用同一个 Docker Hub 账号**
2. **本地已成功构建镜像**（运行过 `./start.sh start --build`）
3. **已登录 Docker Hub**（本地和服务器都需要）

## 🚀 本地操作：推送镜像

### 步骤 1: 登录 Docker Hub（如果未登录）

```bash
docker login
```

### 步骤 2: 推送镜像

**重要提示：** 推送脚本会自动为镜像添加日期标签（格式：YYYY-MM-DD），这样每次推送都会保留历史版本，不会覆盖之前的镜像。

有两种方式：

#### 方式一：使用推送脚本（推荐）

```bash
# 设置 Docker Hub 用户名（可选，脚本会提示）
export DOCKERHUB_USERNAME=baimastryke

# 运行推送脚本（会自动添加日期标签，如 2024-12-15）
./push_images.sh
```

脚本会同时推送两个标签：
- `latest` - 最新版本
- `YYYY-MM-DD` - 日期标签（如 `2024-12-15`）

#### 方式二：手动推送

```bash
# 设置 Docker Hub 用户名
export DOCKERHUB_USERNAME=baimastryke

# 生成日期标签
DATE_TAG=$(date +%Y-%m-%d)

# 给镜像打标签（latest 和日期标签）
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker tag nofx-nofx:latest ${DOCKERHUB_USERNAME}/nofx-backend:${DATE_TAG}
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:latest
docker tag nofx-nofx-frontend:latest ${DOCKERHUB_USERNAME}/nofx-frontend:${DATE_TAG}

# 推送镜像
docker push ${DOCKERHUB_USERNAME}/nofx-backend:latest
docker push ${DOCKERHUB_USERNAME}/nofx-backend:${DATE_TAG}
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:latest
docker push ${DOCKERHUB_USERNAME}/nofx-frontend:${DATE_TAG}
```

### 步骤 3: 验证推送成功

访问 Docker Hub 网站，确认镜像已上传：
- `https://hub.docker.com/r/baimastryke/nofx-backend`
- `https://hub.docker.com/r/baimastryke/nofx-frontend`

## 🌐 服务器操作：拉取并运行

### 步骤 1: 准备服务器环境

确保服务器已安装：
- Docker
- Docker Compose

### 步骤 2: 上传必要文件到服务器

将以下文件/目录上传到服务器：

```bash
# 必需文件
docker-compose.prod.yml
server_deploy.sh
.env                    # 环境变量文件（包含 DATA_ENCRYPTION_KEY, JWT_SECRET 等）
config.json             # 配置文件
config.db               # 数据库文件（如果已有）
secrets/                # RSA密钥目录（包含 rsa_key 和 rsa_key.pub）
beta_codes.txt          # Beta码文件（如果使用）
prompts/                # 提示词目录（如果使用）
decision_logs/          # 决策日志目录（会自动创建）

# 可选文件
.env.example            # 环境变量模板
config.json.example     # 配置文件模板
```

**重要提示：**
- `secrets/` 目录必须包含 RSA 密钥对
- `.env` 文件必须包含 `DATA_ENCRYPTION_KEY` 和 `JWT_SECRET`
- 确保文件权限正确（secrets 目录 700，.env 文件 600）

### 步骤 3: 在服务器上部署

#### 方式一：使用部署脚本（推荐）

```bash
# 给脚本添加执行权限
chmod +x server_deploy.sh

# 运行部署脚本（使用 latest 标签）
./server_deploy.sh baimastryke

# 或使用特定日期标签（如 2024-12-15）
./server_deploy.sh baimastryke 2024-12-15

# 或者先设置环境变量
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2024-12-15  # 可选，不设置则使用 latest
./server_deploy.sh
```

#### 方式二：手动部署

```bash
# 1. 设置 Docker Hub 用户名
export DOCKERHUB_USERNAME=baimastryke

# 2. 设置镜像标签（可选，不设置则使用 latest）
export IMAGE_TAG=2024-12-15  # 或使用 latest

# 3. 登录 Docker Hub（如果未登录）
docker login

# 4. 拉取镜像
docker compose -f docker-compose.prod.yml pull

# 5. 启动服务
docker compose -f docker-compose.prod.yml up -d

# 6. 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

### 步骤 4: 验证服务运行

```bash
# 查看服务状态
docker compose -f docker-compose.prod.yml ps

# 查看日志
docker compose -f docker-compose.prod.yml logs -f

# 检查健康状态
curl http://localhost:8080/api/health
```

## 🔄 更新镜像流程

当代码更新后，需要重新构建和推送：

### 本地操作

```bash
# 1. 更新代码
git pull

# 2. 重新构建镜像
./start.sh start --build

# 3. 推送新镜像
./push_images.sh
```

### 服务器操作

```bash
# 1. 拉取最新镜像（使用 latest 标签）
export DOCKERHUB_USERNAME=baimastryke
docker compose -f docker-compose.prod.yml pull

# 或使用特定日期标签
export DOCKERHUB_USERNAME=baimastryke
export IMAGE_TAG=2024-12-15
docker compose -f docker-compose.prod.yml pull

# 2. 重启服务（使用新镜像）
docker compose -f docker-compose.prod.yml up -d

# 3. 查看日志确认更新成功
docker compose -f docker-compose.prod.yml logs -f
```

## 📝 配置文件说明

### docker-compose.prod.yml vs docker-compose.yml

- **docker-compose.yml**: 本地开发使用，从源码构建镜像
- **docker-compose.prod.yml**: 生产环境使用，从 Docker Hub 拉取镜像

两者的服务配置完全相同，只是镜像来源不同。

### 环境变量

确保 `.env` 文件包含以下变量：

```bash
# Docker Hub 用户名（用于拉取镜像）
DOCKERHUB_USERNAME=baimastryke

# 端口配置
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080

# 时区
NOFX_TIMEZONE=Asia/Shanghai

# 加密密钥（必须）
DATA_ENCRYPTION_KEY=your_encryption_key
JWT_SECRET=your_jwt_secret
```

## 🐛 常见问题

### 1. 推送失败：认证错误

```bash
# 重新登录 Docker Hub
docker logout
docker login
```

### 2. 拉取失败：镜像不存在

- 确认镜像已成功推送到 Docker Hub
- 检查 Docker Hub 用户名是否正确
- 确认镜像名称和标签正确

### 3. 服务启动失败：缺少密钥

确保以下文件存在：
- `secrets/rsa_key`
- `secrets/rsa_key.pub`
- `.env` 文件包含 `DATA_ENCRYPTION_KEY` 和 `JWT_SECRET`

### 4. 权限问题

```bash
# 修复文件权限
chmod 600 .env
chmod 700 secrets
chmod 600 secrets/rsa_key
chmod 644 secrets/rsa_key.pub
```

## 📚 相关文件

- `push_images.sh` - 推送镜像脚本
- `server_deploy.sh` - 服务器部署脚本
- `docker-compose.prod.yml` - 生产环境配置
- `docker-compose.yml` - 开发环境配置

## 💡 提示

1. **首次部署**：建议先在本地测试 `docker-compose.prod.yml` 配置
2. **镜像标签**：可以推送带版本号的标签，如 `v1.0.0`，便于版本管理
3. **备份**：部署前备份服务器上的 `config.db` 和 `.env` 文件
4. **监控**：使用 `docker compose logs` 监控服务运行状态

