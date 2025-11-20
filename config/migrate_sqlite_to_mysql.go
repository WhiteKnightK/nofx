package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

// MigrateSQLiteToMySQL 从SQLite迁移数据到MySQL
func MigrateSQLiteToMySQL(mysqlDB *Database, sqlitePath string) error {
	// 检查SQLite文件是否存在
	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		log.Printf("📋 SQLite数据库文件不存在，跳过数据迁移")
		return nil
	}

	log.Printf("🔄 检测到SQLite数据库文件: %s", sqlitePath)

	// 打开SQLite数据库
	sqliteDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return fmt.Errorf("打开SQLite数据库失败: %w", err)
	}
	defer sqliteDB.Close()

	// 测试连接
	if err := sqliteDB.Ping(); err != nil {
		return fmt.Errorf("SQLite数据库连接测试失败: %w", err)
	}

	// 检查SQLite是否有数据需要迁移
	var userCount, traderCount int
	sqliteDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	sqliteDB.QueryRow("SELECT COUNT(*) FROM traders").Scan(&traderCount)

	if userCount == 0 && traderCount == 0 {
		log.Printf("✅ SQLite数据库为空，无需迁移")
		return nil
	}

	log.Printf("📊 SQLite数据统计: %d 个用户, %d 个交易员", userCount, traderCount)
	log.Printf("🔄 开始数据迁移...")

	// 开始迁移
	migrated := 0

	// 1. 迁移用户数据
	if count, err := migrateUsers(sqliteDB, mysqlDB); err != nil {
		log.Printf("⚠️  迁移用户数据失败: %v", err)
	} else {
		migrated += count
		log.Printf("✓ 迁移了 %d 个用户", count)
	}

	// 2. 迁移AI模型配置
	if count, err := migrateAIModels(sqliteDB, mysqlDB); err != nil {
		log.Printf("⚠️  迁移AI模型失败: %v", err)
	} else {
		migrated += count
		log.Printf("✓ 迁移了 %d 个AI模型配置", count)
	}

	// 3. 迁移交易所配置
	if count, err := migrateExchanges(sqliteDB, mysqlDB); err != nil {
		log.Printf("⚠️  迁移交易所配置失败: %v", err)
	} else {
		migrated += count
		log.Printf("✓ 迁移了 %d 个交易所配置", count)
	}

	// 4. 迁移交易员配置
	if count, err := migrateTraders(sqliteDB, mysqlDB); err != nil {
		log.Printf("⚠️  迁移交易员配置失败: %v", err)
	} else {
		migrated += count
		log.Printf("✓ 迁移了 %d 个交易员配置", count)
	}

	// 5. 迁移系统配置
	if count, err := migrateSystemConfig(sqliteDB, mysqlDB); err != nil {
		log.Printf("⚠️  迁移系统配置失败: %v", err)
	} else {
		migrated += count
		log.Printf("✓ 迁移了 %d 个系统配置项", count)
	}

	// 6. 迁移用户信号源配置
	if count, err := migrateUserSignalSources(sqliteDB, mysqlDB); err != nil {
		log.Printf("⚠️  迁移用户信号源失败: %v", err)
	} else if count > 0 {
		migrated += count
		log.Printf("✓ 迁移了 %d 个用户信号源配置", count)
	}

	log.Printf("✅ 数据迁移完成！共迁移 %d 条记录", migrated)
	log.Printf("💡 建议: 迁移成功后，可以备份SQLite文件并删除，避免混淆")

	return nil
}

// migrateUsers 迁移用户数据
func migrateUsers(sqliteDB *sql.DB, mysqlDB *Database) (int, error) {
	rows, err := sqliteDB.Query(`
		SELECT id, email, password_hash, otp_secret, otp_verified, role, trader_id, category, created_at, updated_at
		FROM users
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, email, passwordHash string
		var otpSecret, role, traderID, category sql.NullString
		var otpVerified sql.NullBool
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&id, &email, &passwordHash, &otpSecret, &otpVerified, &role, &traderID, &category, &createdAt, &updatedAt); err != nil {
			log.Printf("  ⚠️  读取用户数据失败: %v", err)
			continue
		}

		// 检查用户是否已存在
		existingUser, _ := mysqlDB.GetUserByEmail(email)
		if existingUser != nil {
			log.Printf("  ⚠️  用户已存在，跳过: %s", email)
			continue
		}

		// 插入用户
		_, err := mysqlDB.db.Exec(`
			INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, role, trader_id, category, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, email, passwordHash, otpSecret, otpVerified, role, traderID, category, createdAt, updatedAt)

		if err != nil {
			log.Printf("  ⚠️  插入用户失败 (%s): %v", email, err)
			continue
		}

		count++
	}

	return count, nil
}

// migrateAIModels 迁移AI模型配置
func migrateAIModels(sqliteDB *sql.DB, mysqlDB *Database) (int, error) {
	rows, err := sqliteDB.Query(`
		SELECT id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at
		FROM ai_models
		WHERE user_id != 'default'
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, userID, name, provider string
		var enabled bool
		var apiKey, customAPIURL, customModelName sql.NullString
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&id, &userID, &name, &provider, &enabled, &apiKey, &customAPIURL, &customModelName, &createdAt, &updatedAt); err != nil {
			log.Printf("  ⚠️  读取AI模型数据失败: %v", err)
			continue
		}

		// 插入或更新AI模型配置
		_, err := mysqlDB.db.Exec(`
			INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				enabled = VALUES(enabled),
				api_key = VALUES(api_key),
				custom_api_url = VALUES(custom_api_url),
				custom_model_name = VALUES(custom_model_name),
				updated_at = VALUES(updated_at)
		`, id, userID, name, provider, enabled, apiKey, customAPIURL, customModelName, createdAt, updatedAt)

		if err != nil {
			log.Printf("  ⚠️  插入AI模型失败 (%s): %v", id, err)
			continue
		}

		count++
	}

	return count, nil
}

// migrateExchanges 迁移交易所配置
func migrateExchanges(sqliteDB *sql.DB, mysqlDB *Database) (int, error) {
	rows, err := sqliteDB.Query(`
		SELECT id, user_id, name, type, enabled, api_key, secret_key, passphrase, testnet,
		       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at
		FROM exchanges
		WHERE user_id != 'default'
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, userID, name, typ string
		var enabled, testnet bool
		var apiKey, secretKey, passphrase sql.NullString
		var hlWalletAddr, asterUser, asterSigner, asterPrivateKey sql.NullString
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&id, &userID, &name, &typ, &enabled, &apiKey, &secretKey, &passphrase, &testnet,
			&hlWalletAddr, &asterUser, &asterSigner, &asterPrivateKey, &createdAt, &updatedAt); err != nil {
			log.Printf("  ⚠️  读取交易所数据失败: %v", err)
			continue
		}

		// 插入或更新交易所配置
		_, err := mysqlDB.db.Exec(`
			INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, passphrase, testnet,
			                      hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				enabled = VALUES(enabled),
				api_key = VALUES(api_key),
				secret_key = VALUES(secret_key),
				passphrase = VALUES(passphrase),
				testnet = VALUES(testnet),
				hyperliquid_wallet_addr = VALUES(hyperliquid_wallet_addr),
				aster_user = VALUES(aster_user),
				aster_signer = VALUES(aster_signer),
				aster_private_key = VALUES(aster_private_key),
				updated_at = VALUES(updated_at)
		`, id, userID, name, typ, enabled, apiKey, secretKey, passphrase, testnet,
			hlWalletAddr, asterUser, asterSigner, asterPrivateKey, createdAt, updatedAt)

		if err != nil {
			log.Printf("  ⚠️  插入交易所失败 (%s): %v", id, err)
			continue
		}

		count++
	}

	return count, nil
}

// migrateTraders 迁移交易员配置
func migrateTraders(sqliteDB *sql.DB, mysqlDB *Database) (int, error) {
	rows, err := sqliteDB.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top,
		       custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin,
		       category, trader_account_id, owner_user_id, created_at, updated_at
		FROM traders
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, userID, name, aiModelID, exchangeID string
		var initialBalance float64
		var scanIntervalMinutes, btcEthLeverage, altcoinLeverage int
		var isRunning, useCoinPool, useOITop, overrideBasePrompt, isCrossMargin bool
		var tradingSymbols, customPrompt, systemPromptTemplate sql.NullString
		var category, traderAccountID, ownerUserID sql.NullString
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&id, &userID, &name, &aiModelID, &exchangeID, &initialBalance, &scanIntervalMinutes, &isRunning,
			&btcEthLeverage, &altcoinLeverage, &tradingSymbols, &useCoinPool, &useOITop,
			&customPrompt, &overrideBasePrompt, &systemPromptTemplate, &isCrossMargin,
			&category, &traderAccountID, &ownerUserID, &createdAt, &updatedAt); err != nil {
			log.Printf("  ⚠️  读取交易员数据失败: %v", err)
			continue
		}

		// 插入交易员配置
		_, err := mysqlDB.db.Exec(`
			INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
			                    btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top,
			                    custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin,
			                    category, trader_account_id, owner_user_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				ai_model_id = VALUES(ai_model_id),
				exchange_id = VALUES(exchange_id),
				initial_balance = VALUES(initial_balance),
				scan_interval_minutes = VALUES(scan_interval_minutes),
				btc_eth_leverage = VALUES(btc_eth_leverage),
				altcoin_leverage = VALUES(altcoin_leverage),
				trading_symbols = VALUES(trading_symbols),
				use_coin_pool = VALUES(use_coin_pool),
				use_oi_top = VALUES(use_oi_top),
				custom_prompt = VALUES(custom_prompt),
				override_base_prompt = VALUES(override_base_prompt),
				system_prompt_template = VALUES(system_prompt_template),
				is_cross_margin = VALUES(is_cross_margin),
				category = VALUES(category),
				trader_account_id = VALUES(trader_account_id),
				owner_user_id = VALUES(owner_user_id),
				updated_at = VALUES(updated_at)
		`, id, userID, name, aiModelID, exchangeID, initialBalance, scanIntervalMinutes, isRunning,
			btcEthLeverage, altcoinLeverage, tradingSymbols, useCoinPool, useOITop,
			customPrompt, overrideBasePrompt, systemPromptTemplate, isCrossMargin,
			category, traderAccountID, ownerUserID, createdAt, updatedAt)

		if err != nil {
			log.Printf("  ⚠️  插入交易员失败 (%s): %v", id, err)
			continue
		}

		count++
	}

	return count, nil
}

// migrateSystemConfig 迁移系统配置
func migrateSystemConfig(sqliteDB *sql.DB, mysqlDB *Database) (int, error) {
	rows, err := sqliteDB.Query(`
		SELECT key, value FROM system_config
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			log.Printf("  ⚠️  读取系统配置失败: %v", err)
			continue
		}

		// 使用REPLACE INTO来更新或插入
		_, err := mysqlDB.db.Exec(`
			INSERT INTO system_config (`+"`key`"+`, value, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON DUPLICATE KEY UPDATE
				value = VALUES(value),
				updated_at = CURRENT_TIMESTAMP
		`, key, value)

		if err != nil {
			log.Printf("  ⚠️  插入系统配置失败 (%s): %v", key, err)
			continue
		}

		count++
	}

	return count, nil
}

// migrateUserSignalSources 迁移用户信号源配置
func migrateUserSignalSources(sqliteDB *sql.DB, mysqlDB *Database) (int, error) {
	rows, err := sqliteDB.Query(`
		SELECT id, user_id, coin_pool_url, oi_top_url, created_at, updated_at
		FROM user_signal_sources
	`)
	if err != nil {
		// 表可能不存在
		return 0, nil
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		var userID string
		var coinPoolURL, oiTopURL sql.NullString
		var createdAt, updatedAt sql.NullString

		if err := rows.Scan(&id, &userID, &coinPoolURL, &oiTopURL, &createdAt, &updatedAt); err != nil {
			log.Printf("  ⚠️  读取用户信号源失败: %v", err)
			continue
		}

		// 插入或更新
		_, err := mysqlDB.db.Exec(`
			INSERT INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				coin_pool_url = VALUES(coin_pool_url),
				oi_top_url = VALUES(oi_top_url),
				updated_at = VALUES(updated_at)
		`, userID, coinPoolURL, oiTopURL, createdAt, updatedAt)

		if err != nil {
			log.Printf("  ⚠️  插入用户信号源失败 (%s): %v", userID, err)
			continue
		}

		count++
	}

	return count, nil
}
