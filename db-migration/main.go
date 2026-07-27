package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	dbURL := os.Getenv("DSN")
	if dbURL == "" {
		log.Fatal("DSN environment variable is required")
	}

	var db *sql.DB
	var err error

	// Retry connecting until DB is responsive
	for i := 0; i < 20; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("Database not ready yet... retrying (%d/10)", i+1)
		log.Println("error is ",err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database. Executing schema migrations...")

	// Execute migration query
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		status BOOLEAN DEFAULT false,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to run schema migration: %v", err)
	}

	log.Println("Schema migrations executed successfully!")
}