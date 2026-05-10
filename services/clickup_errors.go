package services

import (
	"fmt"
	"time"
)

// ClickUpHTTPError describes a non-success response from the ClickUp API.
type ClickUpHTTPError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (e *ClickUpHTTPError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("clickup: %s (HTTP %d)", e.Message, e.StatusCode)
}
