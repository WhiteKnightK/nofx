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
	fmt.Println("║           加密数据解密工具                                   ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 0. 加载环境变量（尝试从.env文件加载）
	if _, err := os.Stat(".env"); err == nil {
		// 简单读取.env文件并设置环境变量
		data, _ := os.ReadFile(".env")
		lines := string(data)
		for _, line := range []string{} {
			_ = line // 占位
		}
		_ = lines
		fmt.Println("📋 已加载 .env 文件")
	}

	// 1. 检查数据库文件
	dbPath := "config.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("❌ 数据库文件不存在: %s", dbPath)
	}

	fmt.Printf("📁 数据库文件: %s\n", dbPath)

	// 检查文件大小
	fileInfo, _ := os.Stat(dbPath)
	fmt.Printf("   文件大小: %.2f KB\n", float64(fileInfo.Size())/1024)
	fmt.Println()

	// 2. 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("❌ 打开数据库失败: %v", err)
	}
	defer db.Close()

	// 3. 初始化加密服务
	fmt.Println("🔐 初始化加密服务...")

	// 检查DATA_ENCRYPTION_KEY环境变量
	if os.Getenv("DATA_ENCRYPTION_KEY") == "" {
		fmt.Println("⚠️  未设置 DATA_ENCRYPTION_KEY 环境变量")
		fmt.Println()
		fmt.Println("请使用以下命令之一运行：")
		fmt.Println("  方式1: DATA_ENCRYPTION_KEY=你的密钥 go run decrypt_tool.go")
		fmt.Println("  方式2: 在 .env 文件中设置 DATA_ENCRYPTION_KEY=你的密钥")
		fmt.Println()
		os.Exit(1)
	}

	cryptoService, err := crypto.NewCryptoService("secrets/rsa_key")
	if err != nil {
		log.Fatalf("❌ 初始化加密服务失败: %v", err)
	}
	fmt.Println("✅ 加密服务初始化成功")
	fmt.Println()

	// 4. 读取并解密交易所配置
	fmt.Println("🏦 读取交易所配置...")
	rows, err := db.Query(`
		SELECT id, user_id, name, api_key, secret_key, passphrase, 
		       hyperliquid_wallet_addr, aster_private_key
		FROM exchanges
		WHERE (api_key != '' AND api_key IS NOT NULL)
		   OR (secret_key != '' AND secret_key IS NOT NULL)
	`)
	if err != nil {
		log.Fatalf("❌ 查询交易所配置失败: %v", err)
	}
	defer rows.Close()

	exchangeCount := 0
	for rows.Next() {
		var id, userID, name string
		var apiKey, secretKey, passphrase, hlWallet, asterKey sql.NullString

		if err := rows.Scan(&id, &userID, &name, &apiKey, &secretKey, &passphrase, &hlWallet, &asterKey); err != nil {
			log.Printf("⚠️  读取记录失败: %v", err)
			continue
		}

		exchangeCount++
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("交易所: %s (ID: %s, User: %s)\n", name, id, userID)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// 解密API Key
		if apiKey.Valid && apiKey.String != "" {
			fmt.Printf("📌 API Key (加密): %s\n", truncate(apiKey.String, 50))

			if cryptoService.IsEncryptedStorageValue(apiKey.String) {
				decrypted, err := cryptoService.DecryptFromStorage(apiKey.String)
				if err != nil {
					fmt.Printf("   ❌ 解密失败: %v\n", err)
				} else {
					fmt.Printf("   ✅ API Key (明文): %s\n", decrypted)
				}
			} else {
				fmt.Printf("   ℹ️  API Key (明文): %s\n", apiKey.String)
			}
		}

		// 解密Secret Key
		if secretKey.Valid && secretKey.String != "" {
			fmt.Printf("📌 Secret Key (加密): %s\n", truncate(secretKey.String, 50))

			if cryptoService.IsEncryptedStorageValue(secretKey.String) {
				decrypted, err := cryptoService.DecryptFromStorage(secretKey.String)
				if err != nil {
					fmt.Printf("   ❌ 解密失败: %v\n", err)
				} else {
					fmt.Printf("   ✅ Secret Key (明文): %s\n", decrypted)
				}
			} else {
				fmt.Printf("   ℹ️  Secret Key (明文): %s\n", secretKey.String)
			}
		}

		// 解密Passphrase
		if passphrase.Valid && passphrase.String != "" {
			fmt.Printf("📌 Passphrase (加密): %s\n", truncate(passphrase.String, 50))

			if cryptoService.IsEncryptedStorageValue(passphrase.String) {
				decrypted, err := cryptoService.DecryptFromStorage(passphrase.String)
				if err != nil {
					fmt.Printf("   ❌ 解密失败: %v\n", err)
				} else {
					fmt.Printf("   ✅ Passphrase (明文): %s\n", decrypted)
				}
			} else {
				fmt.Printf("   ℹ️  Passphrase (明文): %s\n", passphrase.String)
			}
		}

		// Hyperliquid Wallet
		if hlWallet.Valid && hlWallet.String != "" {
			fmt.Printf("📌 Hyperliquid Wallet: %s\n", hlWallet.String)
		}

		// Aster Private Key
		if asterKey.Valid && asterKey.String != "" {
			fmt.Printf("📌 Aster Private Key (加密): %s\n", truncate(asterKey.String, 50))

			if cryptoService.IsEncryptedStorageValue(asterKey.String) {
				decrypted, err := cryptoService.DecryptFromStorage(asterKey.String)
				if err != nil {
					fmt.Printf("   ❌ 解密失败: %v\n", err)
				} else {
					fmt.Printf("   ✅ Aster Private Key (明文): %s\n", decrypted)
				}
			} else {
				fmt.Printf("   ℹ️  Aster Private Key (明文): %s\n", asterKey.String)
			}
		}

		fmt.Println()
	}

	if exchangeCount == 0 {
		fmt.Println("⚠️  数据库中没有找到交易所配置")
	}

	// 5. 读取并解密AI模型配置
	fmt.Println("🤖 读取AI模型配置...")
	rows2, err := db.Query(`
		SELECT id, user_id, name, api_key
		FROM ai_models
		WHERE api_key != '' AND api_key IS NOT NULL
	`)
	if err != nil {
		log.Fatalf("❌ 查询AI模型配置失败: %v", err)
	}
	defer rows2.Close()

	aiCount := 0
	for rows2.Next() {
		var id, userID, name string
		var apiKey sql.NullString

		if err := rows2.Scan(&id, &userID, &name, &apiKey); err != nil {
			log.Printf("⚠️  读取AI模型记录失败: %v", err)
			continue
		}

		aiCount++
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("AI模型: %s (ID: %s, User: %s)\n", name, id, userID)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		if apiKey.Valid && apiKey.String != "" {
			fmt.Printf("📌 API Key (加密): %s\n", truncate(apiKey.String, 50))

			if cryptoService.IsEncryptedStorageValue(apiKey.String) {
				decrypted, err := cryptoService.DecryptFromStorage(apiKey.String)
				if err != nil {
					fmt.Printf("   ❌ 解密失败: %v\n", err)
				} else {
					fmt.Printf("   ✅ API Key (明文): %s\n", decrypted)
				}
			} else {
				fmt.Printf("   ℹ️  API Key (明文): %s\n", apiKey.String)
			}
		}

		fmt.Println()
	}

	if aiCount == 0 {
		fmt.Println("⚠️  数据库中没有找到AI模型配置")
	}

	fmt.Println()
	fmt.Println("✅ 解密完成！")
	fmt.Println()
	fmt.Println("💡 提示:")
	fmt.Println("   1. 请记录上面显示的明文密钥")
	fmt.Println("   2. 在Web界面重新输入这些密钥")
	fmt.Println("   3. 或者检查secrets/rsa_key是否与服务器上的一致")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
