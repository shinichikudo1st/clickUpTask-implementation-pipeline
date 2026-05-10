package services

import "context"

// MilestoneRunner is the minimal surface for running the milestone pipeline (webhook, manual API, poller).
type MilestoneRunner interface {
	GenerateForTask(ctx context.Context, taskID string, force bool) error
}
