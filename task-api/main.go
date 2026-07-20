package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
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
	taskStore map[string]Task
	mut       *sync.Mutex
}

func NewTaskApi() *TaskApi {
	return &TaskApi{
		taskStore: make(map[string]Task),
		mut:       &sync.Mutex{},
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

	t.mut.Lock()
	t.taskStore[newTask.ID] = newTask
	t.mut.Unlock()
	ctx.JSON(http.StatusOK, gin.H{
		"message": "task added successfully",
	})

}

func (t *TaskApi) GetTask(ctx *gin.Context) {
	var tasks []Task

	t.mut.Lock()
	for _, v := range t.taskStore {
		tasks = append(tasks, v)
	}
	t.mut.Unlock()
	ctx.JSON(http.StatusOK, gin.H{
		"message": "task fetched successfully",
		"data":    tasks,
	})

}

func main() {

	mux := gin.New()
	taksController := NewTaskApi()
	mux.POST("/task/add", taksController.CreateTask)
	mux.GET("/task/get", taksController.GetTask)
	err:=mux.Run(":8080")
	if err!=nil{
		log.Println("error in running the application ",err)
		return
	}

}
