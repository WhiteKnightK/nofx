# 解决 Docker Hub 连接超时问题

## 🔍 问题分析

**错误信息：**
```
failed to fetch oauth token: Post "https://auth.docker.io/token": dial tcp 198.18.0.20:443: i/o timeout
```

**原因：** 无法访问 Docker Hub（`auth.docker.io`），需要配置镜像加速器

## ✅ 解决方案

### 方案1：配置 Docker 镜像加速器（推荐）

#### Windows (WSL2)

```bash
# 1. 创建或编辑 Docker 配置文件
sudo mkdir -p /etc/docker
sudo nano /etc/docker/daemon.json
```

**添加以下内容：**
```json
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com",
    "https://dockerproxy.com"
  ]
}
```

**保存后重启 Docker：**
```bash
# 在 Windows 上重启 Docker Desktop
# 或者在 WSL2 中：
sudo service docker restart
# 或者重启 Docker Desktop
```

#### Linux (Ubuntu/Debian)

```bash
# 1. 创建或编辑 Docker 配置文件
sudo mkdir -p /etc/docker
sudo nano /etc/docker/daemon.json
```

**添加以下内容：**
```json
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com",
    "https://dockerproxy.com"
  ]
}
```

**保存后重启 Docker：**
```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

#### macOS

1. 打开 Docker Desktop
2. 点击设置（Settings）
3. 选择 Docker Engine
4. 添加以下配置：

```json
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com",
    "https://dockerproxy.com"
  ]
}
```

5. 点击 Apply & Restart

### 方案2：使用代理（如果有）

```bash
# 设置代理环境变量
export HTTP_PROXY=http://your-proxy:port
export HTTPS_PROXY=http://your-proxy:port

# 然后构建
docker compose build
```

### 方案3：手动拉取基础镜像（临时方案）

如果镜像加速器配置后还是有问题，可以手动拉取基础镜像：

```bash
# 拉取所需的基础镜像
docker pull node:20-alpine
docker pull nginx:alpine
docker pull alpine:latest
docker pull golang:1.25-alpine

# 然后再构建
docker compose build
```

## 🔧 快速修复脚本

创建 `fix_docker_mirror.sh`：

```bash
#!/bin/bash

echo "=========================================="
echo "配置 Docker 镜像加速器"
echo "=========================================="

# 检测操作系统
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    DOCKER_CONFIG="/etc/docker/daemon.json"
    RESTART_CMD="sudo systemctl restart docker"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    echo "macOS 请手动在 Docker Desktop 中配置"
    exit 0
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
    # Windows (WSL2)
    DOCKER_CONFIG="/etc/docker/daemon.json"
    RESTART_CMD="echo '请重启 Docker Desktop'"
fi

# 创建配置目录
sudo mkdir -p /etc/docker

# 备份现有配置
if [ -f "$DOCKER_CONFIG" ]; then
    sudo cp "$DOCKER_CONFIG" "$DOCKER_CONFIG.backup"
    echo "✓ 已备份现有配置"
fi

# 创建新配置
sudo tee "$DOCKER_CONFIG" > /dev/null <<EOF
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com",
    "https://dockerproxy.com"
  ]
}
EOF

echo "✓ 已配置镜像加速器"

# 重启 Docker
echo "正在重启 Docker..."
eval $RESTART_CMD

echo ""
echo "=========================================="
echo "✅ 配置完成！"
echo "=========================================="
echo ""
echo "验证配置："
docker info | grep -A 10 "Registry Mirrors"
```

## ✅ 验证配置

```bash
# 检查镜像加速器是否生效
docker info | grep -A 10 "Registry Mirrors"

# 应该看到类似输出：
# Registry Mirrors:
#  https://docker.mirrors.ustc.edu.cn/
#  https://hub-mirror.c.163.com/
#  https://mirror.baidubce.com/
#  https://dockerproxy.com/
```

## 🚀 配置后重新构建

```bash
# 清理之前的构建缓存
docker builder prune -f

# 重新构建
docker compose build --no-cache
```

## 📋 完整的解决步骤

1. **配置 Docker 镜像加速器**（见上方）
2. **重启 Docker**
3. **验证配置**：`docker info | grep "Registry Mirrors"`
4. **重新构建**：`docker compose build --no-cache`

## 🎯 如果还是不行

### 使用国内 Docker 镜像仓库

如果 Docker Hub 完全无法访问，可以考虑使用：
- **阿里云容器镜像服务**：https://cr.console.aliyun.com/
- **腾讯云容器镜像服务**：https://cloud.tencent.com/product/tcr
- **华为云容器镜像服务**：https://console.huaweicloud.com/swr/

这些服务通常提供更稳定的国内访问。

## ⚠️ 注意事项

1. **Windows WSL2**：配置后需要重启 Docker Desktop，不是重启 WSL2
2. **权限问题**：Linux 需要使用 `sudo`
3. **配置格式**：JSON 格式必须正确，否则 Docker 无法启动
4. **多个镜像源**：Docker 会按顺序尝试，第一个失败会自动尝试下一个

## 🎉 完成！

配置镜像加速器后，Docker 构建应该可以正常访问基础镜像了！






