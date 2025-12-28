# 🚀 中国国内快速开始指南

在中国国内使用 Docker 推送镜像的完整解决方案。**无需代理，开箱即用！**

---

## ⚡ 5分钟快速配置

### 步骤 1: 配置 Docker 镜像加速

运行自动配置脚本：

```bash
cd /home/master/code/nofx/nofx
./setup_docker_china.sh
```

这个脚本会自动：
- ✅ 配置国内 Docker 镜像加速器
- ✅ 配置国内 DNS
- ✅ 优化 Docker 性能
- ✅ 备份现有配置
- ✅ 自动测试配置

### 步骤 2: 配置阿里云容器镜像服务

#### 2.1 开通服务

1. 访问 [阿里云容器镜像服务](https://cr.console.aliyun.com/)
2. 登录并开通个人版（免费）
3. 创建命名空间，例如：`nofx`

#### 2.2 获取配置信息

在控制台记录以下信息：
- **镜像仓库地址**：`registry.cn-hangzhou.aliyuncs.com/nofx`
- **用户名**：您的阿里云账号
- **密码**：在"访问凭证"中设置

#### 2.3 设置环境变量

编辑 `~/.bashrc`（或 `~/.zshrc`）：

```bash
nano ~/.bashrc
```

添加以下内容：

```bash
# 阿里云 ACR 配置
export ALIYUN_REGISTRY="registry.cn-hangzhou.aliyuncs.com/nofx"
export ALIYUN_USERNAME="your-aliyun-username"
```

保存后执行：

```bash
source ~/.bashrc
```

### 步骤 3: 登录阿里云 ACR

```bash
docker login --username=${ALIYUN_USERNAME} registry.cn-hangzhou.aliyuncs.com
```

输入您在步骤 2.2 中设置的密码。

### 步骤 4: 构建和推送镜像

```bash
./quick_build_push.sh
```

选择选项 **1**（阿里云容器镜像服务）

---

## 🎯 使用说明

### 构建推送脚本选项

运行 `./quick_build_push.sh` 时，您有 3 个选项：

```
请选择镜像仓库:
1) 阿里云容器镜像服务 (推荐，国内无需代理)  ← 推荐
2) Docker Hub (需要稳定网络)
3) 同时推送到两个仓库
```

**推荐选择选项 1**：
- ✅ 完全无需代理
- ✅ 推送速度快
- ✅ 稳定可靠
- ✅ 免费使用

### 自动化推送（跳过交互）

设置所有环境变量后，脚本会自动运行：

```bash
export ALIYUN_REGISTRY="registry.cn-hangzhou.aliyuncs.com/nofx"
export IMAGE_TAG="v1.0.0"  # 可选

./quick_build_push.sh
# 脚本会自动使用配置的仓库
```

---

## 📦 文件说明

本次配置创建/修改了以下文件：

| 文件名 | 说明 |
|--------|------|
| `setup_docker_china.sh` | Docker 自动配置脚本 |
| `docker-daemon-china.json` | Docker daemon 配置文件 |
| `quick_build_push.sh` | 构建和推送镜像脚本（已更新） |
| `DOCKER_CHINA_CONFIG.md` | 完整配置指南 |
| `QUICK_START_CHINA.md` | 本快速开始指南 |

---

## 🔧 常见问题

### Q1: 构建时仍然超时怎么办？

**A:** 确保已运行 `setup_docker_china.sh` 配置 Docker 镜像加速。

验证配置：
```bash
docker info | grep -A 5 "Registry Mirrors"
```

如果没有输出，重新运行配置脚本。

### Q2: 推送到阿里云 ACR 失败

**A:** 检查登录状态：

```bash
# 重新登录
docker logout registry.cn-hangzhou.aliyuncs.com
docker login --username=${ALIYUN_USERNAME} registry.cn-hangzhou.aliyuncs.com
```

### Q3: 环境变量不生效

**A:** 确保已执行 `source ~/.bashrc`，或重新打开终端。

验证环境变量：
```bash
echo $ALIYUN_REGISTRY
echo $ALIYUN_USERNAME
```

### Q4: 服务器如何拉取阿里云镜像？

**A:** 修改服务器上的 `docker-compose.prod.yml`：

```yaml
services:
  nofx:
    image: registry.cn-hangzhou.aliyuncs.com/nofx/nofx-backend:latest
  
  nofx-frontend:
    image: registry.cn-hangzhou.aliyuncs.com/nofx/nofx-frontend:latest
```

然后拉取更新：

```bash
ssh ubuntu@your-server
cd /home/ubuntu/nofx
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

---

## 📊 配置验证

运行以下命令验证配置：

```bash
# 1. 检查 Docker 镜像加速
docker info | grep "Registry Mirrors"

# 2. 检查环境变量
echo "ALIYUN_REGISTRY: $ALIYUN_REGISTRY"
echo "ALIYUN_USERNAME: $ALIYUN_USERNAME"

# 3. 测试登录状态
docker login registry.cn-hangzhou.aliyuncs.com

# 4. 测试推送（使用测试镜像）
docker tag alpine:latest ${ALIYUN_REGISTRY}/test:latest
docker push ${ALIYUN_REGISTRY}/test:latest
```

如果以上都成功，配置完成！🎉

---

## 🚀 完整工作流程

### 本地开发 → 推送镜像

```bash
# 1. 编写代码
vim main.go

# 2. 构建并推送镜像
./quick_build_push.sh
# 选择选项 1（阿里云）

# 等待构建和推送完成...
```

### 服务器部署

```bash
# SSH 到服务器
ssh -i A.pem ubuntu@43.202.115.56

# 进入项目目录
cd /home/ubuntu/nofx

# 拉取最新镜像
docker compose -f docker-compose.prod.yml pull

# 重启服务
docker compose -f docker-compose.prod.yml up -d

# 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

---

## 🎯 最佳实践

### 1. 使用镜像标签管理版本

```bash
# 使用语义化版本号
export IMAGE_TAG="v1.2.3"
./quick_build_push.sh

# 或使用日期
export IMAGE_TAG=$(date +%Y-%m-%d-%H%M)
./quick_build_push.sh
```

### 2. 自动化脚本

创建 `deploy.sh`：

```bash
#!/bin/bash
set -e

# 构建和推送
export IMAGE_TAG="v1.0.0"
./quick_build_push.sh

# 自动部署到服务器
ssh ubuntu@your-server "cd /home/ubuntu/nofx && \
  docker compose -f docker-compose.prod.yml pull && \
  docker compose -f docker-compose.prod.yml up -d"

echo "部署完成！"
```

### 3. 定期清理镜像

```bash
# 清理悬挂镜像
docker image prune -f

# 清理所有未使用的镜像
docker image prune -a -f

# 清理构建缓存
docker builder prune -f
```

---

## 📖 深入了解

想了解更多配置细节和高级用法？查看完整文档：

- **[DOCKER_CHINA_CONFIG.md](./DOCKER_CHINA_CONFIG.md)** - 完整配置指南
- **[BUILD_AND_DEPLOY.md](./BUILD_AND_DEPLOY.md)** - 构建和部署指南
- **[SERVER_SETUP.md](./SERVER_SETUP.md)** - 服务器设置指南

---

## 🎉 总结

通过本指南，您已经：

✅ 配置了 Docker 镜像加速（国内高速）  
✅ 设置了阿里云容器镜像服务（无需代理）  
✅ 掌握了构建和推送镜像的流程  
✅ 了解了完整的 CI/CD 工作流

**现在您可以在国内无障碍地使用 Docker 了！** 🚀

如有问题，请参考 `DOCKER_CHINA_CONFIG.md` 的故障排除部分。



