package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"to-do-list/internal/models"
	"to-do-list/internal/services"
)

type TaskController struct {
	service        services.TaskService
	logger         *slog.Logger
	requestTimeout time.Duration
}

const maxRequestBodyBytes int64 = 1 << 20

type errorResponse struct {
	Error string `json:"error"`
}

type taskResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	DueDate     string    `json:"due_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type paginatedTaskResponse struct {
	Items        []taskResponse `json:"items"`
	TotalItems   int64          `json:"total_items"`
	Page         int            `json:"page"`
	PageSize     int            `json:"page_size"`
	TotalPages   int            `json:"total_pages"`
	PreviousPage *int           `json:"previous_page,omitempty"`
	NextPage     *int           `json:"next_page,omitempty"`
}

func NewTaskController(service services.TaskService, logger *slog.Logger, requestTimeout time.Duration) *TaskController {
	return &TaskController{
		service:        service,
		logger:         logger,
		requestTimeout: requestTimeout,
	}
}

func (controller *TaskController) Create(responseWriter http.ResponseWriter, request *http.Request) {
	var createTaskInput models.CreateTaskInput
	if err := decodeJSON(responseWriter, request, &createTaskInput); err != nil {
		controller.respondError(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	requestContext, cancelRequest := controller.requestContext(request)
	defer cancelRequest()

	createdTask, err := controller.service.Create(requestContext, createTaskInput)
	if err != nil {
		controller.handleServiceError(responseWriter, request, err)
		return
	}

	controller.respondJSON(responseWriter, request, http.StatusCreated, toTaskResponse(*createdTask))
}

func (controller *TaskController) List(responseWriter http.ResponseWriter, request *http.Request) {
	page, err := parseOptionalIntQuery(request.URL.Query().Get("page"), "page")
	if err != nil {
		controller.respondError(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	pageSize, err := parseOptionalIntQuery(request.URL.Query().Get("page_size"), "page_size")
	if err != nil {
		controller.respondError(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	requestContext, cancelRequest := controller.requestContext(request)
	defer cancelRequest()

	listTaskInput := models.ListTaskInput{
		Status:   request.URL.Query().Get("status"),
		Priority: request.URL.Query().Get("priority"),
		Page:     page,
		PageSize: pageSize,
	}

	taskPage, err := controller.service.List(requestContext, listTaskInput)
	if err != nil {
		controller.handleServiceError(responseWriter, request, err)
		return
	}

	controller.respondJSON(responseWriter, request, http.StatusOK, toPaginatedTaskResponse(*taskPage))
}

func (controller *TaskController) GetByID(responseWriter http.ResponseWriter, request *http.Request) {
	requestContext, cancelRequest := controller.requestContext(request)
	defer cancelRequest()

	taskID := request.PathValue("id")

	task, err := controller.service.GetByID(requestContext, taskID)
	if err != nil {
		controller.handleServiceError(responseWriter, request, err)
		return
	}

	controller.respondJSON(responseWriter, request, http.StatusOK, toTaskResponse(*task))
}

func (controller *TaskController) Update(responseWriter http.ResponseWriter, request *http.Request) {
	var updateTaskInput models.UpdateTaskInput
	if err := decodeJSON(responseWriter, request, &updateTaskInput); err != nil {
		controller.respondError(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	requestContext, cancelRequest := controller.requestContext(request)
	defer cancelRequest()

	taskID := request.PathValue("id")

	updatedTask, err := controller.service.Update(requestContext, taskID, updateTaskInput)
	if err != nil {
		controller.handleServiceError(responseWriter, request, err)
		return
	}

	controller.respondJSON(responseWriter, request, http.StatusOK, toTaskResponse(*updatedTask))
}

func (controller *TaskController) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	requestContext, cancelRequest := controller.requestContext(request)
	defer cancelRequest()

	taskID := request.PathValue("id")

	if err := controller.service.Delete(requestContext, taskID); err != nil {
		controller.handleServiceError(responseWriter, request, err)
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (controller *TaskController) handleServiceError(responseWriter http.ResponseWriter, request *http.Request, serviceError error) {
	var validationError *services.ValidationError

	switch {
	case errors.As(serviceError, &validationError):
		controller.respondError(responseWriter, request, http.StatusBadRequest, validationError.Error())
	case errors.Is(serviceError, services.ErrInvalidTaskID):
		controller.respondError(responseWriter, request, http.StatusBadRequest, services.ErrInvalidTaskID.Error())
	case errors.Is(serviceError, services.ErrTaskNotFound):
		controller.respondError(responseWriter, request, http.StatusNotFound, services.ErrTaskNotFound.Error())
	case errors.Is(serviceError, services.ErrTaskCompleted):
		controller.respondError(responseWriter, request, http.StatusConflict, services.ErrTaskCompleted.Error())
	case errors.Is(serviceError, context.DeadlineExceeded):
		controller.logRequestError(request, slog.LevelError, "request timed out", serviceError)
		controller.respondError(responseWriter, request, http.StatusInternalServerError, "request timed out")
	case errors.Is(serviceError, context.Canceled):
		controller.logRequestError(request, slog.LevelWarn, "request canceled", serviceError)
	default:
		controller.logRequestError(request, slog.LevelError, "unexpected controller error", serviceError)
		controller.respondError(responseWriter, request, http.StatusInternalServerError, "internal server error")
	}
}

func decodeJSON(responseWriter http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)

	jsonDecoder := json.NewDecoder(request.Body)
	jsonDecoder.DisallowUnknownFields()

	if err := jsonDecoder.Decode(destination); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var maxBytesError *http.MaxBytesError

		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("request body contains malformed JSON at position %d", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("request body contains malformed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("request body contains invalid value for field %q", unmarshalTypeError.Field)
			}

			return fmt.Errorf("request body contains invalid value at position %d", unmarshalTypeError.Offset)
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("request body must not exceed %d bytes", maxRequestBodyBytes)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("request body contains unknown field %s", fieldName)
		default:
			return fmt.Errorf("invalid JSON: %w", err)
		}
	}

	if err := jsonDecoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func parseOptionalIntQuery(rawValue string, fieldName string) (int, error) {
	trimmedValue := strings.TrimSpace(rawValue)
	if trimmedValue == "" {
		return 0, nil
	}

	parsedValue, err := strconv.Atoi(trimmedValue)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", fieldName)
	}

	return parsedValue, nil
}

func (controller *TaskController) respondJSON(responseWriter http.ResponseWriter, request *http.Request, statusCode int, payload any) {
	if err := writeJSON(responseWriter, statusCode, payload); err != nil {
		controller.logResponseWriteError(request, statusCode, "failed to write JSON response", err)
	}
}

func (controller *TaskController) respondError(responseWriter http.ResponseWriter, request *http.Request, statusCode int, message string) {
	if err := writeJSON(responseWriter, statusCode, errorResponse{Error: message}); err != nil {
		controller.logResponseWriteError(request, statusCode, "failed to write error response", err)
	}
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) error {
	responseBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal response body: %w", err)
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)

	if _, err := responseWriter.Write(append(responseBody, '\n')); err != nil {
		return fmt.Errorf("write response body: %w", err)
	}

	return nil
}

func (controller *TaskController) requestContext(request *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(request.Context(), controller.requestTimeout)
}

func (controller *TaskController) logRequestError(request *http.Request, level slog.Level, message string, requestError error) {
	logAttributes := controller.requestLogAttrs(request, "error", requestError)

	switch level {
	case slog.LevelWarn:
		controller.logger.Warn(message, logAttributes...)
	default:
		controller.logger.Error(message, logAttributes...)
	}
}

func (controller *TaskController) logResponseWriteError(request *http.Request, statusCode int, message string, writeError error) {
	controller.logger.Error(message, controller.requestLogAttrs(request, "status", statusCode, "error", writeError)...)
}

func (controller *TaskController) requestLogAttrs(request *http.Request, attrs ...any) []any {
	baseAttributes := []any{
		"request_id", request.Header.Get("X-Request-ID"),
		"method", request.Method,
		"path", request.URL.Path,
	}

	return append(baseAttributes, attrs...)
}

func toTaskResponse(task models.Task) taskResponse {
	return taskResponse{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      string(task.Status),
		Priority:    string(task.Priority),
		DueDate:     task.DueDate.Format("2006-01-02"),
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}

func toTaskResponses(tasks []models.Task) []taskResponse {
	taskResponses := make([]taskResponse, 0, len(tasks))
	for _, task := range tasks {
		taskResponses = append(taskResponses, toTaskResponse(task))
	}

	return taskResponses
}

func toPaginatedTaskResponse(taskPage models.PaginatedTasks) paginatedTaskResponse {
	return paginatedTaskResponse{
		Items:        toTaskResponses(taskPage.Items),
		TotalItems:   taskPage.TotalItems,
		Page:         taskPage.Page,
		PageSize:     taskPage.PageSize,
		TotalPages:   taskPage.TotalPages,
		PreviousPage: taskPage.PreviousPage,
		NextPage:     taskPage.NextPage,
	}
}
