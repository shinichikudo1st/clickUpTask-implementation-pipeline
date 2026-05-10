package email

import (
	"context"
	"errors"
	"testing"
	"time"
)

type flakySender struct {
	n   int
	max int
}

func (f *flakySender) SendMilestonePlan(ctx context.Context, in SendMilestoneInput) error {
	_ = ctx
	_ = in
	f.n++
	if f.n < f.max {
		return errors.New("temporary")
	}
	return nil
}

func TestSendWithRetry_successAfterFailures(t *testing.T) {
	f := &flakySender{max: 3}
	ctx := context.Background()
	in := SendMilestoneInput{TaskName: "t", DownloadURL: "https://x"}
	err := SendWithRetry(ctx, f, in, 4)
	if err != nil {
		t.Fatal(err)
	}
	if f.n != 3 {
		t.Fatalf("attempts %d", f.n)
	}
}

func TestSendWithRetry_exhausted(t *testing.T) {
	f := &flakySender{max: 99}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	in := SendMilestoneInput{TaskName: "t", DownloadURL: "https://x"}
	err := SendWithRetry(ctx, f, in, 2)
	if err == nil {
		t.Fatal("expected error")
	}
}
