package models

// AssigneeRef is a minimal assignee record for planning and prompts.
type AssigneeRef struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// TaskContext is normalized ClickUp task data for milestone generation (Phase 4+).
type TaskContext struct {
	TaskID           string        `json:"task_id"`
	Name             string        `json:"name"`
	Description      string        `json:"description"`
	Status           string        `json:"status"`
	Priority         string        `json:"priority"`
	Assignees        []AssigneeRef `json:"assignees"`
	CustomFieldsJSON []byte        `json:"-"`
	ListID           string        `json:"list_id"`
	ListName         string        `json:"list_name"`
	FolderID         string        `json:"folder_id"`
	FolderName       string        `json:"folder_name"`
	SpaceID          string        `json:"space_id"`
	SpaceName        string        `json:"space_name"`
	TeamID           string        `json:"team_id"`
	URL              string        `json:"url"`
	DateCreatedMs    string        `json:"date_created_ms"`
	DateUpdatedMs    string        `json:"date_updated_ms"`
	DueDateMs        string        `json:"due_date_ms"`
	RawTaskJSON      []byte        `json:"-"`
	CommentsJSON     []byte        `json:"-"`
}
