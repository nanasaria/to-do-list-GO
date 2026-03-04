package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"to-do-list/internal/controllers"
	"to-do-list/internal/handlers"
	"to-do-list/internal/models"
	"to-do-list/internal/repositories"
	"to-do-list/internal/server"
	"to-do-list/internal/services"
	"to-do-list/internal/utils"
)

type inMemoryTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]models.Task
}

func newInMemoryTaskRepository(tasks ...models.Task) *inMemoryTaskRepository {
	inMemoryRepository := &inMemoryTaskRepository{
		tasks: make(map[string]models.Task, len(tasks)),
	}

	for _, task := range tasks {
		inMemoryRepository.tasks[task.ID] = task
	}

	return inMemoryRepository
}

func (inMemoryRepository *inMemoryTaskRepository) Create(_ context.Context, task *models.Task) error {
	inMemoryRepository.mu.Lock()
	defer inMemoryRepository.mu.Unlock()

	inMemoryRepository.tasks[task.ID] = cloneTask(*task)
	return nil
}

func (inMemoryRepository *inMemoryTaskRepository) List(_ context.Context, listQuery models.TaskListQuery) (models.TaskListResult, error) {
	inMemoryRepository.mu.RLock()
	defer inMemoryRepository.mu.RUnlock()

	filteredTasks := make([]models.Task, 0, len(inMemoryRepository.tasks))
	for _, task := range inMemoryRepository.tasks {
		if listQuery.Filter.Status != nil && task.Status != *listQuery.Filter.Status {
			continue
		}

		if listQuery.Filter.Priority != nil && task.Priority != *listQuery.Filter.Priority {
			continue
		}

		filteredTasks = append(filteredTasks, cloneTask(task))
	}

	sort.Slice(filteredTasks, func(i, j int) bool {
		return filteredTasks[i].CreatedAt.After(filteredTasks[j].CreatedAt)
	})

	totalItems := int64(len(filteredTasks))
	startIndex := (listQuery.Page - 1) * listQuery.PageSize
	if startIndex >= len(filteredTasks) {
		return models.TaskListResult{
			Items:      []models.Task{},
			TotalItems: totalItems,
		}, nil
	}

	endIndex := startIndex + listQuery.PageSize
	if endIndex > len(filteredTasks) {
		endIndex = len(filteredTasks)
	}

	pageItems := make([]models.Task, 0, endIndex-startIndex)
	for _, task := range filteredTasks[startIndex:endIndex] {
		pageItems = append(pageItems, cloneTask(task))
	}

	return models.TaskListResult{
		Items:      pageItems,
		TotalItems: totalItems,
	}, nil
}

func (inMemoryRepository *inMemoryTaskRepository) GetByID(_ context.Context, taskID string) (*models.Task, error) {
	inMemoryRepository.mu.RLock()
	defer inMemoryRepository.mu.RUnlock()

	task, ok := inMemoryRepository.tasks[taskID]
	if !ok {
		return nil, repositories.ErrNotFound
	}

	taskCopy := cloneTask(task)
	return &taskCopy, nil
}

func (inMemoryRepository *inMemoryTaskRepository) Update(_ context.Context, taskID string, taskUpdate models.TaskUpdate) (*models.Task, error) {
	inMemoryRepository.mu.Lock()
	defer inMemoryRepository.mu.Unlock()

	task, ok := inMemoryRepository.tasks[taskID]
	if !ok {
		return nil, repositories.ErrNotFound
	}

	if taskUpdate.Title != nil {
		task.Title = *taskUpdate.Title
	}

	if taskUpdate.Description != nil {
		task.Description = *taskUpdate.Description
	}

	if taskUpdate.Status != nil {
		task.Status = *taskUpdate.Status
	}

	if taskUpdate.Priority != nil {
		task.Priority = *taskUpdate.Priority
	}

	if taskUpdate.DueDate != nil {
		task.DueDate = *taskUpdate.DueDate
	}

	task.UpdatedAt = time.Now().UTC()
	inMemoryRepository.tasks[taskID] = task

	taskCopy := cloneTask(task)
	return &taskCopy, nil
}

func (inMemoryRepository *inMemoryTaskRepository) Delete(_ context.Context, taskID string) error {
	inMemoryRepository.mu.Lock()
	defer inMemoryRepository.mu.Unlock()

	if _, ok := inMemoryRepository.tasks[taskID]; !ok {
		return repositories.ErrNotFound
	}

	delete(inMemoryRepository.tasks, taskID)
	return nil
}

func TestRouterListTasksReturnsPaginatedResponse(t *testing.T) {
	t.Parallel()

	taskRepository := newInMemoryTaskRepository(seedTasksForPagination()...)
	httpHandler := newTestRouter(taskRepository)

	httpRequest := httptest.NewRequest(http.MethodGet, "/tasks?status=pending&priority=high&page=2&page_size=3", nil)
	httpResponse := httptest.NewRecorder()

	httpHandler.ServeHTTP(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusOK)
	}

	responseRequestID := httpResponse.Header().Get("X-Request-ID")
	if responseRequestID == "" {
		t.Fatal("ServeHTTP() response is missing X-Request-ID header")
	}

	var responsePayload struct {
		Items []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Status   string `json:"status"`
			Priority string `json:"priority"`
		} `json:"items"`
		TotalItems   int64 `json:"total_items"`
		Page         int   `json:"page"`
		PageSize     int   `json:"page_size"`
		TotalPages   int   `json:"total_pages"`
		PreviousPage *int  `json:"previous_page"`
		NextPage     *int  `json:"next_page"`
	}

	if err := json.NewDecoder(httpResponse.Body).Decode(&responsePayload); err != nil {
		t.Fatalf("json.Decode() returned error: %v", err)
	}

	if responsePayload.TotalItems != 8 {
		t.Fatalf("response total_items = %d, want 8", responsePayload.TotalItems)
	}

	if responsePayload.Page != 2 {
		t.Fatalf("response page = %d, want 2", responsePayload.Page)
	}

	if responsePayload.PageSize != 3 {
		t.Fatalf("response page_size = %d, want 3", responsePayload.PageSize)
	}

	if responsePayload.TotalPages != 3 {
		t.Fatalf("response total_pages = %d, want 3", responsePayload.TotalPages)
	}

	if responsePayload.PreviousPage == nil || *responsePayload.PreviousPage != 1 {
		t.Fatalf("response previous_page = %v, want 1", responsePayload.PreviousPage)
	}

	if responsePayload.NextPage == nil || *responsePayload.NextPage != 3 {
		t.Fatalf("response next_page = %v, want 3", responsePayload.NextPage)
	}

	if len(responsePayload.Items) != 3 {
		t.Fatalf("response items length = %d, want 3", len(responsePayload.Items))
	}

	expectedTitles := []string{"Task 5", "Task 4", "Task 3"}
	for index, item := range responsePayload.Items {
		if item.Title != expectedTitles[index] {
			t.Fatalf("response item %d title = %q, want %q", index, item.Title, expectedTitles[index])
		}

		if item.Status != string(models.TaskStatusPending) {
			t.Fatalf("response item %d status = %q, want %q", index, item.Status, models.TaskStatusPending)
		}

		if item.Priority != string(models.TaskPriorityHigh) {
			t.Fatalf("response item %d priority = %q, want %q", index, item.Priority, models.TaskPriorityHigh)
		}
	}
}

func TestRouterListTasksRejectsPageSizeAboveLimit(t *testing.T) {
	t.Parallel()

	httpHandler := newTestRouter(newInMemoryTaskRepository())

	httpRequest := httptest.NewRequest(http.MethodGet, "/tasks?page_size=101", nil)
	httpResponse := httptest.NewRecorder()

	httpHandler.ServeHTTP(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusBadRequest {
		t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusBadRequest)
	}

	assertJSONError(t, httpResponse.Body, "page_size must be less than or equal to 100")
}

func TestRouterCreateTaskRejectsPastDueDate(t *testing.T) {
	t.Parallel()

	httpHandler := newTestRouter(newInMemoryTaskRepository())

	requestPayload := map[string]string{
		"title":    "Criar testes",
		"priority": "high",
		"due_date": time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"),
	}

	httpRequest := httptest.NewRequest(http.MethodPost, "/tasks", marshalJSON(t, requestPayload))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse := httptest.NewRecorder()

	httpHandler.ServeHTTP(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusBadRequest {
		t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusBadRequest)
	}

	assertJSONError(t, httpResponse.Body, "due_date cannot be in the past")
}

func TestRouterCreateTaskRejectsInvalidTitle(t *testing.T) {
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

			httpHandler := newTestRouter(newInMemoryTaskRepository())
			httpRequest := httptest.NewRequest(http.MethodPost, "/tasks", marshalJSON(t, map[string]string{
				"title":    testCase.title,
				"priority": "medium",
				"due_date": time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02"),
			}))
			httpRequest.Header.Set("Content-Type", "application/json")
			httpResponse := httptest.NewRecorder()

			httpHandler.ServeHTTP(httpResponse, httpRequest)

			if httpResponse.Code != http.StatusBadRequest {
				t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusBadRequest)
			}

			assertJSONError(t, httpResponse.Body, testCase.message)
		})
	}
}

func TestRouterCreateTaskRejectsInvalidPriority(t *testing.T) {
	t.Parallel()

	httpHandler := newTestRouter(newInMemoryTaskRepository())

	httpRequest := httptest.NewRequest(http.MethodPost, "/tasks", marshalJSON(t, map[string]string{
		"title":    "Prioridade invalida",
		"priority": "urgent",
		"due_date": time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02"),
	}))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse := httptest.NewRecorder()

	httpHandler.ServeHTTP(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusBadRequest {
		t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusBadRequest)
	}

	assertJSONError(t, httpResponse.Body, "priority must be one of: low, medium, high")
}

func TestRouterUpdateCompletedTaskReturnsConflict(t *testing.T) {
	t.Parallel()

	completedTask := models.Task{
		ID:        utils.NewUUID(),
		Title:     "Task concluida",
		Status:    models.TaskStatusCompleted,
		Priority:  models.TaskPriorityMedium,
		DueDate:   time.Now().UTC().AddDate(0, 0, 2),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
		UpdatedAt: time.Now().UTC().Add(-1 * time.Hour),
	}

	httpHandler := newTestRouter(newInMemoryTaskRepository(completedTask))

	httpRequest := httptest.NewRequest(http.MethodPut, "/tasks/"+completedTask.ID, marshalJSON(t, map[string]string{
		"title": "Novo titulo",
	}))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse := httptest.NewRecorder()

	httpHandler.ServeHTTP(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusConflict {
		t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusConflict)
	}

	assertJSONError(t, httpResponse.Body, services.ErrTaskCompleted.Error())
}

func TestRouterUpdateRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	task := models.Task{
		ID:        utils.NewUUID(),
		Title:     "Task editavel",
		Status:    models.TaskStatusPending,
		Priority:  models.TaskPriorityMedium,
		DueDate:   time.Now().UTC().AddDate(0, 0, 2),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
		UpdatedAt: time.Now().UTC().Add(-1 * time.Hour),
	}

	httpHandler := newTestRouter(newInMemoryTaskRepository(task))

	httpRequest := httptest.NewRequest(http.MethodPut, "/tasks/"+task.ID, marshalJSON(t, map[string]string{
		"status": "archived",
	}))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse := httptest.NewRecorder()

	httpHandler.ServeHTTP(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusBadRequest {
		t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusBadRequest)
	}

	assertJSONError(t, httpResponse.Body, "status must be one of: pending, in_progress, completed, cancelled")
}

func TestRouterDeleteCompletedTaskReturnsNoContent(t *testing.T) {
	t.Parallel()

	completedTask := models.Task{
		ID:        utils.NewUUID(),
		Title:     "Task concluida",
		Status:    models.TaskStatusCompleted,
		Priority:  models.TaskPriorityMedium,
		DueDate:   time.Now().UTC().AddDate(0, 0, 2),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
		UpdatedAt: time.Now().UTC().Add(-1 * time.Hour),
	}

	taskRepository := newInMemoryTaskRepository(completedTask)
	httpHandler := newTestRouter(taskRepository)

	httpRequest := httptest.NewRequest(http.MethodDelete, "/tasks/"+completedTask.ID, nil)
	httpResponse := httptest.NewRecorder()

	httpHandler.ServeHTTP(httpResponse, httpRequest)

	if httpResponse.Code != http.StatusNoContent {
		t.Fatalf("ServeHTTP() status = %d, want %d", httpResponse.Code, http.StatusNoContent)
	}

	if _, err := taskRepository.GetByID(context.Background(), completedTask.ID); err == nil {
		t.Fatal("Delete() did not remove completed task")
	}
}

func newTestRouter(taskRepository repositories.TaskRepository) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	taskService := services.NewTaskService(taskRepository)
	controller := controllers.NewTaskController(taskService, logger, 5*time.Second)
	taskHandler := handlers.NewTaskHandler(controller)
	return server.NewRouter(taskHandler, logger)
}

func seedTasksForPagination() []models.Task {
	baseTime := time.Date(2030, time.January, 10, 12, 0, 0, 0, time.UTC)
	tasks := make([]models.Task, 0, 10)

	for taskNumber := 1; taskNumber <= 8; taskNumber++ {
		tasks = append(tasks, models.Task{
			ID:          utils.NewUUID(),
			Title:       "Task " + strconv.Itoa(taskNumber),
			Description: "Matching task",
			Status:      models.TaskStatusPending,
			Priority:    models.TaskPriorityHigh,
			DueDate:     baseTime.AddDate(0, 0, taskNumber),
			CreatedAt:   baseTime.Add(time.Duration(taskNumber) * time.Hour),
			UpdatedAt:   baseTime.Add(time.Duration(taskNumber) * time.Hour),
		})
	}

	tasks = append(tasks,
		models.Task{
			ID:        utils.NewUUID(),
			Title:     "Ignored by status",
			Status:    models.TaskStatusCompleted,
			Priority:  models.TaskPriorityHigh,
			DueDate:   baseTime.AddDate(0, 0, 20),
			CreatedAt: baseTime.Add(20 * time.Hour),
			UpdatedAt: baseTime.Add(20 * time.Hour),
		},
		models.Task{
			ID:        utils.NewUUID(),
			Title:     "Ignored by priority",
			Status:    models.TaskStatusPending,
			Priority:  models.TaskPriorityLow,
			DueDate:   baseTime.AddDate(0, 0, 21),
			CreatedAt: baseTime.Add(21 * time.Hour),
			UpdatedAt: baseTime.Add(21 * time.Hour),
		},
	)

	return tasks
}

func assertJSONError(t *testing.T, responseBody *bytes.Buffer, expectedMessage string) {
	t.Helper()

	var errorResponse struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(responseBody).Decode(&errorResponse); err != nil {
		t.Fatalf("json.Decode() returned error: %v", err)
	}

	if errorResponse.Error != expectedMessage {
		t.Fatalf("response error = %q, want %q", errorResponse.Error, expectedMessage)
	}
}

func marshalJSON(t *testing.T, payload any) io.Reader {
	t.Helper()

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() returned error: %v", err)
	}

	return bytes.NewReader(jsonBody)
}

func cloneTask(task models.Task) models.Task {
	return models.Task{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		Priority:    task.Priority,
		DueDate:     task.DueDate,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}
