package email

import "context"

// NoopEmailService implements EmailService without sending (tests / disabled email).
type NoopEmailService struct{}

// SendMilestonePlan validates input and succeeds without network I/O.
func (NoopEmailService) SendMilestonePlan(ctx context.Context, in SendMilestoneInput) error {
	_ = ctx
	return in.validate()
}
