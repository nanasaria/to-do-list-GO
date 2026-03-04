package controllers_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"to-do-list/internal/controllers"
	"to-do-list/internal/models"
	"to-do-list/internal/services"
)

type stubTaskService struct {
	createFunc  func(context.Context, models.CreateTaskInput) (*models.Task, error)
	listFunc    func(context.Context, models.ListTaskInput) (*models.PaginatedTasks, error)
	getByIDFunc func(context.Context, string) (*models.Task, error)
	updateFunc  func(context.Context, string, models.UpdateTaskInput) (*models.Task, error)
	deleteFunc  func(context.Context, string) error
}

func (serviceStub stubTaskService) Create(ctx context.Context, input models.CreateTaskInput) (*models.Task, error) {
	if serviceStub.createFunc != nil {
		return serviceStub.createFunc(ctx, input)
	}

	return nil, nil
}

func (serviceStub stubTaskService) List(ctx context.Context, input models.ListTaskInput) (*models.PaginatedTasks, error) {
	if serviceStub.listFunc != nil {
		return serviceStub.listFunc(ctx, input)
	}

	return nil, nil
}

func (serviceStub stubTaskService) GetByID(ctx context.Context, id string) (*models.Task, error) {
	if serviceStub.getByIDFunc != nil {
		return serviceStub.getByIDFunc(ctx, id)
	}

	return nil, nil
}

func (serviceStub stubTaskService) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	if serviceStub.updateFunc != nil {
		return serviceStub.updateFunc(ctx, id, input)
	}

	return nil, nil
}

func (serviceStub stubTaskService) Delete(ctx context.Context, id string) error {
	if serviceStub.deleteFunc != nil {
		return serviceStub.deleteFunc(ctx, id)
	}

	return nil
}

func TestTaskControllerUpdateHandlesPatchRequest(t *testing.T) {
	t.Parallel()

	taskID := "123e4567-e89b-12d3-a456-426614174000"
	controller := controllers.NewTaskController(
		stubTaskService{
			updateFunc: func(_ context.Context, rawID string, input models.UpdateTaskInput) (*models.Task, error) {
				if rawID != taskID {
					t.Fatalf("Update() rawID = %q, want %q", rawID, taskID)
				}

				if input.Priority == nil || *input.Priority != "high" {
					t.Fatalf("Update() input.Priority = %v, want %q", input.Priority, "high")
				}

				if input.Title != nil || input.Status != nil || input.Description != nil || input.DueDate != nil {
					t.Fatal("Update() should receive only the patched field")
				}

				return &models.Task{
					ID:        taskID,
					Title:     "Task editavel",
					Status:    models.TaskStatusPending,
					Priority:  models.TaskPriorityHigh,
					DueDate:   time.Date(2030, time.January, 15, 0, 0, 0, 0, time.UTC),
					CreatedAt: time.Date(2030, time.January, 10, 12, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2030, time.January, 10, 13, 0, 0, 0, time.UTC),
				}, nil
			},
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		2*time.Second,
	)

	httpRequest := httptest.NewRequest(http.MethodPatch, "/tasks/"+taskID, strings.NewReader(`{"priority":"high"}`))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.SetPathValue("id", taskID)
	httpResponse := httptest.NewRecorder()

	controller.Update(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusOK {
		t.Fatalf("Update() status = %d, want %d", httpResponse.Code, http.StatusOK)
	}

	var responseBody struct {
		ID       string `json:"id"`
		Priority string `json:"priority"`
	}

	if err := json.NewDecoder(httpResponse.Body).Decode(&responseBody); err != nil {
		t.Fatalf("json.Decode() returned error: %v", err)
	}

	if responseBody.ID != taskID {
		t.Fatalf("response ID = %q, want %q", responseBody.ID, taskID)
	}

	if responseBody.Priority != string(models.TaskPriorityHigh) {
		t.Fatalf("response priority = %q, want %q", responseBody.Priority, models.TaskPriorityHigh)
	}
}

var _ services.TaskService = stubTaskService{}
