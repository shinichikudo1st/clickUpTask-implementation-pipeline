package services

import (
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
)

// SkipCompletedPolicy reports whether GenerateForTask should no-op for idempotency:
// latest generation exists and is completed, and force is false.
func SkipCompletedPolicy(force bool, err error, row db.MilestoneGenerationRow) bool {
	if force {
		return false
	}
	if err != nil {
		return false
	}
	return row.Status == "completed"
}
