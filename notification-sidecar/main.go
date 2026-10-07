package main

import (
	"bufio"
	"log"
	"os"
	"time"
)

func main() {
	log.Println("=== Sidecar starting ===")

	logPath := getEnv("LOG_PATH", "/app/logs/worker.log")
	pollInterval := getEnvDuration("POLL_INTERVAL", 2*time.Second)

	log.Printf("Config: LOG_PATH=%s, POLL_INTERVAL=%v", logPath, pollInterval)

	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		log.Fatalf("Log file does not exist: %s", logPath)
	}
	log.Printf("Log file exists: %s", logPath)

	// Tail the file continuously
	log.Println("Starting to tail log file...")
	pollCount := 0
	for {
		pollCount++
		log.Printf("Poll cycle #%d - checking for new lines...", pollCount)

		// Open file each poll to pick up new content
		file, err := os.Open(logPath)
		if err != nil {
			log.Printf("Error opening log file: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		scanner := bufio.NewScanner(file)
		linesThisPoll := 0
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[LOG] %s", line)
			linesThisPoll++
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error reading log file: %v", err)
		}

		file.Close()

		if linesThisPoll > 0 {
			log.Printf("Poll #%d: printed %d lines", pollCount, linesThisPoll)
		} else {
			log.Printf("Poll #%d: no new lines", pollCount)
		}

		log.Printf("Poll #%d: sleeping for %v", pollCount, pollInterval)
		time.Sleep(pollInterval)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		log.Printf("Env %s = %s", key, value)
		return value
	}
	log.Printf("Env %s not set, using default: %s", key, defaultValue)
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			log.Printf("Env %s = %v (parsed)", key, d)
			return d
		}
		log.Printf("Env %s = %s (invalid duration, using default)", key, value)
	}
	log.Printf("Env %s not set, using default: %v", key, defaultValue)
	return defaultValue
}
