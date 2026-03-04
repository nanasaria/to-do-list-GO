package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"to-do-list/internal/models"
	"to-do-list/internal/repositories"
	"to-do-list/internal/utils"
)

var (
	ErrInvalidTaskID = errors.New("invalid task id")
	ErrTaskNotFound  = errors.New("task not found")
	ErrTaskCompleted = errors.New("completed task cannot be updated")
)

const (
	minTaskTitleLength  = 3
	maxTaskTitleLength  = 100
	defaultTaskListPage = 1
	defaultTaskPageSize = 10
	maxTaskPageSize     = 100
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type TaskService interface {
	Create(ctx context.Context, input models.CreateTaskInput) (*models.Task, error)
	List(ctx context.Context, input models.ListTaskInput) (*models.PaginatedTasks, error)
	GetByID(ctx context.Context, rawID string) (*models.Task, error)
	Update(ctx context.Context, rawID string, input models.UpdateTaskInput) (*models.Task, error)
	Delete(ctx context.Context, rawID string) error
}

type taskService struct {
	repository repositories.TaskRepository
}

func NewTaskService(repository repositories.TaskRepository) TaskService {
	return &taskService{repository: repository}
}

func (service *taskService) Create(serviceContext context.Context, createTaskInput models.CreateTaskInput) (*models.Task, error) {
	title, err := normalizeRequiredTitle(createTaskInput.Title)
	if err != nil {
		return nil, err
	}

	priority, err := parseTaskPriority(createTaskInput.Priority)
	if err != nil {
		return nil, err
	}

	dueDate, err := parseDueDate(createTaskInput.DueDate)
	if err != nil {
		return nil, err
	}

	task := newTask(title, createTaskInput.Description, priority, dueDate)

	if err := service.repository.Create(serviceContext, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	return task, nil
}

func (service *taskService) List(serviceContext context.Context, listTaskInput models.ListTaskInput) (*models.PaginatedTasks, error) {
	listQuery, err := normalizeListTaskInput(listTaskInput)
	if err != nil {
		return nil, err
	}

	listResult, err := service.repository.List(serviceContext, listQuery)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	totalPages := calculateTotalPages(listResult.TotalItems, listQuery.PageSize)

	return &models.PaginatedTasks{
		Items:        listResult.Items,
		TotalItems:   listResult.TotalItems,
		Page:         listQuery.Page,
		PageSize:     listQuery.PageSize,
		TotalPages:   totalPages,
		PreviousPage: calculatePreviousPage(listQuery.Page),
		NextPage:     calculateNextPage(listQuery.Page, totalPages),
	}, nil
}

func (service *taskService) GetByID(serviceContext context.Context, rawTaskID string) (*models.Task, error) {
	taskID, err := parseTaskID(rawTaskID)
	if err != nil {
		return nil, err
	}

	return service.loadTask(serviceContext, taskID)
}

func (service *taskService) Update(serviceContext context.Context, rawTaskID string, updateTaskInput models.UpdateTaskInput) (*models.Task, error) {
	taskID, err := parseTaskID(rawTaskID)
	if err != nil {
		return nil, err
	}

	normalizedUpdate, err := normalizeUpdateInput(updateTaskInput)
	if err != nil {
		return nil, err
	}

	currentTask, err := service.loadTask(serviceContext, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task before update: %w", err)
	}

	if currentTask.Status == models.TaskStatusCompleted {
		return nil, ErrTaskCompleted
	}

	updatedTask, err := service.repository.Update(serviceContext, taskID, normalizedUpdate)
	if err != nil {
		return nil, service.handleUpdateError(serviceContext, taskID, err)
	}

	return updatedTask, nil
}

func (service *taskService) Delete(serviceContext context.Context, rawTaskID string) error {
	taskID, err := parseTaskID(rawTaskID)
	if err != nil {
		return err
	}

	if err := service.repository.Delete(serviceContext, taskID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrTaskNotFound
		}

		return fmt.Errorf("delete task: %w", err)
	}

	return nil
}

func parseTaskID(rawTaskID string) (string, error) {
	taskID := strings.TrimSpace(rawTaskID)
	if !utils.IsValidUUID(taskID) {
		return "", ErrInvalidTaskID
	}

	return taskID, nil
}

func normalizeListTaskInput(listTaskInput models.ListTaskInput) (models.TaskListQuery, error) {
	filter, err := normalizeTaskFilter(listTaskInput)
	if err != nil {
		return models.TaskListQuery{}, err
	}

	page, err := normalizePage(listTaskInput.Page)
	if err != nil {
		return models.TaskListQuery{}, err
	}

	pageSize, err := normalizePageSize(listTaskInput.PageSize)
	if err != nil {
		return models.TaskListQuery{}, err
	}

	return models.TaskListQuery{
		Filter:   filter,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func normalizeTaskFilter(listTaskInput models.ListTaskInput) (models.TaskFilter, error) {
	var filter models.TaskFilter

	if strings.TrimSpace(listTaskInput.Status) != "" {
		status, err := parseTaskStatus(listTaskInput.Status)
		if err != nil {
			return models.TaskFilter{}, err
		}

		filter.Status = &status
	}

	if strings.TrimSpace(listTaskInput.Priority) != "" {
		priority, err := parseTaskPriority(listTaskInput.Priority)
		if err != nil {
			return models.TaskFilter{}, err
		}

		filter.Priority = &priority
	}

	return filter, nil
}

func normalizePage(rawPage int) (int, error) {
	if rawPage == 0 {
		return defaultTaskListPage, nil
	}

	if rawPage < 1 {
		return 0, &ValidationError{Message: "page must be greater than or equal to 1"}
	}

	return rawPage, nil
}

func normalizePageSize(rawPageSize int) (int, error) {
	if rawPageSize == 0 {
		return defaultTaskPageSize, nil
	}

	if rawPageSize < 1 {
		return 0, &ValidationError{Message: "page_size must be greater than or equal to 1"}
	}

	if rawPageSize > maxTaskPageSize {
		return 0, &ValidationError{Message: fmt.Sprintf("page_size must be less than or equal to %d", maxTaskPageSize)}
	}

	return rawPageSize, nil
}

func normalizeUpdateInput(updateTaskInput models.UpdateTaskInput) (models.TaskUpdate, error) {
	if updateTaskInput.Title == nil && updateTaskInput.Description == nil && updateTaskInput.Status == nil && updateTaskInput.Priority == nil && updateTaskInput.DueDate == nil {
		return models.TaskUpdate{}, &ValidationError{Message: "at least one field must be provided"}
	}

	var normalized models.TaskUpdate

	if updateTaskInput.Title != nil {
		title, err := normalizeRequiredTitle(*updateTaskInput.Title)
		if err != nil {
			return models.TaskUpdate{}, err
		}

		normalized.Title = &title
	}

	if updateTaskInput.Description != nil {
		description := strings.TrimSpace(*updateTaskInput.Description)
		normalized.Description = &description
	}

	if updateTaskInput.Status != nil {
		status, err := parseTaskStatus(*updateTaskInput.Status)
		if err != nil {
			return models.TaskUpdate{}, err
		}

		normalized.Status = &status
	}

	if updateTaskInput.Priority != nil {
		priority, err := parseTaskPriority(*updateTaskInput.Priority)
		if err != nil {
			return models.TaskUpdate{}, err
		}

		normalized.Priority = &priority
	}

	if updateTaskInput.DueDate != nil {
		dueDate, err := parseDueDate(*updateTaskInput.DueDate)
		if err != nil {
			return models.TaskUpdate{}, err
		}

		normalized.DueDate = &dueDate
	}

	return normalized, nil
}

func parseTaskStatus(value string) (models.TaskStatus, error) {
	switch strings.TrimSpace(value) {
	case string(models.TaskStatusPending):
		return models.TaskStatusPending, nil
	case string(models.TaskStatusInProgress):
		return models.TaskStatusInProgress, nil
	case string(models.TaskStatusCompleted):
		return models.TaskStatusCompleted, nil
	case string(models.TaskStatusCancelled):
		return models.TaskStatusCancelled, nil
	default:
		return "", &ValidationError{Message: "status must be one of: pending, in_progress, completed, cancelled"}
	}
}

func parseTaskPriority(value string) (models.TaskPriority, error) {
	switch strings.TrimSpace(value) {
	case string(models.TaskPriorityLow):
		return models.TaskPriorityLow, nil
	case string(models.TaskPriorityMedium):
		return models.TaskPriorityMedium, nil
	case string(models.TaskPriorityHigh):
		return models.TaskPriorityHigh, nil
	default:
		return "", &ValidationError{Message: "priority must be one of: low, medium, high"}
	}
}

func calculateTotalPages(totalItems int64, pageSize int) int {
	if totalItems == 0 {
		return 0
	}

	return int((totalItems + int64(pageSize) - 1) / int64(pageSize))
}

func calculatePreviousPage(currentPage int) *int {
	if currentPage <= 1 {
		return nil
	}

	previousPage := currentPage - 1
	return &previousPage
}

func calculateNextPage(currentPage int, totalPages int) *int {
	if currentPage >= totalPages {
		return nil
	}

	nextPage := currentPage + 1
	return &nextPage
}

func parseDueDate(value string) (time.Time, error) {
	dueDate := strings.TrimSpace(value)
	if dueDate == "" {
		return time.Time{}, &ValidationError{Message: "due_date is required and must use format YYYY-MM-DD"}
	}

	parsed, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		return time.Time{}, &ValidationError{Message: "due_date must use format YYYY-MM-DD"}
	}

	normalized := parsed.UTC()
	if normalized.Before(currentDate()) {
		return time.Time{}, &ValidationError{Message: "due_date cannot be in the past"}
	}

	return normalized, nil
}

func normalizeRequiredTitle(value string) (string, error) {
	title := strings.TrimSpace(value)
	if title == "" {
		return "", &ValidationError{Message: "title is required"}
	}

	titleLength := utf8.RuneCountInString(title)
	if titleLength < minTaskTitleLength || titleLength > maxTaskTitleLength {
		return "", &ValidationError{Message: "title must be between 3 and 100 characters"}
	}

	return title, nil
}

func currentDate() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func newTask(title, description string, priority models.TaskPriority, dueDate time.Time) *models.Task {
	now := time.Now().UTC()

	return &models.Task{
		ID:          utils.NewUUID(),
		Title:       title,
		Description: strings.TrimSpace(description),
		Status:      models.TaskStatusPending,
		Priority:    priority,
		DueDate:     dueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (service *taskService) loadTask(serviceContext context.Context, taskID string) (*models.Task, error) {
	task, err := service.repository.GetByID(serviceContext, taskID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrTaskNotFound
		}

		return nil, fmt.Errorf("get task by id: %w", err)
	}

	return task, nil
}

func (service *taskService) handleUpdateError(serviceContext context.Context, taskID string, updateError error) error {
	if !errors.Is(updateError, repositories.ErrNotFound) {
		return fmt.Errorf("update task: %w", updateError)
	}

	task, lookupError := service.repository.GetByID(serviceContext, taskID)
	switch {
	case lookupError == nil && task.Status == models.TaskStatusCompleted:
		return ErrTaskCompleted
	case errors.Is(lookupError, repositories.ErrNotFound):
		return ErrTaskNotFound
	case lookupError != nil:
		return fmt.Errorf("check task after failed update: %w", lookupError)
	default:
		return ErrTaskNotFound
	}
}
