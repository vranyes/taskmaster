package taskmaster

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errDownstreamBoom = errors.New("downstream boom")

func TestHandlePerformTask_ReturnsDownstreamResult(t *testing.T) {
	fake := &FakeCaller{Result: "You have 3 emails. The urgent one is from Ana."}
	got, err := HandlePerformTask(context.Background(), fake, "summarize my inbox")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "You have 3 emails. The urgent one is from Ana." {
		t.Fatalf("unexpected result: %q", got)
	}
}

func TestHandlePerformTask_RejectsEmptyTask(t *testing.T) {
	fake := &FakeCaller{Result: "should never be returned"}
	_, err := HandlePerformTask(context.Background(), fake, "   ")
	if err == nil {
		t.Fatal("expected validation error for empty task, got nil")
	}
}

func TestHandlePerformTask_PropagatesDownstreamError(t *testing.T) {
	fake := &FakeCaller{Err: errDownstreamBoom}
	_, err := HandlePerformTask(context.Background(), fake, "summarize my inbox")
	if err == nil {
		t.Fatal("expected downstream error, got nil")
	}
	if err.Error() != "downstream boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandlePerformTask_RejectsEmptyResult(t *testing.T) {
	fake := &FakeCaller{Result: "   "}
	_, err := HandlePerformTask(context.Background(), fake, "summarize my inbox")
	if err == nil {
		t.Fatal("expected validation error for empty result, got nil")
	}
}

func TestHandlePerformTask_RejectsOversizeResult(t *testing.T) {
	big := make([]byte, 8193)
	for i := range big {
		big[i] = 'a'
	}
	fake := &FakeCaller{Result: string(big)}
	_, err := HandlePerformTask(context.Background(), fake, "summarize my inbox")
	if err == nil {
		t.Fatal("expected validation error for oversize result, got nil")
	}
}

func TestHandlePerformTask_PropagatesContextTimeout(t *testing.T) {
	fake := &FakeCaller{BlockUntilCtxDone: true}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := HandlePerformTask(ctx, fake, "summarize my inbox")
	if err == nil {
		t.Fatal("expected context timeout error, got nil")
	}
}

func TestWithDefaultTimeout_Adds30sWhenMissing(t *testing.T) {
	ctx, cancel := withDefaultTimeout(context.Background())
	defer cancel()
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set, got none")
	}
	remaining := time.Until(dl)
	if remaining < 29*time.Second || remaining > 30*time.Second {
		t.Fatalf("expected ~30s deadline, got %v", remaining)
	}
}

func TestWithDefaultTimeout_KeepsExistingDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, cancel2 := withDefaultTimeout(ctx)
	defer cancel2()
	dl, ok := got.Deadline()
	if !ok {
		t.Fatal("expected deadline to be kept, got none")
	}
	if time.Until(dl) > 5*time.Second {
		t.Fatalf("expected existing ~5s deadline kept, got %v", time.Until(dl))
	}
}

func TestHandlePerformTask_SetsDefaultTimeout(t *testing.T) {
	fake := &FakeCaller{Result: "You have 3 emails. The urgent one is from Ana."}
	_, err := HandlePerformTask(context.Background(), fake, "summarize my inbox")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !fake.SawDeadline {
		t.Fatal("expected downstream call to carry a deadline, got none")
	}
}
