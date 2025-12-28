package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"nofx/crypto"
	"nofx/market"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite" // 保留以兼容旧代码
)

// DatabaseInterface 定义了数据库实现需要提供的方法集合
type DatabaseInterface interface {
	SetCryptoService(cs *crypto.CryptoService)
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(userID string) (*User, error)
	GetAllUsers() ([]string, error)
	UpdateUserOTPVerified(userID string, verified bool) error
	GetAIModels(userID string) ([]*AIModelConfig, error)
	UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error
	GetExchanges(userID string) ([]*ExchangeConfig, error)
	UpdateExchange(userID, id string, enabled bool, apiKey, secretKey, passphrase string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, provider, label string) error
	CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error
	CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error
	CreateTrader(trader *TraderRecord) error
	GetTraders(userID string) ([]*TraderRecord, error)
	UpdateTraderStatus(userID, id string, isRunning bool) error
	UpdateTrader(trader *TraderRecord) error
	UpdateTraderInitialBalance(userID, id string, newBalance float64) error
	UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error
	DeleteTrader(userID, id string) error
	GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error)
	GetSystemConfig(key string) (string, error)
	SetSystemConfig(key, value string) error
	CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GetUserSignalSource(userID string) (*UserSignalSource, error)
	UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GetCustomCoins() []string
	LoadBetaCodesFromFile(filePath string) error
	ValidateBetaCode(code string) (bool, error)
	UseBetaCode(code, userEmail string) error
	GetBetaCodeStats() (total, used int, err error)
	Close() error
}

// Database 配置数据库
type Database struct {
	db            *sql.DB
	cryptoService *crypto.CryptoService
	isMySQL       bool // 标记是否为MySQL数据库
}

// getTimeFunc 根据数据库类型返回时间函数
func (d *Database) getTimeFunc() string {
	if d.isMySQL {
		return "NOW()"
	}
	return "datetime('now')"
}

// NewDatabase 创建配置数据库
// dbPath可以是SQLite文件路径，也可以是MySQL连接字符串
// MySQL连接字符串格式: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
// 如果dbPath包含"@tcp("则认为是MySQL连接，否则认为是SQLite文件路径
func NewDatabase(dbPath string) (*Database, error) {
	var db *sql.DB
	var err error
	var isMySQL bool

	// 判断是MySQL还是SQLite
	if strings.Contains(dbPath, "@tcp(") {
		// MySQL连接
		isMySQL = true
		db, err = sql.Open("mysql", dbPath)
		if err != nil {
			return nil, fmt.Errorf("打开MySQL数据库失败: %w", err)
		}
		// 设置MySQL连接池参数（优化以解决 connection lost 问题）
		// 增加最大连接数，适应并发请求
		db.SetMaxOpenConns(50)
		// 增加空闲连接数，减少频繁握手
		db.SetMaxIdleConns(10)
		// 关键：设置连接生命周期为3分钟（小于MySQL默认的wait_timeout 8小时，也小于常见的防火墙/代理超时）
		// 这能强制客户端定期丢弃旧连接，避免复用已被服务端或中间件关闭的连接
		db.SetConnMaxLifetime(3 * time.Minute)
		db.SetConnMaxIdleTime(1 * time.Minute) // 空闲连接最大存活时间
		log.Printf("✅ 使用MySQL数据库连接 (连接池已优化)")
	} else {
		// SQLite连接（向后兼容）
		isMySQL = false
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, fmt.Errorf("打开SQLite数据库失败: %w", err)
		}

		// 🔒 启用 WAL 模式,提高并发性能和崩溃恢复能力
		// WAL (Write-Ahead Logging) 模式的优势:
		// 1. 更好的并发性能:读操作不会被写操作阻塞
		// 2. 崩溃安全:即使在断电或强制终止时也能保证数据完整性
		// 3. 更快的写入:不需要每次都写入主数据库文件
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("启用WAL模式失败: %w", err)
		}

		// 🔒 设置 synchronous=FULL 确保数据持久性
		// FULL (2) 模式: 确保数据在关键时刻完全写入磁盘
		// 配合 WAL 模式,在保证数据安全的同时获得良好性能
		if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("设置synchronous失败: %w", err)
		}
		log.Printf("✅ 使用SQLite数据库，已启用 WAL 模式和 FULL 同步")
	}

	// 测试数据库连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	database := &Database{db: db, isMySQL: isMySQL}
	if err := database.createTables(isMySQL); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	if err := database.initDefaultData(isMySQL); err != nil {
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	return database, nil
}

// createTables 创建数据库表
func (d *Database) createTables(isMySQL bool) error {
	// 根据数据库类型选择合适的数据类型和语法
	var (
		textType          string
		boolType          string
		datetimeFunc      string
		autoIncrementType string
	)

	if isMySQL {
		textType = "VARCHAR(255)"
		boolType = "TINYINT(1)"
		datetimeFunc = "CURRENT_TIMESTAMP"
		autoIncrementType = "AUTO_INCREMENT"
	} else {
		textType = "TEXT"
		boolType = "BOOLEAN"
		datetimeFunc = "CURRENT_TIMESTAMP"
		autoIncrementType = "AUTOINCREMENT"
	}

	queries := []string{
		// AI模型配置表
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS ai_models (
			id %s PRIMARY KEY,
			user_id %s NOT NULL DEFAULT 'default',
			name %s NOT NULL,
			provider %s NOT NULL,
			enabled %s DEFAULT 0,
			api_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT %s,
			updated_at DATETIME DEFAULT %s%s
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`, textType, textType, textType, textType, boolType, datetimeFunc, datetimeFunc, func() string {
			if isMySQL {
				return ",\n\t\t\t"
			}
			return ",\n\t\t\t"
		}()),

		// 交易所配置表
		`CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- 'cex' or 'dex'
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			-- Hyperliquid 特定字段
			hyperliquid_wallet_addr TEXT DEFAULT '',
			-- Aster 特定字段
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// 用户信号源配置表
		`CREATE TABLE IF NOT EXISTS user_signal_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			coin_pool_url TEXT DEFAULT '',
			oi_top_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		)`,

		// 交易员配置表
		`CREATE TABLE IF NOT EXISTS traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		)`,

		// 用户表
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 内测码表
		`CREATE TABLE IF NOT EXISTS beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 分类表（多用户观测系统）
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			description TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(owner_user_id, name)
		)`,

		// 小组组长分类关联表（多用户观测系统）
		`CREATE TABLE IF NOT EXISTS group_leader_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_leader_id TEXT NOT NULL,
			category TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (group_leader_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(group_leader_id, category)
		)`,

		// 交易员策略状态表 (记录跟随执行情况 - 升级版: 支持多策略)
		`CREATE TABLE IF NOT EXISTS trader_strategy_status (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			strategy_id TEXT DEFAULT '',
			status TEXT DEFAULT 'WAITING', -- WAITING, ENTRY, ADD_1, ADD_2, CLOSED
			entry_price REAL DEFAULT 0,
			quantity REAL DEFAULT 0,
			realized_pnl REAL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE,
			UNIQUE(trader_id, strategy_id)
		)`,

		// 策略决策历史表 (记录每次AI决策,包括WAIT)
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS strategy_decision_history (
			id %s PRIMARY KEY %s,
			trader_id %s NOT NULL,
			strategy_id %s NOT NULL,
			decision_time DATETIME DEFAULT %s,
			action %s NOT NULL,
			symbol %s NOT NULL,
			current_price REAL DEFAULT 0,
			target_price REAL DEFAULT 0,
			position_side %s DEFAULT '',
			position_qty REAL DEFAULT 0,
			amount_percent REAL DEFAULT 0,
			reason %s DEFAULT '',
			rsi_1h REAL DEFAULT 0,
			rsi_4h REAL DEFAULT 0,
			macd_4h REAL DEFAULT 0,
			execution_success %s DEFAULT 0,
			execution_error %s DEFAULT '',
			FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE
		)`, func() string {
			if isMySQL {
				return "BIGINT"
			}
			return "INTEGER"
		}(), autoIncrementType, textType, textType, datetimeFunc, textType, textType, textType, textType, boolType, textType),

		// 为策略决策历史表创建索引
		`CREATE INDEX IF NOT EXISTS idx_strategy_decision_trader ON strategy_decision_history(trader_id, decision_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_strategy_decision_strategy ON strategy_decision_history(strategy_id, decision_time DESC)`,

		// 触发器：自动更新 updated_at
		`CREATE TRIGGER IF NOT EXISTS update_users_updated_at
			AFTER UPDATE ON users
			BEGIN
				UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
			AFTER UPDATE ON traders
			BEGIN
				UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_user_signal_sources_updated_at
			AFTER UPDATE ON user_signal_sources
			BEGIN
				UPDATE user_signal_sources SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_system_config_updated_at
			AFTER UPDATE ON system_config
			BEGIN
				UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
			END`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败 [%s]: %w", query, err)
		}
	}

	// 为现有数据库添加新字段（向后兼容）
	alterQueries := []string{
		`ALTER TABLE exchanges ADD COLUMN hyperliquid_wallet_addr TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_user TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_signer TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_private_key TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN passphrase TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN provider TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN label TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN custom_prompt TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN override_base_prompt BOOLEAN DEFAULT 0`,
		`ALTER TABLE traders ADD COLUMN is_cross_margin BOOLEAN DEFAULT 1`,             // 默认为全仓模式
		`ALTER TABLE traders ADD COLUMN use_default_coins BOOLEAN DEFAULT 1`,           // 默认使用默认币种
		`ALTER TABLE traders ADD COLUMN custom_coins TEXT DEFAULT ''`,                  // 自定义币种列表（JSON格式）
		`ALTER TABLE traders ADD COLUMN btc_eth_leverage INTEGER DEFAULT 5`,            // BTC/ETH杠杆倍数
		`ALTER TABLE traders ADD COLUMN altcoin_leverage INTEGER DEFAULT 5`,            // 山寨币杠杆倍数
		`ALTER TABLE traders ADD COLUMN trading_symbols TEXT DEFAULT ''`,               // 交易币种，逗号分隔
		`ALTER TABLE traders ADD COLUMN use_coin_pool BOOLEAN DEFAULT 0`,               // 是否使用COIN POOL信号源
		`ALTER TABLE traders ADD COLUMN use_oi_top BOOLEAN DEFAULT 0`,                  // 是否使用OI TOP信号源
		`ALTER TABLE traders ADD COLUMN system_prompt_template TEXT DEFAULT 'default'`, // 系统提示词模板名称
		`ALTER TABLE ai_models ADD COLUMN custom_api_url TEXT DEFAULT ''`,              // 自定义API地址
		`ALTER TABLE ai_models ADD COLUMN custom_model_name TEXT DEFAULT ''`,           // 自定义模型名称
		`ALTER TABLE strategy_decision_history ADD COLUMN system_prompt TEXT DEFAULT ''`,
		`ALTER TABLE strategy_decision_history ADD COLUMN input_prompt TEXT DEFAULT ''`,
		`ALTER TABLE strategy_decision_history ADD COLUMN raw_ai_response TEXT DEFAULT ''`,
		// 多用户观测系统扩展字段
		`ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'user'`,              // 用户角色: 'admin' | 'user' | 'group_leader' | 'trader_account'
		`ALTER TABLE users ADD COLUMN trader_id TEXT DEFAULT NULL`,           // 交易员账号关联的交易员ID
		`ALTER TABLE users ADD COLUMN category TEXT DEFAULT NULL`,            // 交易员账号的分类（冗余字段）
		`ALTER TABLE traders ADD COLUMN category TEXT DEFAULT ''`,            // 交易员分类
		`ALTER TABLE traders ADD COLUMN trader_account_id TEXT DEFAULT NULL`, // 关联的交易员账号用户ID
		`ALTER TABLE traders ADD COLUMN owner_user_id TEXT DEFAULT NULL`,     // 创建该交易员的用户ID
	}

	for _, query := range alterQueries {
		// 忽略已存在字段的错误
		d.db.Exec(query)
	}

	// 检查是否需要迁移exchanges表的主键结构
	err := d.migrateExchangesTable()
	if err != nil {
		log.Printf("⚠️ 迁移exchanges表失败: %v", err)
	}

	// 创建索引（多用户观测系统）
	indexQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_group_leader ON group_leader_categories(group_leader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_category ON group_leader_categories(category)`,
		`CREATE INDEX IF NOT EXISTS idx_owner_user ON group_leader_categories(owner_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_owner_user_categories ON categories(owner_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traders_category ON traders(category)`,
		`CREATE INDEX IF NOT EXISTS idx_traders_owner_user_id ON traders(owner_user_id)`,
	}

	for _, query := range indexQueries {
		d.db.Exec(query)
	}

	// 数据迁移：设置现有用户的role和现有交易员的owner_user_id
	d.migrateUserRoles()
	d.migrateTradersOwnerUserID()

	return nil
}

// initDefaultData 初始化默认数据
func (d *Database) initDefaultData(isMySQL bool) error {
	// 初始化AI模型（使用default用户）
	aiModels := []struct {
		id, name, provider string
	}{
		{"deepseek", "DeepSeek", "deepseek"},
		{"qwen", "Qwen", "qwen"},
	}

	// 根据数据库类型选择INSERT语法
	insertIgnore := "INSERT OR IGNORE"
	if isMySQL {
		insertIgnore = "INSERT IGNORE"
	}

	for _, model := range aiModels {
		_, err := d.db.Exec(fmt.Sprintf(`
			%s INTO ai_models (id, user_id, name, provider, enabled) 
			VALUES (?, 'default', ?, ?, 0)
		`, insertIgnore), model.id, model.name, model.provider)
		if err != nil {
			return fmt.Errorf("初始化AI模型失败: %w", err)
		}
	}

	// 初始化交易所（使用default用户）
	exchanges := []struct {
		id, name, typ string
	}{
		{"binance", "Binance Futures", "binance"},
		{"hyperliquid", "Hyperliquid", "hyperliquid"},
		{"aster", "Aster DEX", "aster"},
		{"bitget", "Bitget Futures", "bitget"},
	}

	for _, exchange := range exchanges {
		_, err := d.db.Exec(fmt.Sprintf(`
			%s INTO exchanges (id, user_id, name, type, enabled) 
			VALUES (?, 'default', ?, ?, 0)
		`, insertIgnore), exchange.id, exchange.name, exchange.typ)
		if err != nil {
			return fmt.Errorf("初始化交易所失败: %w", err)
		}
	}

	// 初始化系统配置 - 创建所有字段，设置默认值，后续由config.json同步更新
	systemConfigs := map[string]string{
		"beta_mode":            "false",                                                                               // 默认关闭内测模式
		"api_server_port":      "8080",                                                                                // 默认API端口
		"use_default_coins":    "true",                                                                                // 默认使用内置币种列表
		"default_coins":        `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`, // 默认币种列表（JSON格式）
		"max_daily_loss":       "10.0",                                                                                // 最大日损失百分比
		"max_drawdown":         "20.0",                                                                                // 最大回撤百分比
		"stop_trading_minutes": "60",                                                                                  // 停止交易时间（分钟）
		"btc_eth_leverage":     "5",                                                                                   // BTC/ETH杠杆倍数
		"altcoin_leverage":     "5",                                                                                   // 山寨币杠杆倍数
		"jwt_secret":           "",                                                                                    // JWT密钥，默认为空，由config.json或系统生成
	}

	for key, value := range systemConfigs {
		_, err := d.db.Exec(fmt.Sprintf(`
			%s INTO system_config (`+"`key`"+`, value) 
			VALUES (?, ?)
		`, insertIgnore), key, value)
		if err != nil {
			return fmt.Errorf("初始化系统配置失败: %w", err)
		}
	}

	return nil
}

// migrateExchangesTable 迁移exchanges表支持多用户
func (d *Database) migrateExchangesTable() error {
	// 检查是否已经迁移过
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='exchanges_new'
	`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已经迁移过，直接返回
	if count > 0 {
		return nil
	}

	log.Printf("🔄 开始迁移exchanges表...")

	// 创建新的exchanges表，使用复合主键
	_, err = d.db.Exec(`
		CREATE TABLE exchanges_new (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			passphrase TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("创建新exchanges表失败: %w", err)
	}

	// 复制数据到新表
	_, err = d.db.Exec(`
		INSERT INTO exchanges_new 
		SELECT * FROM exchanges
	`)
	if err != nil {
		return fmt.Errorf("复制数据失败: %w", err)
	}

	// 删除旧表
	_, err = d.db.Exec(`DROP TABLE exchanges`)
	if err != nil {
		return fmt.Errorf("删除旧表失败: %w", err)
	}

	// 重命名新表
	_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
	if err != nil {
		return fmt.Errorf("重命名表失败: %w", err)
	}

	// 重新创建触发器
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP 
				WHERE id = NEW.id AND user_id = NEW.user_id;
			END
	`)
	if err != nil {
		return fmt.Errorf("创建触发器失败: %w", err)
	}

	log.Printf("✅ exchanges表迁移完成")
	return nil
}

// User 用户配置
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // 不返回到前端
	OTPSecret    string    `json:"-"` // 不返回到前端
	OTPVerified  bool      `json:"otp_verified"`
	Role         string    `json:"role"`      // 用户角色: 'admin' | 'user' | 'group_leader' | 'trader_account'
	TraderID     string    `json:"trader_id"` // 交易员账号关联的交易员ID
	Category     string    `json:"category"`  // 交易员账号的分类（冗余字段）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Category 分类配置
type Category struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	OwnerUserID string    `json:"owner_user_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AIModelConfig AI模型配置
type AIModelConfig struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	Enabled         bool      `json:"enabled"`
	APIKey          string    `json:"apiKey"`
	CustomAPIURL    string    `json:"customApiUrl"`
	CustomModelName string    `json:"customModelName"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ExchangeConfig 交易所配置
type ExchangeConfig struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Provider   string `json:"provider"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	Label      string `json:"label"`      // 用户自定义标签，用于区分同一交易所的多个账号
	APIKey     string `json:"apiKey"`     // For Binance: API Key; For Hyperliquid: Agent Private Key (should have ~0 balance)
	SecretKey  string `json:"secretKey"`  // For Binance: Secret Key; Not used for Hyperliquid
	Passphrase string `json:"passphrase"` // For OKX/Bitget: Passphrase
	Testnet    bool   `json:"testnet"`
	// Hyperliquid Agent Wallet configuration (following official best practices)
	// Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Main Wallet Address (holds funds, never expose private key)
	// Aster 特定字段
	AsterUser       string    `json:"asterUser"`
	AsterSigner     string    `json:"asterSigner"`
	AsterPrivateKey string    `json:"asterPrivateKey"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TraderRecord 交易员配置（数据库实体）
type TraderRecord struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Name                 string    `json:"name"`
	AIModelID            string    `json:"ai_model_id"`
	ExchangeID           string    `json:"exchange_id"`
	InitialBalance       float64   `json:"initial_balance"`
	ScanIntervalMinutes  int       `json:"scan_interval_minutes"`
	IsRunning            bool      `json:"is_running"`
	BTCETHLeverage       int       `json:"btc_eth_leverage"`       // BTC/ETH杠杆倍数
	AltcoinLeverage      int       `json:"altcoin_leverage"`       // 山寨币杠杆倍数
	TradingSymbols       string    `json:"trading_symbols"`        // 交易币种，逗号分隔
	UseCoinPool          bool      `json:"use_coin_pool"`          // 是否使用COIN POOL信号源
	UseOITop             bool      `json:"use_oi_top"`             // 是否使用OI TOP信号源
	CustomPrompt         string    `json:"custom_prompt"`          // 自定义交易策略prompt
	OverrideBasePrompt   bool      `json:"override_base_prompt"`   // 是否覆盖基础prompt
	SystemPromptTemplate string    `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        bool      `json:"is_cross_margin"`        // 是否为全仓模式（true=全仓，false=逐仓）
	Category             string    `json:"category"`               // 交易员分类
	TraderAccountID      string    `json:"trader_account_id"`      // 关联的交易员账号用户ID
	OwnerUserID          string    `json:"owner_user_id"`          // 创建该交易员的用户ID
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// UserSignalSource 用户信号源配置
type UserSignalSource struct {
	ID          int       `json:"id"`
	UserID      string    `json:"user_id"`
	CoinPoolURL string    `json:"coin_pool_url"`
	OITopURL    string    `json:"oi_top_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

// CreateUser 创建用户
func (d *Database) CreateUser(user *User) error {
	role := user.Role
	if role == "" {
		role = "user"
	}
	_, err := d.db.Exec(`
		INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, role, trader_id, category)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.OTPSecret, user.OTPVerified, role, user.TraderID, user.Category)
	return err
}

// EnsureAdminUser 确保admin用户存在（用于管理员模式）
func (d *Database) EnsureAdminUser() error {
	// 检查admin用户是否已存在
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已存在，直接返回
	if count > 0 {
		return nil
	}

	// 创建admin用户（密码为空，因为管理员模式下不需要密码）
	adminUser := &User{
		ID:           "admin",
		Email:        "admin@localhost",
		PasswordHash: "", // 管理员模式下不使用密码
		OTPSecret:    "",
		OTPVerified:  true,
	}

	return d.CreateUser(adminUser)
}

// GetUserByEmail 通过邮箱获取用户
func (d *Database) GetUserByEmail(email string) (*User, error) {
	var user User
	var role, traderID, category sql.NullString
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, 
		       COALESCE(role, 'user') as role, trader_id, category,
		       created_at, updated_at
		FROM users WHERE email = ?
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &role, &traderID, &category,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if role.Valid {
		user.Role = role.String
	} else {
		user.Role = "user"
	}
	if traderID.Valid {
		user.TraderID = traderID.String
	}
	if category.Valid {
		user.Category = category.String
	}
	return &user, nil
}

// GetUserByID 通过ID获取用户
func (d *Database) GetUserByID(userID string) (*User, error) {
	var user User
	var role, traderID, category sql.NullString
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified,
		       COALESCE(role, 'user') as role, trader_id, category,
		       created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &role, &traderID, &category,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if role.Valid {
		user.Role = role.String
	} else {
		user.Role = "user"
	}
	if traderID.Valid {
		user.TraderID = traderID.String
	}
	if category.Valid {
		user.Category = category.String
	}
	return &user, nil
}

// GetAllUsers 获取所有用户ID列表
func (d *Database) GetAllUsers() ([]string, error) {
	rows, err := d.db.Query(`SELECT id FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

// UpdateUserOTPVerified 更新用户OTP验证状态
func (d *Database) UpdateUserOTPVerified(userID string, verified bool) error {
	_, err := d.db.Exec(`UPDATE users SET otp_verified = ? WHERE id = ?`, verified, userID)
	return err
}

// UpdateUserPassword 更新用户密码
func (d *Database) UpdateUserPassword(userID, passwordHash string) error {
	_, err := d.db.Exec(fmt.Sprintf(`
		UPDATE users
		SET password_hash = ?, updated_at = %s
		WHERE id = ?
	`, d.getTimeFunc()), passwordHash, userID)
	return err
}

// GetAIModels 获取用户的AI模型配置
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, provider, enabled, 
		       COALESCE(api_key, '') as api_key,
		       COALESCE(custom_api_url, '') as custom_api_url,
		       COALESCE(custom_model_name, '') as custom_model_name,
		       created_at, updated_at
		FROM ai_models WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	models := make([]*AIModelConfig, 0)
	for rows.Next() {
		var model AIModelConfig
		err := rows.Scan(
			&model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&model.CreatedAt, &model.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		// 解密API Key（如果为空字符串则跳过解密）
		if model.APIKey != "" {
			model.APIKey = d.decryptSensitiveData(model.APIKey)
		}
		models = append(models, &model)
	}

	return models, nil
}

// UpdateAIModel 更新AI模型配置，如果不存在则创建用户特定配置
func (d *Database) UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	// 先尝试精确匹配 ID（新版逻辑，支持多个相同 provider 的模型）
	var existingID string
	err := d.db.QueryRow(`
		SELECT id FROM ai_models WHERE user_id = ? AND id = ? LIMIT 1
	`, userID, id).Scan(&existingID)

	if err == nil {
		// 找到了现有配置（精确匹配 ID），更新它
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(fmt.Sprintf(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = %s
			WHERE id = ? AND user_id = ?
		`, d.getTimeFunc()), enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		return err
	}

	// ID 不存在，尝试兼容旧逻辑：将 id 作为 provider 查找
	provider := id
	err = d.db.QueryRow(`
		SELECT id FROM ai_models WHERE user_id = ? AND provider = ? LIMIT 1
	`, userID, provider).Scan(&existingID)

	if err == nil {
		// 找到了现有配置（通过 provider 匹配，兼容旧版），更新它
		log.Printf("✓ 通过 provider 匹配更新模型: %s -> %s（建议前端使用完整ID）", provider, existingID)
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(fmt.Sprintf(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = %s
			WHERE id = ? AND user_id = ?
		`, d.getTimeFunc()), enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		return err
	}

	// 没有找到任何现有配置，创建新的
	// 推断 provider（从 id 中提取，或者直接使用 id）
	if provider == id && (provider == "deepseek" || provider == "qwen") {
		// id 本身就是 provider
		provider = id
	} else {
		// 从 id 中提取 provider（假设格式是 userID_provider 或 timestamp_userID_provider）
		parts := strings.Split(id, "_")
		if len(parts) >= 2 {
			provider = parts[len(parts)-1] // 取最后一部分作为 provider
		} else {
			provider = id
		}
	}

	// 获取模型的基本信息
	var name string
	err = d.db.QueryRow(`
		SELECT name FROM ai_models WHERE provider = ? LIMIT 1
	`, provider).Scan(&name)
	if err != nil {
		// 如果找不到基本信息，使用默认值
		if provider == "deepseek" {
			name = "DeepSeek AI"
		} else if provider == "qwen" {
			name = "Qwen AI"
		} else {
			name = provider + " AI"
		}
	}

	// 如果传入的 ID 已经是完整格式（如 "admin_deepseek_custom1"），直接使用
	// 否则生成新的 ID
	newModelID := id
	if id == provider {
		// id 就是 provider，生成新的用户特定 ID
		newModelID = fmt.Sprintf("%s_%s", userID, provider)
	}

	log.Printf("✓ 创建新的 AI 模型配置: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	timeFunc := d.getTimeFunc()
	_, err = d.db.Exec(fmt.Sprintf(`
		INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, %s, %s)
	`, timeFunc, timeFunc), newModelID, userID, name, provider, enabled, encryptedAPIKey, customAPIURL, customModelName)

	return err
}

// GetExchanges 获取用户的交易所配置
func (d *Database) GetExchanges(userID string) ([]*ExchangeConfig, error) {
	// 构建查询SQL，包含provider和label字段
	query := `
		SELECT id, user_id, name, type, enabled, 
		       COALESCE(api_key, '') as api_key, 
		       COALESCE(secret_key, '') as secret_key, 
		       testnet, 
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       COALESCE(passphrase, '') as passphrase,
		       COALESCE(provider, '') as provider,
		       COALESCE(label, '') as label,
		       created_at, updated_at 
		FROM exchanges WHERE user_id = ? ORDER BY id
	`

	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	exchanges := make([]*ExchangeConfig, 0)
	for rows.Next() {
		var exchange ExchangeConfig
		var dbProvider, dbLabel string

		err := rows.Scan(
			&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type,
			&exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
			&exchange.HyperliquidWalletAddr, &exchange.AsterUser,
			&exchange.AsterSigner, &exchange.AsterPrivateKey, &exchange.Passphrase,
			&dbProvider, &dbLabel,
			&exchange.CreatedAt, &exchange.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// 解密敏感字段（如果为空字符串则跳过解密）
		if exchange.APIKey != "" {
			exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
		}
		if exchange.SecretKey != "" {
			exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
		}
		if exchange.AsterPrivateKey != "" {
			exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)
		}
		if exchange.Passphrase != "" {
			exchange.Passphrase = d.decryptSensitiveData(exchange.Passphrase)
		}

		// 如果数据库中有provider，使用数据库值，否则推导
		if dbProvider != "" {
			exchange.Provider = dbProvider
		} else {
			exchange.Provider = inferExchangeProvider(exchange.Type, exchange.ID)
		}

		// 🔑 关键修复：将数据库中的label赋值给Label字段，前端会优先显示此字段
		exchange.Label = dbLabel

		exchanges = append(exchanges, &exchange)
	}

	return exchanges, nil
}

// UpdateExchange 更新交易所配置，如果不存在则创建用户特定配置
// 🔒 安全特性：空值不会覆盖现有的敏感字段（api_key, secret_key, aster_private_key）
func (d *Database) UpdateExchange(userID, id string, enabled bool, apiKey, secretKey, passphrase string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, provider, label string) error {
	log.Printf("🔧 UpdateExchange: userID=%s, id=%s, enabled=%v, provider=%s, label=%s", userID, id, enabled, provider, label)

	// 构建动态 UPDATE SET 子句
	// 基础字段：总是更新
	timeFunc := d.getTimeFunc()
	setClauses := []string{
		"enabled = ?",
		"testnet = ?",
		"hyperliquid_wallet_addr = ?",
		"aster_user = ?",
		"aster_signer = ?",
		"provider = ?",
		"label = ?",
		fmt.Sprintf("updated_at = %s", timeFunc),
	}
	args := []interface{}{enabled, testnet, hyperliquidWalletAddr, asterUser, asterSigner, provider, label}

	// 🔒 敏感字段：只在非空时更新（保护现有数据）
	if apiKey != "" {
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		setClauses = append(setClauses, "api_key = ?")
		args = append(args, encryptedAPIKey)
	}

	if secretKey != "" {
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		setClauses = append(setClauses, "secret_key = ?")
		args = append(args, encryptedSecretKey)
	}

	if passphrase != "" {
		encryptedPassphrase := d.encryptSensitiveData(passphrase)
		setClauses = append(setClauses, "passphrase = ?")
		args = append(args, encryptedPassphrase)
	}

	if asterPrivateKey != "" {
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
		setClauses = append(setClauses, "aster_private_key = ?")
		args = append(args, encryptedAsterPrivateKey)
	}

	// WHERE 条件
	args = append(args, id, userID)

	// 构建完整的 UPDATE 语句
	query := fmt.Sprintf(`
		UPDATE exchanges SET %s
		WHERE id = ? AND user_id = ?
	`, strings.Join(setClauses, ", "))

	// 执行更新
	result, err := d.db.Exec(query, args...)
	if err != nil {
		log.Printf("❌ UpdateExchange: 更新失败: %v", err)
		return err
	}

	// 检查是否有行被更新
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("❌ UpdateExchange: 获取影响行数失败: %v", err)
		return err
	}

	log.Printf("📊 UpdateExchange: 影响行数 = %d", rowsAffected)

	// 如果没有行被更新，说明用户没有这个交易所的配置，需要创建
	if rowsAffected == 0 {
		log.Printf("💡 UpdateExchange: 没有现有记录，创建新记录")

		// 根据交易所ID确定基本信息
		var name, typ string
		if id == "binance" {
			name = "Binance Futures"
			typ = "cex"
		} else if id == "hyperliquid" {
			name = "Hyperliquid"
			typ = "dex"
		} else if id == "aster" {
			name = "Aster DEX"
			typ = "dex"
		} else {
			name = id + " Exchange"
			typ = "cex"
		}

		log.Printf("🆕 UpdateExchange: 创建新记录 ID=%s, name=%s, type=%s", id, name, typ)

		// 创建用户特定的配置，使用原始的交易所ID
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		encryptedPassphrase := d.encryptSensitiveData(passphrase)
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)

		timeFunc := d.getTimeFunc()
		_, err = d.db.Exec(fmt.Sprintf(`
			INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, passphrase, testnet,
			                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, provider, label, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, timeFunc, timeFunc), id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, encryptedPassphrase, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey, provider, label)

		if err != nil {
			log.Printf("❌ UpdateExchange: 创建记录失败: %v", err)
		} else {
			log.Printf("✅ UpdateExchange: 创建记录成功")
		}
		return err
	}

	log.Printf("✅ UpdateExchange: 更新现有记录成功")
	return nil
}

// CreateAIModel 创建AI模型配置
func (d *Database) CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
	timeFunc := d.getTimeFunc()
	encryptedAPIKey := d.encryptSensitiveData(apiKey)

	if d.isMySQL {
		// MySQL语法（INSERT IGNORE）
		_, err := d.db.Exec(fmt.Sprintf(`
			INSERT IGNORE INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, timeFunc, timeFunc), id, userID, name, provider, enabled, encryptedAPIKey, customAPIURL)
		return err
	} else {
		// SQLite语法（INSERT OR IGNORE）
		_, err := d.db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, timeFunc, timeFunc), id, userID, name, provider, enabled, encryptedAPIKey, customAPIURL)
		return err
	}
}

// CreateExchange 创建交易所配置
func (d *Database) CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	// 加密敏感字段
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	encryptedSecretKey := d.encryptSensitiveData(secretKey)
	encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
	timeFunc := d.getTimeFunc()

	if d.isMySQL {
		// MySQL语法（INSERT IGNORE）
		_, err := d.db.Exec(fmt.Sprintf(`
			INSERT IGNORE INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, timeFunc, timeFunc), id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey)
		return err
	} else {
		// SQLite语法（INSERT OR IGNORE）
		_, err := d.db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, timeFunc, timeFunc), id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey)
		return err
	}
}

// CreateTrader 创建交易员
func (d *Database) CreateTrader(trader *TraderRecord) error {
	category := trader.Category
	if category == "" {
		category = ""
	}
	ownerUserID := trader.OwnerUserID
	if ownerUserID == "" {
		ownerUserID = trader.UserID // 默认使用user_id作为owner_user_id
	}
	_, err := d.db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin, category, owner_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate, trader.IsCrossMargin, category, ownerUserID)
	return err
}

// GetTraders 获取用户的交易员
func (d *Database) GetTraders(userID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(category, '') as category,
		       COALESCE(trader_account_id, '') as trader_account_id,
		       COALESCE(owner_user_id, '') as owner_user_id,
		       created_at, updated_at
		FROM traders WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&trader.Category, &trader.TraderAccountID, &trader.OwnerUserID,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// UpdateTraderStatus 更新交易员状态
func (d *Database) UpdateTraderStatus(userID, id string, isRunning bool) error {
	_, err := d.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
	return err
}

// UpdateTrader 更新交易员配置
func (d *Database) UpdateTrader(trader *TraderRecord) error {
	_, err := d.db.Exec(fmt.Sprintf(`
		UPDATE traders SET
			name = ?, ai_model_id = ?, exchange_id = ?, initial_balance = ?,
			scan_interval_minutes = ?, btc_eth_leverage = ?, altcoin_leverage = ?,
			trading_symbols = ?, custom_prompt = ?, override_base_prompt = ?,
			system_prompt_template = ?, is_cross_margin = ?, updated_at = %s
		WHERE id = ? AND user_id = ?
	`, d.getTimeFunc()), trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance,
		trader.ScanIntervalMinutes, trader.BTCETHLeverage, trader.AltcoinLeverage,
		trader.TradingSymbols, trader.CustomPrompt, trader.OverrideBasePrompt,
		trader.SystemPromptTemplate, trader.IsCrossMargin, trader.ID, trader.UserID)
	return err
}

// UpdateTraderCustomPrompt 更新交易员自定义Prompt
func (d *Database) UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
	_, err := d.db.Exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`, customPrompt, overrideBase, id, userID)
	return err
}

// UpdateTraderInitialBalance 更新交易员初始余额（用于自动同步交易所实际余额）
func (d *Database) UpdateTraderInitialBalance(userID, id string, newBalance float64) error {
	// 🚫 严格禁止：为了防止意外覆盖用户设置的初始余额，此函数已被禁用
	// 只有手动同步API（handleSyncBalance）被允许调用此函数
	log.Printf("🚫 BLOCKED: UpdateTraderInitialBalance 调用被拒绝 - userID: %s, traderID: %s, newBalance: %.2f", userID, id, newBalance)

	// 获取调用栈信息用于调试
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	log.Printf("🚫 调用来源: %s:%d %s", frame.File, frame.Line, frame.Function)

	// 检查是否来自于允许的调用路径
	if strings.Contains(frame.Function, "handleSyncBalance") ||
		strings.Contains(frame.File, "server.go") && strings.Contains(frame.Function, "handleSyncBalance") {
		log.Printf("✅ 允许的手动同步操作")
		_, err := d.db.Exec(`UPDATE traders SET initial_balance = ? WHERE id = ? AND user_id = ?`, newBalance, id, userID)
		return err
	}

	// 拒绝所有其他调用
	return fmt.Errorf("UpdateTraderInitialBalance 已被禁用，只允许通过手动同步API调用")
}

// DeleteTrader 删除交易员
func (d *Database) DeleteTrader(userID, id string) error {
	_, err := d.db.Exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// GetTraderConfig 获取交易员完整配置（包含AI模型和交易所信息）
func (d *Database) GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error) {
	var trader TraderRecord
	var aiModel AIModelConfig
	var exchange ExchangeConfig

	var exchangeProvider, exchangeLabel string

	err := d.db.QueryRow(`
		SELECT
			t.id, t.user_id, t.name, t.ai_model_id, t.exchange_id, t.initial_balance, t.scan_interval_minutes, t.is_running,
			COALESCE(t.btc_eth_leverage, 5) as btc_eth_leverage,
			COALESCE(t.altcoin_leverage, 5) as altcoin_leverage,
			COALESCE(t.trading_symbols, '') as trading_symbols,
			COALESCE(t.use_coin_pool, 0) as use_coin_pool,
			COALESCE(t.use_oi_top, 0) as use_oi_top,
			COALESCE(t.custom_prompt, '') as custom_prompt,
			COALESCE(t.override_base_prompt, 0) as override_base_prompt,
			COALESCE(t.system_prompt_template, 'default') as system_prompt_template,
			COALESCE(t.is_cross_margin, 1) as is_cross_margin,
			t.created_at, t.updated_at,
			a.id, a.user_id, a.name, a.provider, a.enabled, a.api_key,
			COALESCE(a.custom_api_url, '') as custom_api_url,
			COALESCE(a.custom_model_name, '') as custom_model_name,
			a.created_at, a.updated_at,
			e.id, e.user_id, e.name, e.type, e.enabled, e.api_key, e.secret_key, e.testnet,
			COALESCE(e.hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
			COALESCE(e.aster_user, '') as aster_user,
			COALESCE(e.aster_signer, '') as aster_signer,
			COALESCE(e.aster_private_key, '') as aster_private_key,
			COALESCE(e.provider, '') as provider,
			COALESCE(e.label, '') as label,
			e.created_at, e.updated_at
		FROM traders t
		JOIN ai_models a ON t.ai_model_id = a.id AND t.user_id = a.user_id
		JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id
		WHERE t.id = ? AND t.user_id = ?
	`, traderID, userID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop,
		&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
		&trader.IsCrossMargin,
		&trader.CreatedAt, &trader.UpdatedAt,
		&aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CustomAPIURL, &aiModel.CustomModelName,
		&aiModel.CreatedAt, &aiModel.UpdatedAt,
		&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.HyperliquidWalletAddr, &exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchangeProvider, &exchangeLabel,
		&exchange.CreatedAt, &exchange.UpdatedAt,
	)

	if err != nil {
		return nil, nil, nil, err
	}

	// 解密敏感数据
	aiModel.APIKey = d.decryptSensitiveData(aiModel.APIKey)
	exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
	exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
	exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)

	// 推导 Provider（优先使用数据库值，否则从 Type 或 ID 推导）
	if exchangeProvider != "" {
		exchange.Provider = exchangeProvider
	} else {
		exchange.Provider = inferExchangeProvider(exchange.Type, exchange.ID)
	}

	// 设置 Label 字段
	exchange.Label = exchangeLabel

	return &trader, &aiModel, &exchange, nil
}

// inferExchangeProvider 根据 type 或 id 推导交易所 provider
func inferExchangeProvider(typ, id string) string {
	known := map[string]struct{}{
		"binance":     {},
		"bitget":      {},
		"hyperliquid": {},
		"aster":       {},
		"okx":         {},
		"bybit":       {},
	}
	lt := strings.ToLower(typ)
	if _, ok := known[lt]; ok {
		return lt
	}
	if idx := strings.Index(id, "_"); idx > 0 {
		return strings.ToLower(id[:idx])
	}
	if id != "" {
		return strings.ToLower(id)
	}
	return lt
}

// GetSystemConfig 获取系统配置
func (d *Database) GetSystemConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SetSystemConfig 设置系统配置
func (d *Database) SetSystemConfig(key, value string) error {
	timeFunc := d.getTimeFunc()

	if d.isMySQL {
		// MySQL语法（ON DUPLICATE KEY UPDATE）
		_, err := d.db.Exec(fmt.Sprintf(`
			INSERT INTO system_config (`+"`key`"+`, value, updated_at) 
			VALUES (?, ?, %s)
			ON DUPLICATE KEY UPDATE 
				value = VALUES(value), 
				updated_at = %s
		`, timeFunc, timeFunc), key, value)
		return err
	} else {
		// SQLite语法（INSERT OR REPLACE）
		_, err := d.db.Exec(fmt.Sprintf(`
			INSERT OR REPLACE INTO system_config (key, value, updated_at) 
			VALUES (?, ?, %s)
		`, timeFunc), key, value)
		return err
	}
}

// CreateUserSignalSource 创建用户信号源配置
func (d *Database) CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	timeFunc := d.getTimeFunc()

	if d.isMySQL {
		// MySQL语法（ON DUPLICATE KEY UPDATE）
		_, err := d.db.Exec(fmt.Sprintf(`
			INSERT INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
			VALUES (?, ?, ?, %s)
			ON DUPLICATE KEY UPDATE 
				coin_pool_url = VALUES(coin_pool_url),
				oi_top_url = VALUES(oi_top_url),
				updated_at = %s
		`, timeFunc, timeFunc), userID, coinPoolURL, oiTopURL)
		return err
	} else {
		// SQLite语法（INSERT OR REPLACE）
		_, err := d.db.Exec(fmt.Sprintf(`
		INSERT OR REPLACE INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
			VALUES (?, ?, ?, %s)
		`, timeFunc), userID, coinPoolURL, oiTopURL)
		return err
	}
}

// GetUserSignalSource 获取用户信号源配置
func (d *Database) GetUserSignalSource(userID string) (*UserSignalSource, error) {
	var source UserSignalSource
	err := d.db.QueryRow(`
		SELECT id, user_id, coin_pool_url, oi_top_url, created_at, updated_at
		FROM user_signal_sources WHERE user_id = ?
	`, userID).Scan(
		&source.ID, &source.UserID, &source.CoinPoolURL, &source.OITopURL,
		&source.CreatedAt, &source.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// UpdateUserSignalSource 更新用户信号源配置
func (d *Database) UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	_, err := d.db.Exec(fmt.Sprintf(`
		UPDATE user_signal_sources SET coin_pool_url = ?, oi_top_url = ?, updated_at = %s
		WHERE user_id = ?
	`, d.getTimeFunc()), coinPoolURL, oiTopURL, userID)
	return err
}

// GetCustomCoins 获取所有交易员自定义币种 / Get all trader-customized currencies
func (d *Database) GetCustomCoins() []string {
	var symbol string
	var symbols []string
	_ = d.db.QueryRow(`
		SELECT GROUP_CONCAT(custom_coins , ',') as symbol
		FROM main.traders where custom_coins != ''
	`).Scan(&symbol)
	// 检测用户是否未配置币种 - 兼容性
	if symbol == "" {
		symbolJSON, _ := d.GetSystemConfig("default_coins")
		if err := json.Unmarshal([]byte(symbolJSON), &symbols); err != nil {
			log.Printf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
			symbols = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
		}
	}
	// filter Symbol
	for _, s := range strings.Split(symbol, ",") {
		if s == "" {
			continue
		}
		coin := market.Normalize(s)
		if !slices.Contains(symbols, coin) {
			symbols = append(symbols, coin)
		}
	}
	return symbols
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	return d.db.Close()
}

// LoadBetaCodesFromFile 从文件加载内测码到数据库
func (d *Database) LoadBetaCodesFromFile(filePath string) error {
	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取内测码文件失败: %w", err)
	}

	// 按行分割内测码
	lines := strings.Split(string(content), "\n")
	var codes []string
	for _, line := range lines {
		code := strings.TrimSpace(line)
		if code != "" && !strings.HasPrefix(code, "#") {
			codes = append(codes, code)
		}
	}

	// 批量插入内测码
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO beta_codes (code) VALUES (?)`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, code := range codes {
		result, err := stmt.Exec(code)
		if err != nil {
			log.Printf("插入内测码 %s 失败: %v", code, err)
			continue
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("✅ 成功加载 %d 个内测码到数据库 (总计 %d 个)", insertedCount, len(codes))
	return nil
}

// ValidateBetaCode 验证内测码是否有效且未使用
func (d *Database) ValidateBetaCode(code string) (bool, error) {
	var used bool
	err := d.db.QueryRow(`SELECT used FROM beta_codes WHERE code = ?`, code).Scan(&used)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // 内测码不存在
		}
		return false, err
	}
	return !used, nil // 内测码存在且未使用
}

// UseBetaCode 使用内测码（标记为已使用）
func (d *Database) UseBetaCode(code, userEmail string) error {
	result, err := d.db.Exec(fmt.Sprintf(`
		UPDATE beta_codes SET used = 1, used_by = ?, used_at = %s 
		WHERE code = ? AND used = 0
	`, d.getTimeFunc()), userEmail, code)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("内测码无效或已被使用")
	}

	return nil
}

// GetBetaCodeStats 获取内测码统计信息
func (d *Database) GetBetaCodeStats() (total, used int, err error) {
	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes`).Scan(&total)
	if err != nil {
		return 0, 0, err
	}

	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes WHERE used = 1`).Scan(&used)
	if err != nil {
		return 0, 0, err
	}

	return total, used, nil
}

// SetCryptoService 设置加密服务
func (d *Database) SetCryptoService(cs *crypto.CryptoService) {
	d.cryptoService = cs
}

// encryptSensitiveData 加密敏感数据用于存储
func (d *Database) encryptSensitiveData(plaintext string) string {
	if d.cryptoService == nil || plaintext == "" {
		return plaintext
	}

	encrypted, err := d.cryptoService.EncryptForStorage(plaintext)
	if err != nil {
		log.Printf("⚠️ 加密失败: %v", err)
		return plaintext // 返回明文作为降级处理
	}

	return encrypted
}

// decryptSensitiveData 解密敏感数据
func (d *Database) decryptSensitiveData(encrypted string) string {
	if d.cryptoService == nil || encrypted == "" {
		return encrypted
	}

	// 如果不是加密格式，直接返回
	if !d.cryptoService.IsEncryptedStorageValue(encrypted) {
		return encrypted
	}

	decrypted, err := d.cryptoService.DecryptFromStorage(encrypted)
	if err != nil {
		log.Printf("⚠️ 解密失败: %v", err)
		// 🔴 CRITICAL FIX: 解密失败时返回空字符串，不要返回加密文本
		// 这样可以防止加密格式的文本被当作API密钥使用
		return ""
	}

	return decrypted
}

// migrateUserRoles 数据迁移：设置现有用户的role字段
func (d *Database) migrateUserRoles() {
	_, err := d.db.Exec(`UPDATE users SET role = 'user' WHERE role IS NULL OR role = ''`)
	if err != nil {
		log.Printf("⚠️ 迁移用户角色失败: %v", err)
	} else {
		log.Printf("✅ 用户角色迁移完成")
	}
}

// migrateTradersOwnerUserID 数据迁移：设置现有交易员的owner_user_id
func (d *Database) migrateTradersOwnerUserID() {
	// 获取所有owner_user_id为NULL的交易员
	rows, err := d.db.Query("SELECT id, user_id FROM traders WHERE owner_user_id IS NULL")
	if err != nil {
		log.Printf("⚠️ 查询交易员失败: %v", err)
		return
	}
	defer rows.Close()

	updatedCount := 0
	for rows.Next() {
		var traderID, userID string
		if err := rows.Scan(&traderID, &userID); err != nil {
			continue
		}

		// 如果user_id存在，设置为owner_user_id
		if userID != "" {
			_, err := d.db.Exec("UPDATE traders SET owner_user_id = ? WHERE id = ?", userID, traderID)
			if err != nil {
				log.Printf("⚠️ 更新交易员 %s 的owner_user_id失败: %v", traderID, err)
			} else {
				updatedCount++
			}
		}
	}

	if updatedCount > 0 {
		log.Printf("✅ 交易员owner_user_id迁移完成，更新了 %d 条记录", updatedCount)
	}
}

// GetAllTraders 获取所有交易员（Admin用）
func (d *Database) GetAllTraders() ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(category, '') as category,
		       COALESCE(trader_account_id, '') as trader_account_id,
		       COALESCE(owner_user_id, '') as owner_user_id,
		       created_at, updated_at
		FROM traders ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&trader.Category, &trader.TraderAccountID, &trader.OwnerUserID,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetTradersByOwnerUserID 根据owner_user_id获取交易员列表
func (d *Database) GetTradersByOwnerUserID(userID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(category, '') as category,
		       COALESCE(trader_account_id, '') as trader_account_id,
		       COALESCE(owner_user_id, '') as owner_user_id,
		       created_at, updated_at
		FROM traders WHERE owner_user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&trader.Category, &trader.TraderAccountID, &trader.OwnerUserID,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetTradersByCategories 根据分类列表获取交易员
func (d *Database) GetTradersByCategories(categories []string) ([]*TraderRecord, error) {
	if len(categories) == 0 {
		return []*TraderRecord{}, nil
	}

	// 构建IN子句
	placeholders := make([]string, len(categories))
	args := make([]interface{}, len(categories))
	for i, cat := range categories {
		placeholders[i] = "?"
		args[i] = cat
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(category, '') as category,
		       COALESCE(trader_account_id, '') as trader_account_id,
		       COALESCE(owner_user_id, '') as owner_user_id,
		       created_at, updated_at
		FROM traders WHERE category IN (%s) ORDER BY created_at DESC
	`, strings.Join(placeholders, ","))

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&trader.Category, &trader.TraderAccountID, &trader.OwnerUserID,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetTradersByID 根据ID获取交易员（返回数组，即使只有一个）
func (d *Database) GetTradersByID(traderID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(category, '') as category,
		       COALESCE(trader_account_id, '') as trader_account_id,
		       COALESCE(owner_user_id, '') as owner_user_id,
		       created_at, updated_at
		FROM traders WHERE id = ? ORDER BY created_at DESC
	`, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&trader.Category, &trader.TraderAccountID, &trader.OwnerUserID,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetTraderByID 根据ID获取单个交易员（包含owner_user_id和category）
func (d *Database) GetTraderByID(traderID string) (*TraderRecord, error) {
	var trader TraderRecord
	err := d.db.QueryRow(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(category, '') as category,
		       COALESCE(trader_account_id, '') as trader_account_id,
		       COALESCE(owner_user_id, '') as owner_user_id,
		       created_at, updated_at
		FROM traders WHERE id = ?
	`, traderID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop,
		&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
		&trader.IsCrossMargin,
		&trader.Category, &trader.TraderAccountID, &trader.OwnerUserID,
		&trader.CreatedAt, &trader.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &trader, nil
}

// GetUserCategories 获取用户创建的所有分类名称
func (d *Database) GetUserCategories(userID string) ([]string, error) {
	rows, err := d.db.Query(`SELECT name FROM categories WHERE owner_user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		categories = append(categories, name)
	}

	return categories, nil
}

// GetGroupLeaderCategories 获取小组组长可以观测的分类
func (d *Database) GetGroupLeaderCategories(userID string) ([]string, error) {
	rows, err := d.db.Query(`SELECT category FROM group_leader_categories WHERE group_leader_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			continue
		}
		categories = append(categories, category)
	}

	return categories, nil
}

// CreateCategory 创建分类
func (d *Database) CreateCategory(userID, name, description string) (*Category, error) {
	timeFunc := d.getTimeFunc()
	result, err := d.db.Exec(fmt.Sprintf(`
		INSERT INTO categories (name, owner_user_id, description, created_at, updated_at)
		VALUES (?, ?, ?, %s, %s)
	`, timeFunc, timeFunc), name, userID, description)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	category := &Category{
		ID:          int(id),
		Name:        name,
		OwnerUserID: userID,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return category, nil
}

// GetCategoryByID 根据ID获取分类
func (d *Database) GetCategoryByID(categoryID int) (*Category, error) {
	var category Category
	err := d.db.QueryRow(`
		SELECT id, name, owner_user_id, description, created_at, updated_at
		FROM categories WHERE id = ?
	`, categoryID).Scan(
		&category.ID, &category.Name, &category.OwnerUserID, &category.Description,
		&category.CreatedAt, &category.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// GetCategoryByName 根据名称获取分类
func (d *Database) GetCategoryByName(categoryName string) (*Category, error) {
	var category Category
	err := d.db.QueryRow(`
		SELECT id, name, owner_user_id, description, created_at, updated_at
		FROM categories WHERE name = ? LIMIT 1
	`, categoryName).Scan(
		&category.ID, &category.Name, &category.OwnerUserID, &category.Description,
		&category.CreatedAt, &category.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// GetCategoryByNameAndOwner 根据名称和所有者获取分类
func (d *Database) GetCategoryByNameAndOwner(categoryName, ownerUserID string) (*Category, error) {
	var category Category
	err := d.db.QueryRow(`
		SELECT id, name, owner_user_id, description, created_at, updated_at
		FROM categories WHERE name = ? AND owner_user_id = ?
	`, categoryName, ownerUserID).Scan(
		&category.ID, &category.Name, &category.OwnerUserID, &category.Description,
		&category.CreatedAt, &category.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// GetCategoriesByOwner 获取用户创建的分类列表
func (d *Database) GetCategoriesByOwner(userID string) ([]*Category, error) {
	rows, err := d.db.Query(`
		SELECT id, name, owner_user_id, description, created_at, updated_at
		FROM categories WHERE owner_user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*Category
	for rows.Next() {
		var category Category
		err := rows.Scan(
			&category.ID, &category.Name, &category.OwnerUserID, &category.Description,
			&category.CreatedAt, &category.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

// GetAllCategories 获取所有分类
func (d *Database) GetAllCategories() ([]*Category, error) {
	rows, err := d.db.Query(`
		SELECT id, name, owner_user_id, description, created_at, updated_at
		FROM categories ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*Category
	for rows.Next() {
		var category Category
		err := rows.Scan(
			&category.ID, &category.Name, &category.OwnerUserID, &category.Description,
			&category.CreatedAt, &category.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

// UpdateCategory 更新分类信息
func (d *Database) UpdateCategory(categoryID int, name, description string) error {
	timeFunc := d.getTimeFunc()
	_, err := d.db.Exec(fmt.Sprintf(`
		UPDATE categories SET name = ?, description = ?, updated_at = %s
		WHERE id = ?
	`, timeFunc), name, description, categoryID)
	return err
}

// DeleteCategory 删除分类
func (d *Database) DeleteCategory(categoryID int) error {
	_, err := d.db.Exec(`DELETE FROM categories WHERE id = ?`, categoryID)
	return err
}

// UpdateTraderCategory 更新交易员分类
func (d *Database) UpdateTraderCategory(traderID, category string) error {
	_, err := d.db.Exec(`UPDATE traders SET category = ? WHERE id = ?`, category, traderID)
	return err
}

// UpdateTradersCategoryToEmpty 将指定分类下的所有交易员的category设为空字符串
func (d *Database) UpdateTradersCategoryToEmpty(categoryName string) error {
	_, err := d.db.Exec(`UPDATE traders SET category = '' WHERE category = ?`, categoryName)
	return err
}

// InsertGroupLeaderCategory 插入小组组长分类关联
func (d *Database) InsertGroupLeaderCategory(groupLeaderID, category, ownerUserID string) error {
	timeFunc := d.getTimeFunc()
	// MySQL使用INSERT IGNORE，SQLite使用INSERT OR IGNORE
	insertStmt := "INSERT OR IGNORE INTO"
	if d.isMySQL {
		insertStmt = "INSERT IGNORE INTO"
	}
	_, err := d.db.Exec(fmt.Sprintf(`
		%s group_leader_categories (group_leader_id, category, owner_user_id, created_at, updated_at)
		VALUES (?, ?, ?, %s, %s)
	`, insertStmt, timeFunc, timeFunc), groupLeaderID, category, ownerUserID)
	return err
}

// UpdateTraderAccountID 更新交易员的账号ID
func (d *Database) UpdateTraderAccountID(traderID, accountID string) error {
	_, err := d.db.Exec(`UPDATE traders SET trader_account_id = ? WHERE id = ?`, accountID, traderID)
	return err
}

// GetTraderByAccountID 通过交易员账号ID查询交易员
func (d *Database) GetTraderByAccountID(accountID string) (*TraderRecord, error) {
	var trader TraderRecord
	err := d.db.QueryRow(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(category, '') as category,
		       COALESCE(trader_account_id, '') as trader_account_id,
		       COALESCE(owner_user_id, '') as owner_user_id,
		       created_at, updated_at
		FROM traders WHERE trader_account_id = ?
	`, accountID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop,
		&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
		&trader.IsCrossMargin,
		&trader.Category,
		&trader.TraderAccountID,
		&trader.OwnerUserID,
		&trader.CreatedAt, &trader.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &trader, nil
}

// DeleteUser 删除用户
func (d *Database) DeleteUser(userID string) error {
	_, err := d.db.Exec(`DELETE FROM users WHERE id = ?`, userID)
	return err
}

// DeleteGroupLeaderCategories 删除小组组长的所有分类关联
func (d *Database) DeleteGroupLeaderCategories(groupLeaderID string) error {
	_, err := d.db.Exec(`DELETE FROM group_leader_categories WHERE group_leader_id = ?`, groupLeaderID)
	return err
}

// TraderStrategyStatus 交易员策略状态
type TraderStrategyStatus struct {
	ID          int64     `json:"id"`
	TraderID    string    `json:"trader_id"`
	StrategyID  string    `json:"strategy_id"`
	Status      string    `json:"status"`
	EntryPrice  float64   `json:"entry_price"`
	Quantity    float64   `json:"quantity"`
	RealizedPnL float64   `json:"realized_pnl"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StrategyDecisionHistory 策略决策历史
type StrategyDecisionHistory struct {
	ID               int64     `json:"id"`
	TraderID         string    `json:"trader_id"`
	StrategyID       string    `json:"strategy_id"`
	DecisionTime     time.Time `json:"decision_time"`
	Action           string    `json:"action"`
	Symbol           string    `json:"symbol"`
	CurrentPrice     float64   `json:"current_price"`
	TargetPrice      float64   `json:"target_price"`
	PositionSide     string    `json:"position_side"`
	PositionQty      float64   `json:"position_qty"`
	AmountPercent    float64   `json:"amount_percent"`
	Reason           string    `json:"reason"`
	RSI1H            float64   `json:"rsi_1h"`
	RSI4H            float64   `json:"rsi_4h"`
	MACD4H           float64   `json:"macd_4h"`
	SystemPrompt     string    `json:"system_prompt"`
	InputPrompt      string    `json:"input_prompt"`
	RawAIResponse    string    `json:"raw_ai_response"`
	ExecutionSuccess bool      `json:"execution_success"`
	ExecutionError   string    `json:"execution_error"`
}

// UpdateTraderStrategyStatus 更新策略状态
func (d *Database) UpdateTraderStrategyStatus(status *TraderStrategyStatus) error {
	var query string
	if d.isMySQL {
		query = `
			INSERT INTO trader_strategy_status (trader_id, strategy_id, status, entry_price, quantity, realized_pnl, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			status=VALUES(status),
			entry_price=VALUES(entry_price),
			quantity=VALUES(quantity),
			realized_pnl=VALUES(realized_pnl),
			updated_at=VALUES(updated_at)
		`
	} else {
		query = `
			INSERT INTO trader_strategy_status (trader_id, strategy_id, status, entry_price, quantity, realized_pnl, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(trader_id, strategy_id) DO UPDATE SET
			status=excluded.status,
			entry_price=excluded.entry_price,
			quantity=excluded.quantity,
			realized_pnl=excluded.realized_pnl,
			updated_at=excluded.updated_at
		`
	}

	_, err := d.db.Exec(query, status.TraderID, status.StrategyID, status.Status, status.EntryPrice, status.Quantity, status.RealizedPnL, time.Now())
	return err
}

// GetTraderStrategyStatuses 获取交易员的所有策略状态
func (d *Database) GetTraderStrategyStatuses(traderID string) ([]*TraderStrategyStatus, error) {
	query := `SELECT id, trader_id, strategy_id, status, entry_price, quantity, realized_pnl, updated_at FROM trader_strategy_status WHERE trader_id = ?`
	rows, err := d.db.Query(query, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*TraderStrategyStatus
	for rows.Next() {
		var s TraderStrategyStatus
		if err := rows.Scan(&s.ID, &s.TraderID, &s.StrategyID, &s.Status, &s.EntryPrice, &s.Quantity, &s.RealizedPnL, &s.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, &s)
	}
	return results, nil
}

// GetTraderStrategyStatus (Deprecated: use GetTraderStrategyStatuses) 获取策略状态 (返回最新的一个，兼容旧接口)
func (d *Database) GetTraderStrategyStatus(traderID string) (*TraderStrategyStatus, error) {
	statuses, err := d.GetTraderStrategyStatuses(traderID)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, sql.ErrNoRows
	}
	// 返回最新的一个
	return statuses[len(statuses)-1], nil
}

// SaveStrategyDecision 保存策略决策历史
func (d *Database) SaveStrategyDecision(history *StrategyDecisionHistory) error {
	query := `
		INSERT INTO strategy_decision_history (
			trader_id, strategy_id, decision_time, action, symbol,
			current_price, target_price, position_side, position_qty,
			amount_percent, reason, rsi_1h, rsi_4h, macd_4h,
			system_prompt, input_prompt, raw_ai_response,
			execution_success, execution_error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query,
		history.TraderID, history.StrategyID, history.DecisionTime, history.Action, history.Symbol,
		history.CurrentPrice, history.TargetPrice, history.PositionSide, history.PositionQty,
		history.AmountPercent, history.Reason, history.RSI1H, history.RSI4H, history.MACD4H,
		history.SystemPrompt, history.InputPrompt, history.RawAIResponse,
		history.ExecutionSuccess, history.ExecutionError,
	)
	return err
}

// GetStrategyDecisionHistory 获取策略决策历史(按时间倒序,支持分页)
func (d *Database) GetStrategyDecisionHistory(traderID string, limit int) ([]*StrategyDecisionHistory, error) {
	if limit <= 0 {
		limit = 50 // 默认50条
	}
	
	query := `
		SELECT id, trader_id, strategy_id, decision_time, action, symbol,
		       current_price, target_price, position_side, position_qty,
		       amount_percent, reason, rsi_1h, rsi_4h, macd_4h,
		       system_prompt, input_prompt, raw_ai_response,
		       execution_success, execution_error
		FROM strategy_decision_history
		WHERE trader_id = ?
		ORDER BY decision_time DESC
		LIMIT ?
	`
	
	rows, err := d.db.Query(query, traderID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var histories []*StrategyDecisionHistory
	for rows.Next() {
		h := &StrategyDecisionHistory{}
		err := rows.Scan(
			&h.ID, &h.TraderID, &h.StrategyID, &h.DecisionTime, &h.Action, &h.Symbol,
			&h.CurrentPrice, &h.TargetPrice, &h.PositionSide, &h.PositionQty,
			&h.AmountPercent, &h.Reason, &h.RSI1H, &h.RSI4H, &h.MACD4H,
			&h.SystemPrompt, &h.InputPrompt, &h.RawAIResponse,
			&h.ExecutionSuccess, &h.ExecutionError,
		)
		if err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	
	return histories, nil
}

// GetStrategyDecisionsByStrategyID 获取特定策略的决策历史
func (d *Database) GetStrategyDecisionsByStrategyID(strategyID string, limit int) ([]*StrategyDecisionHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	
	query := `
		SELECT id, trader_id, strategy_id, decision_time, action, symbol,
		       current_price, target_price, position_side, position_qty,
		       amount_percent, reason, rsi_1h, rsi_4h, macd_4h,
		       system_prompt, input_prompt, raw_ai_response,
		       execution_success, execution_error
		FROM strategy_decision_history
		WHERE strategy_id = ?
		ORDER BY decision_time DESC
		LIMIT ?
	`
	
	rows, err := d.db.Query(query, strategyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var histories []*StrategyDecisionHistory
	for rows.Next() {
		h := &StrategyDecisionHistory{}
		err := rows.Scan(
			&h.ID, &h.TraderID, &h.StrategyID, &h.DecisionTime, &h.Action, &h.Symbol,
			&h.CurrentPrice, &h.TargetPrice, &h.PositionSide, &h.PositionQty,
			&h.AmountPercent, &h.Reason, &h.RSI1H, &h.RSI4H, &h.MACD4H,
			&h.SystemPrompt, &h.InputPrompt, &h.RawAIResponse,
			&h.ExecutionSuccess, &h.ExecutionError,
		)
		if err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	
	return histories, nil
}
