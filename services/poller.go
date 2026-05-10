package services

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
)

var pollerWarmupCutoff = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// PollCycleStats summarizes one poller pass.
type PollCycleStats struct {
	Listed             int
	Runs               int
	SkippedCompleted   int
	GenerationFailures int
}

// watermarkQueryLowerBound returns the exclusive lower bound for ClickUp date_updated_gt.
// First run (watermark before year 2000) uses a lookback window from now.
func watermarkQueryLowerBound(watermark time.Time, lookback time.Duration, now time.Time) time.Time {
	if watermark.Before(pollerWarmupCutoff) {
		t := now.Add(-lookback)
		return t
	}
	t := watermark.Add(-2 * time.Minute)
	if t.Before(pollerWarmupCutoff) {
		return watermark
	}
	return t
}

// TeamTaskLister lists ClickUp team tasks for an assignee (implemented by *ClickUpClient).
type TeamTaskLister interface {
	ListTeamTasksForAssignee(ctx context.Context, teamID, assigneeID string, dateUpdatedGt time.Time) ([]ListedTeamTask, error)
}

// RunPollCycle lists team tasks assigned to CLICKUP_ASSIGNEE_USER_ID updated since the stored
// watermark (with overlap), skips tasks whose latest generation is completed (same as webhook idempotency),
// runs GenerateForTask(force=false) for the rest, then advances the watermark to now.
func RunPollCycle(ctx context.Context, cfg *config.Config, store *db.Store, lister TeamTaskLister, runner MilestoneRunner) (PollCycleStats, error) {
	var stats PollCycleStats
	if cfg == nil || !cfg.ClickUpPollerEnabled {
		return stats, nil
	}
	if store == nil {
		return stats, errors.New("poller: store is nil")
	}
	if lister == nil {
		return stats, errors.New("poller: team task lister is nil")
	}
	if runner == nil {
		return stats, errors.New("poller: milestone runner is nil")
	}
	teamID := cfg.ClickUpTeamID
	assigneeID := cfg.ClickUpAssigneeID
	if teamID == "" || assigneeID == "" {
		return stats, fmt.Errorf("poller: CLICKUP_TEAM_ID and CLICKUP_ASSIGNEE_USER_ID are required when CLICKUP_POLLER_ENABLED is true")
	}

	lookback := time.Duration(cfg.ClickUpPollerLookbackH) * time.Hour
	if lookback <= 0 {
		lookback = 168 * time.Hour
	}

	watermark, err := store.GetPollerLastPolledAt(ctx)
	if err != nil {
		return stats, err
	}
	now := time.Now().UTC()
	queryFrom := watermarkQueryLowerBound(watermark, lookback, now)

	tasks, err := lister.ListTeamTasksForAssignee(ctx, teamID, assigneeID, queryFrom)
	if err != nil {
		return stats, err
	}

	slices.SortFunc(tasks, func(a, b ListedTeamTask) int {
		if c := a.DateUpdated.Compare(b.DateUpdated); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})

	for _, t := range tasks {
		stats.Listed++
		row, errLatest := store.LatestGenerationByTaskID(ctx, t.ID)
		if SkipCompletedPolicy(false, errLatest, row) {
			stats.SkippedCompleted++
			continue
		}
		if errLatest != nil && !errors.Is(errLatest, sql.ErrNoRows) {
			stats.GenerationFailures++
			continue
		}
		if err := runner.GenerateForTask(ctx, t.ID, false); err != nil {
			stats.GenerationFailures++
			continue
		}
		stats.Runs++
	}

	// Advance watermark even when listing was empty or generations failed, so we do not
	// re-scan the same window forever on persistent upstream errors.
	if err := store.SetPollerLastPolledAt(context.WithoutCancel(ctx), time.Now().UTC()); err != nil {
		return stats, err
	}
	return stats, nil
}
