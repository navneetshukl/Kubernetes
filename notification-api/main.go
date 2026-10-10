package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Notification represents a notification object (matches database schema)
type Notification struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id" binding:"required"`
	Message   string    `json:"message" binding:"required"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

var db *sql.DB

func main() {
	// Initialize database connection
	initDB()
	defer db.Close()

	r := gin.Default()

	r.POST("/notifications", createNotification)
	r.GET("/notifications", getNotifications)

	println("Server starting on port :8080")
	r.Run(":8080")
}

func initDB() {
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

	// Verify table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count)
	if err != nil {
		log.Fatalf("Failed to verify table: %v", err)
	}
	log.Printf("Table verified. Current notification count: %d", count)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func createNotification(c *gin.Context) {
	var notification Notification
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Insert notification into database
	query := `INSERT INTO notifications (user_id, message, status) VALUES ($1, $2, 'pending') RETURNING id, created_at`
	err := db.QueryRow(query, notification.UserID, notification.Message).Scan(&notification.ID, &notification.CreatedAt)
	if err != nil {
		log.Printf("Failed to insert notification: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	notification.Status = "pending"
	c.JSON(http.StatusCreated, notification)
}

func getNotifications(c *gin.Context) {
	rows, err := db.Query("SELECT id, user_id, message, status, created_at FROM notifications ORDER BY created_at DESC")
	if err != nil {
		log.Printf("Failed to query notifications: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve notifications"})
		return
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Message, &n.Status, &n.CreatedAt); err != nil {
			log.Printf("Failed to scan notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve notifications"})
			return
		}
		notifications = append(notifications, n)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Row iteration error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve notifications"})
		return
	}

	c.JSON(http.StatusOK, notifications)
}