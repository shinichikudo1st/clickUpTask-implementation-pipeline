// Command poll-milestones runs one ClickUp poller cycle (Phase 10) when CLICKUP_POLLER_ENABLED is true.
// Intended for cron, CI smoke, or local backfill. Requires DATABASE_URL, CLICKUP_TEAM_ID,
// CLICKUP_ASSIGNEE_USER_ID, and the same integrations as the main planner.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/internal/safelog"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %s", safelog.Redact(err.Error()))
	}
	if !cfg.ClickUpPollerEnabled {
		log.Print("poll-milestones: CLICKUP_POLLER_ENABLED is not true; exiting without work")
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %s", safelog.Redact(err.Error()))
	}
	defer func() { _ = database.Close() }()

	store := db.NewStore(database)
	client, err := services.NewClickUpClient(cfg)
	if err != nil {
		log.Fatalf("clickup: %s", safelog.Redact(err.Error()))
	}
	planner, err := services.TryNewPlanner(cfg, store)
	if err != nil {
		log.Fatalf("planner: %s", safelog.Redact(err.Error()))
	}
	if planner == nil {
		log.Fatal("planner: TryNewPlanner returned nil")
	}

	stats, err := services.RunPollCycle(ctx, cfg, store, client, planner)
	if err != nil {
		log.Fatalf("poller: %s", safelog.Redact(err.Error()))
	}
	log.Printf("poll-milestones: listed=%d runs=%d skipped_completed=%d generation_failures=%d",
		stats.Listed, stats.Runs, stats.SkippedCompleted, stats.GenerationFailures)
}
