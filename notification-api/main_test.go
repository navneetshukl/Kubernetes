package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *sql.DB

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/notifications", createNotification)
	r.GET("/notifications", getNotifications)
	return r
}

func setupTestDB(t *testing.T) {
	// Get database connection parameters from environment
	host := getEnv("TEST_DB_HOST", "localhost")
	port := getEnv("TEST_DB_PORT", "5432")
	user := getEnv("TEST_DB_USER", "postgres")
	password := getEnv("TEST_DB_PASSWORD", "postgres")
	dbname := getEnv("TEST_DB_NAME", "notifications_test")
	sslmode := getEnv("TEST_DB_SSLMODE", "disable")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	var err error
	testDB, err = sql.Open("postgres", connStr)
	require.NoError(t, err)

	// Retry connection
	maxRetries := 5
	baseDelay := 1 * time.Second
	for i := 0; i < maxRetries; i++ {
		err = testDB.Ping()
		if err == nil {
			break
		}
		time.Sleep(baseDelay * time.Duration(1<<i))
	}
	require.NoError(t, err, "Failed to connect to test database")

	// Override the global db for testing
	db = testDB

	// Clean up test data before each test
	_, err = testDB.Exec("DELETE FROM notifications")
	require.NoError(t, err)
}

func teardownTestDB() {
	if testDB != nil {
		testDB.Close()
	}
}

func TestMain(m *testing.M) {
	// Setup test database
	gin.SetMode(gin.TestMode)

	// Check if test database is available
	if os.Getenv("TEST_DB_HOST") == "" {
		// Skip tests if no test database configured
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func TestCreateNotification(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

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
	assert.Equal(t, "pending", resp.Status)
	assert.NotZero(t, resp.ID)
	assert.NotZero(t, resp.CreatedAt)
}

func TestCreateNotification_ValidationError(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/notifications", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "required")
}

func TestGetNotifications(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	r := setupRouter()

	// Add a notification first via API
	notification := Notification{
		UserID:  "101",
		Message: "Test message",
	}
	body, _ := json.Marshal(notification)

	req, _ := http.NewRequest(http.MethodPost, "/notifications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Now get notifications
	req, _ = http.NewRequest(http.MethodGet, "/notifications", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []Notification
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Len(t, resp, 1)
	assert.Equal(t, "101", resp[0].UserID)
	assert.Equal(t, "Test message", resp[0].Message)
	assert.Equal(t, "pending", resp[0].Status)
}

func TestGetNotifications_Empty(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/notifications", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []Notification
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Empty(t, resp)
}
