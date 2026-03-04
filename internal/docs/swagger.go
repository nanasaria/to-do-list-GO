package docs

import (
	"encoding/json"
	"net/http"
)

func OpenAPIHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(openAPISpec())
}

func SwaggerUIHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIHTML))
}

func openAPISpec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "To-Do List API",
			"description": "REST API para gerenciamento de tarefas.",
			"version":     "1.0.0",
		},
		"servers": []map[string]any{
			{"url": "/"},
		},
		"paths": map[string]any{
			"/health": map[string]any{
				"get": map[string]any{
					"summary":     "Health check",
					"operationId": "healthCheck",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "API online",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/HealthResponse",
									},
								},
							},
						},
					},
				},
			},
			"/tasks": map[string]any{
				"get": map[string]any{
					"summary":     "Listar tarefas",
					"operationId": "listTasks",
					"parameters": []map[string]any{
						queryParameter("status", "Filtra pelo status da tarefa", map[string]any{
							"type": "string",
							"enum": []string{"pending", "in_progress", "completed", "cancelled"},
						}),
						queryParameter("priority", "Filtra pela prioridade da tarefa", map[string]any{
							"type": "string",
							"enum": []string{"low", "medium", "high"},
						}),
						queryParameter("page", "Numero da pagina. Padrao: 1", map[string]any{
							"type":    "integer",
							"minimum": 1,
							"example": 1,
						}),
						queryParameter("page_size", "Quantidade de itens por pagina. Padrao: 10, maximo: 100", map[string]any{
							"type":    "integer",
							"minimum": 1,
							"maximum": 100,
							"example": 10,
						}),
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Lista paginada de tarefas",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/TaskListResponse",
									},
								},
							},
						},
						"400": errorResponse("Filtro inválido"),
					},
				},
				"post": map[string]any{
					"summary":     "Criar tarefa",
					"operationId": "createTask",
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/CreateTaskRequest",
								},
								"example": map[string]any{
									"title":       "Estudar Golang",
									"description": "Revisar conceitos de goroutines",
									"priority":    "high",
									"due_date":    "2026-02-10",
								},
							},
						},
					},
					"responses": map[string]any{
						"201": successResponse("Tarefa criada", "#/components/schemas/Task"),
						"400": errorResponse("Payload inválido"),
					},
				},
			},
			"/tasks/{id}": map[string]any{
				"parameters": []map[string]any{
					pathParameter("id", "UUID da tarefa", map[string]any{
						"type":   "string",
						"format": "uuid",
					}),
				},
				"get": map[string]any{
					"summary":     "Buscar tarefa por ID",
					"operationId": "getTaskByID",
					"responses": map[string]any{
						"200": successResponse("Tarefa encontrada", "#/components/schemas/Task"),
						"400": errorResponse("ID inválido"),
						"404": errorResponse("Tarefa não encontrada"),
					},
				},
				"put": map[string]any{
					"summary":     "Atualizar tarefa",
					"operationId": "updateTaskPut",
					"requestBody": updateTaskRequestBody(),
					"responses": map[string]any{
						"200": successResponse("Tarefa atualizada", "#/components/schemas/Task"),
						"400": errorResponse("Payload inválido"),
						"404": errorResponse("Tarefa não encontrada"),
						"409": errorResponse("Tarefa concluída não pode ser editada"),
					},
				},
				"delete": map[string]any{
					"summary":     "Deletar tarefa",
					"operationId": "deleteTask",
					"responses": map[string]any{
						"204": map[string]any{
							"description": "Tarefa removida",
						},
						"400": errorResponse("ID inválido"),
						"404": errorResponse("Tarefa não encontrada"),
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"Task": map[string]any{
					"type": "object",
					"required": []string{
						"id",
						"title",
						"status",
						"priority",
						"due_date",
						"created_at",
						"updated_at",
					},
					"properties": map[string]any{
						"id": map[string]any{
							"type":   "string",
							"format": "uuid",
						},
						"title": map[string]any{
							"type":      "string",
							"minLength": 3,
							"maxLength": 100,
						},
						"description": map[string]any{
							"type": "string",
						},
						"status": map[string]any{
							"type": "string",
							"enum": []string{"pending", "in_progress", "completed", "cancelled"},
						},
						"priority": map[string]any{
							"type": "string",
							"enum": []string{"low", "medium", "high"},
						},
						"due_date": map[string]any{
							"type":   "string",
							"format": "date",
						},
						"created_at": map[string]any{
							"type":   "string",
							"format": "date-time",
						},
						"updated_at": map[string]any{
							"type":   "string",
							"format": "date-time",
						},
					},
				},
				"TaskListResponse": map[string]any{
					"type": "object",
					"required": []string{
						"items",
						"total_items",
						"page",
						"page_size",
						"total_pages",
					},
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"$ref": "#/components/schemas/Task",
							},
						},
						"total_items": map[string]any{
							"type":    "integer",
							"format":  "int64",
							"example": 42,
						},
						"page": map[string]any{
							"type":    "integer",
							"example": 1,
						},
						"page_size": map[string]any{
							"type":    "integer",
							"example": 10,
						},
						"total_pages": map[string]any{
							"type":    "integer",
							"example": 5,
						},
						"previous_page": map[string]any{
							"type":    "integer",
							"example": 1,
						},
						"next_page": map[string]any{
							"type":    "integer",
							"example": 3,
						},
					},
				},
				"CreateTaskRequest": map[string]any{
					"type": "object",
					"required": []string{
						"title",
						"priority",
						"due_date",
					},
					"properties": map[string]any{
						"title": map[string]any{
							"type":      "string",
							"minLength": 3,
							"maxLength": 100,
						},
						"description": map[string]any{
							"type": "string",
						},
						"priority": map[string]any{
							"type": "string",
							"enum": []string{"low", "medium", "high"},
						},
						"due_date": map[string]any{
							"type":   "string",
							"format": "date",
						},
					},
				},
				"UpdateTaskRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{
							"type":      "string",
							"minLength": 3,
							"maxLength": 100,
						},
						"description": map[string]any{
							"type": "string",
						},
						"status": map[string]any{
							"type": "string",
							"enum": []string{"pending", "in_progress", "completed", "cancelled"},
						},
						"priority": map[string]any{
							"type": "string",
							"enum": []string{"low", "medium", "high"},
						},
						"due_date": map[string]any{
							"type":   "string",
							"format": "date",
						},
					},
				},
				"ErrorResponse": map[string]any{
					"type":     "object",
					"required": []string{"error"},
					"properties": map[string]any{
						"error": map[string]any{
							"type": "string",
						},
					},
				},
				"HealthResponse": map[string]any{
					"type":     "object",
					"required": []string{"status"},
					"properties": map[string]any{
						"status": map[string]any{
							"type":    "string",
							"example": "ok",
						},
					},
				},
			},
		},
	}
}

func queryParameter(name, description string, schema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"description": description,
		"required":    false,
		"schema":      schema,
	}
}

func pathParameter(name, description string, schema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"description": description,
		"required":    true,
		"schema":      schema,
	}
}

func successResponse(description, schemaRef string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"$ref": schemaRef,
				},
			},
		},
	}
}

func errorResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"$ref": "#/components/schemas/ErrorResponse",
				},
			},
		},
	}
}

func updateTaskRequestBody() map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"$ref": "#/components/schemas/UpdateTaskRequest",
				},
				"example": map[string]any{
					"title":  "Estudar Golang - Atualizado",
					"status": "in_progress",
				},
			},
		},
	}
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>To-Do List API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #faf7f0; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: '/swagger/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
      });
    };
  </script>
</body>
</html>`
