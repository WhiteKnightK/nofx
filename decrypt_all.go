package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"nofx/crypto"

	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║        完整数据库解密工具 - 包含所有用户                     ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 检查数据库文件
	dbPath := "config.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("❌ 数据库文件不存在: %s", dbPath)
	}

	fmt.Printf("📁 数据库文件: %s\n", dbPath)
	fileInfo, _ := os.Stat(dbPath)
	fmt.Printf("   文件大小: %.2f KB\n", float64(fileInfo.Size())/1024)
	fmt.Println()

	// 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("❌ 打开数据库失败: %v", err)
	}
	defer db.Close()

	// 初始化加密服务（同时支持RSA和AES）
	fmt.Println("🔐 初始化加密服务...")

	// 检查环境变量
	dataEncKey := os.Getenv("DATA_ENCRYPTION_KEY")
	if dataEncKey == "" {
		fmt.Println("⚠️  未设置 DATA_ENCRYPTION_KEY，将只能解密RSA加密的数据")
	} else {
		fmt.Printf("✅ 已加载 DATA_ENCRYPTION_KEY (长度: %d)\n", len(dataEncKey))
	}

	cryptoService, err := crypto.NewCryptoService("secrets/rsa_key")
	if err != nil {
		log.Fatalf("❌ 初始化加密服务失败: %v", err)
	}
	fmt.Println("✅ 加密服务初始化成功")
	fmt.Println()

	// 1. 读取所有用户
	fmt.Println("👥 读取所有用户...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	rows, err := db.Query(`SELECT id, email, role FROM users`)
	if err != nil {
		log.Fatalf("❌ 查询用户失败: %v", err)
	}

	userCount := 0
	userEmails := make(map[string]string) // userID -> email

	for rows.Next() {
		var userID, email, role string
		if err := rows.Scan(&userID, &email, &role); err != nil {
			log.Printf("⚠️  读取用户失败: %v", err)
			continue
		}
		userCount++
		userEmails[userID] = email
		fmt.Printf("%d. Email: %s\n", userCount, email)
		fmt.Printf("   用户ID: %s\n", userID)
		fmt.Printf("   角色: %s\n", role)
		fmt.Println()
	}
	rows.Close()

	if userCount == 0 {
		fmt.Println("⚠️  数据库中没有用户")
		return
	}

	fmt.Printf("✅ 找到 %d 个用户\n", userCount)
	fmt.Println()

	// 2. 读取所有交易所配置
	fmt.Println("🏦 读取所有交易所配置...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	rows2, err := db.Query(`
		SELECT id, user_id, name, type, enabled, api_key, secret_key, passphrase, testnet,
		       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key
		FROM exchanges
		ORDER BY user_id, id
	`)
	if err != nil {
		log.Fatalf("❌ 查询交易所配置失败: %v", err)
	}
	defer rows2.Close()

	exchangeCount := 0
	for rows2.Next() {
		var id, userID, name, typ string
		var enabled, testnet sql.NullBool
		var apiKey, secretKey, passphrase sql.NullString
		var hlWallet, asterUser, asterSigner, asterKey sql.NullString

		if err := rows2.Scan(&id, &userID, &name, &typ, &enabled, &apiKey, &secretKey, &passphrase, &testnet,
			&hlWallet, &asterUser, &asterSigner, &asterKey); err != nil {
			log.Printf("⚠️  读取交易所记录失败: %v", err)
			continue
		}

		exchangeCount++

		userEmail := userEmails[userID]
		if userEmail == "" {
			userEmail = "未知用户"
		}

		fmt.Printf("\n【交易所 #%d】\n", exchangeCount)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("名称: %s (%s)\n", name, typ)
		fmt.Printf("用户: %s\n", userEmail)
		fmt.Printf("用户ID: %s\n", userID)
		fmt.Printf("交易所ID: %s\n", id)
		fmt.Printf("启用状态: %v\n", enabled.Valid && enabled.Bool)

		if testnet.Valid {
			fmt.Printf("测试网: %v\n", testnet.Bool)
		}

		fmt.Println()

		// 解密API Key
		if apiKey.Valid && apiKey.String != "" {
			fmt.Printf("📌 API Key:\n")
			fmt.Printf("   原始数据: %s\n", truncate(apiKey.String, 70))

			decrypted := tryDecrypt(cryptoService, apiKey.String)
			if decrypted != apiKey.String {
				fmt.Printf("   ✅ 解密成功: %s\n", decrypted)
			} else {
				fmt.Printf("   ⚠️  可能是明文或解密失败: %s\n", decrypted)
			}
			fmt.Println()
		}

		// 解密Secret Key
		if secretKey.Valid && secretKey.String != "" {
			fmt.Printf("📌 Secret Key:\n")
			fmt.Printf("   原始数据: %s\n", truncate(secretKey.String, 70))

			decrypted := tryDecrypt(cryptoService, secretKey.String)
			if decrypted != secretKey.String {
				fmt.Printf("   ✅ 解密成功: %s\n", decrypted)
			} else {
				fmt.Printf("   ⚠️  可能是明文或解密失败: %s\n", decrypted)
			}
			fmt.Println()
		}

		// Passphrase
		if passphrase.Valid && passphrase.String != "" && passphrase.String != "0" {
			fmt.Printf("📌 Passphrase:\n")
			fmt.Printf("   原始数据: %s\n", truncate(passphrase.String, 70))

			decrypted := tryDecrypt(cryptoService, passphrase.String)
			if decrypted != passphrase.String {
				fmt.Printf("   ✅ 解密成功: %s\n", decrypted)
			} else {
				fmt.Printf("   ⚠️  可能是明文: %s\n", decrypted)
			}
			fmt.Println()
		}

		// Hyperliquid钱包地址
		if hlWallet.Valid && hlWallet.String != "" {
			fmt.Printf("📌 Hyperliquid Wallet: %s\n\n", hlWallet.String)
		}

		// Aster配置
		if asterUser.Valid && asterUser.String != "" {
			fmt.Printf("📌 Aster User: %s\n", asterUser.String)
		}
		if asterSigner.Valid && asterSigner.String != "" {
			fmt.Printf("📌 Aster Signer: %s\n", asterSigner.String)
		}
		if asterKey.Valid && asterKey.String != "" {
			fmt.Printf("📌 Aster Private Key:\n")
			fmt.Printf("   原始数据: %s\n", truncate(asterKey.String, 70))

			decrypted := tryDecrypt(cryptoService, asterKey.String)
			if decrypted != asterKey.String {
				fmt.Printf("   ✅ 解密成功: %s\n", decrypted)
			} else {
				fmt.Printf("   ⚠️  可能是明文: %s\n", decrypted)
			}
			fmt.Println()
		}
	}

	if exchangeCount == 0 {
		fmt.Println("⚠️  数据库中没有交易所配置")
	} else {
		fmt.Printf("\n✅ 共找到 %d 个交易所配置\n", exchangeCount)
	}

	// 3. 读取AI模型配置
	fmt.Println()
	fmt.Println("🤖 读取所有AI模型配置...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	rows3, err := db.Query(`
		SELECT id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name
		FROM ai_models
		WHERE user_id != 'default'
		ORDER BY user_id, id
	`)
	if err != nil {
		log.Fatalf("❌ 查询AI模型配置失败: %v", err)
	}
	defer rows3.Close()

	aiCount := 0
	for rows3.Next() {
		var id, userID, name, provider string
		var enabled bool
		var apiKey, customURL, customModel sql.NullString

		if err := rows3.Scan(&id, &userID, &name, &provider, &enabled, &apiKey, &customURL, &customModel); err != nil {
			log.Printf("⚠️  读取AI模型记录失败: %v", err)
			continue
		}

		aiCount++

		userEmail := userEmails[userID]
		if userEmail == "" {
			userEmail = "未知用户"
		}

		fmt.Printf("\n【AI模型 #%d】\n", aiCount)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("名称: %s (%s)\n", name, provider)
		fmt.Printf("用户: %s\n", userEmail)
		fmt.Printf("用户ID: %s\n", userID)
		fmt.Printf("AI模型ID: %s\n", id)
		fmt.Printf("启用状态: %v\n", enabled)
		fmt.Println()

		if apiKey.Valid && apiKey.String != "" {
			fmt.Printf("📌 API Key:\n")
			fmt.Printf("   原始数据: %s\n", truncate(apiKey.String, 70))

			decrypted := tryDecrypt(cryptoService, apiKey.String)
			if decrypted != apiKey.String {
				fmt.Printf("   ✅ 解密成功: %s\n", decrypted)
			} else {
				fmt.Printf("   ⚠️  可能是明文或解密失败: %s\n", decrypted)
			}
			fmt.Println()
		}

		if customURL.Valid && customURL.String != "" {
			fmt.Printf("📌 自定义API URL: %s\n", customURL.String)
		}
		if customModel.Valid && customModel.String != "" {
			fmt.Printf("📌 自定义模型名: %s\n", customModel.String)
		}
	}

	if aiCount == 0 {
		fmt.Println("⚠️  数据库中没有AI模型配置（除了默认的）")
	} else {
		fmt.Printf("\n✅ 共找到 %d 个AI模型配置\n", aiCount)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ 解密完成！")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// tryDecrypt 尝试解密数据
func tryDecrypt(cs *crypto.CryptoService, encrypted string) string {
	if encrypted == "" || encrypted == "0" {
		return encrypted
	}

	// 检查是否是加密格式
	if cs.IsEncryptedStorageValue(encrypted) {
		decrypted, err := cs.DecryptFromStorage(encrypted)
		if err != nil {
			return fmt.Sprintf("[解密失败: %v]", err)
		}
		return decrypted
	}

	// 不是加密格式，可能是明文
	return encrypted
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
