package email

import (
	"strings"
	"testing"
)

func TestPrepareSendInput_attach(t *testing.T) {
	in := SendMilestoneInput{
		TaskName:     "Ship feature",
		TaskURL:      "https://clickup.example/t/1",
		FileName:     "plan.md",
		MarkdownBody: []byte("# hi"),
		DownloadURL:  "https://signed.example/x",
	}
	out, attach, err := PrepareSendInput(in, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !attach {
		t.Fatal("expected attachment")
	}
	if out.TaskName != in.TaskName {
		t.Fatal("mutated?")
	}
}

func TestPrepareSendInput_oversizeNeedsURL(t *testing.T) {
	in := SendMilestoneInput{
		TaskName:     "t",
		MarkdownBody: bytesRepeat('a', 20),
		DownloadURL:  "https://dl",
	}
	_, attach, err := PrepareSendInput(in, 10)
	if err != nil {
		t.Fatal(err)
	}
	if attach {
		t.Fatal("expected no attachment")
	}
}

func TestPrepareSendInput_oversizeNoURL(t *testing.T) {
	in := SendMilestoneInput{
		TaskName:     "t",
		MarkdownBody: bytesRepeat('b', 100),
	}
	_, _, err := PrepareSendInput(in, 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPrepareSendInput_linkOnly(t *testing.T) {
	in := SendMilestoneInput{
		TaskName:    "t",
		DownloadURL: "https://only",
	}
	_, attach, err := PrepareSendInput(in, 100)
	if err != nil {
		t.Fatal(err)
	}
	if attach {
		t.Fatal("no attachment for empty body")
	}
}

func TestSubjectFor_longTitle(t *testing.T) {
	long := strings.Repeat("x", 2000)
	s := subjectFor(SendMilestoneInput{TaskName: long})
	if len(s) > 950 {
		t.Fatalf("subject too long: %d", len(s))
	}
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
