package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"))
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check latest strategy orders for BTCUSDT
	query := `SELECT id, symbol, order_id, order_type, side, price, quantity, leverage, status, created_at 
	          FROM strategy_orders 
	          WHERE symbol = 'BTCUSDT' 
	          ORDER BY created_at DESC 
	          LIMIT 10`
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- Latest BTCUSDT Strategy Orders in DB ---")
	for rows.Next() {
		var id int
		var symbol, orderID, orderType, side, status string
		var price, quantity float64
		var leverage int
		var createdAt string
		if err := rows.Scan(&id, &symbol, &orderID, &orderType, &side, &price, &quantity, &leverage, &status, &createdAt); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %d | Order: %s | Type: %-6s | Side: %-4s | Price: %.2f | Qty: %.4f | Lev: %dx | Status: %s | Time: %s\n", 
			id, orderID[:16], orderType, side, price, quantity, leverage, status, createdAt)
	}
	fmt.Println("---------------------------------------------")
}
