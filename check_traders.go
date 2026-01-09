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

	query := `SELECT id, name, is_running FROM traders`
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- Trader Status (DB) ---")
	for rows.Next() {
		var id, name string
		var isRunning bool // MySQL TINYINT(1) maps to bool
		if err := rows.Scan(&id, &name, &isRunning); err != nil {
			log.Fatal(err)
		}
		status := "STOPPED"
		if isRunning {
			status = "RUNNING"
		}
		fmt.Printf("Trader: %-15s | ID: %s | Status: %s\n", name, id, status)
	}
	fmt.Println("--------------------------")
}
