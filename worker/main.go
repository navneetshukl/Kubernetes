package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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

func getLogPaths() (string, string) {
	dir := os.Getenv("LOG_DIR")
	if dir == "" {
		dir = "./logs"
	}
	return dir, filepath.Join(dir, "audit.log")
}

// Helper to format and write log blocks to the audit file and sync immediately
func writeAuditEntry(file *os.File, eventType, status, details string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	entry := fmt.Sprintf(
		"========================================\n"+
			"AUDIT EVENT : %s\n"+
			"TIMESTAMP   : %s\n"+
			"STATUS      : %s\n"+
			"%s\n"+
			"========================================\n\n",
		eventType,
		timestamp,
		status,
		details,
	)

	if _, err := file.WriteString(entry); err != nil {
		log.Printf("Failed to write to audit log: %v", err)
	} else {
		_ = file.Sync()
	}
}

func main() {
	apiBaseURL := os.Getenv("TASK_API_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080"
	}

	logDirPath, logFilePath := getLogPaths()

	// 1. Ensure logs directory exists
	if err := os.MkdirAll(logDirPath, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// 2. Open audit.log file in append mode
	auditFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open audit log file: %v", err)
	}
	defer auditFile.Close()

	log.Printf("Worker daemon active. Polling target: %s", apiBaseURL)
	log.Printf("Audit logs written to: %s", logFilePath)

	writeAuditEntry(
		auditFile,
		"WORKER_LIFECYCLE",
		"SUCCESS",
		fmt.Sprintf("Details     : Worker daemon booted successfully\nTarget URL  : %s", apiBaseURL),
	)

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
			writeAuditEntry(
				auditFile,
				"WORKER_LIFECYCLE",
				"SUCCESS",
				"Details     : Worker received termination signal and stopped gracefully",
			)
			return
		case <-ticker.C:
			pollAndProcess(ctx, client, apiBaseURL, auditFile)
		}
	}
}

func pollAndProcess(ctx context.Context, client *http.Client, baseURL string, auditFile *os.File) {
	getURL := fmt.Sprintf("%s/task/get", baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		writeAuditEntry(
			auditFile,
			"POLL_ERROR",
			"FAILURE",
			fmt.Sprintf("Stage       : Request Creation\nError       : %v", err),
		)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		writeAuditEntry(
			auditFile,
			"NETWORK_ERROR",
			"FAILURE",
			fmt.Sprintf("Target URL  : %s\nExact Error : %v", getURL, err),
		)
		return
	}
	defer resp.Body.Close()

	// Read full response body to capture exact payload or error response text
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeAuditEntry(
			auditFile,
			"READ_ERROR",
			"FAILURE",
			fmt.Sprintf("Target URL  : %s\nError       : %v", getURL, err),
		)
		return
	}

	// Handle Non-200 HTTP responses by logging exact server error payload
	if resp.StatusCode != http.StatusOK {
		writeAuditEntry(
			auditFile,
			"API_HTTP_ERROR",
			"FAILURE",
			fmt.Sprintf("Endpoint    : %s\nStatus Code : %d\nAPI Payload : %s", getURL, resp.StatusCode, string(bodyBytes)),
		)
		return
	}

	var apiResponse ApiResponse
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		writeAuditEntry(
			auditFile,
			"PAYLOAD_DECODE_ERROR",
			"FAILURE",
			fmt.Sprintf("Raw Body    : %s\nExact Error : %v", string(bodyBytes), err),
		)
		return
	}

	foundPending := false
	for _, task := range apiResponse.Data {
		if task.Status == "Pending" {
			foundPending = true
			log.Printf("[WORKER] Picking up task: ID=%s | Title=%s", task.ID, task.Title)
			processTask(task, apiResponse.Message, auditFile)
		}
	}

	if !foundPending {
		log.Println("[WORKER] Queue clear. No pending tasks found.")
		writeAuditEntry(
			auditFile,
			"QUEUE_POLL",
			"SUCCESS",
			fmt.Sprintf("API Message : %s\nTotal Items : %d\nDetails     : Queue clear. No pending tasks found.", apiResponse.Message, len(apiResponse.Data)),
		)
	}
}

func processTask(task Task, apiMessage string, auditFile *os.File) {
	startTime := time.Now().UTC()

	// Simulating work delivery
	time.Sleep(1 * time.Second)

	endTime := time.Now().UTC()
	duration := endTime.Sub(startTime)

	details := fmt.Sprintf(
		"Task ID     : %s\n"+
			"Title       : %s\n"+
			"API Message : %s\n"+
			"Started At  : %s\n"+
			"Finished At : %s\n"+
			"Duration    : %s",
		task.ID,
		task.Title,
		apiMessage,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
		duration,
	)

	writeAuditEntry(auditFile, "TASK_PROCESSED", "SUCCESS", details)
}