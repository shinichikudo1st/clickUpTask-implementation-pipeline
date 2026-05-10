package clickupwebhook

import (
	"testing"
)

func TestDedupeEventKey_historyItem(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"event":"taskAssigneeUpdated","webhook_id":"w1","task_id":"t1","history_items":[{"id":"h1","field":"assignee_add"}]}`)
	key, err := DedupeEventKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if key != "w1:h1" {
		t.Fatalf("got %q", key)
	}
}

func TestDedupeEventKey_fallbackBodyHash(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"event":"taskCreated","webhook_id":"w2","task_id":"t2","history_items":[]}`)
	key, err := DedupeEventKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) < 10 || key[:5] != "body:" {
		t.Fatalf("got %q", key)
	}
}

func TestParsePayload_errors(t *testing.T) {
	t.Parallel()
	if _, err := ParsePayload([]byte(`{`)); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParsePayload([]byte(`{}`)); err == nil {
		t.Fatal("expected error for missing event")
	}
}

func TestTaskUpdatedAssigneeChange(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"event":"taskUpdated","webhook_id":"w","task_id":"t","history_items":[{"id":"1","field":"assignee_add"}]}`)
	if !TaskUpdatedAssigneeChange(raw) {
		t.Fatal("expected true")
	}
	raw2 := []byte(`{"event":"taskUpdated","webhook_id":"w","task_id":"t","history_items":[{"id":"1","field":"status"}]}`)
	if TaskUpdatedAssigneeChange(raw2) {
		t.Fatal("expected false")
	}
}

func TestAssigneeAddMatchesUser(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"event":"taskAssigneeUpdated","history_items":[{"field":"assignee_add","after":{"id":184}}]}`)
	if !AssigneeAddMatchesUser(raw, "184") {
		t.Fatal("expected match")
	}
	if AssigneeAddMatchesUser(raw, "999") {
		t.Fatal("expected no match")
	}
	if !AssigneeAddMatchesUser(raw, "") {
		t.Fatal("empty filter should match")
	}
}
