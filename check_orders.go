package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Connect to MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"))
	
	fmt.Printf("Connecting to MySQL: %s@%s\n", os.Getenv("DB_USER"), os.Getenv("DB_HOST"))
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	query := `SELECT id, symbol, status, order_type, price, quantity, order_id FROM strategy_orders WHERE status IN ('new', 'partially_filled')`
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- Active Strategy Orders (DB) ---")
	for rows.Next() {
		var id int
		var symbol, status, orderType, orderID string
		var price, quantity float64
		if err := rows.Scan(&id, &symbol, &status, &orderType, &price, &quantity, &orderID); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %d | Symbol: %s | Type: %s | Price: %.4f | Qty: %.4f | Status: %s | OrderID: %s\n", 
			id, symbol, orderType, price, quantity, status, orderID)
	}
	fmt.Println("-----------------------------------")
}
