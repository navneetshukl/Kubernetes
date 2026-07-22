package main

import (
	"database/sql"
	"log"
	_ "github.com/lib/pq"
)

func connectToDb(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("Error opening database: %v\n", err)
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		log.Printf("Error connecting to the database: %v\n", err)
		return nil, err
	}

	log.Println("Successfully connected to the PostgreSQL database using a URL string!")
	return db, nil
}

type DBService struct {
	db *sql.DB
}

type DBServiceImpl interface {
	InsertTask(task Task) error
	GetAllTasks() ([]Task, error)
}

func NewDBService(db *sql.DB) DBServiceImpl {
	return &DBService{
		db: db,
	}
}

func (d *DBService) InsertTask(task Task) error {
	query := `INSERT INTO tasks (id, title, status, created_at) VALUES ($1, $2, $3, $4)`

	_, err := d.db.Exec(query, task.ID, task.Title, task.Status, task.CreatedAt)
	return err
}

func (d *DBService) GetAllTasks() ([]Task, error) {
	query := `SELECT id, title, status, created_at FROM tasks`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	// Check for errors encountered during iteration
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}
