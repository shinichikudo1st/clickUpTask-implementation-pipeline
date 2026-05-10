package db

import (
	"context"
	"testing"
)

func TestConnect_emptyURLReturnsNilDB(t *testing.T) {
	t.Parallel()
	sqlDB, err := Connect(context.Background(), "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sqlDB != nil {
		t.Fatal("expected nil database")
	}
}
