# 📦 镜像与数据的关系说明

## ⚠️ 重要结论

**镜像里不包含运行时数据！** 本地操作的数据不会自动同步到服务器。

---

## 🔍 详细说明

### 镜像里包含什么？

镜像只包含：
- ✅ **代码**（Go 编译后的二进制文件）
- ✅ **依赖库**（TA-Lib、Go 模块等）
- ✅ **运行时环境**（Alpine Linux、系统库）

### 镜像里不包含什么？

镜像不包含：
- ❌ **数据库文件**（`config.db`）
- ❌ **决策日志**（`decision_logs/`）
- ❌ **配置文件**（`config.json`）
- ❌ **密钥文件**（`secrets/`）
- ❌ **提示词文件**（`prompts/`）

---

## 📋 数据存储方式

从 `docker-compose.yml` 可以看到，所有数据文件都是通过 **Volume 挂载** 的：

```yaml
volumes:
  - ./config.json:/app/config.json:ro      # 配置文件（只读）
  - ./config.db:/app/config.db              # 数据库文件（可读写）
  - ./decision_logs:/app/decision_logs      # 决策日志（可读写）
  - ./prompts:/app/prompts                  # 提示词（可读写）
  - ./secrets:/app/secrets:ro               # 密钥（只读）
```

### Volume 挂载的含义

- 文件存储在**宿主机**（你的电脑或服务器）的文件系统中
- 容器运行时，这些文件通过挂载映射到容器内
- 容器删除后，文件仍然保留在宿主机上
- **镜像构建时，这些文件不会被打包进镜像**

---

## 🔄 实际场景分析

### 场景1：本地操作数据后推送镜像

**操作流程：**
1. 本地构建镜像 ✅
2. 本地运行容器，操作数据（创建交易员、配置等）✅
3. 数据保存在本地的 `config.db` 中 ✅
4. 推送镜像到 Docker Hub ✅
5. 服务器拉取镜像 ✅

**结果：**
- ❌ 服务器上**没有**你本地操作的数据
- ✅ 服务器上需要**单独上传** `config.db` 文件
- ✅ 或者服务器上重新配置（通过 Web 界面）

### 场景2：服务器上已有数据，更新镜像

**操作流程：**
1. 服务器上已有运行中的容器和数据 ✅
2. 本地更新代码，构建新镜像 ✅
3. 推送新镜像 ✅
4. 服务器拉取新镜像并重启 ✅

**结果：**
- ✅ 服务器上的数据**不会丢失**（因为数据在 volume 中）
- ✅ 服务器使用新代码运行
- ✅ 数据文件保持不变

---

## 📤 如何同步数据到服务器

### 方法1：上传数据库文件（推荐）

```bash
# 在本地执行
scp config.db user@server:~/nofx/

# 在服务器上设置权限
ssh user@server
cd ~/nofx
chmod 600 config.db
docker compose -f docker-compose.prod.yml restart nofx
```

### 方法2：通过 Web 界面重新配置

1. 服务器启动后，访问 Web 界面
2. 通过界面重新配置 AI 模型、交易所、交易员等
3. 数据会保存到服务器的 `config.db` 中

### 方法3：导出/导入配置（如果系统支持）

如果系统有导出/导入功能，可以：
1. 本地导出配置
2. 上传到服务器
3. 服务器导入配置

---

## 🎯 最佳实践

### 开发环境（本地）

```bash
# 本地数据存储在：
~/code/nofx/nofx/config.db
~/code/nofx/nofx/decision_logs/
```

### 生产环境（服务器）

```bash
# 服务器数据存储在：
~/nofx/config.db
~/nofx/decision_logs/
```

### 数据备份

**本地备份：**
```bash
cd ~/code/nofx/nofx
cp config.db config.db.backup.$(date +%Y%m%d)
tar -czf backup_$(date +%Y%m%d).tar.gz config.db decision_logs/
```

**服务器备份：**
```bash
cd ~/nofx
cp config.db config.db.backup.$(date +%Y%m%d)
tar -czf backup_$(date +%Y%m%d).tar.gz config.db decision_logs/
```

---

## ⚠️ 常见误解

### ❌ 误解1：镜像包含数据

**错误想法：** "我本地操作了数据，推送到服务器后，服务器也有这些数据"

**实际情况：** 镜像只包含代码，数据需要单独上传

### ❌ 误解2：更新镜像会丢失数据

**错误想法：** "更新镜像后，服务器上的数据会丢失"

**实际情况：** 数据在 volume 中，更新镜像不会影响数据

### ❌ 误解3：本地和服务器数据自动同步

**错误想法：** "本地和服务器使用同一个镜像，数据会自动同步"

**实际情况：** 本地和服务器有各自独立的数据文件

---

## ✅ 正确理解

1. **镜像 = 代码 + 依赖**（不包含数据）
2. **数据 = 存储在宿主机文件系统中**（通过 volume 挂载）
3. **本地和服务器数据独立**（需要手动同步）
4. **更新镜像不影响数据**（数据在 volume 中）

---

## 📝 总结

| 项目 | 是否在镜像中 | 存储位置 | 如何同步 |
|------|------------|---------|---------|
| 代码 | ✅ 是 | 镜像内 | 推送镜像 |
| 数据库 | ❌ 否 | 宿主机文件系统 | 上传 `config.db` |
| 日志 | ❌ 否 | 宿主机文件系统 | 上传 `decision_logs/` |
| 配置 | ❌ 否 | 宿主机文件系统 | 上传 `config.json` |
| 密钥 | ❌ 否 | 宿主机文件系统 | 上传 `secrets/` |

---

## 🔧 实际操作建议

### 首次部署

1. 推送镜像 ✅
2. 上传配置文件（`config.json`, `.env`）✅
3. 上传密钥（`secrets/`）✅
4. 服务器上创建空数据库，通过 Web 界面配置 ✅

### 更新部署

1. 推送新镜像 ✅
2. 服务器拉取新镜像 ✅
3. 重启容器 ✅
4. **数据保持不变** ✅

### 数据迁移

如果需要将本地数据迁移到服务器：

```bash
# 1. 本地备份
tar -czf nofx_data_backup.tar.gz config.db decision_logs/ prompts/

# 2. 上传到服务器
scp nofx_data_backup.tar.gz user@server:~/nofx/

# 3. 在服务器上解压
ssh user@server
cd ~/nofx
tar -xzf nofx_data_backup.tar.gz
chmod 600 config.db
chmod 700 decision_logs
docker compose -f docker-compose.prod.yml restart nofx
```

---

**记住：镜像只包含代码，数据需要单独管理！** 🎯





