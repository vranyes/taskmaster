package taskmaster

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func testBufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestHashForLog_StableAndShort(t *testing.T) {
	a := HashForLog("+15550109999")
	b := HashForLog("+15550109999")
	if a != b || len(a) != 16 {
		t.Fatalf("expected stable 16-char hash, got %q", a)
	}
	if c := HashForLog("+15550109000"); c == a {
		t.Fatal("expected different input to hash differently")
	}
	if strings.Contains(a, "555") {
		t.Fatalf("hash leaks input: %q", a)
	}
}

func TestHandlePerformTask_LogsSizesNotContent(t *testing.T) {
	var buf bytes.Buffer
	log := testBufferLogger(&buf)
	secretTask := "summarize my inbox about SECRET-TOPIC-xyz"
	fake := &FakeCaller{Result: "ok result SECRET-RESULT-xyz"}
	if _, err := HandlePerformTaskWithLogger(context.Background(), log, fake, secretTask); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "SECRET-TOPIC-xyz") || strings.Contains(out, "SECRET-RESULT-xyz") {
		t.Fatalf("logs must not contain task/result content:\n%s", out)
	}
	if !strings.Contains(out, "task_bytes") || !strings.Contains(out, "result_bytes") {
		t.Fatalf("expected size fields in logs:\n%s", out)
	}
}

func TestAuthHandler_LogsHashesNotRaw(t *testing.T) {
	var buf bytes.Buffer
	log := testBufferLogger(&buf)
	h := &AuthHandler{
		EdgeSecret: testEdgeSecret(),
		Directory:  &FakeDirectory{Sub: "kanidm-sub-1", Key: "user-1-key"},
		NewCaller: func(apiKey string) DownstreamCaller {
			return &FakeCaller{Result: "ok"}
		},
		Logger: log,
	}
	tok := mintTestToken(t, testEdgeSecret(), "kanidm-sub-1", "+15550109999", "call-9", "jti-9", time.Now().Add(time.Hour))
	if _, err := h.PerformTask(context.Background(), tok, "hello"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	out := buf.String()
	for _, raw := range []string{"+15550109999", "kanidm-sub-1", "user-1-key", tok} {
		if raw != "" && strings.Contains(out, raw) {
			t.Fatalf("logs must not contain raw %q:\n%s", raw, out)
		}
	}
	if !strings.Contains(out, "user_sub_hash") || !strings.Contains(out, "call_id=call-9") {
		t.Fatalf("expected hashed identity + call_id in logs:\n%s", out)
	}
}
