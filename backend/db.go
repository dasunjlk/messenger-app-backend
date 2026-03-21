package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DB is the shared database connection pool.
var DB *sql.DB

// InitDB connects to MySQL using DATABASE_URL and ensures the users table exists.
// Example DSN: user:password@tcp(localhost:3306)/messenger?parseTime=true
func InitDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "root:password@tcp(127.0.0.1:3306)/messenger?parseTime=true"
	} else if !strings.Contains(dsn, "parseTime=") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	schema := `
    CREATE TABLE IF NOT EXISTS users (
        id INT AUTO_INCREMENT PRIMARY KEY,
        username VARCHAR(255) NOT NULL UNIQUE,
        email VARCHAR(255) NOT NULL UNIQUE,
        password_hash VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`

	if _, err := conn.Exec(schema); err != nil {
		return fmt.Errorf("ensure users table: %w", err)
	}

	messagesSchema := `
    CREATE TABLE IF NOT EXISTS messages (
        id INT AUTO_INCREMENT PRIMARY KEY,
        sender_id INT NOT NULL,
        receiver_id INT NOT NULL,
        content TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        INDEX idx_messages_sender (sender_id),
        INDEX idx_messages_receiver (receiver_id)
    )`

	if _, err := conn.Exec(messagesSchema); err != nil {
		return fmt.Errorf("ensure messages table: %w", err)
	}

	DB = conn
	log.Println("Database connected and schema ready")
	return nil
}
