package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Attempt to connect to MySQL first (as per main.go logic)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"))
	
	if os.Getenv("DB_HOST") == "" {
		// Fallback to SQLite check if env vars not set (though user context implies MySQL)
		fmt.Println("No DB_HOST, skipping MySQL check...")
	}

	fmt.Printf("Connecting to MySQL: %s@%s\n", os.Getenv("DB_USER"), os.Getenv("DB_HOST"))
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Printf("MySQL Connect Failed: %v. Trying SQLite...\n", err)
	} else {
		fmt.Println("MySQL Connected!")
		queryAndPrint(db)
		return
	}
}

func queryAndPrint(db *sql.DB) {
	query := `
		SELECT id, action, symbol, execution_success, execution_error, decision_time 
		FROM strategy_decision_history 
		ORDER BY decision_time DESC 
		LIMIT 10
	`
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal("Query failed:", err)
	}
	defer rows.Close()

	fmt.Println("\n--- Last 10 Strategy Decisions ---")
	for rows.Next() {
		var id int
		var action, symbol string
		var success bool
		var errInfo string
		var timeStr string
		if err := rows.Scan(&id, &action, &symbol, &success, &errInfo, &timeStr); err != nil {
			log.Fatal(err)
		}
		status := "✅"
		if !success {
			status = "❌"
		}
		fmt.Printf("[%s] ID: %d | Time: %s | Action: %s | Symbol: %s | Result: %s %s\n", 
			status, id, timeStr, action, symbol, success, errInfo)
	}
	fmt.Println("----------------------------------\n")
}
