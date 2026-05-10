package email

import (
	"fmt"
	"strings"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

// NewFromConfig returns an EmailService for the configured provider.
// Empty EMAIL_PROVIDER (or none/noop) yields NoopEmailService.
func NewFromConfig(cfg *config.Config) (EmailService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("email: config is nil")
	}
	p := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	switch p {
	case "", "none", "noop":
		return NoopEmailService{}, nil
	case "resend":
		return NewResendEmailService(cfg)
	default:
		return nil, fmt.Errorf("email: unsupported EMAIL_PROVIDER %q", cfg.EmailProvider)
	}
}
