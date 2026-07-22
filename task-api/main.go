package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskApi struct {
	db DBServiceImpl
}

func NewTaskApi(db DBServiceImpl) *TaskApi {
	return &TaskApi{
		db: db,
	}
}

func (t *TaskApi) CreateTask(ctx *gin.Context) {
	var newTask Task
	err := ctx.ShouldBindJSON(&newTask)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "error in decoding request",
		})
		return
	}
	newTask.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	newTask.Status = "Pending"
	newTask.CreatedAt = time.Now()

	err = t.db.InsertTask(newTask)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": "task added successfully",
	})

}

func (t *TaskApi) GetTask(ctx *gin.Context) {
	log.Println("inside get task")

	tasks, err := t.db.GetAllTasks()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": "task fetched successfully",
		"data":    tasks,
	})

}

func main() {
	dsn := os.Getenv("DSN")
	db, err := connectToDb(dsn)
	if err != nil {
		log.Println(err)
		return
	}

	repo := NewDBService(db)

	mux := gin.New()
	mux.Use(gin.Logger())
	mux.Use(gin.Recovery())
	taksController := NewTaskApi(repo)

	mux.POST("/task/add", taksController.CreateTask)
	mux.GET("/task/get", taksController.GetTask)
	err = mux.Run(":8080")
	if err != nil {
		log.Println("error in running the application ", err)
		return
	}

}
