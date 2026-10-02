package main

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// Notification represents a notification object
type Notification struct {
	UserID  string `json:"user_id" binding:"required"`
	Message string `json:"message" binding:"required"`
}

// In-memory storage for notifications
var (
	notifications []Notification
	mu            sync.Mutex
)

func main() {
	r := gin.Default()

	r.POST("/notifications", createNotification)
	r.GET("/notifications", getNotifications)

	println("Server starting on port :8080")
	r.Run(":8080")
}

func createNotification(c *gin.Context) {
	var notification Notification
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store notification in memory
	mu.Lock()
	notifications = append(notifications, notification)
	mu.Unlock()

	c.JSON(http.StatusCreated, notification)
}

func getNotifications(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	c.JSON(http.StatusOK, notifications)
}