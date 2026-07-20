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

func main() {
	apiBaseURL := os.Getenv("TASK_API_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080" 
	}

	log.Printf("Worker daemon active. Polling target: %s", apiBaseURL)

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
			pollAndProcess(ctx, client, apiBaseURL)
		}
	}
}

func pollAndProcess(ctx context.Context, client *http.Client, baseURL string) {
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

			processTask(task)
		}
	}

	if !foundPending {
		log.Println("[WORKER] Queue clear. No pending tasks found.")
	}
}

func processTask(task Task) {
	log.Printf("[WORKER] Processing task '%s'...", task.ID)
	// Simulating work delivery
	time.Sleep(1 * time.Second)
	log.Printf("[WORKER] Completed task '%s'!", task.ID)
}
