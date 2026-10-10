package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	log.Println("=== Migration InitContainer starting ===")

	// Get database connection parameters from environment
	host := getEnv("DB_HOST", "postgres")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "notifications")
	sslmode := getEnv("DB_SSLMODE", "disable")

	// Build connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	log.Printf("Connecting to database: host=%s port=%s dbname=%s user=%s", host, port, dbname, user)

	// Retry connection with exponential backoff
	var db *sql.DB
	var err error
	maxRetries := 10
	baseDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("Failed to open database connection (attempt %d/%d): %v", i+1, maxRetries, err)
		} else {
			err = db.Ping()
			if err == nil {
				log.Println("Successfully connected to database")
				break
			}
			log.Printf("Failed to ping database (attempt %d/%d): %v", i+1, maxRetries, err)
		}

		if i < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<i) // exponential backoff
			log.Printf("Retrying in %v...", delay)
			time.Sleep(delay)
		}
	}

	if err != nil {
		log.Fatalf("Failed to connect to database after %d attempts: %v", maxRetries, err)
	}
	defer db.Close()

	// Create notifications table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS notifications (
		id          SERIAL PRIMARY KEY,
		user_id     VARCHAR(255) NOT NULL,
		message     TEXT NOT NULL,
		status      VARCHAR(50) NOT NULL DEFAULT 'pending',
		created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);`

	log.Println("Creating notifications table...")
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create notifications table: %v", err)
	}

	log.Println("Notifications table created successfully")

	// Verify table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count)
	if err != nil {
		log.Fatalf("Failed to verify table: %v", err)
	}
	log.Printf("Table verified. Current notification count: %d", count)

	log.Println("=== Migration InitContainer completed successfully ===")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}