package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/microsoft/go-mssqldb"
)

// DB is the global database connection.
var DB *sql.DB

// InitDB connects to SQL Server and creates the users table if it doesn't exist.
// Uses DATABASE_URL environment variable (e.g., sqlserver://user:pass@localhost:1433?database=messenger).
func InitDB() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		// Default: SQL Server with Windows auth or SQL auth (adjust user/password for your setup)
		connStr = "sqlserver://sa:YourPassword@localhost:1433?database=messenger"
	}

	var err error
	DB, err = sql.Open("sqlserver", connStr)
	if err != nil {
		return err
	}

	if err := DB.Ping(); err != nil {
		return err
	}

	// Create users table if not exists (SQL Server syntax)
	query := `
	IF OBJECT_ID('users', 'U') IS NULL
	CREATE TABLE users (
		id INT IDENTITY(1,1) PRIMARY KEY,
		username NVARCHAR(255) UNIQUE NOT NULL,
		email NVARCHAR(255) UNIQUE NOT NULL,
		password_hash NVARCHAR(255) NOT NULL,
		created_at DATETIME2 DEFAULT GETDATE()
	);
	`
	if _, err := DB.Exec(query); err != nil {
		return err
	}

	log.Println("Database connected and users table ready")
	return nil
}
