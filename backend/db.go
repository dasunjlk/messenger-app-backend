package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// DB is the global database connection.
var DB *sql.DB

// InitDB connects to PostgreSQL and creates the users table if it doesn't exist.
// Uses DATABASE_URL environment variable (e.g., postgres://user:pass@localhost:5432/messenger?sslmode=disable).
func InitDB() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/messenger?sslmode=disable"
	}

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	if err := DB.Ping(); err != nil {
		return err
	}

	// Create users table if not exists
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := DB.Exec(query); err != nil {
		return err
	}

	log.Println("Database connected and users table ready")
	return nil
}
