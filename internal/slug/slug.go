package slug

// For will produce a filesystem-safe slug from a title (Phase 5+).
func For(title string) string {
	if title == "" {
		return "task"
	}
	return title
}
