//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"to-do-list/internal/controllers"
	"to-do-list/internal/database"
	"to-do-list/internal/handlers"
	"to-do-list/internal/models"
	"to-do-list/internal/repositories"
	"to-do-list/internal/server"
	"to-do-list/internal/services"
	"to-do-list/internal/utils"
)

const (
	e2eRequestTimeout  = 5 * time.Second
	e2eCollectionName  = "tasks"
	defaultE2EMongoURI = "mongodb://localhost:27017"
)

type e2eTestApp struct {
	httpClient  *http.Client
	httpServer  *httptest.Server
	mongoClient *mongo.Client
	collection  *mongo.Collection
}

type taskResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	DueDate     string    `json:"due_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type paginatedTasksResponse struct {
	Items        []taskResponse `json:"items"`
	TotalItems   int64          `json:"total_items"`
	Page         int            `json:"page"`
	PageSize     int            `json:"page_size"`
	TotalPages   int            `json:"total_pages"`
	PreviousPage *int           `json:"previous_page"`
	NextPage     *int           `json:"next_page"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func TestTaskAPIE2ECreateAndGetTask(t *testing.T) {
	t.Parallel()

	testEnvironment := newE2ETestApp(t)
	createPayload := map[string]string{
		"title":       "Criar tarefa E2E",
		"description": "Persistir e consultar no Mongo",
		"priority":    "high",
		"due_date":    time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02"),
	}

	createHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodPost, "/tasks", createPayload)
	defer createHTTPResponse.Body.Close()

	if createHTTPResponse.StatusCode != http.StatusCreated {
		t.Fatalf("POST /tasks status = %d, want %d", createHTTPResponse.StatusCode, http.StatusCreated)
	}

	var createdTaskResponse taskResponse
	decodeJSONResponse(t, createHTTPResponse.Body, &createdTaskResponse)

	if createdTaskResponse.ID == "" {
		t.Fatal("POST /tasks returned empty ID")
	}

	storedTaskDocument := testEnvironment.findTaskInMongo(t, createdTaskResponse.ID)
	if storedTaskDocument.Title != createPayload["title"] {
		t.Fatalf("stored task title = %q, want %q", storedTaskDocument.Title, createPayload["title"])
	}

	if storedTaskDocument.Status != models.TaskStatusPending {
		t.Fatalf("stored task status = %q, want %q", storedTaskDocument.Status, models.TaskStatusPending)
	}

	getHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodGet, "/tasks/"+createdTaskResponse.ID, nil)
	defer getHTTPResponse.Body.Close()

	if getHTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /tasks/{id} status = %d, want %d", getHTTPResponse.StatusCode, http.StatusOK)
	}

	var fetchedTaskResponse taskResponse
	decodeJSONResponse(t, getHTTPResponse.Body, &fetchedTaskResponse)

	if fetchedTaskResponse.ID != createdTaskResponse.ID {
		t.Fatalf("GET /tasks/{id} returned ID = %q, want %q", fetchedTaskResponse.ID, createdTaskResponse.ID)
	}

	if fetchedTaskResponse.Description != createPayload["description"] {
		t.Fatalf("GET /tasks/{id} description = %q, want %q", fetchedTaskResponse.Description, createPayload["description"])
	}
}

func TestTaskAPIE2EListTasksWithFiltersAndPagination(t *testing.T) {
	t.Parallel()

	testEnvironment := newE2ETestApp(t)
	testEnvironment.insertTaskFixture(t, newTaskFixture("Task 1", models.TaskStatusPending, models.TaskPriorityHigh, 1))
	testEnvironment.insertTaskFixture(t, newTaskFixture("Task 2", models.TaskStatusPending, models.TaskPriorityHigh, 2))
	testEnvironment.insertTaskFixture(t, newTaskFixture("Task 3", models.TaskStatusPending, models.TaskPriorityHigh, 3))
	testEnvironment.insertTaskFixture(t, newTaskFixture("Task 4", models.TaskStatusPending, models.TaskPriorityHigh, 4))
	testEnvironment.insertTaskFixture(t, newTaskFixture("Task 5", models.TaskStatusPending, models.TaskPriorityHigh, 5))
	testEnvironment.insertTaskFixture(t, newTaskFixture("Ignored by status", models.TaskStatusCompleted, models.TaskPriorityHigh, 6))
	testEnvironment.insertTaskFixture(t, newTaskFixture("Ignored by priority", models.TaskStatusPending, models.TaskPriorityLow, 7))

	listHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodGet, "/tasks?status=pending&priority=high&page=2&page_size=2", nil)
	defer listHTTPResponse.Body.Close()

	if listHTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /tasks status = %d, want %d", listHTTPResponse.StatusCode, http.StatusOK)
	}

	var listTasksResponse paginatedTasksResponse
	decodeJSONResponse(t, listHTTPResponse.Body, &listTasksResponse)

	if listTasksResponse.TotalItems != 5 {
		t.Fatalf("GET /tasks total_items = %d, want 5", listTasksResponse.TotalItems)
	}

	if listTasksResponse.TotalPages != 3 {
		t.Fatalf("GET /tasks total_pages = %d, want 3", listTasksResponse.TotalPages)
	}

	if listTasksResponse.Page != 2 {
		t.Fatalf("GET /tasks page = %d, want 2", listTasksResponse.Page)
	}

	if listTasksResponse.PageSize != 2 {
		t.Fatalf("GET /tasks page_size = %d, want 2", listTasksResponse.PageSize)
	}

	if len(listTasksResponse.Items) != 2 {
		t.Fatalf("GET /tasks items length = %d, want 2", len(listTasksResponse.Items))
	}

	expectedTitles := []string{"Task 3", "Task 2"}
	for index, listedTask := range listTasksResponse.Items {
		if listedTask.Title != expectedTitles[index] {
			t.Fatalf("GET /tasks item %d title = %q, want %q", index, listedTask.Title, expectedTitles[index])
		}
	}
}

func TestTaskAPIE2EUpdateTaskPersistsChanges(t *testing.T) {
	t.Parallel()

	testEnvironment := newE2ETestApp(t)
	initialTaskDocument := newTaskFixture("Atualizar tarefa", models.TaskStatusPending, models.TaskPriorityMedium, 1)
	testEnvironment.insertTaskFixture(t, initialTaskDocument)

	updatePayload := map[string]string{
		"status":   "in_progress",
		"priority": "low",
		"title":    "Atualizar tarefa revisada",
	}

	updateHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodPut, "/tasks/"+initialTaskDocument.ID, updatePayload)
	defer updateHTTPResponse.Body.Close()

	if updateHTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("PUT /tasks/{id} status = %d, want %d", updateHTTPResponse.StatusCode, http.StatusOK)
	}

	var updatedTaskResponse taskResponse
	decodeJSONResponse(t, updateHTTPResponse.Body, &updatedTaskResponse)

	if updatedTaskResponse.Status != "in_progress" {
		t.Fatalf("PUT /tasks/{id} status = %q, want %q", updatedTaskResponse.Status, "in_progress")
	}

	updatedTaskDocument := testEnvironment.findTaskInMongo(t, initialTaskDocument.ID)
	if updatedTaskDocument.Title != updatePayload["title"] {
		t.Fatalf("stored task title = %q, want %q", updatedTaskDocument.Title, updatePayload["title"])
	}

	if updatedTaskDocument.Status != models.TaskStatusInProgress {
		t.Fatalf("stored task status = %q, want %q", updatedTaskDocument.Status, models.TaskStatusInProgress)
	}

	if updatedTaskDocument.Priority != models.TaskPriorityLow {
		t.Fatalf("stored task priority = %q, want %q", updatedTaskDocument.Priority, models.TaskPriorityLow)
	}
}

func TestTaskAPIE2EPatchTaskPersistsChanges(t *testing.T) {
	t.Parallel()

	testEnvironment := newE2ETestApp(t)
	initialTaskDocument := newTaskFixture("Atualizar parcialmente tarefa", models.TaskStatusPending, models.TaskPriorityMedium, 1)
	testEnvironment.insertTaskFixture(t, initialTaskDocument)

	updatePayload := map[string]string{
		"description": "Descricao ajustada via patch",
		"priority":    "high",
	}

	updateHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodPatch, "/tasks/"+initialTaskDocument.ID, updatePayload)
	defer updateHTTPResponse.Body.Close()

	if updateHTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("PATCH /tasks/{id} status = %d, want %d", updateHTTPResponse.StatusCode, http.StatusOK)
	}

	var updatedTaskResponse taskResponse
	decodeJSONResponse(t, updateHTTPResponse.Body, &updatedTaskResponse)

	if updatedTaskResponse.Priority != "high" {
		t.Fatalf("PATCH /tasks/{id} priority = %q, want %q", updatedTaskResponse.Priority, "high")
	}

	if updatedTaskResponse.Description != updatePayload["description"] {
		t.Fatalf("PATCH /tasks/{id} description = %q, want %q", updatedTaskResponse.Description, updatePayload["description"])
	}

	updatedTaskDocument := testEnvironment.findTaskInMongo(t, initialTaskDocument.ID)
	if updatedTaskDocument.Description != updatePayload["description"] {
		t.Fatalf("stored task description = %q, want %q", updatedTaskDocument.Description, updatePayload["description"])
	}

	if updatedTaskDocument.Priority != models.TaskPriorityHigh {
		t.Fatalf("stored task priority = %q, want %q", updatedTaskDocument.Priority, models.TaskPriorityHigh)
	}

	if updatedTaskDocument.Title != initialTaskDocument.Title {
		t.Fatalf("stored task title = %q, want %q", updatedTaskDocument.Title, initialTaskDocument.Title)
	}

	if updatedTaskDocument.Status != initialTaskDocument.Status {
		t.Fatalf("stored task status = %q, want %q", updatedTaskDocument.Status, initialTaskDocument.Status)
	}
}

func TestTaskAPIE2EUpdateCompletedTaskReturnsConflict(t *testing.T) {
	t.Parallel()

	testEnvironment := newE2ETestApp(t)
	completedTaskDocument := newTaskFixture("Tarefa concluida", models.TaskStatusCompleted, models.TaskPriorityHigh, 1)
	testEnvironment.insertTaskFixture(t, completedTaskDocument)

	updateHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodPut, "/tasks/"+completedTaskDocument.ID, map[string]string{
		"title": "Nao deveria atualizar",
	})
	defer updateHTTPResponse.Body.Close()

	if updateHTTPResponse.StatusCode != http.StatusConflict {
		t.Fatalf("PUT /tasks/{id} status = %d, want %d", updateHTTPResponse.StatusCode, http.StatusConflict)
	}

	var apiErrorResponse errorResponse
	decodeJSONResponse(t, updateHTTPResponse.Body, &apiErrorResponse)

	if apiErrorResponse.Error != services.ErrTaskCompleted.Error() {
		t.Fatalf("PUT /tasks/{id} error = %q, want %q", apiErrorResponse.Error, services.ErrTaskCompleted.Error())
	}

	unchangedTaskDocument := testEnvironment.findTaskInMongo(t, completedTaskDocument.ID)
	if unchangedTaskDocument.Title != completedTaskDocument.Title {
		t.Fatalf("stored task title = %q, want %q", unchangedTaskDocument.Title, completedTaskDocument.Title)
	}
}

func TestTaskAPIE2EPatchCompletedTaskReturnsConflict(t *testing.T) {
	t.Parallel()

	testEnvironment := newE2ETestApp(t)
	completedTaskDocument := newTaskFixture("Tarefa concluida via patch", models.TaskStatusCompleted, models.TaskPriorityHigh, 1)
	testEnvironment.insertTaskFixture(t, completedTaskDocument)

	updateHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodPatch, "/tasks/"+completedTaskDocument.ID, map[string]string{
		"priority": "low",
	})
	defer updateHTTPResponse.Body.Close()

	if updateHTTPResponse.StatusCode != http.StatusConflict {
		t.Fatalf("PATCH /tasks/{id} status = %d, want %d", updateHTTPResponse.StatusCode, http.StatusConflict)
	}

	var apiErrorResponse errorResponse
	decodeJSONResponse(t, updateHTTPResponse.Body, &apiErrorResponse)

	if apiErrorResponse.Error != services.ErrTaskCompleted.Error() {
		t.Fatalf("PATCH /tasks/{id} error = %q, want %q", apiErrorResponse.Error, services.ErrTaskCompleted.Error())
	}

	unchangedTaskDocument := testEnvironment.findTaskInMongo(t, completedTaskDocument.ID)
	if unchangedTaskDocument.Priority != completedTaskDocument.Priority {
		t.Fatalf("stored task priority = %q, want %q", unchangedTaskDocument.Priority, completedTaskDocument.Priority)
	}
}

func TestTaskAPIE2EDeleteTaskRemovesDocument(t *testing.T) {
	t.Parallel()

	testEnvironment := newE2ETestApp(t)
	taskToDelete := newTaskFixture("Remover tarefa", models.TaskStatusCompleted, models.TaskPriorityMedium, 1)
	testEnvironment.insertTaskFixture(t, taskToDelete)

	deleteHTTPResponse := testEnvironment.doJSONRequest(t, http.MethodDelete, "/tasks/"+taskToDelete.ID, nil)
	defer deleteHTTPResponse.Body.Close()

	if deleteHTTPResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /tasks/{id} status = %d, want %d", deleteHTTPResponse.StatusCode, http.StatusNoContent)
	}

	findDocumentContext, cancelFindDocument := context.WithTimeout(context.Background(), e2eRequestTimeout)
	defer cancelFindDocument()

	err := testEnvironment.collection.FindOne(findDocumentContext, bson.M{"_id": taskToDelete.ID}).Err()
	if err == nil {
		t.Fatal("DELETE /tasks/{id} did not remove the Mongo document")
	}

	if !errorsIsNoDocuments(err) {
		t.Fatalf("FindOne() error = %v, want mongo.ErrNoDocuments", err)
	}
}

func newE2ETestApp(t *testing.T) *e2eTestApp {
	t.Helper()

	mongoURI := strings.TrimSpace(os.Getenv("MONGO_URI"))
	if mongoURI == "" {
		mongoURI = defaultE2EMongoURI
	}

	connectContext, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConnect()

	mongoClient, err := database.Connect(connectContext, mongoURI)
	if err != nil {
		t.Fatalf("database.Connect() returned error: %v", err)
	}

	databaseName := "todo_e2e_" + strings.ReplaceAll(utils.NewUUID(), "-", "")
	testDatabase := mongoClient.Database(databaseName)
	taskRepository := repositories.NewMongoTaskRepository(testDatabase, e2eCollectionName)
	taskService := services.NewTaskService(taskRepository)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	taskController := controllers.NewTaskController(taskService, logger, e2eRequestTimeout)
	taskHandler := handlers.NewTaskHandler(taskController)
	testHTTPServer := httptest.NewServer(server.NewRouter(taskHandler, logger))

	t.Cleanup(func() {
		testHTTPServer.Close()

		dropDatabaseContext, cancelDropDatabase := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelDropDatabase()

		if err := testDatabase.Drop(dropDatabaseContext); err != nil {
			t.Fatalf("Database.Drop() returned error: %v", err)
		}

		disconnectMongoContext, cancelDisconnectMongo := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelDisconnectMongo()

		if err := mongoClient.Disconnect(disconnectMongoContext); err != nil {
			t.Fatalf("Disconnect() returned error: %v", err)
		}
	})

	return &e2eTestApp{
		httpClient:  testHTTPServer.Client(),
		httpServer:  testHTTPServer,
		mongoClient: mongoClient,
		collection:  testDatabase.Collection(e2eCollectionName),
	}
}

func (testEnvironment *e2eTestApp) doJSONRequest(t *testing.T, method string, path string, payload any) *http.Response {
	t.Helper()

	var httpRequestBody io.Reader = http.NoBody
	if payload != nil {
		requestPayloadJSON, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() returned error: %v", err)
		}

		httpRequestBody = bytes.NewReader(requestPayloadJSON)
	}

	httpRequest, err := http.NewRequest(method, testEnvironment.httpServer.URL+path, httpRequestBody)
	if err != nil {
		t.Fatalf("http.NewRequest() returned error: %v", err)
	}

	if payload != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	httpResponse, err := testEnvironment.httpClient.Do(httpRequest)
	if err != nil {
		t.Fatalf("httpClient.Do() returned error: %v", err)
	}

	return httpResponse
}

func (testEnvironment *e2eTestApp) insertTaskFixture(t *testing.T, task models.Task) {
	t.Helper()

	insertDocumentContext, cancelInsertDocument := context.WithTimeout(context.Background(), e2eRequestTimeout)
	defer cancelInsertDocument()

	if _, err := testEnvironment.collection.InsertOne(insertDocumentContext, task); err != nil {
		t.Fatalf("InsertOne() returned error: %v", err)
	}
}

func (testEnvironment *e2eTestApp) findTaskInMongo(t *testing.T, taskID string) models.Task {
	t.Helper()

	findDocumentContext, cancelFindDocument := context.WithTimeout(context.Background(), e2eRequestTimeout)
	defer cancelFindDocument()

	var storedTaskDocument models.Task
	if err := testEnvironment.collection.FindOne(findDocumentContext, bson.M{"_id": taskID}).Decode(&storedTaskDocument); err != nil {
		t.Fatalf("FindOne().Decode() returned error: %v", err)
	}

	return storedTaskDocument
}

func newTaskFixture(title string, status models.TaskStatus, priority models.TaskPriority, creationOrder int) models.Task {
	baseTime := time.Date(2030, time.January, 10, 12, 0, 0, 0, time.UTC)
	creationTimestamp := baseTime.Add(time.Duration(creationOrder) * time.Hour)

	return models.Task{
		ID:          utils.NewUUID(),
		Title:       title,
		Description: "Fixture de teste",
		Status:      status,
		Priority:    priority,
		DueDate:     baseTime.AddDate(0, 0, creationOrder),
		CreatedAt:   creationTimestamp,
		UpdatedAt:   creationTimestamp,
	}
}

func decodeJSONResponse(t *testing.T, responseBody io.Reader, destination any) {
	t.Helper()

	if err := json.NewDecoder(responseBody).Decode(destination); err != nil {
		t.Fatalf("json.Decode() returned error: %v", err)
	}
}

func errorsIsNoDocuments(err error) bool {
	return err == mongo.ErrNoDocuments
}
