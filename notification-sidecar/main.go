package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

func main() {
	logPath := "logs/worker.log"

	// Open the log file
	file, err := os.Open(logPath)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Create a scanner to read line by line
	scanner := bufio.NewScanner(file)

	// Read existing lines first
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	// Then tail the file for new lines (follow mode)
	// Seek to end of file
	file.Seek(0, os.SEEK_END)

	for {
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		// Wait a bit before checking for new lines
		time.Sleep(100 * time.Millisecond)

		// Check if there was an error (other than EOF)
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading log file: %v\n", err)
			return
		}
	}
}
