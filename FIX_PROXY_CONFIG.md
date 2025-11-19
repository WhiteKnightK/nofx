# 修复 Docker 代理配置问题

## 🔍 问题分析

**错误信息：**
```
proxyconnect tcp: dial tcp :7890: connect: connection refused
```

**原因：**
1. 配置文件中的 IP 地址解析失败，导致代理地址变成了 `http://:7890`（缺少 IP）
2. 需要使用正确的 WSL 网络接口 IP：`192.168.144.1`

## ✅ 解决方案

### 方法1：使用固定 IP（推荐）

根据 Clash 显示的网络接口，WSL 的 IP 是 `192.168.144.1`。

**修复配置文件：**

```bash
# 使用固定的 WSL 网络接口 IP
HOST_IP="192.168.144.1"
PROXY_PORT=7890

sudo tee /etc/systemd/system/docker.service.d/proxy.conf > /dev/null <<EOF
# proxy.conf
[Service]
ExecStartPre=/bin/bash -c "echo http_proxy=http://${HOST_IP}:${PROXY_PORT} > /tmp/docker_env"
ExecStartPre=/bin/bash -c "echo https_proxy=http://${HOST_IP}:${PROXY_PORT} >> /tmp/docker_env"

[Service]
EnvironmentFile=-/tmp/docker_env
Environment=no_proxy="127.0.0.1,localhost"
EOF

# 重新加载并重启
sudo systemctl daemon-reload
sudo systemctl restart docker
```

### 方法2：修复动态 IP 获取（如果 IP 会变化）

如果 WSL IP 会变化，使用以下脚本动态获取：

```bash
sudo tee /etc/systemd/system/docker.service.d/proxy.conf > /dev/null <<'EOF'
# proxy.conf
[Service]
ExecStartPre=/bin/bash -c 'HOST_IP=$(ip route show | grep -i default | awk '\''{ print $3 }'\''); echo http_proxy=http://${HOST_IP}:7890 > /tmp/docker_env'
ExecStartPre=/bin/bash -c 'HOST_IP=$(ip route show | grep -i default | awk '\''{ print $3 }'\''); echo https_proxy=http://${HOST_IP}:7890 >> /tmp/docker_env'

[Service]
EnvironmentFile=-/tmp/docker_env
Environment=no_proxy="127.0.0.1,localhost"
EOF

sudo systemctl daemon-reload
sudo systemctl restart docker
```

## 🔧 重要：配置 Clash 允许 WSL2 连接

**Clash 默认只监听 `127.0.0.1`，需要修改为 `0.0.0.0` 才能被 WSL2 访问：**

1. **打开 Clash 设置**
   - 点击 "设置" → "外部控制"

2. **修改监听地址**
   - 找到 "允许局域网连接" 或 "Allow LAN"
   - **启用** 这个选项
   - 或者修改监听地址为 `0.0.0.0:7890`

3. **保存并重启 Clash**

## ✅ 验证配置

```bash
# 1. 检查环境变量
sudo cat /tmp/docker_env
# 应该显示：
# http_proxy=http://192.168.144.1:7890
# https_proxy=http://192.168.144.1:7890

# 2. 检查 Docker 服务状态
sudo systemctl status docker

# 3. 测试代理连接
docker pull hello-world

# 4. 如果还是失败，检查 Clash 日志
# 在 Clash 中查看 "日志" 标签页
```

## 🐛 故障排查

### 问题1：连接被拒绝

**可能原因：**
- Clash 没有启用"允许局域网连接"
- Clash 监听地址是 `127.0.0.1` 而不是 `0.0.0.0`

**解决：**
1. 在 Clash 中启用"允许局域网连接"
2. 重启 Clash

### 问题2：IP 地址不对

**检查方法：**
```bash
# 查看 WSL2 的默认网关（这就是 Windows 主机的 IP）
ip route show | grep default

# 或者查看 resolv.conf
cat /etc/resolv.conf | grep nameserver
```

**如果 IP 不是 192.168.144.1，修改配置文件中的 IP**

### 问题3：代理端口不对

**检查 Clash 的 HTTP 代理端口：**
1. 打开 Clash
2. 查看 "设置" → "端口设置"
3. 确认 HTTP 代理端口（默认是 7890）

**如果端口不是 7890，修改配置文件中的端口**

## 📝 完整配置步骤总结

1. ✅ **修复 Docker 代理配置**（使用正确的 IP）
2. ✅ **配置 Clash 允许局域网连接**
3. ✅ **重启 Docker 服务**
4. ✅ **测试连接**

## 🎯 快速修复命令

```bash
# 一键修复（使用固定 IP 192.168.144.1，端口 7890）
HOST_IP="192.168.144.1"
PROXY_PORT=7890

sudo tee /etc/systemd/system/docker.service.d/proxy.conf > /dev/null <<EOF
# proxy.conf
[Service]
ExecStartPre=/bin/bash -c "echo http_proxy=http://${HOST_IP}:${PROXY_PORT} > /tmp/docker_env"
ExecStartPre=/bin/bash -c "echo https_proxy=http://${HOST_IP}:${PROXY_PORT} >> /tmp/docker_env"

[Service]
EnvironmentFile=-/tmp/docker_env
Environment=no_proxy="127.0.0.1,localhost"
EOF

sudo systemctl daemon-reload
sudo systemctl restart docker

# 验证
sudo cat /tmp/docker_env
docker pull hello-world
```

## ⚠️ 重要提醒

**在 Clash 中必须启用"允许局域网连接"，否则 WSL2 无法访问代理！**







