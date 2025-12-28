# 🚀 Docker 中国国内配置指南

本指南帮助您在中国国内无需代理即可流畅使用 Docker。

---

## 📋 目录

1. [配置 Docker 镜像加速](#1-配置-docker-镜像加速)
2. [配置阿里云容器镜像服务](#2-配置阿里云容器镜像服务-acr)
3. [使用构建推送脚本](#3-使用构建推送脚本)
4. [常见问题解决](#4-常见问题解决)

---

## 1. 配置 Docker 镜像加速

### 1.1 WSL2 / Linux 系统

#### 方法一：使用提供的配置文件（推荐）

```bash
# 1. 创建或编辑 Docker daemon 配置
sudo mkdir -p /etc/docker
sudo cp docker-daemon-china.json /etc/docker/daemon.json

# 2. 重启 Docker
sudo systemctl daemon-reload
sudo systemctl restart docker

# 3. 验证配置
docker info | grep -A 10 "Registry Mirrors"
```

#### 方法二：手动配置

```bash
# 1. 编辑配置文件
sudo nano /etc/docker/daemon.json

# 2. 添加以下内容：
{
  "registry-mirrors": [
    "https://docker.1panel.live",
    "https://docker.m.daocloud.io",
    "https://docker.unsee.tech",
    "https://docker.awsl9527.cn"
  ],
  "dns": ["223.5.5.5", "114.114.114.114", "8.8.8.8"]
}

# 3. 保存后重启 Docker
sudo systemctl daemon-reload
sudo systemctl restart docker
```

### 1.2 Docker Desktop (Windows/Mac)

1. 打开 Docker Desktop
2. 点击 Settings (设置) → Docker Engine
3. 添加镜像配置：

```json
{
  "registry-mirrors": [
    "https://docker.1panel.live",
    "https://docker.m.daocloud.io"
  ]
}
```

4. 点击 "Apply & Restart"

### 1.3 验证配置

```bash
# 测试拉取镜像速度
docker pull alpine:latest

# 查看镜像加速器配置
docker info | grep -A 10 "Registry Mirrors"
```

---

## 2. 配置阿里云容器镜像服务 (ACR)

阿里云 ACR 是国内最佳的容器镜像仓库，**完全无需代理**。

### 2.1 创建阿里云 ACR 账号

1. 访问：https://cr.console.aliyun.com/
2. 登录或注册阿里云账号
3. 开通容器镜像服务（免费个人版足够使用）

### 2.2 创建命名空间

1. 在 ACR 控制台，点击 "命名空间"
2. 创建一个命名空间，例如：`nofx`
3. 记录您的镜像仓库地址，格式如：
   ```
   registry.cn-hangzhou.aliyuncs.com/nofx
   ```

### 2.3 获取访问凭证

1. 在 ACR 控制台，点击 "访问凭证"
2. 设置固定密码或使用访问令牌
3. 记录用户名和密码

### 2.4 登录阿里云 ACR

```bash
# 登录到您的阿里云镜像仓库
docker login --username=您的阿里云账号 registry.cn-hangzhou.aliyuncs.com

# 输入密码后登录成功
```

### 2.5 配置环境变量

在 `~/.bashrc` 或 `~/.zshrc` 中添加：

```bash
# 阿里云 ACR 配置
export ALIYUN_REGISTRY="registry.cn-hangzhou.aliyuncs.com/nofx"
export ALIYUN_USERNAME="您的阿里云账号"
```

然后执行：

```bash
source ~/.bashrc  # 或 source ~/.zshrc
```

---

## 3. 使用构建推送脚本

我们的 `quick_build_push.sh` 脚本已支持阿里云 ACR。

### 3.1 快速使用

```bash
# 进入项目目录
cd /home/master/code/nofx/nofx

# 运行脚本
./quick_build_push.sh
```

### 3.2 选择镜像仓库

脚本会提示您选择：

```
请选择镜像仓库:
1) 阿里云容器镜像服务 (推荐，国内无需代理)
2) Docker Hub (需要稳定网络)
3) 同时推送到两个仓库

请输入选项 [1/2/3] (默认: 1):
```

**推荐选择选项 1**，使用阿里云 ACR。

### 3.3 首次使用

首次运行时，脚本会提示输入阿里云镜像仓库地址：

```bash
请输入阿里云镜像仓库地址 (格式: registry.cn-hangzhou.aliyuncs.com/your-namespace):
```

输入您在步骤 2.2 中创建的地址。

### 3.4 自动化配置

设置环境变量后，可跳过手动输入：

```bash
# 设置环境变量
export ALIYUN_REGISTRY="registry.cn-hangzhou.aliyuncs.com/nofx"
export IMAGE_TAG="2025-11-27"  # 可选，默认使用当前日期

# 运行脚本（自动使用环境变量）
./quick_build_push.sh
```

---

## 4. 常见问题解决

### 4.1 问题：`operation timed out` 或网络超时

**原因：** Alpine 镜像源访问失败

**解决方案：**

1. 确认 Docker daemon 已配置镜像加速（见步骤 1）
2. 检查 Dockerfile 中的镜像源配置：

```dockerfile
# 应使用阿里云或 USTC 镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
```

3. 重试构建：

```bash
# 清除构建缓存后重试
docker system prune -af
./quick_build_push.sh
```

### 4.2 问题：推送到 Docker Hub 失败

**原因：** Docker Hub 在国内访问不稳定

**解决方案：**

1. **推荐：使用阿里云 ACR**（见步骤 2）
2. 或配置代理：

```bash
# 临时使用代理
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890

# 重新登录和推送
docker login
docker push your-image
```

### 4.3 问题：Go module 下载失败

**原因：** Go 默认代理不可用

**解决方案：**

Dockerfile 中已配置国内 Go 代理：

```dockerfile
ENV GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct
ENV GOSUMDB=off
```

如仍失败，手动设置：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=off
```

### 4.4 问题：TA-Lib 源码下载失败

**原因：** SourceForge 在国内访问不稳定

**解决方案：**

1. 手动下载 ta-lib 源码：

```bash
# 从备用源下载
wget https://github.com/TA-Lib/ta-lib/releases/download/v0.4.0/ta-lib-0.4.0-src.tar.gz
```

2. 修改 Dockerfile，使用本地文件：

```dockerfile
# 替换 wget 命令为 COPY
COPY ta-lib-0.4.0-src.tar.gz /tmp/
RUN cd /tmp && tar -xzf ta-lib-0.4.0-src.tar.gz && ...
```

### 4.5 问题：阿里云 ACR 配额不足

**免费个人版限制：**
- 命名空间：3 个
- 镜像仓库：300 个
- 存储空间：10 GB

**解决方案：**

1. 清理旧镜像
2. 或升级到企业版（按需付费）

---

## 5. 最佳实践

### 5.1 本地开发

```bash
# 使用镜像加速 + 本地构建
./quick_build_push.sh
# 选择选项 1（阿里云 ACR）
```

### 5.2 服务器部署

在服务器 `docker-compose.prod.yml` 中使用阿里云镜像：

```yaml
services:
  nofx:
    image: registry.cn-hangzhou.aliyuncs.com/nofx/nofx-backend:latest
  
  nofx-frontend:
    image: registry.cn-hangzhou.aliyuncs.com/nofx/nofx-frontend:latest
```

### 5.3 CI/CD 流程

```bash
# 1. 本地构建推送
./quick_build_push.sh

# 2. 服务器拉取更新
ssh ubuntu@your-server
cd /home/ubuntu/nofx
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

---

## 6. 验证配置成功

运行以下命令验证：

```bash
# 1. 检查 Docker 镜像加速
docker info | grep -A 5 "Registry Mirrors"

# 2. 测试拉取速度
time docker pull alpine:latest

# 3. 测试推送到阿里云 ACR
docker tag alpine:latest ${ALIYUN_REGISTRY}/test:latest
docker push ${ALIYUN_REGISTRY}/test:latest

# 4. 运行构建脚本
./quick_build_push.sh
```

如果以上步骤都成功，说明配置完成！

---

## 📞 需要帮助？

如遇到问题，请检查：

1. Docker daemon 配置是否正确
2. 阿里云 ACR 是否已登录
3. 网络连接是否正常
4. 防火墙设置

**常用命令：**

```bash
# 查看 Docker 日志
sudo journalctl -u docker -n 50

# 查看镜像加速器状态
docker info

# 重启 Docker
sudo systemctl restart docker

# 清理 Docker 缓存
docker system prune -af
```

---

## 🎉 总结

通过本指南的配置，您可以：

✅ 在国内无需代理流畅使用 Docker  
✅ 快速构建和推送镜像到阿里云 ACR  
✅ 解决常见的网络超时问题  
✅ 实现高效的 CI/CD 工作流

**推荐方案：** 阿里云 ACR + Docker 镜像加速 = 完美体验！



