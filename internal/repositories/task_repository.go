package repositories

import (
	"context"
	"errors"

	"to-do-list/internal/models"
)

var ErrNotFound = errors.New("resource not found")

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	List(ctx context.Context, filter models.TaskFilter) ([]models.Task, error)
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, id string, input models.TaskUpdate) (*models.Task, error)
	Delete(ctx context.Context, id string) error
}
