package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ApiResponse struct {
	Message string `json:"message"`
	Data    []Task `json:"data"`
}

const (
	logDirPath  = "/var/log/worker"
	logFilePath = "/var/log/worker/audit.log"
)

func main() {
	apiBaseURL := os.Getenv("TASK_API_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080"
	}

	// 1. Ensure the log directory exists
	if err := os.MkdirAll(logDirPath, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// 2. Open /var/log/worker/audit.log in append mode
	auditFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open audit log file: %v", err)
	}
	defer auditFile.Close()

	log.Printf("Worker daemon active. Polling target: %s", apiBaseURL)
	log.Printf("Audit logs directed to: %s", logFilePath)

	// Setup clean OS signal listening for Kubernetes lifecycle management
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Gracefully stopping worker process...")
			return
		case <-ticker.C:
			pollAndProcess(ctx, client, apiBaseURL, auditFile)
		}
	}
}

func pollAndProcess(ctx context.Context, client *http.Client, baseURL string, auditFile *os.File) {
	// Call your specific endpoint: /task/get
	getURL := fmt.Sprintf("%s/task/get", baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		log.Printf("Failed to prepare request: %v", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Network error trying to hit API: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("API returned unhealthy status code: %d", resp.StatusCode)
		return
	}

	// Decode the payload matching your gin response layout
	var apiResponse ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		log.Printf("Failed parsing response body: %v", err)
		return
	}

	// Iterate through the fetched data array looking for "Pending" items
	foundPending := false
	for _, task := range apiResponse.Data {
		if task.Status == "Pending" {
			foundPending = true
			log.Printf("[WORKER] Picking up task: ID=%s | Title=%s", task.ID, task.Title)

			processTask(task, auditFile)
		}
	}

	if !foundPending {
		log.Println("[WORKER] Queue clear. No pending tasks found.")
	}
}

func processTask(task Task, auditFile *os.File) {
	startTime := time.Now().UTC()

	// Simulating work delivery
	time.Sleep(1 * time.Second)

	endTime := time.Now().UTC()
	duration := endTime.Sub(startTime)

	// Format multi-line processing audit log
	auditEntry := fmt.Sprintf(
		"========================================\n"+
			"AUDIT EVENT : TASK_PROCESSED\n"+
			"Task ID     : %s\n"+
			"Title       : %s\n"+
			"Status      : SUCCESS\n"+
			"Started At  : %s\n"+
			"Finished At : %s\n"+
			"Duration    : %s\n"+
			"========================================\n\n",
		task.ID,
		task.Title,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
		duration,
	)

	// Write directly to /var/log/worker/audit.log
	if _, err := auditFile.WriteString(auditEntry); err != nil {
		log.Printf("Error writing to audit log: %v", err)
	} else {
		// Flush buffer immediately so the sidecar can tail it in real-time
		_ = auditFile.Sync()
	}
}