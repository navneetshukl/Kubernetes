package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func getLogFilePath() string {
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	return filepath.Join(logDir, "audit.log")
}

func main() {
	logFilePath := getLogFilePath()
	log.Printf("[SIDECAR] Logger daemon active. Target log file: %s", logFilePath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Wait for worker app to create the log file if it doesn't exist yet
	for {
		if _, err := os.Stat(logFilePath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			log.Println("[SIDECAR] Stopping logger before file was created.")
			return
		case <-time.After(1 * time.Second):
			log.Printf("[SIDECAR] Waiting for %s to be created...", logFilePath)
		}
	}

	// 2. Open file for reading
	file, err := os.Open(logFilePath)
	if err != nil {
		log.Fatalf("[SIDECAR] Failed to open log file: %v", err)
	}
	defer file.Close()

	log.Printf("[SIDECAR] Attached to %s. Streaming logs to stdout...", logFilePath)

	reader := bufio.NewReader(file)

	// 3. Continuous tailing loop
	for {
		select {
		case <-ctx.Done():
			log.Println("[SIDECAR] Termination signal received. Flushing remaining lines...")
			// Read any remaining content before exiting
			for {
				line, err := reader.ReadString('\n')
				if len(line) > 0 {
					fmt.Print(line)
				}
				if err != nil {
					break
				}
			}
			return
		default:
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				// Output directly to standard out so kubectl/Docker captures it
				fmt.Print(line)
			}

			if err != nil {
				if err == io.EOF {
					// Reached end of current file, sleep briefly before polling for new writes
					time.Sleep(500 * time.Millisecond)
					continue
				}
				log.Printf("[SIDECAR] Read error: %v", err)
				time.Sleep(1 * time.Second)
			}
		}
	}
}