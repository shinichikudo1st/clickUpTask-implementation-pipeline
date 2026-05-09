package clock

import "time"

// Now returns the current UTC time (injectable in tests later).
func Now() time.Time {
	return time.Now().UTC()
}
