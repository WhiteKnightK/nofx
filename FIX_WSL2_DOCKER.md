# WSL2 中配置 Docker 镜像加速器

## 🔍 问题分析

在 WSL2 中，Docker 是通过 Docker Desktop 运行的，配置文件位置不同：
- ❌ `/etc/docker/daemon.json`（Linux 方式，在 WSL2 中可能不生效）
- ✅ Docker Desktop 设置（Windows 方式，正确的方法）

## ✅ 正确的配置方法

### 方法1：通过 Docker Desktop GUI 配置（推荐）

1. **打开 Docker Desktop**
   - 在 Windows 系统托盘中找到 Docker 图标
   - 右键点击 → Settings（设置）

2. **进入 Docker Engine 设置**
   - 左侧菜单选择 "Docker Engine"
   - 右侧会显示 JSON 配置

3. **添加镜像加速器配置**
   - 在 JSON 配置中添加或修改 `registry-mirrors`：

```json
{
  "builder": {
    "gc": {
      "defaultKeepStorage": "20GB",
      "enabled": true
    }
  },
  "experimental": false,
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com",
    "https://dockerproxy.com"
  ]
}
```

4. **应用并重启**
   - 点击 "Apply & Restart"
   - 等待 Docker Desktop 重启完成

5. **验证配置**
   ```bash
   docker info | grep -A 10 "Registry Mirrors"
   ```

### 方法2：直接编辑 Docker Desktop 配置文件

Docker Desktop 的配置文件位置：
- Windows: `%USERPROFILE%\.docker\daemon.json`
- 或者: `C:\Users\你的用户名\.docker\daemon.json`

**编辑步骤：**
1. 在 Windows 中打开文件资源管理器
2. 输入路径：`%USERPROFILE%\.docker\`
3. 如果 `daemon.json` 不存在，创建它
4. 添加以下内容：

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

5. 保存文件
6. 重启 Docker Desktop

## 🔧 快速操作步骤

### 步骤1：重启 Docker Desktop

1. 右键点击系统托盘中的 Docker 图标
2. 选择 "Quit Docker Desktop"（退出）
3. 等待完全退出
4. 重新启动 Docker Desktop
5. 等待 Docker 完全启动（图标不再闪烁）

### 步骤2：验证配置

在 WSL2 终端中执行：

```bash
docker info | grep -A 10 "Registry Mirrors"
```

**应该看到：**
```
Registry Mirrors:
 https://docker.mirrors.ustc.edu.cn/
 https://hub-mirror.c.163.com/
 https://mirror.baidubce.com/
 https://dockerproxy.com/
```

### 步骤3：测试拉取镜像

```bash
# 测试拉取一个小镜像
docker pull alpine:latest

# 如果成功，说明配置生效了
```

### 步骤4：重新构建项目

```bash
cd /home/master/code/nofx/nofx
docker compose build --no-cache
```

## ⚠️ 常见问题

### 问题1：配置后还是超时

**可能原因：**
- Docker Desktop 没有完全重启
- 镜像加速器本身也访问不了

**解决方案：**
1. 完全退出 Docker Desktop（任务管理器中确认没有 docker 进程）
2. 重新启动 Docker Desktop
3. 等待 1-2 分钟让 Docker 完全启动
4. 再试一次

### 问题2：找不到 Docker Desktop 设置

**解决方案：**
- 确保 Docker Desktop 正在运行
- 在 Windows 开始菜单搜索 "Docker Desktop"
- 或者直接编辑配置文件：`%USERPROFILE%\.docker\daemon.json`

### 问题3：WSL2 中 docker 命令找不到

**解决方案：**
```bash
# 确保 Docker Desktop 正在运行
# 在 Windows 中检查 Docker Desktop 是否启动

# 如果还是不行，可能需要重新安装 Docker Desktop
```

## 🎯 完整操作流程

1. ✅ **配置镜像加速器**（通过 Docker Desktop GUI）
2. ✅ **重启 Docker Desktop**（完全退出后重新启动）
3. ✅ **验证配置**：`docker info | grep "Registry Mirrors"`
4. ✅ **测试拉取**：`docker pull alpine:latest`
5. ✅ **重新构建**：`docker compose build --no-cache`

## 📝 配置文件示例

**Docker Desktop 配置文件位置：**
- Windows: `C:\Users\你的用户名\.docker\daemon.json`

**完整配置示例：**
```json
{
  "builder": {
    "gc": {
      "defaultKeepStorage": "20GB",
      "enabled": true
    }
  },
  "experimental": false,
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com",
    "https://dockerproxy.com"
  ],
  "insecure-registries": [],
  "debug": false
}
```

## 🚀 配置完成后

配置生效后，Docker 构建应该可以正常拉取基础镜像了！

```bash
# 清理之前的构建缓存
docker builder prune -f

# 重新构建
docker compose build --no-cache
```





