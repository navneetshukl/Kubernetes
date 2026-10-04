package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Notification represents a notification from the API
type Notification struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// NotificationStatus represents the processing state of a notification
type NotificationStatus string

const (
	StatusPending    NotificationStatus = "pending"
	StatusProcessing NotificationStatus = "processing"
	StatusCompleted  NotificationStatus = "completed"
)

// processNotification simulates processing: pending -> processing -> completed
func processNotification(n Notification) {
	// Step 1: pending
	log.Printf("[worker] found pending notification: user_id=%s message=%q", n.UserID, n.Message)

	// Step 2: processing
	log.Printf("[worker] status=pending -> processing for user_id=%s", n.UserID)
	time.Sleep(100 * time.Millisecond)

	// Step 3: completed
	log.Printf("[worker] status=processing -> completed for user_id=%s", n.UserID)
}

// fetchPendingNotifications calls GET /notifications and returns pending ones
func fetchPendingNotifications(apiURL string) ([]Notification, error) {
	url := apiURL + "/notifications"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var notifications []Notification
	if err := json.NewDecoder(resp.Body).Decode(&notifications); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return notifications, nil
}

// runOnce fetches notifications and processes them once
func runOnce(apiURL string) {
	notifications, err := fetchPendingNotifications(apiURL)
	if err != nil {
		log.Printf("[worker] error fetching notifications: %v", err)
		return
	}

	log.Printf("[worker] received %d notifications from %s", len(notifications), apiURL+"/notifications")

	for _, n := range notifications {
		processNotification(n)
	}
}

func main() {
	apiURL := getEnv("API_URL", "http://localhost:8080")
	interval := getEnvDuration("POLL_INTERVAL", 5*time.Second)

	log.Printf("[worker] starting: API_URL=%s POLL_INTERVAL=%s", apiURL, interval)

	// Simple cron: run immediately, then periodically
	for {
		runOnce(apiURL)
		time.Sleep(interval)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
