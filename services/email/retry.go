package email

import (
	"context"
	"fmt"
	"time"
)

// SendWithRetry calls SendMilestonePlan with bounded exponential backoff (Phase 7).
func SendWithRetry(ctx context.Context, svc EmailService, in SendMilestoneInput, attempts int) error {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 5 {
		attempts = 5
	}
	var last error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			d := time.Duration(100*(1<<uint(i-1))) * time.Millisecond
			if d > 2*time.Second {
				d = 2 * time.Second
			}
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
		last = svc.SendMilestonePlan(ctx, in)
		if last == nil {
			return nil
		}
	}
	return fmt.Errorf("email: gave up after %d attempts: %w", attempts, last)
}
