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

	// Check leverage settings for all traders
	query := `SELECT id, name, btc_eth_leverage, altcoin_leverage FROM traders`
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- Trader Leverage Settings (DB) ---")
	for rows.Next() {
		var id, name string
		var btcEthLeverage, altcoinLeverage int
		if err := rows.Scan(&id, &name, &btcEthLeverage, &altcoinLeverage); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Trader: %-15s | BTC/ETH Leverage: %dx | Altcoin Leverage: %dx\n", 
			name, btcEthLeverage, altcoinLeverage)
	}
	fmt.Println("--------------------------------------")
}
