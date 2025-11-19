# Git 同步上游仓库指南

## 📋 概述

当你基于开源项目开发，但不想推送代码到原仓库时，可以通过设置 `upstream` 远程仓库来同步上游的最新更新。

---

## 🔧 设置步骤

### 1. 初始化 Git 仓库（如果还没有）

```bash
cd /home/master/code/nofx
git init
git add .
git commit -m "Initial commit: Fork from upstream"
```

### 2. 添加远程仓库

#### 2.1 添加你的远程仓库（origin）

```bash
# 如果你有自己的 Git 仓库（GitHub/GitLab/Gitee 等）
git remote add origin https://github.com/your-username/your-repo.git

# 或者如果已经存在，检查一下
git remote -v
```

#### 2.2 添加上游仓库（upstream）

```bash
# 添加上游仓库（原开源项目的仓库）
git remote add upstream https://github.com/original-owner/original-repo.git

# 验证配置
git remote -v
```

**输出示例：**
```
origin    https://github.com/your-username/your-repo.git (fetch)
origin    https://github.com/your-username/your-repo.git (push)
upstream  https://github.com/original-owner/original-repo.git (fetch)
upstream  https://github.com/original-owner/original-repo.git (push)
```

---

## 🔄 同步上游更新

### 方法 1: 合并上游更新（推荐）

```bash
# 1. 获取上游仓库的最新更新
git fetch upstream

# 2. 切换到主分支（通常是 main 或 master）
git checkout main  # 或 git checkout master

# 3. 合并上游的更新到当前分支
git merge upstream/main  # 或 git merge upstream/master

# 4. 如果有冲突，解决冲突后提交
# git add .
# git commit -m "Merge upstream updates"

# 5. 推送到你自己的仓库
git push origin main
```

### 方法 2: 使用 rebase（保持提交历史更清晰）

```bash
# 1. 获取上游更新
git fetch upstream

# 2. 切换到主分支
git checkout main

# 3. 使用 rebase 合并（你的提交会在上游提交之后）
git rebase upstream/main

# 4. 如果有冲突，解决后继续
# git add .
# git rebase --continue

# 5. 推送到你自己的仓库（需要 force push）
git push origin main --force-with-lease
```

**⚠️ 注意：** rebase 会重写提交历史，如果已经推送到远程，需要使用 `--force-with-lease`（比 `--force` 更安全）

---

## 🛠️ 处理冲突

当合并上游更新时，可能会遇到冲突：

### 1. 查看冲突文件

```bash
git status
```

### 2. 手动解决冲突

冲突文件会包含类似这样的标记：

```go
<<<<<<< HEAD
// 你的代码
func yourFunction() {
    // ...
}
=======
// 上游的代码
func upstreamFunction() {
    // ...
}
>>>>>>> upstream/main
```

**解决步骤：**
1. 编辑冲突文件，选择保留的代码（你的、上游的、或两者结合）
2. 删除冲突标记（`<<<<<<<`, `=======`, `>>>>>>>`）
3. 保存文件

### 3. 标记冲突已解决

```bash
# 添加解决后的文件
git add <冲突文件>

# 如果使用 merge
git commit -m "Merge upstream: resolve conflicts"

# 如果使用 rebase
git rebase --continue
```

---

## 📝 最佳实践

### 1. 定期同步上游更新

建议每周或每月同步一次，避免积累太多冲突：

```bash
# 创建同步脚本
cat > sync-upstream.sh << 'EOF'
#!/bin/bash
echo "🔄 同步上游仓库..."
git fetch upstream
git checkout main
git merge upstream/main
echo "✅ 同步完成！"
echo "📝 请检查是否有冲突，然后推送到你的仓库："
echo "   git push origin main"
EOF

chmod +x sync-upstream.sh
```

### 2. 使用分支策略

**推荐的工作流程：**

```bash
# 1. 保持主分支与上游同步
git checkout main
git fetch upstream
git merge upstream/main

# 2. 创建功能分支进行开发
git checkout -b feature/your-feature

# 3. 开发完成后合并到主分支
git checkout main
git merge feature/your-feature
git push origin main

# 4. 定期同步上游更新到主分支
git fetch upstream
git merge upstream/main
```

### 3. 保护主分支

在你的 Git 托管平台（GitHub/GitLab）设置：
- 禁止直接推送到主分支
- 使用 Pull Request/Merge Request 进行代码审查
- 要求 CI/CD 通过后才能合并

---

## 🔍 常用命令

### 查看远程仓库

```bash
# 查看所有远程仓库
git remote -v

# 查看上游仓库的更新
git fetch upstream
git log HEAD..upstream/main --oneline
```

### 比较差异

```bash
# 比较你的代码和上游的差异
git diff upstream/main

# 查看你的提交（上游没有的）
git log upstream/main..HEAD

# 查看上游的提交（你没有的）
git log HEAD..upstream/main
```

### 更新远程仓库信息

```bash
# 更新所有远程仓库的信息
git remote update

# 更新特定远程仓库
git fetch upstream
```

---

## 🎯 完整工作流程示例

### 场景：同步上游的安全补丁

```bash
# 1. 确保当前工作已保存
git status

# 2. 提交或暂存当前更改
git add .
git commit -m "WIP: My current changes"

# 或者暂存更改
git stash

# 3. 切换到主分支
git checkout main

# 4. 获取上游更新
git fetch upstream

# 5. 查看上游有什么更新
git log HEAD..upstream/main --oneline

# 6. 合并上游更新
git merge upstream/main

# 7. 如果有冲突，解决冲突
# 编辑冲突文件...
git add .
git commit -m "Merge upstream: security patches"

# 8. 推送到你的仓库
git push origin main

# 9. 如果有暂存的更改，恢复
git checkout feature/your-feature
git stash pop
```

---

## ⚠️ 注意事项

### 1. 不要推送到上游仓库

```bash
# ❌ 错误：不要这样做
git push upstream main

# ✅ 正确：只推送到你自己的仓库
git push origin main
```

### 2. 保护你的更改

在合并上游更新前，确保你的重要更改已提交：

```bash
# 查看未提交的更改
git status

# 提交更改
git add .
git commit -m "Save my changes before sync"

# 或者创建备份分支
git branch backup-$(date +%Y%m%d)
```

### 3. 测试合并后的代码

合并上游更新后，务必测试：

```bash
# 合并后
git merge upstream/main

# 运行测试
go test ./...

# 启动服务测试
go run main.go

# 确认无误后推送
git push origin main
```

---

## 🔄 自动化同步脚本

创建一个自动化同步脚本：

```bash
#!/bin/bash
# sync-upstream.sh

set -e  # 遇到错误立即退出

echo "🔄 开始同步上游仓库..."

# 检查是否有未提交的更改
if ! git diff-index --quiet HEAD --; then
    echo "⚠️  检测到未提交的更改，请先提交或暂存"
    echo "   使用: git stash 暂存更改"
    exit 1
fi

# 获取当前分支
CURRENT_BRANCH=$(git branch --show-current)
echo "📍 当前分支: $CURRENT_BRANCH"

# 切换到主分支
echo "📦 切换到主分支..."
git checkout main

# 获取上游更新
echo "⬇️  获取上游更新..."
git fetch upstream

# 查看更新内容
echo "📋 上游更新内容:"
git log HEAD..upstream/main --oneline

# 合并更新
echo "🔀 合并上游更新..."
if git merge upstream/main --no-edit; then
    echo "✅ 合并成功！"
else
    echo "❌ 合并冲突！请手动解决冲突后运行:"
    echo "   git add ."
    echo "   git commit"
    exit 1
fi

# 推送到自己的仓库
echo "⬆️  推送到自己的仓库..."
git push origin main

# 切换回原分支
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "🔄 切换回原分支: $CURRENT_BRANCH"
    git checkout "$CURRENT_BRANCH"
fi

echo "🎉 同步完成！"
```

**使用方法：**

```bash
chmod +x sync-upstream.sh
./sync-upstream.sh
```

---

## 📚 参考资源

- [Git 官方文档 - 远程仓库](https://git-scm.com/book/zh/v2/Git-基础-远程仓库的使用)
- [GitHub 文档 - 同步 Fork](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/syncing-a-fork)
- [GitLab 文档 - 同步 Fork](https://docs.gitlab.com/ee/user/project/repository/forking_workflow.html)

---

## 🎯 总结

### 核心步骤

1. ✅ 添加 `upstream` 远程仓库
2. ✅ 定期 `git fetch upstream` 获取更新
3. ✅ `git merge upstream/main` 合并更新
4. ✅ 解决冲突（如果有）
5. ✅ `git push origin main` 推送到自己的仓库

### 关键原则

- ✅ **只读 upstream**：只从 upstream 拉取，不推送
- ✅ **只写 origin**：只推送到自己的仓库
- ✅ **定期同步**：避免积累太多冲突
- ✅ **测试验证**：合并后务必测试

### 推荐配置

```bash
# 一次性设置
git remote add upstream <上游仓库URL>
git remote set-url --push upstream no_push  # 防止误推送到上游

# 验证配置
git remote -v
```

这样即使误操作 `git push upstream`，也会被阻止。





