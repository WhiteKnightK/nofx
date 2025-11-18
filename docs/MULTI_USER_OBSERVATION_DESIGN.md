# 多用户观测系统设计文档（最小化修改版）

## 📋 概述

实现一个四级权限的数据观测系统，支持：
1. **真正的管理员（Admin）** - 可以管理所有交易员（跨用户）、分类、创建账号（一般情况下不使用）
2. **普通用户（User）** - 可以创建分类，管理自己分类下的交易员，为交易员创建账号，为分类创建小组组长账号
3. **小组组长（Group Leader）** - 可以查看指定分类的所有交易员数据（只读）
4. **交易员账号（Trader Account）** - 只能查看自己的交易员数据（只读）

**注意：** 一般情况下使用三级权限体系（User / Group Leader / Trader Account），Admin作为特殊角色保留但不常用。

**核心特点：**
- 普通用户可以创建分类，管理自己分类下的交易员（数据隔离）
- 分类属于创建它的用户，一个用户可以创建多个分类
- 交易员属于某个分类，交易员账号继承交易员的分类
- **普通用户可以为自己的交易员创建账号（Trader Account）**
- **普通用户可以为自己的分类创建小组组长账号（Group Leader）**
- 小组组长可以观测指定分类（只能是自己创建的分类）
- 除了普通用户，其他人只能查看面板，不能配置（隐藏配置功能）

**从属关系链：**
```
注册用户（User）
  └─ 创建分类（Category）
      ├─ 创建交易员（Trader）
      │   └─ 创建交易员账号（Trader Account，属于该分类）
      └─ 创建小组组长账号（Group Leader）
          └─ 观测指定分类（Category）
              └─ 查看该分类下的所有交易员

管理员（Admin，特殊角色，一般不使用）
  └─ 可以管理所有交易员（跨用户）
      └─ 可以创建所有类型的账号
```

**设计原则：**
- ✅ **最小化修改**：尽可能复用现有代码
- ✅ **隐藏/显示**：主要通过UI隐藏/显示操作按钮实现
- ✅ **后端过滤**：后端API根据角色过滤交易员列表
- ✅ **前端复用**：交易员切换下拉列表自动根据API返回的数据显示（无需修改）

---

## ⚠️ 重要说明：Admin Mode vs 用户角色

### 现有系统的两种模式

#### 1. Admin Mode（系统级配置）
- **位置**：`config.json` 中的 `admin_mode` 字段
- **用途**：单用户自托管模式
- **特点**：
  - `admin_mode=true` 时：禁用用户注册，只能通过 `/api/admin-login` 登录
  - 登录后 `user_id` 固定为 `"admin"`，`email` 为 `"admin@localhost"`
  - 这是一个**系统级别的开关**，用于控制整个系统的运行模式

#### 2. 用户注册（多用户模式）
- **位置**：数据库 `users` 表
- **用途**：多用户模式，支持用户注册
- **特点**：
  - `admin_mode=false` 时，用户可以注册
  - 注册的用户需要 OTP 验证
  - 这些用户是真正的数据库用户，有自己的 `user_id` 和 `email`

### 我们的设计（不冲突）

**关键理解：**
- **Admin Mode** = 系统配置开关（控制是否允许注册）
- **用户角色（role）** = 数据库字段（控制用户权限）

**用户角色分配：**
1. **通过注册的用户** → `role='user'`（需要 OTP 验证）
   - 这些用户只能看到和管理**自己创建的交易员**
   - 需要 Google Authenticator 验证
   - 不能看到其他用户的交易员
   - **可以创建分类**
   - **可以为自己的交易员创建账号（trader_account）**
   - **可以为自己的分类创建小组组长账号（group_leader）**

2. **真正的管理员** → `role='admin'`（需要 OTP 验证，特殊角色，一般不使用）
   - 可以管理**所有交易员**（跨用户）
   - 可以创建所有类型的账号（group_leader 或 trader_account）
   - 需要 Google Authenticator 验证
   - 通常由系统管理员手动设置
   - **注意**：一般情况下不使用此角色，默认使用User角色

3. **普通用户创建的账号** → `role='group_leader'` 或 `role='trader_account'`（不需要 OTP）
   - 这些账号由普通用户创建
   - 不需要 OTP 验证，直接登录
   - 只能查看数据，不能配置
   - group_leader 只能查看创建者指定的分类
   - trader_account 只能查看关联的交易员

**兼容性：**
- ✅ 不影响现有的 Admin Mode 功能
- ✅ 注册的用户默认是 `role='user'`（可以创建分类和账号）
- ✅ 普通用户创建的账号是只读角色（不需要 OTP）
- ✅ 向后兼容：没有 `role` 字段的用户默认为 `user`，只能看到自己的交易员

---

## 🏗️ 系统架构设计

### 1. 数据库设计

#### 1.1 用户表扩展 (`users`)

```sql
-- 添加角色字段
ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'user';
-- 角色值: 'admin' | 'user' | 'group_leader' | 'trader_account'

-- 添加关联字段
ALTER TABLE users ADD COLUMN trader_id TEXT DEFAULT NULL;
-- 如果 role = 'trader_account'，此字段指向关联的交易员ID
-- FOREIGN KEY (trader_id) REFERENCES traders(id)

ALTER TABLE users ADD COLUMN category TEXT DEFAULT NULL;
-- 如果 role = 'trader_account'，此字段存储交易员所属的分类（冗余字段，方便查询）
```

**角色说明：**
- `admin`: 真正的管理员，可以管理**所有交易员**（跨用户）、分类、创建账号、配置所有内容（特殊角色，一般不使用）
  - **来源**：由系统管理员手动设置（通常第一个用户或特殊用户）
  - **登录**：需要 OTP 验证（Google Authenticator）
  - **权限**：完整的管理权限（可以看到所有用户的交易员）
  - **注意**：一般情况下不使用此角色，默认使用User角色
- `user`: 普通用户，可以创建分类，管理**自己分类下的交易员**，创建账号
  - **来源**：通过注册的用户（`admin_mode=false` 时）
  - **登录**：需要 OTP 验证（Google Authenticator）
  - **权限**：
    - 可以创建分类
    - 可以创建交易员（属于某个分类）
    - 只能看到和管理自己分类下的交易员
    - **可以为自己的交易员创建账号（trader_account）**
    - **可以为自己的分类创建小组组长账号（group_leader）**
- `group_leader`: 小组组长，可以查看指定分类的所有交易员数据，不能配置
  - **来源**：由普通用户创建
  - **登录**：不需要 OTP，直接登录
  - **权限**：只读权限（查看指定分类内的交易员）
  - **限制**：只能查看创建者指定的分类（不能跨用户查看）
- `trader_account`: 交易员账号，只能查看自己的交易员数据，不能配置
  - **来源**：由普通用户创建
  - **登录**：不需要 OTP，直接登录
  - **权限**：只读权限（查看自己的交易员）
  - **从属关系**：属于某个分类

**注意：**
- `admin_mode` 是系统级开关（`config.json`），控制是否允许注册
- `role` 是用户级角色（数据库字段），控制用户权限
- `role='user'` 是普通注册用户，可以创建分类和管理自己的交易员
- 两者**不冲突**，可以同时存在

#### 1.2 交易员表扩展 (`traders`)

```sql
-- 添加分类字段（保留现有数据）
ALTER TABLE traders ADD COLUMN category TEXT DEFAULT '';
-- 交易员分类/组名，用于分组管理
-- 注意：现有交易员的category默认为空字符串，不影响现有数据

-- 添加交易员账号关联字段（保留现有数据）
ALTER TABLE traders ADD COLUMN trader_account_id TEXT DEFAULT NULL;
-- 关联的交易员账号用户ID（如果有）
-- FOREIGN KEY (trader_account_id) REFERENCES users(id)
-- 注意：现有交易员的trader_account_id默认为NULL，不影响现有数据

-- 添加所有者用户ID字段（保留现有数据）
ALTER TABLE traders ADD COLUMN owner_user_id TEXT DEFAULT NULL;
-- 创建该交易员的用户ID，用于权限控制
-- 注意：现有交易员的owner_user_id默认为NULL，需要数据迁移时根据user_id关联设置
```

#### 1.3 新增表：小组组长分类关联表 (`group_leader_categories`)

```sql
CREATE TABLE IF NOT EXISTS group_leader_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_leader_id TEXT NOT NULL,  -- 小组组长用户ID
    category TEXT NOT NULL,          -- 分类名称
    owner_user_id TEXT NOT NULL,     -- 分类所有者用户ID（创建该分类的用户）
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (group_leader_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(group_leader_id, category)  -- 防止重复关系
);

CREATE INDEX idx_group_leader ON group_leader_categories(group_leader_id);
CREATE INDEX idx_category ON group_leader_categories(category);
CREATE INDEX idx_owner_user ON group_leader_categories(owner_user_id);
```

**用途：**
- 建立小组组长与分类的关联关系
- 一个小组组长可以观测多个分类
- 一个分类可以被多个小组组长观测（如果需要）
- **owner_user_id 确保小组组长只能查看创建者指定的分类（数据隔离）**
- 分类必须属于某个注册用户（通过 categories 表关联）

---

## 🔐 权限控制设计

### 2.1 权限层级

```
┌─────────────────────────────────────────┐
│         User (普通用户)                   │
│  - 创建分类                               │
│  - 创建交易员（属于某个分类）              │
│  - 管理自己分类下的交易员                  │
│  - 为交易员创建账号（Trader Account）     │
│  - 为分类创建小组组长账号（Group Leader）  │
│  - 配置自己的内容                         │
└─────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        │                        │
        │ 创建分类                │ 创建账号
        ▼                        ▼
┌──────────────────┐  ┌─────────────────────────┐
│  Category        │  │  Group Leader           │
│  (分类)           │  │  (小组组长)              │
│  - 属于某个User   │  │  - 观测指定分类的交易员  │
│  - 包含多个交易员  │  │  - 不能配置（只读）       │
└──────────────────┘  └─────────────────────────┘
        │                        │
        │ 创建交易员               │ 观测
        ▼                        ▼
┌──────────────────┐  ┌─────────────────────────┐
│  Trader          │  │  Trader Account         │
│  (交易员)         │  │  (交易员账号)            │
│  - 属于某个分类    │  │  - 属于某个分类          │
│  - 可以创建账号    │  │  - 查看自己的交易员数据 │
└──────────────────┘  │  - 不能配置（只读）      │
                       └─────────────────────────┘
```

### 2.2 数据访问规则

| 操作 | Admin | User | Group Leader | Trader Account |
|------|-------|------|--------------|----------------|
| 查看所有交易员（跨用户） | ✅ | ❌ | ❌ | ❌ |
| 查看自己分类的交易员 | ✅ | ✅ | ❌ | ❌ |
| 查看指定分类的交易员 | ✅ | ❌ | ✅（创建者指定的分类） | ❌ |
| 查看自己的交易员 | ✅ | ✅ | ❌ | ✅ |
| 创建分类 | ✅ | ✅ | ❌ | ❌ |
| 创建交易员（属于分类） | ✅（所有） | ✅（自己的分类） | ❌ | ❌ |
| 编辑/删除交易员 | ✅（所有） | ✅（自己的分类） | ❌ | ❌ |
| 配置交易员参数 | ✅（所有） | ✅（自己的分类） | ❌ | ❌ |
| 创建交易员账号 | ✅（所有） | ✅（自己的交易员） | ❌ | ❌ |
| 创建小组组长账号 | ✅（所有） | ✅（自己的分类） | ❌ | ❌ |
| 查看交易员详情 | ✅（所有） | ✅（自己的分类） | ✅（观测的分类） | ✅（自己的） |
| 查看统计数据 | ✅（所有） | ✅（自己的分类） | ✅（观测的分类） | ✅（自己的） |

**注意：** Admin角色一般情况下不使用，默认使用User角色。

### 2.3 UI功能显示规则

| 功能模块 | Admin | User | Group Leader | Trader Account |
|----------|-------|------|--------------|----------------|
| 交易员列表 | ✅ 所有交易员 | ✅ 自己分类的交易员 | ✅ 只读（观测的分类） | ✅ 只读（自己的） |
| 创建分类按钮 | ✅ | ✅ | ❌ 隐藏 | ❌ 隐藏 |
| 创建交易员按钮 | ✅（所有） | ✅（自己的分类） | ❌ 隐藏 | ❌ 隐藏 |
| 编辑交易员按钮 | ✅（所有） | ✅（自己的分类） | ❌ 隐藏 | ❌ 隐藏 |
| 删除交易员按钮 | ✅（所有） | ✅（自己的分类） | ❌ 隐藏 | ❌ 隐藏 |
| AI模型配置 | ✅（所有） | ✅（自己的分类） | ❌ 隐藏 | ❌ 隐藏 |
| 交易所配置 | ✅（所有） | ✅（自己的分类） | ❌ 隐藏 | ❌ 隐藏 |
| 信号源配置 | ✅（所有） | ✅（自己的分类） | ❌ 隐藏 | ❌ 隐藏 |
| 分类管理 | ✅（所有） | ✅（自己的） | ❌ 隐藏 | ❌ 隐藏 |
| 创建交易员账号 | ✅（所有） | ✅（自己的交易员） | ❌ 隐藏 | ❌ 隐藏 |
| 创建小组组长账号 | ✅（所有） | ✅（自己的分类） | ❌ 隐藏 | ❌ 隐藏 |
| 交易员详情页 | ✅ 完整（所有） | ✅ 完整（自己的分类） | ✅ 只读 | ✅ 只读 |

**注意：** Admin角色一般情况下不使用，默认使用User角色。

---

## 📊 API 设计（最小化修改）

### 3.1 用户信息接口扩展

#### GET `/api/account` (修改现有接口)
**修改位置：** `api/server.go` - `handleAccount` 函数

**响应扩展：**
```json
{
  "id": "user123",
  "email": "user@example.com",
  "role": "admin",  // 新增：用户角色
  "trader_id": null,  // 新增：如果是交易员账号，关联的交易员ID
  "categories": []  // 新增：如果是小组组长，管理的分类列表
}
```

**修改方式：**
- 在返回账户信息时，从数据库查询用户角色和关联信息
- 添加到响应JSON中

### 3.2 交易员列表接口（核心修改）

#### GET `/api/my-traders` (修改现有接口)
**修改位置：** `api/server.go:1096` - `handleTraderList` 函数

**权限控制逻辑：**
```go
// 伪代码示例
func (s *Server) handleTraderList(c *gin.Context) {
    userID := c.GetString("user_id")
    
    // 1. 获取用户角色和权限信息
    user, _ := s.database.GetUserByID(userID)
    role := user.Role // "admin" | "group_leader" | "trader_account"
    
    var traders []*TraderRecord
    
    // 2. 根据角色过滤交易员列表
    switch role {
    case "admin":
        // 管理员：返回所有交易员
        traders, _ = s.database.GetAllTraders()
    case "group_leader":
        // 小组组长：返回管理的分类内的交易员
        categories := s.database.GetGroupLeaderCategories(userID)
        traders, _ = s.database.GetTradersByCategories(categories)
    case "trader_account":
        // 交易员账号：只返回自己的交易员
        traders, _ = s.database.GetTradersByID(user.TraderID)
    default:
        // 默认：返回用户自己的交易员（向后兼容）
        traders, _ = s.database.GetTraders(userID)
    }
    
    // 3. 返回结果（保持现有格式，无需修改前端）
    // ... 现有代码 ...
}
```

**响应格式：**（保持现有格式不变）
```json
[
  {
    "trader_id": "trader123",
    "trader_name": "Trader 1",
    "ai_model": "deepseek",
    "exchange_id": "binance",
    "is_running": true,
    "initial_balance": 1000.0
  }
]
```

**关键点：**
- ✅ **响应格式不变**：前端无需修改
- ✅ **后端过滤**：根据角色在数据库查询时过滤
- ✅ **交易员下拉列表自动适配**：前端下拉列表使用 `traders` 数组，自动只显示有权限的交易员

### 3.3 交易员详情接口

#### GET `/api/traders/:id`
**权限控制：**
- **Admin**: 可以查看所有交易员详情（特殊角色，一般不使用）
- **User**: 只能查看自己分类内的交易员详情（或owner_user_id为自己的交易员）
- **Group Leader**: 只能查看管理的分类内的交易员详情（创建者指定的分类）
- **Trader Account**: 只能查看自己的交易员详情

**权限检查实现：**
```go
func (s *Server) handleTraderDetails(c *gin.Context) {
    userID := c.GetString("user_id")
    traderID := c.Param("id")
    
    // 获取用户角色
    user, err := s.database.GetUserByID(userID)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
        return
    }
    
    role := user.Role
    if role == "" {
        role = "user" // 默认是普通用户
    }
    
    // 获取交易员信息
    trader, err := s.database.GetTraderByID(traderID)
    if err != nil || trader == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
        return
    }
    
    // 权限检查
    canAccess := false
    switch role {
    case "admin":
        // 管理员可以访问所有交易员
        canAccess = true
    case "user":
        // 普通用户：检查owner_user_id或分类权限
        if trader.OwnerUserID == userID {
            canAccess = true
        } else if trader.Category != "" {
            // 检查分类是否属于该用户
            category, _ := s.database.GetCategoryByName(trader.Category)
            if category != nil && category.OwnerUserID == userID {
                canAccess = true
            }
        }
    case "group_leader":
        // 小组组长：检查交易员是否在管理的分类内
        if trader.Category != "" {
            categories := s.database.GetGroupLeaderCategories(userID)
            for _, cat := range categories {
                if cat == trader.Category {
                    canAccess = true
                    break
                }
            }
        }
    case "trader_account":
        // 交易员账号：检查是否是自己的交易员
        if user.TraderID == traderID {
            canAccess = true
        }
    }
    
    if !canAccess {
        c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该交易员"})
        return
    }
    
    // 返回交易员详情（现有逻辑）
    // ...
}
```

**响应格式：**（保持不变，但增加权限检查）

### 3.3.1 创建交易员接口（修改现有接口）

#### POST `/api/traders` (修改现有接口)
**修改位置：** `api/server.go` - `handleCreateTrader` 函数

**修改内容：** 创建交易员时设置 `owner_user_id` 和 `category`

**请求体扩展：**
```json
{
  "name": "Trader 1",
  "ai_model_id": "deepseek",
  "exchange_id": "binance",
  "initial_balance": 1000.0,
  "category": "Group A"  // 新增：可选，分类名称（如果提供，必须属于当前用户）
}
```

**实现逻辑：**
```go
func (s *Server) handleCreateTrader(c *gin.Context) {
    userID := c.GetString("user_id")
    var req CreateTraderRequest
    
    // ... 现有验证逻辑 ...
    
    // 创建交易员配置
    trader := &config.TraderRecord{
        ID:            traderID,
        UserID:        userID,
        OwnerUserID:   userID,  // 新增：设置为当前用户ID
        Category:      "",      // 新增：默认为空字符串
        Name:          req.Name,
        AIModelID:     req.AIModelID,
        ExchangeID:    req.ExchangeID,
        InitialBalance: actualBalance,
        // ... 其他字段 ...
    }
    
    // 如果提供了category，验证并设置
    if req.Category != "" {
        // 验证分类是否属于当前用户
        category, err := s.database.GetCategoryByName(req.Category)
        if err != nil || category == nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "分类不存在"})
            return
        }
        if category.OwnerUserID != userID {
            c.JSON(http.StatusForbidden, gin.H{"error": "只能使用自己的分类"})
            return
        }
        trader.Category = req.Category
    }
    
    // 保存到数据库
    err = s.database.CreateTrader(trader)
    // ... 后续逻辑 ...
}
```

**关键点：**
- ✅ `owner_user_id` 自动设置为当前用户ID
- ✅ `category` 默认为空字符串，可选设置
- ✅ 如果提供category，必须验证属于当前用户
- ✅ 交易员和分类独立：交易员可以不属于任何分类，分类删除不影响交易员

### 3.4 登录接口修改（核心修改）

#### POST `/api/login` (修改现有接口)
**修改位置：** `api/server.go:1732` - `handleLogin` 函数

**修改逻辑：根据用户角色决定是否需要OTP验证**

```go
func (s *Server) handleLogin(c *gin.Context) {
    var req struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required"`
    }
    
    // 获取用户信息
    user, err := s.database.GetUserByEmail(req.Email)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
        return
    }
    
    // 验证密码
    if !auth.CheckPassword(req.Password, user.PasswordHash) {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
        return
    }
    
    // 获取用户角色（默认为user，向后兼容）
    role := user.Role
    if role == "" {
        role = "user"  // 默认是普通用户
    }
    
    // 根据角色决定是否需要OTP验证
    if role == "admin" || role == "user" {
        // 管理员或普通用户（注册用户）：需要OTP验证
        if !user.OTPVerified {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error":              "账户未完成OTP设置",
                "user_id":            user.ID,
                "requires_otp_setup": true,
            })
            return
        }
        
        // 返回需要OTP验证的状态
        c.JSON(http.StatusOK, gin.H{
            "user_id":      user.ID,
            "email":        user.Email,
            "message":      "请输入Google Authenticator验证码",
            "requires_otp": true,
        })
        return
    } else {
        // 创建的账号（group_leader 或 trader_account）：不需要OTP，直接登录
        // 这些账号由普通用户创建，不需要OTP验证
        token, err := auth.GenerateJWT(user.ID, user.Email)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "token":   token,
            "user_id": user.ID,
            "email":   user.Email,
            "role":    role,
            "message": "登录成功",
        })
        return
    }
}
```

**关键点：**
- ✅ **隐性判断**：登录页面看起来一样，后端自动判断角色
- ✅ **注册用户需要OTP**：通过注册的用户（role='admin' 或 'user'）需要OTP验证
- ✅ **创建的账号直接登录**：普通用户创建的账号（role='group_leader' 或 'trader_account'）跳过OTP
- ✅ **向后兼容**：没有role字段的用户默认为user，只能看到自己的交易员
- ✅ **不冲突Admin Mode**：Admin Mode是系统开关，role是用户权限，两者独立工作
- ✅ **数据隔离**：普通用户（role='user'）只能看到自己创建的交易员，不会影响其他用户
- ✅ **Admin角色保留**：Admin角色保留但不常用，一般情况下使用User角色

### 3.5 创建交易员账号接口（新增）

#### POST `/api/traders/:id/create-account` (新增)
**权限：** Admin, User（Admin可以创建所有交易员的账号，User只能为自己的交易员创建账号）

**功能：** 为交易员创建账号（trader_account）

**请求体：**
```json
{
  "generate_random_email": true,    // true=账号随机生成，false=手动输入账号
  "generate_random_password": true, // true=密码随机生成，false=手动输入密码
  "email": "user@example.com",      // 如果generate_random_email=false，必填
  "password": "password123"         // 如果generate_random_password=false，必填
}
```

**四种组合模式：**
1. **账号随机，密码自己输入**：`generate_random_email=true, generate_random_password=false, password必填`
2. **密码随机，账号自己输入**：`generate_random_email=false, generate_random_password=true, email必填`
3. **全随机**：`generate_random_email=true, generate_random_password=true`
4. **全自己输入**：`generate_random_email=false, generate_random_password=false, email和password必填`

**注意：**
- Admin可以为所有交易员创建账号（特殊角色，一般不使用）
- 普通用户只能为自己的交易员创建账号
- 需要验证交易员的owner_user_id与当前用户ID匹配（Admin跳过此检查）
- **交易员账号自动继承交易员的分类**：
  - 如果交易员的 `category` 不为空，交易员账号的 `category` 设置为相同的值
  - 如果交易员的 `category` 为空字符串，交易员账号的 `category` 也设为空字符串
  - **交易员账号可以创建，不受分类限制**（即使交易员没有分类也可以创建账号）
- 如果随机生成，返回的响应中会包含生成的账号和密码（仅此一次）

#### POST `/api/group-leaders/create` (新增)
**权限：** Admin, User（Admin可以创建所有分类的小组组长账号，User只能为自己的分类创建小组组长账号）

**功能：** 创建小组组长账号（Admin可以创建所有分类的小组组长账号，User只能指定自己创建的分类）

**请求体：**
```json
{
  "generate_random_email": true,    // true=账号随机生成，false=手动输入账号
  "generate_random_password": true, // true=密码随机生成，false=手动输入密码
  "email": "leader@example.com",   // 如果generate_random_email=false，必填
  "password": "password123",        // 如果generate_random_password=false，必填
  "categories": ["Group A", "Group B"]  // 必填：可以观测的分类列表
}
```

**四种组合模式：**
1. **账号随机，密码自己输入**：`generate_random_email=true, generate_random_password=false, password必填`
2. **密码随机，账号自己输入**：`generate_random_email=false, generate_random_password=true, email必填`
3. **全随机**：`generate_random_email=true, generate_random_password=true`
4. **全自己输入**：`generate_random_email=false, generate_random_password=false, email和password必填`

**响应：**
```json
{
  "user_id": "trader_account_123",
  "email": "trader1@example.com",  // 最终使用的账号（随机生成或手动输入）
  "password": "random_password_abc123",  // 最终使用的密码（随机生成或手动输入，仅此一次返回）
  "role": "trader_account",
  "trader_id": "trader123",  // 如果role=trader_account
  "categories": []  // 如果role=group_leader
}
```

**响应说明：**
- `email` 和 `password` 字段始终返回，无论是否随机生成
- 如果随机生成，返回生成的账号和密码（仅此一次，需要用户保存）
- 如果手动输入，返回用户输入的账号和密码（确认信息）
- 前端需要显示这些信息并提示用户保存（特别是随机生成的情况）

**实现逻辑：**
```go
func (s *Server) handleCreateTraderAccount(c *gin.Context) {
    userID := c.GetString("user_id")
    traderID := c.Param("id")
    
    // 检查用户角色（必须是admin或user）
    user, _ := s.database.GetUserByID(userID)
    if user.Role != "admin" && user.Role != "user" {
        c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
        return
    }
    
    // 验证交易员是否存在
    trader, _ := s.database.GetTraderByID(traderID)
    if trader == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
        return
    }
    
    // 如果不是admin，验证交易员是否属于当前用户
    if user.Role != "admin" && trader.OwnerUserID != userID {
        c.JSON(http.StatusForbidden, gin.H{"error": "只能为自己的交易员创建账号"})
        return
    }
    
    var req struct {
        GenerateRandomEmail    bool   `json:"generate_random_email"`
        GenerateRandomPassword bool   `json:"generate_random_password"`
        Email                  string `json:"email"`
        Password               string `json:"password"`
    }
    
    // 验证必填字段
    if !req.GenerateRandomEmail && req.Email == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "账号未选择随机生成时，必须提供email"})
        return
    }
    if !req.GenerateRandomPassword && req.Password == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "密码未选择随机生成时，必须提供password"})
        return
    }
    
    // 根据四种组合模式生成账号信息
    var accountEmail, accountPassword string
    
    // 1. 账号处理：随机生成或使用输入的
    if req.GenerateRandomEmail {
        accountEmail = generateRandomEmail()
    } else {
        accountEmail = req.Email
    }
    
    // 2. 密码处理：随机生成或使用输入的
    if req.GenerateRandomPassword {
        accountPassword = generateRandomPassword(12)  // 12位随机密码
    } else {
        accountPassword = req.Password
    }
    
    // 创建用户（trader_account角色）
    passwordHash, _ := auth.HashPassword(accountPassword)
    newUserID := uuid.New().String()
    
    newUser := &config.User{
        ID:           newUserID,
        Email:        accountEmail,
        PasswordHash: passwordHash,
        Role:         "trader_account",
        TraderID:     traderID,
        Category:     trader.Category,  // 自动继承交易员的分类（如果为空字符串，也设为空）
        OTPSecret:    "",  // 不需要OTP
        OTPVerified:  true, // 直接设置为已验证（跳过OTP）
    }
    
    // 注意：如果交易员的category为空字符串，交易员账号的category也设为空字符串
    // 交易员账号可以创建，不受分类限制
    
    s.database.CreateUser(newUser)
    
    // 更新交易员的trader_account_id
    s.database.UpdateTraderAccountID(traderID, newUserID)
    
    c.JSON(http.StatusOK, gin.H{
        "user_id":  newUserID,
        "email":    accountEmail,
        "password": accountPassword,  // 返回密码（仅此一次）
        "role":     "trader_account",
        "trader_id": traderID,
    })
}

func (s *Server) handleCreateGroupLeader(c *gin.Context) {
    userID := c.GetString("user_id")
    
    // 检查用户角色（必须是admin或user）
    user, _ := s.database.GetUserByID(userID)
    if user.Role != "admin" && user.Role != "user" {
        c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
        return
    }
    
    var req struct {
        GenerateRandomEmail    bool     `json:"generate_random_email"`
        GenerateRandomPassword bool     `json:"generate_random_password"`
        Email                  string   `json:"email"`
        Password               string   `json:"password"`
        Categories             []string `json:"categories" binding:"required"`  // 必填：可以观测的分类列表
    }
    
    // 验证必填字段
    if !req.GenerateRandomEmail && req.Email == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "账号未选择随机生成时，必须提供email"})
        return
    }
    if !req.GenerateRandomPassword && req.Password == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "密码未选择随机生成时，必须提供password"})
        return
    }
    
    // 如果不是admin，验证分类是否属于当前用户
    if user.Role != "admin" {
        userCategories := s.database.GetUserCategories(userID)
        for _, cat := range req.Categories {
            if !contains(userCategories, cat) {
                c.JSON(http.StatusForbidden, gin.H{"error": "只能为自己的分类创建小组组长"})
                return
            }
        }
    }
    
    // 根据四种组合模式生成账号信息
    var accountEmail, accountPassword string
    
    // 1. 账号处理：随机生成或使用输入的
    if req.GenerateRandomEmail {
        accountEmail = generateRandomEmail()
    } else {
        accountEmail = req.Email
    }
    
    // 2. 密码处理：随机生成或使用输入的
    if req.GenerateRandomPassword {
        accountPassword = generateRandomPassword(12)  // 12位随机密码
    } else {
        accountPassword = req.Password
    }
    
    // 创建用户（group_leader角色）
    passwordHash, _ := auth.HashPassword(accountPassword)
    newUserID := uuid.New().String()
    
    newUser := &config.User{
        ID:           newUserID,
        Email:        accountEmail,
        PasswordHash: passwordHash,
        Role:         "group_leader",
        OTPSecret:    "",  // 不需要OTP
        OTPVerified:  true, // 直接设置为已验证（跳过OTP）
    }
    
    s.database.CreateUser(newUser)
    
    // 关联分类（group_leader_categories表）
    // 注意：owner_user_id必须设置为创建者的用户ID，确保数据隔离
    for _, cat := range req.Categories {
        err := s.database.InsertGroupLeaderCategory(newUserID, cat, userID)  // 第三个参数是owner_user_id
        if err != nil {
            // 如果关联失败，回滚用户创建
            s.database.DeleteUser(newUserID)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "创建小组组长失败"})
            return
        }
    }
    
    c.JSON(http.StatusOK, gin.H{
        "user_id":   newUserID,
        "email":     accountEmail,
        "password":  accountPassword,  // 返回密码（仅此一次）
        "role":      "group_leader",
        "categories": req.Categories,
    })
}
```

### 3.6 注册接口修改（重要）

#### POST `/api/register` (修改现有接口)
**修改位置：** `api/server.go:1572` - `handleRegister` 函数

**修改内容：** 注册时设置 `role='user'`

```go
// 在 handleRegister 函数中，创建用户时设置role
user := &config.User{
    ID:           userID,
    Email:        req.Email,
    PasswordHash: passwordHash,
    OTPSecret:    otpSecret,
    OTPVerified:  false,
    Role:         "user",  // 注册的用户默认是user角色（只能管理自己的交易员）
}
```

**关键点：**
- ✅ 注册的用户默认是 `role='user'`（普通用户）
- ✅ 只能看到和管理自己创建的交易员
- ✅ 需要 OTP 验证（保持现有行为）
- ✅ 如果需要成为真正的管理员，需要手动设置 `role='admin'`

### 3.7 随机生成工具函数

```go
// generateRandomEmail 生成随机邮箱
func generateRandomEmail() string {
    randomStr := uuid.New().String()[:8]
    return fmt.Sprintf("trader_%s@nofx.local", randomStr)
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return string(b)
}
```

### 3.4 分类管理接口（新增）

#### GET `/api/categories`
**权限：** Admin, User

**功能：** 获取分类列表
- **Admin**: 返回所有分类（特殊角色，一般不使用）
- **User**: 返回自己创建的分类

**响应：**
```json
{
  "categories": [
    {
      "id": 1,
      "name": "Group A",
      "owner_user_id": "user123",
      "owner_email": "user@example.com",
      "trader_count": 5,
      "running_count": 3,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### POST `/api/categories`
**权限：** Admin, User

**功能：** 创建分类

**请求体：**
```json
{
  "name": "Group A",
  "description": "分类描述（可选）"
}
```

**响应：**
```json
{
  "id": 1,
  "name": "Group A",
  "owner_user_id": "user123",
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### PUT `/api/categories/:id`
**权限：** Admin, User（Admin可以修改所有分类，User只能修改自己的分类）

**功能：** 更新分类信息

**请求体：**
```json
{
  "name": "Group A Updated",
  "description": "更新后的分类描述（可选）"
}
```

**响应：**
```json
{
  "id": 1,
  "name": "Group A Updated",
  "owner_user_id": "user123",
  "description": "更新后的分类描述",
  "updated_at": "2024-01-02T00:00:00Z"
}
```

**实现逻辑：**
```go
func (s *Server) handleUpdateCategory(c *gin.Context) {
    userID := c.GetString("user_id")
    categoryID := c.Param("id")
    
    // 获取用户角色
    user, _ := s.database.GetUserByID(userID)
    
    // 获取分类信息
    category, err := s.database.GetCategoryByID(categoryID)
    if err != nil || category == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
        return
    }
    
    // 权限检查：如果不是admin，验证分类是否属于当前用户
    if user.Role != "admin" && category.OwnerUserID != userID {
        c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己的分类"})
        return
    }
    
    var req struct {
        Name        string `json:"name"`
        Description string `json:"description"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 如果修改了名称，检查新名称是否已存在（同一用户下）
    if req.Name != "" && req.Name != category.Name {
        existing, _ := s.database.GetCategoryByNameAndOwner(req.Name, userID)
        if existing != nil && existing.ID != categoryID {
            c.JSON(http.StatusConflict, gin.H{"error": "分类名称已存在"})
            return
        }
    }
    
    // 更新分类
    err = s.database.UpdateCategory(categoryID, req.Name, req.Description)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "更新分类失败"})
        return
    }
    
    // 返回更新后的分类信息
    updatedCategory, _ := s.database.GetCategoryByID(categoryID)
    c.JSON(http.StatusOK, updatedCategory)
}
```

#### DELETE `/api/categories/:id`
**权限：** Admin, User（Admin可以删除所有分类，User只能删除自己的分类）

**功能：** 删除分类

**级联处理：**
- **交易员和分类独立设计**：删除分类时，不会删除交易员
- 删除分类时，将该分类下的所有交易员的 `category` 字段设置为空字符串
- 交易员仍然存在，只是不再属于任何分类
- 小组组长如果观测该分类，删除分类后该关联关系也会被删除（通过FOREIGN KEY CASCADE）

**实现逻辑：**
```go
func (s *Server) handleDeleteCategory(c *gin.Context) {
    userID := c.GetString("user_id")
    categoryID := c.Param("id")
    
    // 获取用户角色
    user, _ := s.database.GetUserByID(userID)
    
    // 获取分类信息
    category, err := s.database.GetCategoryByID(categoryID)
    if err != nil || category == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
        return
    }
    
    // 权限检查：如果不是admin，验证分类是否属于当前用户
    if user.Role != "admin" && category.OwnerUserID != userID {
        c.JSON(http.StatusForbidden, gin.H{"error": "只能删除自己的分类"})
        return
    }
    
    categoryName := category.Name
    
    // 1. 将该分类下的所有交易员的category设为空字符串
    err = s.database.UpdateTradersCategoryToEmpty(categoryName)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "更新交易员分类失败"})
        return
    }
    
    // 2. 删除分类（会自动删除group_leader_categories中的关联，通过FOREIGN KEY CASCADE）
    err = s.database.DeleteCategory(categoryID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "删除分类失败"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "分类删除成功",
        "category_name": categoryName,
    })
}
```

**注意：**
- ✅ 交易员和分类是独立的关系，删除分类不会删除交易员
- ✅ 删除分类后，交易员的 `category` 字段会被设置为空字符串
- ✅ 小组组长如果只观测该分类，删除分类后该关联关系会被删除
- ✅ 如果小组组长还观测其他分类，其他关联关系不受影响

#### POST `/api/traders/:id/category`
**权限：** Admin, User（Admin可以设置所有交易员的分类，User只能设置自己分类下的交易员）

**功能：** 设置交易员分类

**请求体：**
```json
{
  "category": "Group A"  // 分类名称
}
```

**注意：**
- Admin可以设置所有交易员的分类（特殊角色，一般不使用）
- 普通用户只能将交易员设置到自己创建的分类
- 需要验证分类的owner_user_id与当前用户ID匹配（Admin跳过此检查）

### 3.5 新增：交易员账号管理接口

#### POST `/api/traders/:id/create-account`
**权限：** Admin, User（Admin可以创建所有交易员的账号，User只能为自己的交易员创建账号）

**功能：** 为交易员创建账号

**请求体：**
```json
{
  "generate_random_email": true,    // true=账号随机生成，false=手动输入账号
  "generate_random_password": true, // true=密码随机生成，false=手动输入密码
  "email": "trader1@example.com",   // 如果generate_random_email=false，必填
  "password": "password123"         // 如果generate_random_password=false，必填
}
```

**四种组合模式：**
1. **账号随机，密码自己输入**：`generate_random_email=true, generate_random_password=false, password必填`
2. **密码随机，账号自己输入**：`generate_random_email=false, generate_random_password=true, email必填`
3. **全随机**：`generate_random_email=true, generate_random_password=true`
4. **全自己输入**：`generate_random_email=false, generate_random_password=false, email和password必填`

**响应：**
```json
{
  "user_id": "trader_account_123",
  "email": "trader1@example.com",  // 最终使用的账号（随机生成或手动输入）
  "password": "generated_password",  // 最终使用的密码（随机生成或手动输入，仅此一次返回）
  "trader_id": "trader123"
}
```

#### DELETE `/api/traders/:id/account`
**权限：** Admin, User（Admin可以删除所有交易员的账号，User只能删除自己交易员的账号）

**功能：** 删除交易员的账号

#### GET `/api/traders/:id/account`
**权限：** Admin, User（Admin可以查看所有交易员的账号信息，User只能查看自己交易员的账号信息）

**功能：** 获取交易员的账号信息

**响应：**
```json
{
  "account": {
    "user_id": "trader_account_123",
    "email": "trader1@example.com",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### 3.6 新增：小组组长管理接口

#### POST `/api/group-leaders/create`
**权限：** User（只能为自己的分类创建小组组长账号）

**功能：** 创建小组组长账号

**请求体：**
```json
{
  "generate_random": true,   // true=随机生成，false=手动输入
  "email": "leader1@example.com",  // 如果generate_random=false，必填
  "password": "password123",        // 如果generate_random=false，必填
  "categories": ["Group A", "Group B"]  // 必填：可以观测的分类列表（必须是自己的分类）
}
```

**响应：**
```json
{
  "user_id": "group_leader_123",
  "email": "leader1@example.com",
  "password": "generated_password",  // 如果自动生成
  "categories": ["Group A", "Group B"]
}
```

#### GET `/api/group-leaders`
**权限：** Admin, User（Admin可以查看所有小组组长列表，User只能查看自己创建的小组组长列表）

**功能：** 获取小组组长列表

**响应：**
```json
{
  "leaders": [
    {
      "user_id": "group_leader_123",
      "email": "leader1@example.com",
      "categories": ["Group A", "Group B"],
      "trader_count": 8,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### PUT `/api/group-leaders/:id/categories`
**权限：** Admin, User（Admin可以更新所有小组组长的分类，User只能更新自己创建的小组组长的分类）

**功能：** 更新小组组长的分类

**请求体：**
```json
{
  "categories": ["Group A", "Group C"]  // Admin可以是所有分类，User必须是自己的分类
}
```

#### DELETE `/api/group-leaders/:id`
**权限：** Admin, User（Admin可以删除所有小组组长账号，User只能删除自己创建的小组组长账号）

**功能：** 删除小组组长账号

---

## 🎨 前端界面设计（最小化修改）

### 4.1 核心修改点

#### 4.1.1 交易员列表页面 (AITradersPage.tsx)

**修改位置：** `web/src/components/AITradersPage.tsx`

**修改方式：根据用户角色隐藏/显示操作按钮**

```typescript
// 1. 从用户信息获取角色
const { user } = useAuth()
const userRole = user?.role || 'user' // 默认user（向后兼容）

// 2. 判断是否可以编辑
const canEdit = userRole === 'user'  // 普通用户可以编辑自己的交易员
const canCreate = userRole === 'user'  // 普通用户可以创建交易员
const canDelete = userRole === 'user'  // 普通用户可以删除自己的交易员
const canManageCategories = userRole === 'user'  // 普通用户可以管理分类
const canCreateAccount = userRole === 'user'  // 普通用户可以创建账号
const canCreateGroupLeader = userRole === 'user'  // 普通用户可以创建小组组长

// 3. 在JSX中条件渲染
{canCreate && (
  <button onClick={() => setShowCreateModal(true)}>
    创建交易员
  </button>
)}

{traders.map(trader => (
  <div key={trader.trader_id}>
    {/* 交易员信息 */}
    <div>{trader.trader_name}</div>
    
    {/* 操作按钮 - 根据权限显示/隐藏 */}
    {canEdit && (
      <button onClick={() => handleEditTrader(trader.trader_id)}>
        编辑
      </button>
    )}
    {canDelete && (
      <button onClick={() => handleDeleteTrader(trader.trader_id)}>
        删除
      </button>
    )}
    {canManageCategories && (
      <button onClick={() => handleSetCategory(trader.trader_id)}>
        设置分类
      </button>
    )}
    {canManageAccounts && (
      <button onClick={() => handleCreateAccount(trader.trader_id)}>
        创建账号
      </button>
    )}
  </div>
))}
```

**关键点：**
- ✅ **交易员列表自动过滤**：API返回的数据已经根据角色过滤，前端无需修改
- ✅ **只修改显示逻辑**：通过条件渲染隐藏/显示按钮
- ✅ **复用现有组件**：不需要创建新组件

#### 4.1.2 交易员详情页面 (TraderDetailsPage in App.tsx)

**修改位置：** `web/src/App.tsx` - `TraderDetailsPage` 组件

**交易员下拉列表：**
```typescript
// 现有代码（无需修改）
<select
  value={selectedTraderId}
  onChange={(e) => onTraderSelect(e.target.value)}
>
  {traders.map((trader) => (
    <option key={trader.trader_id} value={trader.trader_id}>
      {trader.trader_name}
    </option>
  ))}
</select>
```

**说明：**
- ✅ **下拉列表自动适配**：`traders` 数组已经由API根据角色过滤，下拉列表自动只显示有权限的交易员
- ✅ **无需修改**：现有代码已经满足需求

**操作按钮隐藏：**
```typescript
// 在交易员详情页中，根据角色隐藏编辑功能
const canEdit = user?.role === 'user'  // 普通用户可以编辑自己的交易员

{canEdit && (
  <button onClick={handleEdit}>编辑配置</button>
)}
```

#### 4.1.3 配置管理页面

**修改位置：** `web/src/components/AITradersPage.tsx`

**AI模型配置、交易所配置、信号源配置：**
```typescript
const canManageConfig = user?.role === 'user'  // 普通用户可以配置自己的交易员

{canManageConfig && (
  <>
    <button onClick={() => setShowModelModal(true)}>
      AI模型配置
    </button>
    <button onClick={() => setShowExchangeModal(true)}>
      交易所配置
    </button>
    <button onClick={() => setShowSignalSourceModal(true)}>
      信号源配置
    </button>
  </>
)}
```

**关键点：**
- ✅ **完全隐藏配置入口**：非普通用户（group_leader和trader_account）看不到配置按钮
- ✅ **最小化修改**：只需要添加条件判断

#### 4.1.2 分类管理页面 `/admin/categories` (新增)

**功能：**
- 显示所有分类列表
- 创建/删除分类
- 查看每个分类的交易员数量
- 批量设置交易员分类

**布局：**
```
┌─────────────────────────────────────────┐
│  分类管理                                │
├─────────────────────────────────────────┤
│  [创建分类]                              │
├─────────────────────────────────────────┤
│  ┌───────────────────────────────────┐ │
│  │ Group A                            │ │
│  │ 交易员: 5 | 运行中: 3              │ │
│  │ [查看交易员] [编辑] [删除]          │ │
│  └───────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

#### 4.1.3 账号管理页面 `/admin/accounts` (新增)

**功能：**
- 显示所有交易员账号列表
- 显示所有小组组长账号列表
- 创建/删除账号
- 重置密码

**布局：**
```
┌─────────────────────────────────────────┐
│  账号管理                                │
├─────────────────────────────────────────┤
│  [创建交易员账号] [创建小组组长账号]       │
├─────────────────────────────────────────┤
│  交易员账号:                              │
│  ┌───────────────────────────────────┐ │
│  │ trader1@example.com                │ │
│  │ 关联交易员: Trader 1               │ │
│  │ [重置密码] [删除]                  │ │
│  └───────────────────────────────────┘ │
│  小组组长账号:                            │
│  ┌───────────────────────────────────┐ │
│  │ leader1@example.com                │ │
│  │ 管理分类: Group A, Group B         │ │
│  │ [编辑分类] [重置密码] [删除]        │ │
│  └───────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### 4.2 小组组长界面（只读）

#### 4.2.1 交易员列表页面 `/traders` (AITradersPage - 只读模式)

**功能：**
- 显示管理的分类内的所有交易员
- 按分类分组显示
- 查看交易员详情（只读）
- **隐藏所有编辑/删除/创建按钮**

**布局：**
```
┌─────────────────────────────────────────┐
│  我的交易员（只读）                       │
├─────────────────────────────────────────┤
│  [筛选分类: Group A ▼] [搜索]            │
├─────────────────────────────────────────┤
│  ▼ Group A (5个交易员)                   │
│    ├─ Trader 1 [运行中] [+12.34%]       │
│    │  [查看详情]                        │
│    ├─ Trader 2 [已停止] [-5.67%]       │
│    │  [查看详情]                        │
│    └─ ...                                │
│  ▼ Group B (3个交易员)                   │
│    └─ ...                                │
└─────────────────────────────────────────┘
```

#### 4.2.2 交易员详情页面 `/traders/:id` (TraderDetailsPage - 只读模式)

**功能：**
- 显示交易员详细信息（只读）
- 账户余额、持仓、决策记录等
- **隐藏所有编辑按钮和配置选项**

**布局：**
```
┌─────────────────────────────────────────┐
│  交易员详情 (只读)                        │
├─────────────────────────────────────────┤
│  交易员名称: Trader 1                    │
│  分类: Group A                          │
├─────────────────────────────────────────┤
│  [账户概览卡片]                           │
│  [持仓列表]                              │
│  [决策记录]                              │
│  [统计数据]                              │
│  （所有编辑功能已隐藏）                    │
└─────────────────────────────────────────┘
```

### 4.3 交易员账号界面（只读）

#### 4.3.1 交易员详情页面 `/traders/:id` (TraderDetailsPage - 只读模式)

**功能：**
- 显示自己的交易员详细信息（只读）
- 账户余额、持仓、决策记录等
- **隐藏所有编辑按钮和配置选项**

**布局：**
```
┌─────────────────────────────────────────┐
│  我的交易员 (只读)                         │
├─────────────────────────────────────────┤
│  交易员名称: Trader 1                    │
├─────────────────────────────────────────┤
│  [账户概览卡片]                           │
│  [持仓列表]                              │
│  [决策记录]                              │
│  [统计数据]                              │
│  （所有编辑功能已隐藏）                    │
└─────────────────────────────────────────┘
```

---

## 🔄 实现步骤（最小化修改版）

### Phase 1: 数据库扩展（1天）

1. **用户表扩展（保留现有数据）**
   ```sql
   -- 添加角色字段，默认值为'user'（保留现有数据）
   ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'user';
   -- 注意：现有用户的role会自动设置为'user'，不影响现有数据
   
   -- 添加关联字段（保留现有数据）
   ALTER TABLE users ADD COLUMN trader_id TEXT DEFAULT NULL;
   ALTER TABLE users ADD COLUMN category TEXT DEFAULT NULL;
   -- 注意：现有用户的这些字段默认为NULL，不影响现有数据
   ```

2. **交易员表扩展（保留现有数据）**
   ```sql
   -- 添加分类字段（保留现有数据）
   ALTER TABLE traders ADD COLUMN category TEXT DEFAULT '';
   -- 注意：现有交易员的category默认为空字符串，不影响现有数据
   
   -- 添加交易员账号关联字段（保留现有数据）
   ALTER TABLE traders ADD COLUMN trader_account_id TEXT DEFAULT NULL;
   -- 注意：现有交易员的trader_account_id默认为NULL，不影响现有数据
   
   -- 添加所有者用户ID字段（保留现有数据，需要数据迁移）
   ALTER TABLE traders ADD COLUMN owner_user_id TEXT DEFAULT NULL;
   -- 注意：现有交易员的owner_user_id默认为NULL
   -- 需要根据现有数据关联设置：UPDATE traders SET owner_user_id = (关联的user_id)
   ```

3. **创建分类表（新表，不影响现有数据）**
   ```sql
   CREATE TABLE IF NOT EXISTS categories (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       name TEXT NOT NULL,
       owner_user_id TEXT NOT NULL,
       description TEXT DEFAULT '',
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       UNIQUE(owner_user_id, name)  -- 同一用户不能创建同名分类
   );
   CREATE INDEX idx_owner_user ON categories(owner_user_id);
   ```

4. **创建小组组长分类关联表（新表，不影响现有数据）**
   ```sql
   CREATE TABLE IF NOT EXISTS group_leader_categories (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       group_leader_id TEXT NOT NULL,
       category TEXT NOT NULL,
       owner_user_id TEXT NOT NULL,  -- 分类所有者用户ID
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       FOREIGN KEY (group_leader_id) REFERENCES users(id) ON DELETE CASCADE,
       UNIQUE(group_leader_id, category)
   );
   CREATE INDEX idx_group_leader ON group_leader_categories(group_leader_id);
   CREATE INDEX idx_category ON group_leader_categories(category);
   CREATE INDEX idx_owner_user ON group_leader_categories(owner_user_id);
   ```

5. **数据迁移（保留现有数据）**
   - **用户表迁移**：
     ```sql
     -- 将现有用户的role设置为'user'（如果为NULL）
     UPDATE users SET role = 'user' WHERE role IS NULL OR role = '';
     ```
   - **交易员表迁移**：
     ```sql
     -- 根据现有数据关联设置owner_user_id
     -- 如果现有traders表有user_id字段，直接使用它设置owner_user_id
     UPDATE traders SET owner_user_id = user_id WHERE owner_user_id IS NULL AND user_id IS NOT NULL;
     -- 注意：如果traders表没有user_id字段，需要通过代码逻辑处理
     ```
   - **代码迁移逻辑**（SQLite不支持复杂SQL，需要在代码中处理）：
     ```go
     // 在数据库初始化或升级时执行
     func (d *Database) MigrateTradersOwnerUserID() error {
         // 获取所有owner_user_id为NULL的交易员
         rows, err := d.db.Query("SELECT id, user_id FROM traders WHERE owner_user_id IS NULL")
         if err != nil {
             return err
         }
         defer rows.Close()
         
         for rows.Next() {
             var traderID, userID string
             if err := rows.Scan(&traderID, &userID); err != nil {
                 continue
             }
             
             // 如果user_id存在，设置为owner_user_id
             if userID != "" {
                 _, err := d.db.Exec("UPDATE traders SET owner_user_id = ? WHERE id = ?", userID, traderID)
                 if err != nil {
                     log.Printf("更新交易员 %s 的owner_user_id失败: %v", traderID, err)
                 }
             }
         }
         return nil
     }
     ```
   - **向后兼容**：
     - 现有用户默认是 `role='user'`，可以创建分类和账号
     - 现有交易员数据完全保留，category和trader_account_id默认为空/NULL
     - owner_user_id如果无法确定，保持为NULL，但新创建的交易员会自动设置
     - 不影响Admin Mode功能，两者独立工作
     - 新创建的交易员会自动设置owner_user_id和category（如果提供）

### Phase 2: 后端API修改（1-2天）

#### 2.1 修改 `handleRegister` (设置角色)
**文件：** `api/server.go:1572`

**修改内容：**
- 注册时设置 `role='user'`（普通用户）
- 保持其他逻辑不变

#### 2.2 修改 `handleTraderList` (核心修改)
**文件：** `api/server.go:1096`

**修改内容：**
- 获取用户角色
- 根据角色查询不同的交易员列表
- `admin` → 所有交易员（跨用户，特殊角色，一般不使用）
- `user` → 自己分类下的交易员（通过分类表关联）
- `group_leader` → 观测的分类内的交易员（创建者指定的分类）
- `trader_account` → 自己的交易员（通过trader_id关联）
- 保持响应格式不变

**代码示例：**
```go
func (s *Server) handleTraderList(c *gin.Context) {
    userID := c.GetString("user_id")
    
    // 获取用户角色
    user, err := s.database.GetUserByID(userID)
    if err != nil {
        // 向后兼容：如果获取失败，使用默认行为
        traders, _ := s.database.GetTraders(userID)
        // ... 返回结果
        return
    }
    
    role := user.Role
    if role == "" {
        role = "user" // 默认是普通用户
    }
    
    var traders []*TraderRecord
    switch role {
    case "admin":
        // 真正的管理员：返回所有交易员（跨用户，特殊角色，一般不使用）
        traders, _ = s.database.GetAllTraders()
    case "user":
        // 普通用户：返回自己分类下的所有交易员，或owner_user_id为自己的交易员
        userCategories := s.database.GetUserCategories(userID)
        if len(userCategories) == 0 {
            // 向后兼容：如果没有分类，返回owner_user_id为该用户的交易员
            traders, _ = s.database.GetTradersByOwnerUserID(userID)
        } else {
            // 返回分类下的交易员，以及owner_user_id为该用户但category为空的交易员
            categoryTraders, _ := s.database.GetTradersByCategories(userCategories)
            ownerTraders, _ := s.database.GetTradersByOwnerUserID(userID)
            // 合并并去重
            traderMap := make(map[string]*TraderRecord)
            for _, t := range categoryTraders {
                traderMap[t.ID] = t
            }
            for _, t := range ownerTraders {
                // 只添加category为空或属于用户分类的交易员
                if t.Category == "" || contains(userCategories, t.Category) {
                    traderMap[t.ID] = t
                }
            }
            traders = make([]*TraderRecord, 0, len(traderMap))
            for _, t := range traderMap {
                traders = append(traders, t)
            }
        }
    case "group_leader":
        // 小组组长：返回观测的分类下的交易员
        categories := s.database.GetGroupLeaderCategories(userID)
        traders, _ = s.database.GetTradersByCategories(categories)
    case "trader_account":
        // 交易员账号：返回自己的交易员
        // 注意：GetTradersByID返回数组，但trader_account应该只有一个交易员
        traderList, _ := s.database.GetTradersByID(user.TraderID)
        if len(traderList) > 0 {
            traders = traderList  // 返回数组，即使只有一个元素
        } else {
            traders = []*TraderRecord{}  // 返回空数组
        }
    default:
        // 向后兼容：默认只返回自己的交易员（通过owner_user_id或分类）
        // 如果没有role字段，默认行为是user
        userCategories := s.database.GetUserCategories(userID)
        if len(userCategories) == 0 {
            // 如果没有分类，返回owner_user_id为该用户的交易员
            traders, _ = s.database.GetTradersByOwnerUserID(userID)
        } else {
            // 返回分类下的交易员，以及owner_user_id为该用户但category为空的交易员
            categoryTraders, _ := s.database.GetTradersByCategories(userCategories)
            ownerTraders, _ := s.database.GetTradersByOwnerUserID(userID)
            traderMap := make(map[string]*TraderRecord)
            for _, t := range categoryTraders {
                traderMap[t.ID] = t
            }
            for _, t := range ownerTraders {
                if t.Category == "" || contains(userCategories, t.Category) {
                    traderMap[t.ID] = t
                }
            }
            traders = make([]*TraderRecord, 0, len(traderMap))
            for _, t := range traderMap {
                traders = append(traders, t)
            }
        }
    }
    
    // 后续代码保持不变
    // ...
}
```

#### 2.3 修改 `handleAccount` (添加角色信息)
**文件：** `api/server.go` - `handleAccount` 函数

**修改内容：**
- 在响应中添加 `role`、`trader_id`、`categories` 字段

#### 2.4 添加数据库查询方法
**文件：** `config/database.go`

**新增方法：**
- `GetUserByID(userID string) (*User, error)` - 获取用户信息（包含角色）
- `GetAllTraders() ([]*TraderRecord, error)` - 获取所有交易员（Admin用，特殊角色，一般不使用）
- `GetUserCategories(userID string) ([]string, error)` - 获取用户创建的所有分类名称
- `GetGroupLeaderCategories(userID string) ([]string, error)` - 获取小组组长可以观测的分类
- `GetTradersByCategories(categories []string) ([]*TraderRecord, error)` - 根据分类获取交易员
- `GetTradersByID(traderID string) ([]*TraderRecord, error)` - 根据ID获取交易员（返回数组，即使只有一个）
- `GetTraderByID(traderID string) (*TraderRecord, error)` - 根据ID获取单个交易员（包含owner_user_id和category）
- `GetTradersByOwnerUserID(userID string) ([]*TraderRecord, error)` - 根据owner_user_id获取交易员列表
- `CreateCategory(userID, name, description string) (*Category, error)` - 创建分类
- `GetCategoryByID(categoryID int) (*Category, error)` - 根据ID获取分类
- `GetCategoryByName(categoryName string) (*Category, error)` - 根据名称获取分类
- `GetCategoryByNameAndOwner(categoryName, ownerUserID string) (*Category, error)` - 根据名称和所有者获取分类
- `GetCategoriesByOwner(userID string) ([]*Category, error)` - 获取用户创建的分类列表
- `UpdateCategory(categoryID int, name, description string) error` - 更新分类信息
- `DeleteCategory(categoryID int) error` - 删除分类
- `UpdateTraderCategory(traderID, category string) error` - 更新交易员分类
- `UpdateTradersCategoryToEmpty(categoryName string) error` - 将指定分类下的所有交易员的category设为空字符串
- `InsertGroupLeaderCategory(groupLeaderID, category, ownerUserID string) error` - 插入小组组长分类关联（第三个参数是owner_user_id）
- `MigrateTradersOwnerUserID() error` - 数据迁移：设置现有交易员的owner_user_id

### Phase 3: 登录逻辑修改（1天）

#### 3.1 修改后端登录接口
**文件：** `api/server.go:1732` - `handleLogin` 函数

**修改内容：**
- 根据用户角色判断是否需要OTP验证
- 管理员和普通用户（role='admin'或'user'）需要OTP验证
- 创建的账号（role='group_leader'或'trader_account'）不需要OTP，直接登录

#### 3.2 修改前端登录页面
**文件：** `web/src/components/LoginPage.tsx`

**修改内容：**
- 登录页面保持不变（隐性判断）
- 根据API返回的 `requires_otp` 字段决定是否显示OTP输入框
- 如果 `requires_otp=false` 或没有该字段，直接登录成功

**关键修改：**
```typescript
const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await login(email, password)

    if (result.success) {
        // 如果返回了token，说明是普通用户，直接登录成功
        if (result.token) {
            // AuthContext会自动处理
            return
        }
        
        // 如果需要OTP验证（管理员）
        if (result.requiresOTP && result.userID) {
            setUserID(result.userID)
            setStep('otp')
        }
    } else {
        setError(result.message || t('loginFailed', language))
    }

    setLoading(false)
}
```

#### 3.3 修改 AuthContext
**文件：** `web/src/contexts/AuthContext.tsx`

**修改内容：**
- `login` 函数处理两种情况：
  1. 返回token（普通用户）：直接登录
  2. 返回requires_otp（管理员）：进入OTP验证流程

### Phase 4: 前端UI修改（1-2天）

#### 4.1 修改 AITradersPage.tsx
**文件：** `web/src/components/AITradersPage.tsx`

**修改内容：**
1. 从用户信息获取角色
2. 根据角色条件渲染操作按钮
3. 添加创建账号功能（普通用户可以创建trader_account和group_leader）

**关键修改点：**
```typescript
// 1. 获取用户角色
const { user } = useAuth()
const userRole = user?.role || 'user'  // 默认是普通用户

// 2. 判断权限
const isUser = userRole === 'user'    // 普通用户
const canEdit = isUser     // 普通用户可以编辑自己的交易员
const canCreate = isUser   // 普通用户可以创建交易员（自己的分类）
const canDelete = isUser   // 普通用户可以删除自己的交易员
const canManageConfig = isUser  // 配置功能（普通用户只能配置自己的）
const canCreateAccount = isUser      // 普通用户可以创建交易员账号
const canCreateGroupLeader = isUser  // 普通用户可以创建小组组长账号
const canManageCategories = isUser   // 普通用户可以管理自己的分类

// 3. 条件渲染
{canCreate && <CreateButton />}
{canEdit && <EditButton />}
{canDelete && <DeleteButton />}
{canManageConfig && <ConfigButtons />}
{canCreateAccount && <CreateTraderAccountButton />}
{canCreateGroupLeader && <CreateGroupLeaderButton />}
{canManageCategories && <CategoryManagementButton />}
```

#### 4.2 添加创建账号模态框
**文件：** `web/src/components/AITradersPage.tsx`

**功能：**
- 选择角色（trader_account 或 group_leader）
- **账号生成方式**：复选框选择是否随机生成账号
  - ☑️ 随机生成账号：勾选后自动生成账号，隐藏账号输入框
  - ☐ 手动输入账号：取消勾选后显示账号输入框，必须填写
- **密码生成方式**：复选框选择是否随机生成密码
  - ☑️ 随机生成密码：勾选后自动生成密码，隐藏密码输入框
  - ☐ 手动输入密码：取消勾选后显示密码输入框，必须填写
- 如果group_leader，显示分类选择（多选）
- 创建成功后显示生成的账号和密码（仅此一次）

**四种组合模式UI示例：**
```typescript
// 创建账号模态框组件
const CreateAccountModal = ({ traderId, role, onClose }) => {
  const [generateRandomEmail, setGenerateRandomEmail] = useState(true)
  const [generateRandomPassword, setGenerateRandomPassword] = useState(true)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [categories, setCategories] = useState([])  // group_leader用
  
  const handleSubmit = async () => {
    // 验证必填字段
    if (!generateRandomEmail && !email) {
      alert('请输入账号')
      return
    }
    if (!generateRandomPassword && !password) {
      alert('请输入密码')
      return
    }
    
    const payload = {
      generate_random_email: generateRandomEmail,
      generate_random_password: generateRandomPassword,
      email: generateRandomEmail ? undefined : email,
      password: generateRandomPassword ? undefined : password,
      ...(role === 'group_leader' && { categories })
    }
    
    // 调用API创建账号
    const result = await createAccount(traderId, role, payload)
    
    // 显示生成的账号和密码（仅此一次）
    if (result.success) {
      alert(`账号创建成功！\n账号: ${result.email}\n密码: ${result.password}`)
      onClose()
    }
  }
  
  return (
    <Modal>
      <h3>创建{role === 'trader_account' ? '交易员账号' : '小组组长账号'}</h3>
      
      {/* 账号生成方式 */}
      <div>
        <label>
          <input
            type="checkbox"
            checked={generateRandomEmail}
            onChange={(e) => setGenerateRandomEmail(e.target.checked)}
          />
          随机生成账号
        </label>
        {!generateRandomEmail && (
          <input
            type="email"
            placeholder="请输入账号（邮箱）"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        )}
      </div>
      
      {/* 密码生成方式 */}
      <div>
        <label>
          <input
            type="checkbox"
            checked={generateRandomPassword}
            onChange={(e) => setGenerateRandomPassword(e.target.checked)}
          />
          随机生成密码
        </label>
        {!generateRandomPassword && (
          <input
            type="password"
            placeholder="请输入密码"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        )}
      </div>
      
      {/* 如果是group_leader，显示分类选择 */}
      {role === 'group_leader' && (
        <div>
          <label>选择分类（可多选）：</label>
          <select multiple value={categories} onChange={(e) => setCategories([...e.target.selectedOptions].map(o => o.value))}>
            {/* 分类选项 */}
          </select>
        </div>
      )}
      
      <button onClick={handleSubmit}>创建</button>
      <button onClick={onClose}>取消</button>
    </Modal>
  )
}
```

#### 4.3 修改 App.tsx - TraderDetailsPage
**文件：** `web/src/App.tsx`

**修改内容：**
- 根据角色隐藏编辑功能（如果需要）
- **交易员下拉列表无需修改**（自动适配）

### Phase 5: 新增功能API（1-2天）

#### 5.1 创建交易员账号API
**文件：** `api/server.go` - 新增 `handleCreateTraderAccount` 和 `handleCreateGroupLeader` 函数

**功能：**
- 为交易员创建账号（trader_account）
- 为分类创建小组组长账号（group_leader）
- 支持随机生成或手动输入
- 返回账号信息（包括密码，仅此一次）
- 必须验证交易员/分类的owner_user_id与当前用户ID匹配

#### 5.2 数据库方法
**文件：** `config/database.go`

**新增方法：**
- `UpdateTraderAccountID(traderID, accountID string) error` - 更新交易员的账号ID
- `InsertGroupLeaderCategory(groupLeaderID, category, ownerUserID string) error` - 插入小组组长分类关联（第三个参数是owner_user_id，必须设置为创建者的用户ID）
- `GetGroupLeaderCategories(userID string) ([]string, error)` - 获取小组组长可以观测的分类
- `GetTraderByID(traderID string) (*TraderRecord, error)` - 获取交易员信息（包含owner_user_id和category）
- `GetTradersByOwnerUserID(userID string) ([]*TraderRecord, error)` - 根据owner_user_id获取交易员列表（用于向后兼容）
- `UpdateTradersCategoryToEmpty(categoryName string) error` - 将指定分类下的所有交易员的category设为空字符串（删除分类时使用）
- `MigrateTradersOwnerUserID() error` - 数据迁移：设置现有交易员的owner_user_id（SQLite需要代码处理）

### Phase 6: 测试（1天）

1. **权限测试**
   - 测试各角色的交易员列表过滤
   - 测试UI按钮显示/隐藏
   - 测试交易员下拉列表

2. **向后兼容测试**
   - 确保现有用户（无role字段）正常工作
   - 确保现有功能不受影响

---

## 📝 代码结构（最小化修改版）

### 后端文件结构

```
nofx/
├── api/
│   └── server.go                    # 修改：handleTraderList, handleAccount
├── auth/
│   └── auth.go                      # 无需修改（或添加角色常量）
└── config/
    └── database.go                  # 修改：添加角色查询方法
```

**修改文件：**
- ✅ `api/server.go` - 4个函数修改（handleTraderList, handleLogin, handleRegister, handleCreateTraderAccount）
- ✅ `config/database.go` - 添加几个查询方法

### 前端文件结构

```
web/src/
├── components/
│   └── AITradersPage.tsx            # 修改：添加条件渲染
├── App.tsx                          # 修改：TraderDetailsPage 添加条件渲染（可选）
└── contexts/
    └── AuthContext.tsx              # 修改：确保包含role字段（可选）
```

**修改文件：**
- ✅ `web/src/components/AITradersPage.tsx` - 添加条件渲染 + 创建账号功能
- ✅ `web/src/components/LoginPage.tsx` - 修改登录逻辑（隐性判断）
- ✅ `web/src/contexts/AuthContext.tsx` - 修改login函数处理两种登录方式
- ✅ `web/src/App.tsx` - 可选修改（如果需要隐藏详情页编辑功能）

**无需新增文件：**
- ❌ 不需要新的页面组件
- ❌ 不需要新的API处理文件
- ❌ 不需要新的权限中间件

---

## 🔒 安全考虑

1. **权限验证**
   - 所有API必须在后端验证权限
   - 前端权限控制仅用于UI展示，不能作为安全依据

2. **数据隔离**
   - 交易员账号只能访问自己的交易员数据
   - 小组组长只能访问管理的分类内的交易员数据
   - 使用数据库查询级别的权限过滤

3. **账号管理**
   - 普通用户可以为自己创建账号（trader_account和group_leader）
   - 必须验证交易员/分类的owner_user_id与当前用户ID匹配
   - 账号密码自动生成时，需要安全随机生成
   - 账号创建后需要记录审计日志

4. **只读模式**
   - 小组组长和交易员账号的所有配置功能必须在后端和前端都隐藏
   - 即使绕过前端，后端API也应该拒绝编辑请求

---

## 📊 数据流示例

### 普通用户创建交易员账号

```
1. 前端请求: POST /api/traders/trader123/create-account
   Body: { "generate_random": true }
2. 后端中间件: 检查用户角色是否为 user
3. 权限检查: 验证交易员的owner_user_id是否与当前用户ID匹配
4. 创建用户: INSERT INTO users (id, email, role='trader_account', trader_id, category)
5. 更新交易员: UPDATE traders SET trader_account_id = new_user_id
6. 生成密码: 自动生成安全密码
7. 返回响应: { user_id, email, password, trader_id }
8. 前端展示: 显示账号信息和密码（仅此一次）
```

### 普通用户创建小组组长账号

```
1. 前端请求: POST /api/group-leaders/create
   Body: { "generate_random": true, "categories": ["Group A"] }
2. 后端中间件: 检查用户角色是否为 user
3. 权限检查: 验证分类是否属于当前用户（通过categories表的owner_user_id）
4. 创建用户: INSERT INTO users (id, email, role='group_leader')
5. 关联分类: INSERT INTO group_leader_categories (group_leader_id, category, owner_user_id)
6. 生成密码: 自动生成安全密码
7. 返回响应: { user_id, email, password, categories }
8. 前端展示: 显示账号信息和密码（仅此一次）
```

### 小组组长查看交易员列表

```
1. 前端请求: GET /api/traders
2. 后端中间件: 检查用户角色为 group_leader
3. 权限检查: 获取该小组组长管理的分类列表
4. 数据库查询: SELECT * FROM traders WHERE category IN (管理的分类列表)
5. 返回响应: { traders: [...], can_edit: false }
6. 前端展示: 只读模式，隐藏所有编辑按钮
```

### 交易员账号查看自己的交易员

```
1. 前端请求: GET /api/traders
2. 后端中间件: 检查用户角色为 trader_account
3. 权限检查: 获取关联的交易员ID (trader_id)
4. 数据库查询: SELECT * FROM traders WHERE id = trader_id
5. 返回响应: { traders: [单个交易员], can_edit: false }
6. 前端展示: 只读模式，隐藏所有编辑按钮
```

---

## 🎯 总结

### 核心功能
- ✅ 四级权限体系（Admin / User / Group Leader / Trader Account）
- ✅ **一般情况下使用三级权限体系（User / Group Leader / Trader Account）**
- ✅ Admin角色保留但不常用（特殊角色，可以管理所有交易员）
- ✅ 交易员分类管理（User可以创建和管理自己的分类）
- ✅ 交易员账号创建和管理（User可以为自己的交易员创建账号）
- ✅ 小组组长账号创建和管理（User可以为自己的分类创建小组组长账号）
- ✅ 数据隔离和权限控制（User只能管理自己的数据）
- ✅ 只读模式（Group Leader和Trader Account隐藏配置功能）

### 技术要点
- ✅ 数据库扩展（角色字段、分类字段、账号关联）
- ✅ 后端权限中间件
- ✅ 前端角色感知UI（显示/隐藏功能）
- ✅ API权限验证

### 预估时间（最小化修改版）
- **数据库扩展**: 1天
- **后端API修改**: 1-2天（核心修改：handleRegister, handleTraderList, handleLogin）
- **登录逻辑修改**: 1天（后端+前端）
- **创建账号功能**: 1-2天（API + 前端UI）
- **前端UI修改**: 1-2天（主要是条件渲染）
- **测试**: 1天
- **总计**: 6-9天

**对比原方案：**
- ✅ 减少50%的开发时间
- ✅ 最小化代码修改
- ✅ 复用现有组件和逻辑
- ✅ 降低引入bug的风险

---

## 🎯 最小化修改方案总结

### 核心思路

1. **后端过滤数据**
   - 修改 `handleTraderList` 函数，根据用户角色返回不同的交易员列表
   - `admin` → 所有交易员（跨用户，特殊角色，一般不使用）
   - `user` → 自己分类下的交易员（数据隔离）
   - `group_leader` → 观测的分类内的交易员（创建者指定的分类）
   - `trader_account` → 自己的交易员
   - 响应格式保持不变，前端无需修改

2. **注册时设置角色**
   - 修改 `handleRegister` 函数，注册时设置 `role='user'`
   - 确保注册的用户可以创建分类和管理自己的交易员

3. **登录逻辑隐性判断**
   - 修改 `handleLogin` 函数，根据用户角色决定是否需要OTP验证
   - `admin` 和 `user` 需要OTP（注册用户）
   - `group_leader` 和 `trader_account` 不需要OTP（创建的账号）
   - 登录页面看起来一样，后端自动判断

4. **前端隐藏操作**
   - 在 `AITradersPage.tsx` 中根据用户角色条件渲染操作按钮
   - 普通用户可以看到创建/编辑/删除按钮（但只能操作自己的交易员）
   - 普通用户可以看到账号管理和分类管理功能（但只能管理自己的）

5. **交易员下拉列表自动适配**
   - 下拉列表使用 `traders` 数组（已由API过滤）
   - **无需修改**：现有代码已经满足需求

6. **账号创建功能**
   - Admin可以为所有交易员和分类创建账号（特殊角色，一般不使用）
   - 普通用户可以为自己的交易员创建账号（trader_account）
   - 普通用户可以为自己的分类创建小组组长账号（group_leader）
   - 支持随机生成或手动输入用户名密码
   - 创建的账号不需要OTP设置

### 关键优势

- ✅ **最小化修改**：只修改2-3个文件
- ✅ **复用现有代码**：交易员切换功能完全复用
- ✅ **向后兼容**：现有用户默认是user，可以创建分类和账号
- ✅ **数据保留**：所有数据库迁移使用ALTER TABLE，完全保留现有数据
- ✅ **开发快速**：4-6天即可完成核心功能

### 修改清单

**后端（2个文件）：**
1. `api/server.go` - 修改 `handleTraderList` 和 `handleAccount`
2. `config/database.go` - 添加几个查询方法

**前端（1-2个文件）：**
1. `web/src/components/AITradersPage.tsx` - 添加条件渲染
2. `web/src/App.tsx` - 可选修改（如果需要）

**数据库（3个ALTER + 1个CREATE）：**
1. `users` 表添加 `role` 和 `trader_id` 字段
2. `traders` 表添加 `category` 和 `trader_account_id` 字段
3. 创建 `group_leader_categories` 表

---

## 📌 关键实现细节

### 1. 密码生成策略

```go
// 生成安全的随机密码
func generatePassword(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return string(b)
}
```

### 2. 前端权限检查

```typescript
// 根据角色显示/隐藏功能
const canEdit = user?.role === 'user';  // 普通用户可以编辑自己的交易员
const canCreateAccount = user?.role === 'user';  // 普通用户可以创建交易员账号
const canCreateGroupLeader = user?.role === 'user';  // 普通用户可以创建小组组长账号
const canManageCategories = user?.role === 'user';  // 普通用户可以管理自己的分类

// 在组件中使用
{canEdit && <EditButton />}
{canCreateAccount && <CreateTraderAccountButton />}
{canCreateGroupLeader && <CreateGroupLeaderButton />}
{canManageCategories && <CategoryManagementButton />}
```

### 3. 后端权限检查

```go
// 检查是否可以访问交易员
func canAccessTrader(userID string, role string, traderID string) bool {
    switch role {
    case "admin":
        // 管理员可以访问所有交易员（特殊角色，一般不使用）
        return true
    case "user":
        // 检查交易员的owner_user_id是否与当前用户ID匹配
        trader, _ := s.database.GetTraderByID(traderID)
        return trader != nil && trader.OwnerUserID == userID
    case "group_leader":
        // 检查交易员是否在管理的分类内（创建者指定的分类）
        return isTraderInManagedCategories(userID, traderID)
    case "trader_account":
        // 检查是否是自己的交易员
        return isOwnTrader(userID, traderID)
    default:
        return false
    }
}
```

---

## 📌 后续扩展

1. **密码重置功能**
   - 普通用户可以重置自己创建的交易员账号密码
   - 普通用户可以重置自己创建的小组组长密码

2. **批量操作**
   - 批量设置交易员分类（只能设置到自己的分类）
   - 批量创建交易员账号（只能为自己的交易员创建）

3. **通知系统**
   - 交易员账号接收自己的交易员状态变化通知
   - 小组组长接收管理的交易员状态变化通知

4. **数据导出**
   - 小组组长导出管理的交易员数据
   - 交易员账号导出自己的交易员数据

---

## 🛡️ 错误处理和边界情况

### 1. 数据库迁移错误处理

**场景：** 数据库迁移失败或部分失败

**处理策略：**
```go
// 在数据库初始化时执行迁移，并记录错误
func (d *Database) MigrateSchema() error {
    migrations := []struct {
        name string
        fn   func() error
    }{
        {"add_user_role", d.migrateAddUserRole},
        {"add_trader_category", d.migrateAddTraderCategory},
        {"create_categories_table", d.migrateCreateCategoriesTable},
        {"create_group_leader_categories_table", d.migrateCreateGroupLeaderCategoriesTable},
        {"migrate_traders_owner_user_id", d.migrateTradersOwnerUserID},
    }
    
    for _, migration := range migrations {
        if err := migration.fn(); err != nil {
            log.Printf("迁移失败: %s, 错误: %v", migration.name, err)
            // 记录失败但不中断，继续执行其他迁移
            // 返回错误，让调用者决定如何处理
            return fmt.Errorf("迁移 %s 失败: %w", migration.name, err)
        }
    }
    return nil
}
```

**关键点：**
- ✅ 每个迁移步骤独立执行，失败不影响其他步骤
- ✅ 记录详细的错误日志，便于排查
- ✅ 支持重复执行（使用 `IF NOT EXISTS` 或检查表结构）

### 2. 角色字段为空的情况

**场景：** 现有用户没有 `role` 字段或字段为空

**处理策略：**
```go
// 获取用户角色时，如果为空则默认为 'user'
func (d *Database) GetUserRole(userID string) (string, error) {
    user, err := d.GetUserByID(userID)
    if err != nil {
        return "", err
    }
    
    role := user.Role
    if role == "" {
        // 向后兼容：默认为普通用户
        role = "user"
        // 可选：更新数据库中的role字段
        // d.UpdateUserRole(userID, "user")
    }
    
    return role, nil
}
```

**关键点：**
- ✅ 默认值处理：空值默认为 `'user'`
- ✅ 向后兼容：不影响现有用户
- ✅ 可选自动修复：可以自动更新空值

### 3. 交易员 owner_user_id 为空的情况

**场景：** 现有交易员没有 `owner_user_id` 字段

**处理策略：**
```go
// 在查询交易员列表时，处理owner_user_id为空的情况
func (d *Database) GetTradersByOwnerUserID(userID string) ([]*TraderRecord, error) {
    // 查询owner_user_id为该用户的交易员
    query := `SELECT * FROM traders WHERE owner_user_id = ?`
    rows, err := d.db.Query(query, userID)
    // ... 处理结果
    
    // 向后兼容：如果owner_user_id为空，可以通过user_id关联
    // 但这不是推荐的做法，应该通过迁移脚本设置owner_user_id
}
```

**关键点：**
- ✅ 数据迁移：优先通过迁移脚本设置 `owner_user_id`
- ✅ 查询兼容：如果迁移不完整，可以通过其他字段关联
- ✅ 新创建的交易员必须设置 `owner_user_id`

### 4. 分类删除时的级联处理

**场景：** 删除分类时，关联的交易员和小组组长如何处理

**处理策略：**
```go
func (s *Server) handleDeleteCategory(c *gin.Context) {
    // ... 权限检查 ...
    
    categoryName := category.Name
    
    // 1. 检查是否有交易员使用该分类
    traders, _ := s.database.GetTradersByCategory(categoryName)
    if len(traders) > 0 {
        // 将交易员的category设为空字符串（不删除交易员）
        err = s.database.UpdateTradersCategoryToEmpty(categoryName)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "更新交易员分类失败"})
            return
        }
    }
    
    // 2. 删除分类（会自动删除group_leader_categories中的关联，通过FOREIGN KEY CASCADE）
    err = s.database.DeleteCategory(categoryID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "删除分类失败"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "分类删除成功",
        "affected_traders": len(traders),
    })
}
```

**关键点：**
- ✅ 交易员不删除：只清空 `category` 字段
- ✅ 小组组长关联自动删除：通过 FOREIGN KEY CASCADE
- ✅ 返回受影响的数据数量，便于前端提示

### 5. 账号创建时的邮箱冲突

**场景：** 创建账号时，邮箱已存在

**处理策略：**
```go
func (s *Server) handleCreateTraderAccount(c *gin.Context) {
    // ... 权限检查 ...
    
    var accountEmail string
    if req.GenerateRandomEmail {
        // 随机生成邮箱，检查是否已存在
        maxRetries := 10
        for i := 0; i < maxRetries; i++ {
            accountEmail = generateRandomEmail()
            existing, _ := s.database.GetUserByEmail(accountEmail)
            if existing == nil {
                break // 邮箱可用
            }
        }
        if accountEmail == "" {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "无法生成唯一邮箱，请重试"})
            return
        }
    } else {
        accountEmail = req.Email
        // 检查邮箱是否已存在
        existing, _ := s.database.GetUserByEmail(accountEmail)
        if existing != nil {
            c.JSON(http.StatusConflict, gin.H{"error": "邮箱已存在"})
            return
        }
    }
    
    // ... 创建账号 ...
}
```

**关键点：**
- ✅ 随机生成时：重试机制，确保生成唯一邮箱
- ✅ 手动输入时：检查冲突，返回明确错误
- ✅ 错误码：使用 409 Conflict 表示资源冲突

### 6. 交易员账号重复创建

**场景：** 交易员已经有账号，再次创建账号

**处理策略：**
```go
func (s *Server) handleCreateTraderAccount(c *gin.Context) {
    traderID := c.Param("id")
    
    // 检查交易员是否已有账号
    trader, _ := s.database.GetTraderByID(traderID)
    if trader == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
        return
    }
    
    if trader.TraderAccountID != "" {
        // 检查账号是否仍然存在
        account, _ := s.database.GetUserByID(trader.TraderAccountID)
        if account != nil {
            c.JSON(http.StatusConflict, gin.H{
                "error": "交易员已有账号",
                "account_id": trader.TraderAccountID,
                "account_email": account.Email,
            })
            return
        }
        // 如果账号不存在，清除关联，允许重新创建
        s.database.UpdateTraderAccountID(traderID, "")
    }
    
    // ... 创建账号 ...
}
```

**关键点：**
- ✅ 检查现有账号：避免重复创建
- ✅ 账号已删除的情况：清除关联，允许重新创建
- ✅ 返回现有账号信息：便于前端显示

---

## 🧪 测试用例

### 1. 权限测试用例

#### 测试用例 1.1：普通用户只能看到自己的交易员
```go
func TestUserCanOnlySeeOwnTraders(t *testing.T) {
    // 1. 创建两个用户
    user1 := createTestUser("user1@test.com", "user")
    user2 := createTestUser("user2@test.com", "user")
    
    // 2. 用户1创建交易员
    trader1 := createTestTrader(user1.ID, "Trader 1")
    
    // 3. 用户2创建交易员
    trader2 := createTestTrader(user2.ID, "Trader 2")
    
    // 4. 用户1登录，获取交易员列表
    traders1 := getTradersList(user1.ID)
    assert.Contains(t, traders1, trader1.ID)
    assert.NotContains(t, traders1, trader2.ID)
    
    // 5. 用户2登录，获取交易员列表
    traders2 := getTradersList(user2.ID)
    assert.Contains(t, traders2, trader2.ID)
    assert.NotContains(t, traders2, trader1.ID)
}
```

#### 测试用例 1.2：小组组长只能看到管理的分类的交易员
```go
func TestGroupLeaderCanOnlySeeManagedCategories(t *testing.T) {
    // 1. 创建用户和分类
    user := createTestUser("user@test.com", "user")
    category1 := createTestCategory(user.ID, "Category 1")
    category2 := createTestCategory(user.ID, "Category 2")
    
    // 2. 创建交易员
    trader1 := createTestTraderWithCategory(user.ID, category1.Name)
    trader2 := createTestTraderWithCategory(user.ID, category2.Name)
    
    // 3. 创建小组组长，只管理 category1
    leader := createTestGroupLeader([]string{category1.Name}, user.ID)
    
    // 4. 小组组长登录，获取交易员列表
    traders := getTradersList(leader.ID)
    assert.Contains(t, traders, trader1.ID)
    assert.NotContains(t, traders, trader2.ID)
}
```

#### 测试用例 1.3：交易员账号只能看到自己的交易员
```go
func TestTraderAccountCanOnlySeeOwnTrader(t *testing.T) {
    // 1. 创建用户和交易员
    user := createTestUser("user@test.com", "user")
    trader1 := createTestTrader(user.ID, "Trader 1")
    trader2 := createTestTrader(user.ID, "Trader 2")
    
    // 2. 为 trader1 创建账号
    account := createTestTraderAccount(trader1.ID)
    
    // 3. 交易员账号登录，获取交易员列表
    traders := getTradersList(account.ID)
    assert.Contains(t, traders, trader1.ID)
    assert.NotContains(t, traders, trader2.ID)
    assert.Len(t, traders, 1)
}
```

### 2. 登录测试用例

#### 测试用例 2.1：普通用户登录需要OTP验证
```go
func TestUserLoginRequiresOTP(t *testing.T) {
    // 1. 创建普通用户（已设置OTP）
    user := createTestUserWithOTP("user@test.com", "user")
    
    // 2. 尝试登录
    response := login(user.Email, "password")
    
    // 3. 验证返回需要OTP
    assert.True(t, response.RequiresOTP)
    assert.Empty(t, response.Token)
    assert.Equal(t, user.ID, response.UserID)
}
```

#### 测试用例 2.2：创建的账号登录不需要OTP
```go
func TestCreatedAccountLoginNoOTP(t *testing.T) {
    // 1. 创建交易员账号（不需要OTP）
    account := createTestTraderAccount("trader123")
    
    // 2. 尝试登录
    response := login(account.Email, "password")
    
    // 3. 验证直接返回token
    assert.False(t, response.RequiresOTP)
    assert.NotEmpty(t, response.Token)
    assert.Equal(t, account.ID, response.UserID)
}
```

### 3. 账号创建测试用例

#### 测试用例 3.1：普通用户只能为自己的交易员创建账号
```go
func TestUserCanOnlyCreateAccountForOwnTrader(t *testing.T) {
    // 1. 创建两个用户
    user1 := createTestUser("user1@test.com", "user")
    user2 := createTestUser("user2@test.com", "user")
    
    // 2. 用户1创建交易员
    trader := createTestTrader(user1.ID, "Trader 1")
    
    // 3. 用户2尝试为该交易员创建账号（应该失败）
    response := createTraderAccount(user2.ID, trader.ID)
    assert.Equal(t, http.StatusForbidden, response.StatusCode)
    assert.Contains(t, response.Error, "只能为自己的交易员创建账号")
    
    // 4. 用户1创建账号（应该成功）
    response = createTraderAccount(user1.ID, trader.ID)
    assert.Equal(t, http.StatusOK, response.StatusCode)
}
```

#### 测试用例 3.2：随机生成账号和密码
```go
func TestRandomAccountGeneration(t *testing.T) {
    // 1. 创建用户和交易员
    user := createTestUser("user@test.com", "user")
    trader := createTestTrader(user.ID, "Trader 1")
    
    // 2. 请求随机生成账号和密码
    req := CreateAccountRequest{
        GenerateRandomEmail:    true,
        GenerateRandomPassword: true,
    }
    
    // 3. 创建账号
    response := createTraderAccountWithRequest(user.ID, trader.ID, req)
    
    // 4. 验证返回的账号和密码
    assert.NotEmpty(t, response.Email)
    assert.NotEmpty(t, response.Password)
    assert.Contains(t, response.Email, "@nofx.local")
    assert.Len(t, response.Password, 12) // 默认12位密码
}
```

### 4. 分类管理测试用例

#### 测试用例 4.1：普通用户只能管理自己的分类
```go
func TestUserCanOnlyManageOwnCategories(t *testing.T) {
    // 1. 创建两个用户
    user1 := createTestUser("user1@test.com", "user")
    user2 := createTestUser("user2@test.com", "user")
    
    // 2. 用户1创建分类
    category := createTestCategory(user1.ID, "Category 1")
    
    // 3. 用户2尝试删除该分类（应该失败）
    response := deleteCategory(user2.ID, category.ID)
    assert.Equal(t, http.StatusForbidden, response.StatusCode)
    
    // 4. 用户1删除分类（应该成功）
    response = deleteCategory(user1.ID, category.ID)
    assert.Equal(t, http.StatusOK, response.StatusCode)
}
```

#### 测试用例 4.2：删除分类时交易员分类字段被清空
```go
func TestDeleteCategoryClearsTraderCategory(t *testing.T) {
    // 1. 创建用户、分类和交易员
    user := createTestUser("user@test.com", "user")
    category := createTestCategory(user.ID, "Category 1")
    trader := createTestTraderWithCategory(user.ID, category.Name)
    
    // 2. 验证交易员有分类
    assert.Equal(t, category.Name, trader.Category)
    
    // 3. 删除分类
    deleteCategory(user.ID, category.ID)
    
    // 4. 验证交易员的分类字段被清空
    updatedTrader := getTrader(trader.ID)
    assert.Empty(t, updatedTrader.Category)
}
```

---

## ❓ 常见问题 FAQ

### Q1: 现有用户如何升级到新系统？

**A:** 现有用户无需任何操作，系统会自动处理：
- 现有用户的 `role` 字段会自动设置为 `'user'`（通过数据库迁移）
- 现有交易员的 `owner_user_id` 会根据 `user_id` 字段自动设置（通过迁移脚本）
- 现有功能完全保留，向后兼容

### Q2: 如果交易员没有分类，会怎样？

**A:** 交易员可以没有分类（`category` 为空字符串）：
- 普通用户可以看到 `owner_user_id` 为自己的所有交易员（包括没有分类的）
- 交易员账号可以正常创建和使用（不受分类限制）
- 删除分类时，该分类下的交易员的 `category` 会被清空，但交易员不会被删除

### Q3: 小组组长可以修改分类吗？

**A:** 不可以。小组组长是只读角色：
- 只能查看管理的分类下的交易员数据
- 不能创建、编辑、删除交易员
- 不能修改分类信息
- 不能创建账号

### Q4: 一个交易员可以有多个账号吗？

**A:** 不可以。一个交易员只能关联一个交易员账号（`trader_account_id`）：
- 如果交易员已有账号，再次创建会返回错误
- 如果需要更换账号，需要先删除现有账号，再创建新账号

### Q5: 一个小组组长可以管理多个分类吗？

**A:** 可以。一个小组组长可以观测多个分类：
- 创建小组组长时，可以指定多个分类
- 小组组长可以看到这些分类下的所有交易员
- 后续可以通过更新接口修改管理的分类列表

### Q6: Admin Mode 和 role='admin' 有什么区别？

**A:** 这是两个不同的概念：
- **Admin Mode** (`config.json` 中的 `admin_mode`): 系统级开关，控制是否允许用户注册
  - `admin_mode=true`: 禁用注册，只能通过 `/api/admin-login` 登录
  - `admin_mode=false`: 允许用户注册
- **role='admin'**: 用户级角色，控制用户权限
  - 可以管理所有交易员（跨用户）
  - 可以创建所有类型的账号
  - 需要 OTP 验证
  - **注意：** 一般情况下不使用此角色，默认使用 User 角色

### Q7: 创建的账号密码忘记了怎么办？

**A:** 目前需要普通用户（创建者）重置密码：
- 普通用户可以重置自己创建的交易员账号密码
- 普通用户可以重置自己创建的小组组长密码
- 后续可以添加"忘记密码"功能（需要邮箱验证）

### Q8: 分类名称可以重复吗？

**A:** 同一用户下不能有重复的分类名称：
- 不同用户可以创建相同名称的分类（数据隔离）
- 同一用户不能创建重复的分类名称
- 修改分类名称时，也会检查新名称是否已存在

### Q9: 删除分类会影响交易员吗？

**A:** 不会删除交易员，只会清空分类字段：
- 删除分类时，该分类下的所有交易员的 `category` 字段会被设置为空字符串
- 交易员仍然存在，只是不再属于任何分类
- 小组组长如果只观测该分类，删除分类后该关联关系会被删除

### Q10: 如何将现有交易员分配到分类？

**A:** 有两种方式：
1. **创建交易员时指定分类**：在创建交易员时提供 `category` 参数
2. **后续设置分类**：通过 `POST /api/traders/:id/category` 接口设置交易员分类

---

## 🚀 部署注意事项

### 1. 数据库迁移顺序

**重要：** 必须按照以下顺序执行数据库迁移：

```sql
-- 步骤1: 扩展用户表
ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'user';
ALTER TABLE users ADD COLUMN trader_id TEXT DEFAULT NULL;
ALTER TABLE users ADD COLUMN category TEXT DEFAULT NULL;

-- 步骤2: 扩展交易员表
ALTER TABLE traders ADD COLUMN category TEXT DEFAULT '';
ALTER TABLE traders ADD COLUMN trader_account_id TEXT DEFAULT NULL;
ALTER TABLE traders ADD COLUMN owner_user_id TEXT DEFAULT NULL;

-- 步骤3: 创建分类表
CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    owner_user_id TEXT NOT NULL,
    description TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(owner_user_id, name)
);

-- 步骤4: 创建小组组长分类关联表
CREATE TABLE IF NOT EXISTS group_leader_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_leader_id TEXT NOT NULL,
    category TEXT NOT NULL,
    owner_user_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (group_leader_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(group_leader_id, category)
);

-- 步骤5: 创建索引
CREATE INDEX IF NOT EXISTS idx_owner_user ON categories(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_group_leader ON group_leader_categories(group_leader_id);
CREATE INDEX IF NOT EXISTS idx_category ON group_leader_categories(category);
CREATE INDEX IF NOT EXISTS idx_owner_user_gl ON group_leader_categories(owner_user_id);

-- 步骤6: 数据迁移（通过代码执行）
-- 设置现有用户的role
UPDATE users SET role = 'user' WHERE role IS NULL OR role = '';

-- 设置现有交易员的owner_user_id（如果traders表有user_id字段）
UPDATE traders SET owner_user_id = user_id WHERE owner_user_id IS NULL AND user_id IS NOT NULL;
```

### 2. 备份策略

**部署前必须备份：**
```bash
# 备份数据库
cp nofx.db nofx.db.backup.$(date +%Y%m%d_%H%M%S)

# 备份配置文件
cp config.json config.json.backup.$(date +%Y%m%d_%H%M%S)
```

### 3. 回滚方案

**如果迁移失败，可以回滚：**
```sql
-- 注意：SQLite不支持DROP COLUMN，需要重建表
-- 如果迁移失败，恢复备份数据库
cp nofx.db.backup.xxx nofx.db
```

### 4. 性能优化建议

**数据库索引：**
- ✅ 已为 `categories.owner_user_id` 创建索引
- ✅ 已为 `group_leader_categories.group_leader_id` 创建索引
- ✅ 已为 `group_leader_categories.category` 创建索引
- ✅ 已为 `group_leader_categories.owner_user_id` 创建索引
- ⚠️ 建议为 `traders.owner_user_id` 创建索引（如果数据量大）
- ⚠️ 建议为 `traders.category` 创建索引（如果数据量大）

**查询优化：**
```go
// 使用索引字段查询
SELECT * FROM traders WHERE owner_user_id = ?  // 使用索引
SELECT * FROM traders WHERE category = ?       // 使用索引

// 避免全表扫描
SELECT * FROM traders WHERE name LIKE '%test%'  // 避免使用，如果必须使用，考虑全文搜索
```

### 5. 监控和日志

**关键日志点：**
```go
// 1. 权限检查失败
log.Printf("权限检查失败: user_id=%s, role=%s, action=%s", userID, role, action)

// 2. 账号创建
log.Printf("创建账号: user_id=%s, role=%s, email=%s", userID, role, email)

// 3. 分类操作
log.Printf("分类操作: user_id=%s, category=%s, action=%s", userID, category, action)

// 4. 数据迁移
log.Printf("数据迁移: step=%s, status=%s", step, status)
```

### 6. 环境变量配置

**建议使用环境变量：**
```bash
# 数据库路径
export NOFX_DB_PATH=/path/to/nofx.db

# 日志级别
export NOFX_LOG_LEVEL=info

# Admin Mode
export NOFX_ADMIN_MODE=false
```

---

## 📚 参考文档

### 相关文件位置

**后端文件：**
- `api/server.go` - API路由和处理函数
- `config/database.go` - 数据库操作
- `auth/auth.go` - 认证和授权

**前端文件：**
- `web/src/components/AITradersPage.tsx` - 交易员列表页面
- `web/src/components/LoginPage.tsx` - 登录页面
- `web/src/contexts/AuthContext.tsx` - 认证上下文
- `web/src/App.tsx` - 主应用组件

**数据库：**
- `nofx.db` - SQLite数据库文件

### 相关API端点

**用户相关：**
- `POST /api/register` - 用户注册
- `POST /api/login` - 用户登录
- `GET /api/account` - 获取账户信息

**交易员相关：**
- `GET /api/my-traders` - 获取交易员列表
- `GET /api/traders/:id` - 获取交易员详情
- `POST /api/traders` - 创建交易员
- `POST /api/traders/:id/create-account` - 创建交易员账号

**分类相关：**
- `GET /api/categories` - 获取分类列表
- `POST /api/categories` - 创建分类
- `PUT /api/categories/:id` - 更新分类
- `DELETE /api/categories/:id` - 删除分类
- `POST /api/traders/:id/category` - 设置交易员分类

**小组组长相关：**
- `POST /api/group-leaders/create` - 创建小组组长账号
- `GET /api/group-leaders` - 获取小组组长列表
- `PUT /api/group-leaders/:id/categories` - 更新小组组长分类
- `DELETE /api/group-leaders/:id` - 删除小组组长账号

---

## 🔄 版本历史

### v1.0.0 (2024-01-XX)
- ✅ 初始版本
- ✅ 四级权限体系（Admin / User / Group Leader / Trader Account）
- ✅ 分类管理功能
- ✅ 账号创建功能
- ✅ 数据隔离和权限控制

---

## 📝 更新日志

### 2024-01-XX
- 初始文档创建
- 完成系统架构设计
- 完成API设计
- 完成前端界面设计
- 完成实现步骤规划

---

**文档版本：** 1.0.0  
**最后更新：** 2024-01-XX  
**维护者：** NOFX Team
