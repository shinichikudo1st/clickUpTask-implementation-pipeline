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
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/internal/safelog"
	appmiddleware "github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/middleware"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %s", safelog.Redact(err.Error()))
	}

	database, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %s", safelog.Redact(err.Error()))
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
			log.Printf("milestone planner disabled: %s", safelog.Redact(err.Error()))
		case p != nil:
			milestonePlanner = p
			log.Print("milestone planner enabled: new assignment webhooks run the full pipeline asynchronously (ClickUp → LLM → storage → email)")
		default:
			log.Print("milestone planner inactive: TryNewPlanner returned no planner (unexpected)")
		}
	}

	var taskAPIBlobs storage.BlobStore
	if store != nil {
		if b, err := storage.NewFromConfig(cfg); err != nil {
			log.Printf("task API: signed download URLs on GET /v1/tasks/.../plan unavailable: %s", safelog.Redact(err.Error()))
		} else {
			taskAPIBlobs = b
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
	handlers.RegisterTaskAPI(router, cfg, store, milestonePlanner, taskAPIBlobs)
	router.NotFound(handlers.NotFoundHandler)
	router.MethodNotAllowed(handlers.MethodNotAllowedHandler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Printf("clickup-task-implementation-pipeline listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %s", safelog.Redact(err.Error()))
		}
	}()

	if cfg.ClickUpPollerEnabled && cfg.ClickUpPollIntervalSec > 0 && store != nil && milestonePlanner != nil {
		cu, err := services.NewClickUpClient(cfg)
		if err != nil {
			log.Printf("poller ticker: disabled (clickup client: %s)", safelog.Redact(err.Error()))
		} else {
			interval := time.Duration(cfg.ClickUpPollIntervalSec) * time.Second
			log.Printf("poller ticker: every %s (CLICKUP_POLL_INTERVAL_SECONDS)", interval)
			go func() {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				for range ticker.C {
					ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
					stats, err := services.RunPollCycle(ctx, cfg, store, cu, milestonePlanner)
					cancel()
					if err != nil {
						log.Printf("poller tick: %s", safelog.Redact(err.Error()))
						continue
					}
					if stats.Listed > 0 || stats.Runs > 0 || stats.GenerationFailures > 0 {
						log.Printf("poller tick: listed=%d runs=%d skipped_completed=%d generation_failures=%d",
							stats.Listed, stats.Runs, stats.SkippedCompleted, stats.GenerationFailures)
					}
				}
			}()
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %s", safelog.Redact(err.Error()))
	}
}
