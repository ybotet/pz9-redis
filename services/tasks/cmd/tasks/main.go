package main

import (
	"context"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/cache"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/clients"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/handlers"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/repository"
	"github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/service"

	internalMiddleware "github.com/ybotet/pz8-pipelineCICD-go/services/tasks/internal/middleware"
	"github.com/ybotet/pz8-pipelineCICD-go/shared/logger"
	"github.com/ybotet/pz8-pipelineCICD-go/shared/middleware"
)

func main() {
    tasksPort := os.Getenv("TASKS_PORT")
    if tasksPort == "" {
        tasksPort = "8082"
    }

    authAddr := os.Getenv("AUTH_GRPC_ADDR")
    if authAddr == "" {
        authAddr = "localhost:50051"
    }

    log := logger.New(logger.Config{
        ServiceName: "tasks",
        Environment: "development",
        LogLevel:    "debug",
        JSONFormat:  true,
    })

    // Conectar a PostgreSQL
    db, err := repository.NewPostgresConnection()
    if err != nil {
        log.Fatalf("Error conectando a PostgreSQL: %v", err)
    }
    defer db.Close()

    // Crear repositorio
    taskRepo := repository.NewPostgresTaskRepository(db)


    // ===== INICIALIZAR REDIS CACHE =====
    redisAddr := os.Getenv("REDIS_ADDR")
    if redisAddr == "" {
        redisAddr = "localhost:6379"
    }
    redisPassword := os.Getenv("REDIS_PASSWORD")
    redisDB := 0 // por defecto
    cacheTTL := 120   // segundos
    cacheJitter := 30 // segundos

    redisCache := cache.NewRedisCache(redisAddr, redisPassword, redisDB, cacheTTL, cacheJitter)

    // Probar conexión a Redis (no crítico, solo log)
    if err := redisCache.Ping(context.Background()); err != nil {
        log.Printf("[WARN] No se pudo conectar a Redis: %v. El servicio funcionará sin caché.", err)
    } else {
        log.Printf("[INFO] Conectado a Redis en %s", redisAddr)
    }

    // Crear servicio con caché
    taskService := service.NewTaskService(taskRepo, redisCache)

    // Crear handler con el servicio
    taskHandler := handlers.NewTaskHandler(taskService)

    // Router
    r := mux.NewRouter()

    // Middlewares GLOBALES (en orden correcto)
    r.Use(middleware.RequestID)
    r.Use(middleware.Logging(log))
    r.Use(internalMiddleware.SecurityHeadersMiddleware) // <-- NUEVO

    // Conectar a Auth service
    authClient, err := clients.NewAuthClient(authAddr)
    if err != nil {
        log.Fatalf("Error conectando a Auth service: %v", err)
    }
    defer authClient.Close()

    // Middleware de autenticación (existente)
    authMiddleware := internalMiddleware.NewAuthMiddleware(authClient.GetClient())
    

    // Health check (público)
    r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    }).Methods("GET")

      // ===== RUTAS PROTEGIDAS =====
    // GET /tasks
    r.HandleFunc("/v1/tasks", 
        authMiddleware.Authenticate(taskHandler.GetTasks)).Methods("GET")
    
    // GET /tasks/{id} - NUEVA RUTA CON CACHÉ
    r.HandleFunc("/v1/tasks/{id}", 
        authMiddleware.Authenticate(taskHandler.GetTaskByID)).Methods("GET")
    
    // POST /tasks
    r.HandleFunc("/v1/tasks", 
        authMiddleware.Authenticate(internalMiddleware.CSRFMiddleware(taskHandler.CreateTask))).Methods("POST")
    
    // PATCH /tasks/{id} - ACTUALIZAR (invalida caché)
    r.HandleFunc("/v1/tasks/{id}", 
        authMiddleware.Authenticate(taskHandler.UpdateTask)).Methods("PATCH")
    
    // DELETE /tasks/{id} - ELIMINAR (invalida caché)
    r.HandleFunc("/v1/tasks/{id}", 
        authMiddleware.Authenticate(taskHandler.DeleteTask)).Methods("DELETE")

    r.HandleFunc("/v1/tasks/search/vulnerable", 
        authMiddleware.Authenticate(taskHandler.SearchTasksVulnerable)).Methods("GET")
    r.HandleFunc("/v1/tasks/search", 
        authMiddleware.Authenticate(taskHandler.SearchTasks)).Methods("GET")

    // Debug: mostrar todas las rutas registradas
    log.Println("=== RUTAS REGISTRADAS ===")
    err1 := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
        path, err := route.GetPathTemplate()
        if err == nil {
            methods, _ := route.GetMethods()
            log.Printf("  %s %v", path, methods)
        }
        return nil
    })
    if err1 != nil {
        log.Printf("Error walking routes: %v", err1)
    }


    log.Printf("Servidor Tasks escuchando en puerto %s", tasksPort)
    log.Fatal(http.ListenAndServe(":"+tasksPort, r))


}