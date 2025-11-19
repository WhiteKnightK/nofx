# Clash 配置修复 - 允许 WSL2 连接

## 🔍 问题确认

Docker 配置已经正确：
- ✅ IP 地址：`192.168.144.1`
- ✅ 端口：`7890`
- ❌ **但连接被拒绝**：`connection refused`

**原因：Clash 默认只监听 `127.0.0.1`，WSL2 无法访问！**

## ✅ 解决方案：启用 Clash 的"允许局域网连接"

### 步骤1：打开 Clash 设置

1. **打开 Clash for Windows**
2. **点击左侧"设置"（齿轮图标）**
3. **找到"外部控制"或"External Controller"部分**

### 步骤2：启用允许局域网连接

**方法1：通过界面设置（推荐）**

1. 在设置中找到 **"允许局域网连接"** 或 **"Allow LAN"**
2. **启用/打开** 这个选项
3. 保存设置

**方法2：通过配置文件**

1. 找到 Clash 配置文件（通常在 `%USERPROFILE%\.config\clash\config.yaml`）
2. 找到 `allow-lan` 选项
3. 设置为 `allow-lan: true`
4. 重启 Clash

### 步骤3：验证 Clash 监听地址

**检查 Clash 是否监听在 `0.0.0.0:7890`：**

在 Windows PowerShell 中执行：
```powershell
netstat -an | findstr "7890"
```

**应该看到：**
```
TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING
```

**如果只看到 `127.0.0.1:7890`，说明没有启用局域网连接！**

### 步骤4：测试连接

在 WSL2 中测试：
```bash
# 测试代理端口是否可访问
curl -v --proxy http://192.168.144.1:7890 http://www.google.com

# 或者简单测试端口连通性
nc -zv 192.168.144.1 7890
```

### 步骤5：重启 Docker 并测试

```bash
sudo systemctl restart docker
docker pull hello-world
```

## 🔧 如果还是不行

### 检查1：Windows 防火墙

Windows 防火墙可能阻止了 7890 端口：

1. **打开 Windows 防火墙设置**
2. **高级设置** → **入站规则**
3. **新建规则** → **端口** → **TCP** → **7890**
4. **允许连接**
5. **应用到所有配置文件**

### 检查2：Clash 的 HTTP 代理端口

确认 Clash 的 HTTP 代理端口确实是 7890：

1. 打开 Clash
2. 查看"端口设置"或"Port Settings"
3. 确认 HTTP 代理端口

### 检查3：使用不同的 IP

如果 `192.168.144.1` 不行，尝试：

```bash
# 查看所有网络接口
ip addr show

# 或者查看默认网关
ip route | grep default

# 尝试使用网关 IP
```

## 📝 快速检查清单

- [ ] Clash 中启用了"允许局域网连接"
- [ ] Clash 监听在 `0.0.0.0:7890`（不是 `127.0.0.1:7890`）
- [ ] Windows 防火墙允许 7890 端口
- [ ] 从 WSL2 可以访问 `192.168.144.1:7890`
- [ ] Docker 配置正确（已确认 ✅）

## 🎯 最可能的问题

**99% 的情况是 Clash 没有启用"允许局域网连接"！**

请按照上面的步骤1和步骤2操作，这是最关键的一步！






