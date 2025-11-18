package main

import (
	"database/sql"
	"fmt"
	"log"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "data/database.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 查询用户
	rows, err := db.Query("SELECT id, email, role, trader_id FROM users WHERE id = ?", "a14992a5-182e-4ddf-b7f7-c43459bb08b9")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	if rows.Next() {
		var id, email, role, traderID string
		err = rows.Scan(&id, &email, &role, &traderID)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("User: ID=%s, Email=%s, Role=%s, TraderID=%s\n", id, email, role, traderID)
	} else {
		fmt.Println("User not found")
	}

	// 查询交易员
	rows2, err := db.Query("SELECT trader_id, trader_account_id, owner_user_id FROM traders WHERE trader_id = ?", "a14992a5-182e-4ddf-b7f7-c43459bb08b9")
	if err != nil {
		log.Fatal(err)
	}
	defer rows2.Close()

	if rows2.Next() {
		var traderID, traderAccountID, ownerUserID string
		err = rows2.Scan(&traderID, &traderAccountID, &ownerUserID)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Trader: ID=%s, AccountID=%s, OwnerID=%s\n", traderID, traderAccountID, ownerUserID)
	} else {
		fmt.Println("Trader not found")
	}
}
