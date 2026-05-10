package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/handlers"
	appmiddleware "github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/middleware"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if database != nil {
		defer func() { _ = database.Close() }()
	}

	var store *db.Store
	if database != nil {
		store = db.NewStore(database)
	}

	var milestonePlanner handlers.MilestonePlanner
	if store == nil {
		log.Print("milestone planner inactive: no database store (set DATABASE_URL and ensure the DB is reachable)")
	} else {
		p, err := services.TryNewPlanner(cfg, store)
		switch {
		case err != nil:
			log.Printf("milestone planner disabled: %v", err)
		case p != nil:
			milestonePlanner = p
			log.Print("milestone planner enabled: new assignment webhooks run the full pipeline asynchronously (ClickUp → LLM → storage → email)")
		default:
			log.Print("milestone planner inactive: TryNewPlanner returned no planner (unexpected)")
		}
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(appmiddleware.Recovery)
	router.Use(appmiddleware.RequestLogger)
	router.Use(middleware.Timeout(60 * time.Second))

	router.Get("/v1/health", handlers.HealthHandler(database))
	router.Post("/v1/webhooks/clickup", handlers.ClickUpWebhookHandler(cfg, store, milestonePlanner))
	router.NotFound(handlers.NotFoundHandler)
	router.MethodNotAllowed(handlers.MethodNotAllowedHandler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Printf("clickup-task-implementation-pipeline listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
