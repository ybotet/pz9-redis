package handlers

import (
	"encoding/json"
	"html"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/middleware"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/service"
	"github.com/ybotet/pz8-pipelineCICD-go/shared/models"
)

type TaskHandler struct {
	taskService *service.TaskService
}

func NewTaskHandler(taskService *service.TaskService) *TaskHandler {
	return &TaskHandler{taskService: taskService}
}

// GetTasks - Obtener todas las tareas del usuario autenticado
func (h *TaskHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	
	tasks, err := h.taskService.GetTasksByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("Error getting tasks: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// GetTaskByID - Obtener una tarea específica (NUEVO ENDPOINT)
func (h *TaskHandler) GetTaskByID(w http.ResponseWriter, r *http.Request) {
	// Obtener ID de la URL (asumiendo ruta /v1/tasks/{id})
	vars := mux.Vars(r)
	id := vars["id"]
	
	userID := middleware.GetUserID(r.Context())
	
	task, err := h.taskService.GetTaskByID(r.Context(), id, userID)
	if err != nil {
		log.Printf("Error getting task by ID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// CreateTask - Crear nueva tarea (con sanitización XSS)
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Sanitización XSS
	task.Title = sanitizeInput(task.Title)
	task.Description = sanitizeInput(task.Description)
	
	userID := middleware.GetUserID(r.Context())
	
	task.ID = uuid.New().String()
	task.CreatedAt = time.Now()
	task.Done = false
	task.UserID = userID
	
	if err := h.taskService.CreateTask(&task); err != nil {
		log.Printf("Error creating task: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// UpdateTask - Actualizar tarea existente (NUEVO ENDPOINT)
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Done        bool   `json:"done"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	userID := middleware.GetUserID(r.Context())
	
	// Primero obtener la tarea existente
	task, err := h.taskService.GetTaskByID(r.Context(), id, userID)
	if err != nil {
		log.Printf("Error getting task for update: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	
	// Actualizar campos
	task.Title = sanitizeInput(req.Title)
	task.Description = sanitizeInput(req.Description)
	task.Done = req.Done
	
	if err := h.taskService.UpdateTask(task); err != nil {
		log.Printf("Error updating task: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	json.NewEncoder(w).Encode(task)
}

// DeleteTask - Eliminar tarea (NUEVO ENDPOINT)
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	userID := middleware.GetUserID(r.Context())
	
	if err := h.taskService.DeleteTask(id, userID); err != nil {
		log.Printf("Error deleting task: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// SearchTasksVulnerable - ENDPOINT VULNERABLE (solo para demostración)
func (h *TaskHandler) SearchTasksVulnerable(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if title == "" {
		http.Error(w, "title parameter required", http.StatusBadRequest)
		return
	}
	
	// Necesitamos acceso al repository para esto
	// Por ahora, mantenemos como estaba
	http.Error(w, "Endpoint vulnerable - implementar si es necesario", http.StatusNotImplemented)
}

// SearchTasks - ENDPOINT SEGURO
func (h *TaskHandler) SearchTasks(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if title == "" {
		http.Error(w, "title parameter required", http.StatusBadRequest)
		return
	}
	
	// Necesitamos acceso al repository para esto
	http.Error(w, "Endpoint search - implementar si es necesario", http.StatusNotImplemented)
}

func sanitizeInput(input string) string {
	return html.EscapeString(input)
}