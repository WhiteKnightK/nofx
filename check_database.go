package main

import (
	"fmt"
	"log"
	"nofx/config"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           数据库类型检查工具                                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 检查环境变量
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	databaseURL := os.Getenv("DATABASE_URL")

	fmt.Println("📋 环境变量检查:")
	fmt.Printf("  DB_HOST: %s\n", ifEmpty(dbHost, "未设置"))
	fmt.Printf("  DB_USER: %s\n", ifEmpty(dbUser, "未设置"))
	if dbPassword != "" {
		fmt.Printf("  DB_PASSWORD: 已设置（已隐藏）\n")
	} else {
		fmt.Printf("  DB_PASSWORD: 未设置\n")
	}
	fmt.Printf("  DATABASE_URL: %s\n", ifEmpty(databaseURL, "未设置"))
	fmt.Println()

	// 判断使用哪个数据库
	if (dbHost != "" && dbUser != "" && dbPassword != "") || databaseURL != "" {
		fmt.Println("✅ 判断结果: 系统将使用 MySQL 数据库")
		fmt.Println()

		// 尝试连接MySQL并检查数据
		mysqlDSN := config.GetDatabaseDSNFromEnv()
		fmt.Printf("📡 MySQL DSN: %s\n", maskPassword(mysqlDSN))
		fmt.Println()

		db, err := config.NewMySQLDatabase(mysqlDSN)
		if err != nil {
			log.Fatalf("❌ MySQL连接失败: %v", err)
		}
		defer db.Close()

		// 检查表和数据
		checkMySQLData(db)

	} else {
		fmt.Println("✅ 判断结果: 系统将使用 SQLite 数据库")
		fmt.Println()

		// 检查SQLite文件
		dbPath := "config.db"
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			fmt.Printf("⚠️  SQLite数据库文件不存在: %s\n", dbPath)
			return
		}

		fmt.Printf("📁 SQLite数据库文件: %s\n", dbPath)
		fileInfo, _ := os.Stat(dbPath)
		fmt.Printf("   文件大小: %.2f KB\n", float64(fileInfo.Size())/1024)
		fmt.Println()

		// 连接SQLite并检查数据
		db, err := config.NewDatabase(dbPath)
		if err != nil {
			log.Fatalf("❌ SQLite连接失败: %v", err)
		}
		defer db.Close()

		checkSQLiteData(db)
	}
}

func ifEmpty(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}

func maskPassword(dsn string) string {
	// 简单隐藏密码: user:password@tcp(...) -> user:***@tcp(...)
	// 这里只是简单示例，实际应该更安全地处理
	if len(dsn) > 50 {
		return dsn[:20] + "***" + dsn[len(dsn)-30:]
	}
	return dsn
}

func checkMySQLData(db *config.Database) {
	fmt.Println("📊 MySQL数据库数据统计:")

	// 检查AI模型数量 - 通过GetAIModels方法
	aiModels, err := db.GetAIModels("default")
	if err != nil {
		fmt.Printf("  ⚠️  无法查询AI模型: %v\n", err)
	} else {
		fmt.Printf("  🤖 AI模型数量: %d\n", len(aiModels))
		if len(aiModels) > 0 {
			fmt.Printf("     - %v\n", getModelNames(aiModels))
		}
	}

	// 检查交易所数量 - 通过GetExchanges方法
	exchanges, err := db.GetExchanges("default")
	if err != nil {
		fmt.Printf("  ⚠️  无法查询交易所: %v\n", err)
	} else {
		fmt.Printf("  🏦 交易所数量: %d\n", len(exchanges))
		if len(exchanges) > 0 {
			fmt.Printf("     - %v\n", getExchangeNames(exchanges))
		}
	}

	// 检查交易员数量 - 通过GetTraders方法
	traders, err := db.GetTraders("default")
	if err != nil {
		fmt.Printf("  ⚠️  无法查询交易员: %v\n", err)
	} else {
		fmt.Printf("  📈 交易员数量: %d\n", len(traders))
		if len(traders) > 0 {
			fmt.Printf("     - %v\n", getTraderNames(traders))
		}
	}

	// 检查系统配置 - 尝试获取几个配置项
	betaMode, _ := db.GetSystemConfig("beta_mode")
	apiPort, _ := db.GetSystemConfig("api_server_port")
	fmt.Printf("  ⚙️  系统配置: beta_mode=%s, api_port=%s\n", betaMode, apiPort)

	fmt.Println()

	if len(traders) == 0 {
		fmt.Println("⚠️  MySQL数据库中没有交易员数据")
		fmt.Println("   📌 建议:")
		fmt.Println("   1. 通过Web界面创建新的交易员配置")
		fmt.Println("   2. 或者从SQLite迁移现有数据（如果有）")
	} else {
		fmt.Println("✅ MySQL数据库已准备就绪，可以正常使用！")
	}
}

func checkSQLiteData(db *config.Database) {
	fmt.Println("📊 SQLite数据库数据统计:")

	// 检查AI模型数量
	aiModels, err := db.GetAIModels("default")
	if err != nil {
		fmt.Printf("  ⚠️  无法查询AI模型: %v\n", err)
	} else {
		fmt.Printf("  🤖 AI模型数量: %d\n", len(aiModels))
		if len(aiModels) > 0 {
			fmt.Printf("     - %v\n", getModelNames(aiModels))
		}
	}

	// 检查交易所数量
	exchanges, err := db.GetExchanges("default")
	if err != nil {
		fmt.Printf("  ⚠️  无法查询交易所: %v\n", err)
	} else {
		fmt.Printf("  🏦 交易所数量: %d\n", len(exchanges))
		if len(exchanges) > 0 {
			fmt.Printf("     - %v\n", getExchangeNames(exchanges))
		}
	}

	// 检查交易员数量
	traders, err := db.GetTraders("default")
	if err != nil {
		fmt.Printf("  ⚠️  无法查询交易员: %v\n", err)
	} else {
		fmt.Printf("  📈 交易员数量: %d\n", len(traders))
		if len(traders) > 0 {
			fmt.Printf("     - %v\n", getTraderNames(traders))
		}
	}

	// 检查系统配置
	betaMode, _ := db.GetSystemConfig("beta_mode")
	apiPort, _ := db.GetSystemConfig("api_server_port")
	fmt.Printf("  ⚙️  系统配置: beta_mode=%s, api_port=%s\n", betaMode, apiPort)

	fmt.Println()
	if len(traders) > 0 {
		fmt.Println("💡 提示: 检测到SQLite中有数据，如果想迁移到MySQL，请:")
		fmt.Println("   1. 备份当前SQLite数据库 (config.db)")
		fmt.Println("   2. 设置MySQL环境变量")
		fmt.Println("   3. 在Web界面重新配置交易员")
	}
}

func getModelNames(models []*config.AIModelConfig) string {
	names := []string{}
	for _, m := range models {
		names = append(names, m.Name)
	}
	return fmt.Sprintf("%v", names)
}

func getExchangeNames(exchanges []*config.ExchangeConfig) string {
	names := []string{}
	for _, e := range exchanges {
		names = append(names, e.Name)
	}
	return fmt.Sprintf("%v", names)
}

func getTraderNames(traders []*config.TraderRecord) string {
	names := []string{}
	for _, t := range traders {
		names = append(names, t.Name)
	}
	return fmt.Sprintf("%v", names)
}
