package taskmaster

import (
	"context"
	"testing"
)

func TestCallPerformTask_ReturnsResult(t *testing.T) {
	fake := &FakeCaller{Result: "You have 3 emails. The urgent one is from Ana."}
	got, err := CallPerformTask(context.Background(), fake, []byte(`{"task":"summarize my inbox"}`))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "You have 3 emails. The urgent one is from Ana." {
		t.Fatalf("unexpected result: %q", got)
	}
}

func TestCallPerformTask_RejectsBadJSON(t *testing.T) {
	fake := &FakeCaller{Result: "never"}
	if _, err := CallPerformTask(context.Background(), fake, []byte(`{`)); err == nil {
		t.Fatal("expected JSON error, got nil")
	}
}

func TestCallPerformTask_RejectsMissingTask(t *testing.T) {
	fake := &FakeCaller{Result: "never"}
	if _, err := CallPerformTask(context.Background(), fake, []byte(`{}`)); err == nil {
		t.Fatal("expected validation error for missing task, got nil")
	}
}

func TestCallPerformTask_RejectsEmptyTask(t *testing.T) {
	fake := &FakeCaller{Result: "never"}
	if _, err := CallPerformTask(context.Background(), fake, []byte(`{"task":"   "}`)); err == nil {
		t.Fatal("expected validation error for empty task, got nil")
	}
}
