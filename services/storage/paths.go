package storage

import (
	"errors"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"
)

// MilestoneObjectPath returns a stable object key: {taskID}/{generationID}/{fileNameBase}.
func MilestoneObjectPath(clickUpTaskID string, generationID uuid.UUID, fileName string) (string, error) {
	clickUpTaskID = strings.TrimSpace(clickUpTaskID)
	if clickUpTaskID == "" {
		return "", errors.New("clickup_task_id is required")
	}
	if generationID == uuid.Nil {
		return "", errors.New("generation id is required")
	}
	trimmed := strings.TrimSpace(fileName)
	if strings.Contains(trimmed, "..") {
		return "", errors.New("invalid file name")
	}
	base := path.Base(trimmed)
	if base == "" || base == "." || base == "/" {
		base = "milestone.md"
	}
	if err := validateRelativeKey(path.Join(clickUpTaskID, generationID.String(), base)); err != nil {
		return "", err
	}
	return path.Join(clickUpTaskID, generationID.String(), base), nil
}

func validateRelativeKey(p string) error {
	if p == "" || p == "." {
		return errors.New("invalid object path")
	}
	if path.IsAbs(p) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return errors.New("object path must be relative")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return errors.New("object path must not contain '..'")
		}
	}
	return nil
}

// encodePathSegments joins URL path segments with per-segment escaping (for REST paths).
func encodePathSegments(segments ...string) string {
	var b strings.Builder
	for i, s := range segments {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(url.PathEscape(s))
	}
	return b.String()
}
