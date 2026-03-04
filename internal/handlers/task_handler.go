package handlers

import (
	"net/http"

	"to-do-list/internal/controllers"
)

type TaskHandler struct {
	controller *controllers.TaskController
}

func NewTaskHandler(controller *controllers.TaskController) *TaskHandler {
	return &TaskHandler{controller: controller}
}

func (h *TaskHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /tasks", h.controller.List)
	mux.HandleFunc("POST /tasks", h.controller.Create)
	mux.HandleFunc("GET /tasks/{id}", h.controller.GetByID)
	mux.HandleFunc("PUT /tasks/{id}", h.controller.Update)
	mux.HandleFunc("PATCH /tasks/{id}", h.controller.Update)
	mux.HandleFunc("DELETE /tasks/{id}", h.controller.Delete)
}
