package taskmaster

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFakeCaller_ReturnsScriptedResult(t *testing.T) {
	fake := &FakeCaller{Result: "You have 3 emails. The urgent one is from Ana."}
	got, err := fake.PerformTask(context.Background(), "summarize my inbox")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "You have 3 emails. The urgent one is from Ana." {
		t.Fatalf("unexpected result: %q", got)
	}
}

func TestFakeCaller_ReturnsScriptedError(t *testing.T) {
	fake := &FakeCaller{Err: errors.New("boom")}
	_, err := fake.PerformTask(context.Background(), "summarize my inbox")
	if err == nil {
		t.Fatal("expected scripted error, got nil")
	}
	if err.Error() != "boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFakeCaller_RespectsContextTimeout(t *testing.T) {
	fake := &FakeCaller{BlockUntilCtxDone: true}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := fake.PerformTask(ctx, "summarize my inbox")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected context timeout error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("fake did not respect context timeout, took %v", elapsed)
	}
}
