package models

import "time"

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
)

type Task struct {
	ID          string       `bson:"_id" json:"id"`
	Title       string       `bson:"title" json:"title"`
	Description string       `bson:"description,omitempty" json:"description,omitempty"`
	Status      TaskStatus   `bson:"status" json:"status"`
	Priority    TaskPriority `bson:"priority" json:"priority"`
	DueDate     time.Time    `bson:"dueDate" json:"due_date"`
	CreatedAt   time.Time    `bson:"createdAt" json:"created_at"`
	UpdatedAt   time.Time    `bson:"updatedAt" json:"updated_at"`
}

type CreateTaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	DueDate     string `json:"due_date"`
}

type UpdateTaskInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	DueDate     *string `json:"due_date"`
}

type ListTaskInput struct {
	Status   string
	Priority string
	Page     int
	PageSize int
}

type TaskFilter struct {
	Status   *TaskStatus
	Priority *TaskPriority
}

type TaskListQuery struct {
	Filter   TaskFilter
	Page     int
	PageSize int
}

type TaskListResult struct {
	Items      []Task
	TotalItems int64
}

type PaginatedTasks struct {
	Items        []Task
	TotalItems   int64
	Page         int
	PageSize     int
	TotalPages   int
	PreviousPage *int
	NextPage     *int
}

type TaskUpdate struct {
	Title       *string
	Description *string
	Status      *TaskStatus
	Priority    *TaskPriority
	DueDate     *time.Time
}
