package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	// "strings" // Removed unused
	// "time"    // Removed unused

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := "nofx" // Default schema name

	if dbHost == "" {
		log.Fatal("❌ DB_HOST not found in .env")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPassword, dbHost, dbName)

	fmt.Println("🔌 Connecting to MySQL...")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Connect failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Ping failed: %v", err)
	}
	fmt.Println("✅ Connected to MySQL")

	// Helper types for MySQL
	textType := "VARCHAR(255)"
	datetimeFunc := "CURRENT_TIMESTAMP"
	autoIncrementType := "AUTO_INCREMENT"

	// Create strategy_orders table
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS strategy_orders (
			id BIGINT PRIMARY KEY %s,
			trader_id %s NOT NULL,
			strategy_id %s NOT NULL,
			symbol %s NOT NULL,
			order_id %s NOT NULL,
			client_oid %s DEFAULT '',
			order_type %s NOT NULL,
			side %s NOT NULL,
			price REAL NOT NULL,
			quantity REAL NOT NULL,
			status %s DEFAULT 'new',
			created_at DATETIME DEFAULT %s,
			updated_at DATETIME DEFAULT %s,
			UNIQUE(trader_id, strategy_id, order_id)
		)`, autoIncrementType, textType, textType, textType, textType, textType, textType, textType, textType, datetimeFunc, datetimeFunc)

	fmt.Println("🔨 Creating strategy_orders table...")
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("❌ Failed to create table: %v", err)
	}
	fmt.Println("✅ Table 'strategy_orders' created or already exists")

	// Check if table actually exists now
	var tableName string
	err = db.QueryRow("SHOW TABLES LIKE 'strategy_orders'").Scan(&tableName)
	if err != nil {
		log.Fatalf("❌ Verification failed: Table strategy_orders not found!")
	}
	fmt.Printf("🔍 Verified: Table '%s' exists in database '%s'\n", tableName, dbName)

	// Also verify strategy_decision_history
	err = db.QueryRow("SHOW TABLES LIKE 'strategy_decision_history'").Scan(&tableName)
	if err != nil {
		fmt.Println("⚠️ Table 'strategy_decision_history' NOT found! This explains missing AI decisions in frontend.")
	} else {
		fmt.Printf("🔍 Verified: Table '%s' exists in database '%s'\n", tableName, dbName)
	}
}
