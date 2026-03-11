package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

// DB is the global database connection.
var DB *sql.DB

// InitDB connects to MySQL and creates the users table if it doesn't exist.
// Uses DATABASE_URL environment variable (e.g., user:password@tcp(localhost:3306)/messenger).
func InitDB() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		// Default: MySQL (adjust user/password for your setup)
		connStr = "root:YourPassword@tcp(localhost:3306)/messenger"
	}

	var err error
	DB, err = sql.Open("mysql", connStr)
	if err != nil {
		return err
	}

	if err := DB.Ping(); err != nil {
		return err
	}

	// Create users table if not exists (MySQL syntax)
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INT AUTO_INCREMENT PRIMARY KEY,
		username VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := DB.Exec(query); err != nil {
		return err
	}

	log.Println("Database connected and users table ready")
	return nil
}
