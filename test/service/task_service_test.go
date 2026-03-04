package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"to-do-list/internal/models"
	"to-do-list/internal/repositories"
	taskservices "to-do-list/internal/services"
	"to-do-list/internal/utils"
)

type stubTaskRepository struct {
	createFunc  func(context.Context, *models.Task) error
	listFunc    func(context.Context, models.TaskListQuery) (models.TaskListResult, error)
	getByIDFunc func(context.Context, string) (*models.Task, error)
	updateFunc  func(context.Context, string, models.TaskUpdate) (*models.Task, error)
	deleteFunc  func(context.Context, string) error
}

func (repositoryStub stubTaskRepository) Create(ctx context.Context, task *models.Task) error {
	if repositoryStub.createFunc != nil {
		return repositoryStub.createFunc(ctx, task)
	}

	return nil
}

func (repositoryStub stubTaskRepository) List(ctx context.Context, listQuery models.TaskListQuery) (models.TaskListResult, error) {
	if repositoryStub.listFunc != nil {
		return repositoryStub.listFunc(ctx, listQuery)
	}

	return models.TaskListResult{}, nil
}

func (repositoryStub stubTaskRepository) GetByID(ctx context.Context, taskID string) (*models.Task, error) {
	if repositoryStub.getByIDFunc != nil {
		return repositoryStub.getByIDFunc(ctx, taskID)
	}

	return nil, repositories.ErrNotFound
}

func (repositoryStub stubTaskRepository) Update(ctx context.Context, taskID string, taskUpdate models.TaskUpdate) (*models.Task, error) {
	if repositoryStub.updateFunc != nil {
		return repositoryStub.updateFunc(ctx, taskID, taskUpdate)
	}

	return nil, repositories.ErrNotFound
}

func (repositoryStub stubTaskRepository) Delete(ctx context.Context, taskID string) error {
	if repositoryStub.deleteFunc != nil {
		return repositoryStub.deleteFunc(ctx, taskID)
	}

	return nil
}

func TestTaskServiceCreateAssignsDefaultsAndTrimsFields(t *testing.T) {
	t.Parallel()

	var persistedTask *models.Task

	taskService := taskservices.NewTaskService(stubTaskRepository{
		createFunc: func(_ context.Context, task *models.Task) error {
			taskCopy := *task
			persistedTask = &taskCopy
			return nil
		},
	})

	dueDate := time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02")

	createdTask, err := taskService.Create(context.Background(), models.CreateTaskInput{
		Title:       "  Escrever testes  ",
		Description: "  Cobrir regras de negocio  ",
		Priority:    "high",
		DueDate:     dueDate,
	})
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	expectedDueDate, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		t.Fatalf("time.Parse() returned error: %v", err)
	}

	if persistedTask == nil {
		t.Fatal("Create() did not persist the task")
	}

	if createdTask.ID == "" || !utils.IsValidUUID(createdTask.ID) {
		t.Fatalf("Create() returned invalid task ID: %q", createdTask.ID)
	}

	if createdTask.Title != "Escrever testes" {
		t.Fatalf("Create() title = %q, want %q", createdTask.Title, "Escrever testes")
	}

	if createdTask.Description != "Cobrir regras de negocio" {
		t.Fatalf("Create() description = %q, want %q", createdTask.Description, "Cobrir regras de negocio")
	}

	if createdTask.Status != models.TaskStatusPending {
		t.Fatalf("Create() status = %q, want %q", createdTask.Status, models.TaskStatusPending)
	}

	if createdTask.Priority != models.TaskPriorityHigh {
		t.Fatalf("Create() priority = %q, want %q", createdTask.Priority, models.TaskPriorityHigh)
	}

	if !createdTask.DueDate.Equal(expectedDueDate.UTC()) {
		t.Fatalf("Create() due date = %v, want %v", createdTask.DueDate, expectedDueDate.UTC())
	}

	if createdTask.CreatedAt.IsZero() {
		t.Fatal("Create() returned zero CreatedAt")
	}

	if !createdTask.CreatedAt.Equal(createdTask.UpdatedAt) {
		t.Fatalf("Create() timestamps mismatch: created_at=%v updated_at=%v", createdTask.CreatedAt, createdTask.UpdatedAt)
	}

	if persistedTask.ID != createdTask.ID {
		t.Fatalf("persisted task ID = %q, want %q", persistedTask.ID, createdTask.ID)
	}
}

func TestTaskServiceCreateRejectsPastDueDate(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{})

	_, err := taskService.Create(context.Background(), models.CreateTaskInput{
		Title:    "Tarefa valida",
		Priority: "medium",
		DueDate:  time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"),
	})
	if err == nil {
		t.Fatal("Create() error = nil, want validation error")
	}

	var validationErr *taskservices.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Create() error = %v, want ValidationError", err)
	}

	if validationErr.Message != "due_date cannot be in the past" {
		t.Fatalf("Create() validation message = %q, want %q", validationErr.Message, "due_date cannot be in the past")
	}
}

func TestTaskServiceCreateAcceptsAllowedPriorities(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		priority string
		want     models.TaskPriority
	}{
		{name: "low", priority: "low", want: models.TaskPriorityLow},
		{name: "medium", priority: "medium", want: models.TaskPriorityMedium},
		{name: "high", priority: "high", want: models.TaskPriorityHigh},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			taskService := taskservices.NewTaskService(stubTaskRepository{})

			createdTask, err := taskService.Create(context.Background(), models.CreateTaskInput{
				Title:    "Prioridade valida",
				Priority: testCase.priority,
				DueDate:  time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02"),
			})
			if err != nil {
				t.Fatalf("Create() returned error: %v", err)
			}

			if createdTask.Priority != testCase.want {
				t.Fatalf("Create() priority = %q, want %q", createdTask.Priority, testCase.want)
			}
		})
	}
}

func TestTaskServiceCreateRejectsInvalidPriority(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{})

	_, err := taskService.Create(context.Background(), models.CreateTaskInput{
		Title:    "Prioridade invalida",
		Priority: "urgent",
		DueDate:  time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02"),
	})

	assertValidationErrorMessage(t, err, "priority must be one of: low, medium, high")
}

func TestTaskServiceCreateRejectsInvalidTitle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		title   string
		message string
	}{
		{name: "missing", title: "   ", message: "title is required"},
		{name: "too short", title: "ab", message: "title must be between 3 and 100 characters"},
		{name: "too long", title: strings.Repeat("a", 101), message: "title must be between 3 and 100 characters"},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			taskService := taskservices.NewTaskService(stubTaskRepository{})

			_, err := taskService.Create(context.Background(), models.CreateTaskInput{
				Title:    testCase.title,
				Priority: "medium",
				DueDate:  time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02"),
			})

			assertValidationErrorMessage(t, err, testCase.message)
		})
	}
}

func TestTaskServiceListNormalizesFiltersAndCalculatesPagination(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{
		listFunc: func(_ context.Context, listQuery models.TaskListQuery) (models.TaskListResult, error) {
			if listQuery.Page != 1 {
				t.Fatalf("List() query.Page = %d, want 1", listQuery.Page)
			}

			if listQuery.PageSize != 10 {
				t.Fatalf("List() query.PageSize = %d, want 10", listQuery.PageSize)
			}

			if listQuery.Filter.Status == nil || *listQuery.Filter.Status != models.TaskStatusPending {
				t.Fatalf("List() query.Filter.Status = %v, want %q", listQuery.Filter.Status, models.TaskStatusPending)
			}

			if listQuery.Filter.Priority == nil || *listQuery.Filter.Priority != models.TaskPriorityHigh {
				t.Fatalf("List() query.Filter.Priority = %v, want %q", listQuery.Filter.Priority, models.TaskPriorityHigh)
			}

			return models.TaskListResult{
				Items: []models.Task{
					{ID: utils.NewUUID(), Title: "Task 1"},
					{ID: utils.NewUUID(), Title: "Task 2"},
				},
				TotalItems: 25,
			}, nil
		},
	})

	paginatedTasks, err := taskService.List(context.Background(), models.ListTaskInput{
		Status:   "pending",
		Priority: "high",
	})
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	if paginatedTasks.Page != 1 {
		t.Fatalf("List() page = %d, want 1", paginatedTasks.Page)
	}

	if paginatedTasks.PageSize != 10 {
		t.Fatalf("List() page size = %d, want 10", paginatedTasks.PageSize)
	}

	if paginatedTasks.TotalItems != 25 {
		t.Fatalf("List() total items = %d, want 25", paginatedTasks.TotalItems)
	}

	if paginatedTasks.TotalPages != 3 {
		t.Fatalf("List() total pages = %d, want 3", paginatedTasks.TotalPages)
	}

	if paginatedTasks.PreviousPage != nil {
		t.Fatalf("List() previous page = %v, want nil", *paginatedTasks.PreviousPage)
	}

	if paginatedTasks.NextPage == nil || *paginatedTasks.NextPage != 2 {
		t.Fatalf("List() next page = %v, want 2", paginatedTasks.NextPage)
	}
}

func TestTaskServiceListRejectsPageSizeAboveLimit(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{
		listFunc: func(_ context.Context, _ models.TaskListQuery) (models.TaskListResult, error) {
			t.Fatal("List() should not call repository on invalid page size")
			return models.TaskListResult{}, nil
		},
	})

	_, err := taskService.List(context.Background(), models.ListTaskInput{PageSize: 101})
	if err == nil {
		t.Fatal("List() error = nil, want validation error")
	}

	var validationErr *taskservices.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("List() error = %v, want ValidationError", err)
	}

	if validationErr.Message != "page_size must be less than or equal to 100" {
		t.Fatalf("List() validation message = %q, want %q", validationErr.Message, "page_size must be less than or equal to 100")
	}
}

func TestTaskServiceListRejectsInvalidStatusFilter(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{
		listFunc: func(_ context.Context, _ models.TaskListQuery) (models.TaskListResult, error) {
			t.Fatal("List() should not call repository on invalid status filter")
			return models.TaskListResult{}, nil
		},
	})

	_, err := taskService.List(context.Background(), models.ListTaskInput{Status: "archived"})

	assertValidationErrorMessage(t, err, "status must be one of: pending, in_progress, completed, cancelled")
}

func TestTaskServiceListRejectsInvalidPriorityFilter(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{
		listFunc: func(_ context.Context, _ models.TaskListQuery) (models.TaskListResult, error) {
			t.Fatal("List() should not call repository on invalid priority filter")
			return models.TaskListResult{}, nil
		},
	})

	_, err := taskService.List(context.Background(), models.ListTaskInput{Priority: "urgent"})

	assertValidationErrorMessage(t, err, "priority must be one of: low, medium, high")
}

func TestTaskServiceUpdateRejectsCompletedTask(t *testing.T) {
	t.Parallel()

	taskID := utils.NewUUID()
	completedTask := &models.Task{
		ID:     taskID,
		Title:  "Tarefa concluida",
		Status: models.TaskStatusCompleted,
	}

	taskService := taskservices.NewTaskService(stubTaskRepository{
		getByIDFunc: func(_ context.Context, id string) (*models.Task, error) {
			if id != taskID {
				t.Fatalf("GetByID() id = %q, want %q", id, taskID)
			}

			taskCopy := *completedTask
			return &taskCopy, nil
		},
		updateFunc: func(_ context.Context, _ string, _ models.TaskUpdate) (*models.Task, error) {
			t.Fatal("Update() should not call repository for completed task")
			return nil, nil
		},
	})

	_, err := taskService.Update(context.Background(), taskID, models.UpdateTaskInput{
		Title: stringPointer("Novo titulo"),
	})
	if !errors.Is(err, taskservices.ErrTaskCompleted) {
		t.Fatalf("Update() error = %v, want %v", err, taskservices.ErrTaskCompleted)
	}
}

func TestTaskServiceUpdateAcceptsAllowedStatuses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status string
		want   models.TaskStatus
	}{
		{name: "pending", status: "pending", want: models.TaskStatusPending},
		{name: "in progress", status: "in_progress", want: models.TaskStatusInProgress},
		{name: "completed", status: "completed", want: models.TaskStatusCompleted},
		{name: "cancelled", status: "cancelled", want: models.TaskStatusCancelled},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			taskID := utils.NewUUID()
			taskService := taskservices.NewTaskService(stubTaskRepository{
				getByIDFunc: func(_ context.Context, id string) (*models.Task, error) {
					if id != taskID {
						t.Fatalf("GetByID() id = %q, want %q", id, taskID)
					}

					return &models.Task{
						ID:     taskID,
						Title:  "Atualizar status",
						Status: models.TaskStatusPending,
					}, nil
				},
				updateFunc: func(_ context.Context, id string, taskUpdate models.TaskUpdate) (*models.Task, error) {
					if id != taskID {
						t.Fatalf("Update() id = %q, want %q", id, taskID)
					}

					if taskUpdate.Status == nil {
						t.Fatal("Update() status was nil")
					}

					if *taskUpdate.Status != testCase.want {
						t.Fatalf("Update() normalized status = %q, want %q", *taskUpdate.Status, testCase.want)
					}

					return &models.Task{
						ID:     taskID,
						Title:  "Atualizar status",
						Status: *taskUpdate.Status,
					}, nil
				},
			})

			updatedTask, err := taskService.Update(context.Background(), taskID, models.UpdateTaskInput{
				Status: stringPointer(testCase.status),
			})
			if err != nil {
				t.Fatalf("Update() returned error: %v", err)
			}

			if updatedTask.Status != testCase.want {
				t.Fatalf("Update() status = %q, want %q", updatedTask.Status, testCase.want)
			}
		})
	}
}

func TestTaskServiceUpdateRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{})

	_, err := taskService.Update(context.Background(), utils.NewUUID(), models.UpdateTaskInput{
		Status: stringPointer("archived"),
	})

	assertValidationErrorMessage(t, err, "status must be one of: pending, in_progress, completed, cancelled")
}

func TestTaskServiceUpdateRejectsInvalidPriority(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{})

	_, err := taskService.Update(context.Background(), utils.NewUUID(), models.UpdateTaskInput{
		Priority: stringPointer("urgent"),
	})

	assertValidationErrorMessage(t, err, "priority must be one of: low, medium, high")
}

func TestTaskServiceUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{})

	_, err := taskService.Update(context.Background(), utils.NewUUID(), models.UpdateTaskInput{})
	if err == nil {
		t.Fatal("Update() error = nil, want validation error")
	}

	var validationErr *taskservices.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Update() error = %v, want ValidationError", err)
	}

	if validationErr.Message != "at least one field must be provided" {
		t.Fatalf("Update() validation message = %q, want %q", validationErr.Message, "at least one field must be provided")
	}
}

func TestTaskServiceUpdateRejectsInvalidTitle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		title   string
		message string
	}{
		{name: "missing", title: "   ", message: "title is required"},
		{name: "too short", title: "ab", message: "title must be between 3 and 100 characters"},
		{name: "too long", title: strings.Repeat("b", 101), message: "title must be between 3 and 100 characters"},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			taskService := taskservices.NewTaskService(stubTaskRepository{})

			_, err := taskService.Update(context.Background(), utils.NewUUID(), models.UpdateTaskInput{
				Title: stringPointer(testCase.title),
			})

			assertValidationErrorMessage(t, err, testCase.message)
		})
	}
}

func TestTaskServiceUpdateRejectsPastDueDate(t *testing.T) {
	t.Parallel()

	taskService := taskservices.NewTaskService(stubTaskRepository{})

	_, err := taskService.Update(context.Background(), utils.NewUUID(), models.UpdateTaskInput{
		DueDate: stringPointer(time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")),
	})

	assertValidationErrorMessage(t, err, "due_date cannot be in the past")
}

func TestTaskServiceDeleteAllowsCompletedTask(t *testing.T) {
	t.Parallel()

	taskID := utils.NewUUID()
	taskService := taskservices.NewTaskService(stubTaskRepository{
		deleteFunc: func(_ context.Context, id string) error {
			if id != taskID {
				t.Fatalf("Delete() id = %q, want %q", id, taskID)
			}

			return nil
		},
	})

	if err := taskService.Delete(context.Background(), taskID); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}

func assertValidationErrorMessage(t *testing.T, err error, expectedMessage string) {
	t.Helper()

	if err == nil {
		t.Fatal("error = nil, want validation error")
	}

	var validationErr *taskservices.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want ValidationError", err)
	}

	if validationErr.Message != expectedMessage {
		t.Fatalf("validation message = %q, want %q", validationErr.Message, expectedMessage)
	}
}
