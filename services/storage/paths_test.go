package storage

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMilestoneObjectPath(t *testing.T) {
	gid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	p, err := MilestoneObjectPath("9abc", gid, "my-task-milestone.md")
	if err != nil {
		t.Fatal(err)
	}
	want := "9abc/11111111-1111-1111-1111-111111111111/my-task-milestone.md"
	if p != want {
		t.Fatalf("got %q want %q", p, want)
	}
}

func TestMilestoneObjectPath_emptyFileName(t *testing.T) {
	gid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	p, err := MilestoneObjectPath("t1", gid, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p, "/milestone.md") {
		t.Fatalf("got %q", p)
	}
}

func TestMilestoneObjectPath_rejectsBad(t *testing.T) {
	gid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	if _, err := MilestoneObjectPath("", gid, "x.md"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := MilestoneObjectPath("t", uuid.Nil, "x.md"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := MilestoneObjectPath("t", gid, "../evil.md"); err == nil {
		t.Fatal("expected error")
	}
}
