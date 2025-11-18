package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	os.Chdir("data")
	db, err := sql.Open("sqlite3", "database.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("=== Users ===")
	rows, err := db.Query("SELECT id, email, role, trader_id FROM users LIMIT 10")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, email, role, traderID string
		rows.Scan(&id, &email, &role, &traderID)
		fmt.Printf("ID: %s, Email: %s, Role: %s, TraderID: %s\n", id, email, role, traderID)
	}

	fmt.Println("\n=== Traders ===")
	rows2, err := db.Query("SELECT trader_id, trader_account_id, owner_user_id FROM traders LIMIT 10")
	if err != nil {
		log.Fatal(err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var traderID, traderAccountID, ownerUserID string
		rows2.Scan(&traderID, &traderAccountID, &ownerUserID)
		fmt.Printf("TraderID: %s, AccountID: %s, OwnerID: %s\n", traderID, traderAccountID, ownerUserID)
	}
}
