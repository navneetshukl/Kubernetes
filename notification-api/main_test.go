package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/notifications", createNotification)
	r.GET("/notifications", getNotifications)
	return r
}

func resetNotifications() {
	mu.Lock()
	defer mu.Unlock()
	notifications = []Notification{}
}

func TestCreateNotification(t *testing.T) {
	resetNotifications()
	r := setupRouter()

	notification := Notification{
		UserID:  "101",
		Message: "Your order has shipped",
	}
	body, _ := json.Marshal(notification)

	req, _ := http.NewRequest(http.MethodPost, "/notifications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp Notification
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, notification.UserID, resp.UserID)
	assert.Equal(t, notification.Message, resp.Message)
}

func TestCreateNotification_ValidationError(t *testing.T) {
	resetNotifications()
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/notifications", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "required")
}

func TestGetNotifications(t *testing.T) {
	resetNotifications()
	r := setupRouter()

	// Add a notification first
	mu.Lock()
	notifications = append(notifications, Notification{UserID: "101", Message: "Test message"})
	mu.Unlock()

	req, _ := http.NewRequest(http.MethodGet, "/notifications", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []Notification
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Len(t, resp, 1)
	assert.Equal(t, "101", resp[0].UserID)
	assert.Equal(t, "Test message", resp[0].Message)
}

func TestGetNotifications_Empty(t *testing.T) {
	resetNotifications()
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/notifications", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []Notification
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Empty(t, resp)
}