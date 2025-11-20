package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// NewMySQLDatabase 创建MySQL数据库连接
// dsn格式: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
func NewMySQLDatabase(dsn string) (*Database, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开MySQL数据库失败: %w", err)
	}

	// 设置MySQL连接池参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// 测试数据库连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("MySQL数据库连接测试失败: %w", err)
	}

	log.Printf("✅ MySQL数据库连接成功")

	database := &Database{db: db, isMySQL: true}
	if err := database.createMySQLTables(); err != nil {
		return nil, fmt.Errorf("创建MySQL表失败: %w", err)
	}

	// 执行数据库迁移（必须在创建表之后，初始化数据之前）
	if err := database.RunMigrations(); err != nil {
		return nil, fmt.Errorf("执行数据库迁移失败: %w", err)
	}

	if err := database.initMySQLDefaultData(); err != nil {
		return nil, fmt.Errorf("初始化MySQL默认数据失败: %w", err)
	}

	return database, nil
}

// createMySQLTables 创建MySQL数据库表
func (d *Database) createMySQLTables() error {
	queries := []string{
		// 用户表 (需要先创建，因为其他表有外键引用)
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified TINYINT(1) DEFAULT 0,
			role VARCHAR(50) DEFAULT 'user',
			trader_id VARCHAR(255) DEFAULT NULL,
			category VARCHAR(255) DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_email (email)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// AI模型配置表
		`CREATE TABLE IF NOT EXISTS ai_models (
			id VARCHAR(255) NOT NULL,
			user_id VARCHAR(255) NOT NULL DEFAULT 'default',
			name VARCHAR(255) NOT NULL,
			provider VARCHAR(100) NOT NULL,
			enabled TINYINT(1) DEFAULT 0,
			api_key TEXT DEFAULT NULL,
			custom_api_url TEXT DEFAULT NULL,
			custom_model_name VARCHAR(255) DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// 交易所配置表
		`CREATE TABLE IF NOT EXISTS exchanges (
			id VARCHAR(255) NOT NULL,
			user_id VARCHAR(255) NOT NULL DEFAULT 'default',
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL COMMENT 'cex or dex',
			enabled TINYINT(1) DEFAULT 0,
			api_key TEXT DEFAULT NULL,
			secret_key TEXT DEFAULT NULL,
			passphrase TEXT DEFAULT NULL,
			testnet TINYINT(1) DEFAULT 0,
			hyperliquid_wallet_addr VARCHAR(255) DEFAULT NULL,
			aster_user VARCHAR(255) DEFAULT NULL,
			aster_signer VARCHAR(255) DEFAULT NULL,
			aster_private_key TEXT DEFAULT NULL,
			provider VARCHAR(100) DEFAULT '',
			label VARCHAR(255) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// 交易员配置表
		`CREATE TABLE IF NOT EXISTS traders (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL DEFAULT 'default',
			name VARCHAR(255) NOT NULL,
			ai_model_id VARCHAR(255) NOT NULL,
			exchange_id VARCHAR(255) NOT NULL,
			initial_balance DOUBLE NOT NULL,
			scan_interval_minutes INT DEFAULT 3,
			is_running TINYINT(1) DEFAULT 0,
			btc_eth_leverage INT DEFAULT 5,
			altcoin_leverage INT DEFAULT 5,
			trading_symbols TEXT DEFAULT NULL,
			use_coin_pool TINYINT(1) DEFAULT 0,
			use_oi_top TINYINT(1) DEFAULT 0,
			custom_prompt TEXT DEFAULT NULL,
			override_base_prompt TINYINT(1) DEFAULT 0,
			system_prompt_template VARCHAR(100) DEFAULT 'default',
			is_cross_margin TINYINT(1) DEFAULT 1,
			category VARCHAR(255) DEFAULT NULL,
			trader_account_id VARCHAR(255) DEFAULT NULL,
			owner_user_id VARCHAR(255) DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_user_id (user_id),
			INDEX idx_category (category),
			INDEX idx_owner_user_id (owner_user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// 用户信号源配置表
		`CREATE TABLE IF NOT EXISTS user_signal_sources (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			coin_pool_url TEXT DEFAULT NULL,
			oi_top_url TEXT DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE KEY unique_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_config (
			` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// 内测码表
		`CREATE TABLE IF NOT EXISTS beta_codes (
			code VARCHAR(255) PRIMARY KEY,
			used TINYINT(1) DEFAULT 0,
			used_by VARCHAR(255) DEFAULT NULL,
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// 分类表（多用户观测系统）
		`CREATE TABLE IF NOT EXISTS categories (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			owner_user_id VARCHAR(255) NOT NULL,
			description TEXT DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE KEY unique_owner_name (owner_user_id, name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// 小组组长分类关联表（多用户观测系统）
		`CREATE TABLE IF NOT EXISTS group_leader_categories (
			id INT AUTO_INCREMENT PRIMARY KEY,
			group_leader_id VARCHAR(255) NOT NULL,
			category VARCHAR(255) NOT NULL,
			owner_user_id VARCHAR(255) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (group_leader_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE KEY unique_leader_category (group_leader_id, category),
			INDEX idx_group_leader (group_leader_id),
			INDEX idx_category (category),
			INDEX idx_owner_user (owner_user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败: %w\nSQL: %s", err, query)
		}
	}

	log.Printf("✅ MySQL数据库表创建成功")
	return nil
}

// initMySQLDefaultData 初始化MySQL默认数据
func (d *Database) initMySQLDefaultData() error {
	// 首先确保 default 用户存在（如果不存在则创建）
	_, err := d.db.Exec(`
		INSERT IGNORE INTO users (id, email, password_hash, role) 
		VALUES ('default', 'default@system.local', '', 'system')
	`)
	if err != nil {
		log.Printf("⚠️  创建 default 用户失败（可能已存在）: %v", err)
	}

	// 初始化AI模型（使用default用户）
	aiModels := []struct {
		id, name, provider string
	}{
		{"deepseek", "DeepSeek", "deepseek"},
		{"qwen", "Qwen", "qwen"},
	}

	for _, model := range aiModels {
		_, err := d.db.Exec(`
			INSERT IGNORE INTO ai_models (id, user_id, name, provider, enabled) 
			VALUES (?, 'default', ?, ?, 0)
		`, model.id, model.name, model.provider)
		if err != nil {
			log.Printf("⚠️  初始化AI模型 %s 失败: %v", model.id, err)
			// 不返回错误，继续初始化其他模型
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
		_, err := d.db.Exec(`
			INSERT IGNORE INTO exchanges (id, user_id, name, type, enabled, provider, label) 
			VALUES (?, 'default', ?, ?, 0, ?, ?)
		`, exchange.id, exchange.name, exchange.typ, exchange.id, exchange.name)
		if err != nil {
			log.Printf("⚠️  初始化交易所 %s 失败: %v", exchange.id, err)
			// 不返回错误，继续初始化其他交易所
		}
	}

	// 初始化系统配置
	systemConfigs := map[string]string{
		"beta_mode":            "false",
		"api_server_port":      "8080",
		"use_default_coins":    "true",
		"default_coins":        `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`,
		"max_daily_loss":       "10.0",
		"max_drawdown":         "20.0",
		"stop_trading_minutes": "60",
		"btc_eth_leverage":     "5",
		"altcoin_leverage":     "5",
		"jwt_secret":           "",
	}

	for key, value := range systemConfigs {
		_, err := d.db.Exec(`
			INSERT IGNORE INTO system_config (`+"`key`"+`, value) 
			VALUES (?, ?)
		`, key, value)
		if err != nil {
			return fmt.Errorf("初始化系统配置失败: %w", err)
		}
	}

	log.Printf("✅ MySQL默认数据初始化成功")
	return nil
}

// GetDatabaseDSNFromEnv 从环境变量获取MySQL DSN连接字符串
// 优先级：单独的环境变量 > DATABASE_URL
func GetDatabaseDSNFromEnv() string {
	// 优先使用单独的环境变量（推荐方式）
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")

	// 如果设置了单独的环境变量，优先使用它们
	if host != "" && user != "" && password != "" {
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "3306"
		}
		dbname := os.Getenv("DB_NAME")
		if dbname == "" {
			dbname = "nofx"
		}

		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port, dbname)
	}

	// 备选：使用完整的DATABASE_URL
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		// 如果DATABASE_URL不包含@tcp(，说明可能是简化的格式，需要补充
		if !strings.Contains(dsn, "@tcp(") {
			// 尝试解析格式: user:password@host:port/dbname
			// 或者格式: user:password@host/dbname
			parts := strings.Split(dsn, "@")
			if len(parts) == 2 {
				userPass := parts[0]
				hostDb := parts[1]
				hostDbParts := strings.Split(hostDb, "/")
				if len(hostDbParts) == 2 {
					hostPort := hostDbParts[0]
					dbname := hostDbParts[1]
					// 检查是否有端口
					if !strings.Contains(hostPort, ":") {
						hostPort = hostPort + ":3306"
					}
					dsn = fmt.Sprintf("%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
						userPass, hostPort, dbname)
				}
			}
		}
		return dsn
	}

	// 默认值（不应该到达这里，因为main.go会检查环境变量）
	return "root:password@tcp(localhost:3306)/nofx?charset=utf8mb4&parseTime=True&loc=Local"
}
