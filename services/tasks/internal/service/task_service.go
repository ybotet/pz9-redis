package service

import (
	"context"
	"log"
	"time"

	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/cache"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/repository"
	"github.com/ybotet/pz8-pipelineCICD-go/shared/models"
)

// TaskService es la capa de servicio que implementa cache-aside
type TaskService struct {
	repo  repository.TaskRepository
	cache *cache.RedisCache
}

// NewTaskService crea una nueva instancia del servicio
func NewTaskService(repo repository.TaskRepository, cache *cache.RedisCache) *TaskService {
	return &TaskService{
		repo:  repo,
		cache: cache,
	}
}

// GetTaskByID obtiene una tarea por ID usando cache-aside
func (s *TaskService) GetTaskByID(ctx context.Context, id, userID string) (*models.Task, error) {
	// 1. INTENTAR LEER DE CACHÉ
	taskCache, err := s.cache.GetTask(ctx, id)
	if err == nil {
		// Cache HIT - verificar que la tarea pertenezca al usuario
		if taskCache.UserID == userID {
			log.Printf("[CACHE HIT] Tarea %s servida desde Redis", id)
			// Convertir cache.Task a models.Task
			return &models.Task{
				ID:          taskCache.ID,
				Title:       taskCache.Title,
				Description: taskCache.Description,
				Done:        taskCache.Status == "done",
				CreatedAt:   time.Now(), 
				UserID:      taskCache.UserID,
			}, nil
		}
		// Si no pertenece al usuario, ignorar caché (seguridad)
		log.Printf("[CACHE WARN] Tarea %s en caché no pertenece al usuario %s", id, userID)
	}

	if err != nil && err.Error() != "redis: nil" {
		// Error de Redis (no es un simple miss) - loguear pero continuar con fallback
		log.Printf("[CACHE ERROR] Redis no disponible: %v. Fallback a BD", err)
	}

	// 2. CACHE MISS o ERROR - ir a base de datos
	log.Printf("[CACHE MISS] Tarea %s no encontrada en Redis, consultando BD", id)
	
	task, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	
	if task == nil {
		return nil, nil // No encontrada
	}
	
	// Verificar que la tarea pertenezca al usuario
	if task.UserID != userID {
		log.Printf("[SECURITY] Usuario %s intentó acceder a tarea %s de otro usuario", userID, id)
		return nil, nil
	}
	
	// 3. GUARDAR EN CACHÉ para futuras peticiones
	go func() {
		// Guardar en background para no bloquear la respuesta
		cacheTask := &cache.Task{
			ID:          task.ID,
			Title:       task.Title,
			Description: task.Description,
			Status:      map[bool]string{true: "done", false: "pending"}[task.Done],
			UserID:      task.UserID,
		}
		if err := s.cache.SetTask(context.Background(), cacheTask); err != nil {
			log.Printf("[CACHE WARN] No se pudo guardar tarea %s en caché: %v", id, err)
		}
	}()
	
	return task, nil
}

// GetTasksByUserID obtiene todas las tareas de un usuario (con caché opcional para lista)
func (s *TaskService) GetTasksByUserID(ctx context.Context, userID string) ([]models.Task, error) {
	// Por ahora, vamos directo a BD para las listas
	// En una versión avanzada se podría cachear también la lista
	return s.repo.GetByUserID(userID)
}

// CreateTask crea una nueva tarea (invalida caché si es necesario)
func (s *TaskService) CreateTask(task *models.Task) error {
	// Guardar en BD
	if err := s.repo.Create(task); err != nil {
		return err
	}
	
	// No es necesario invalidar caché para creación (el ID es nuevo)
	return nil
}

// UpdateTask actualiza una tarea e invalida el caché
func (s *TaskService) UpdateTask(task *models.Task) error {
	// Actualizar en BD
	if err := s.repo.Update(task); err != nil {
		return err
	}
	
	// Invalidar caché (borrar la entrada antigua)
	go func() {
		if err := s.cache.DeleteTask(context.Background(), task.ID); err != nil {
			log.Printf("[CACHE WARN] No se pudo invalidar caché para tarea %s: %v", task.ID, err)
		}
	}()
	
	return nil
}

// DeleteTask elimina una tarea e invalida el caché
func (s *TaskService) DeleteTask(id, userID string) error {
	// Eliminar de BD
	if err := s.repo.Delete(id, userID); err != nil {
		return err
	}
	
	// Invalidar caché
	go func() {
		if err := s.cache.DeleteTask(context.Background(), id); err != nil {
			log.Printf("[CACHE WARN] No se pudo invalidar caché para tarea %s: %v", id, err)
		}
	}()
	
	return nil
}