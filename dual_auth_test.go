package taskmaster

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPDirectory_PrefersAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"user_sub":     "kanidm-sub-1",
			"api_key":      "pat-key",
			"access_token": "oauth-bearer",
		})
	}))
	defer srv.Close()

	d := &HTTPDirectory{BaseURL: srv.URL, HTTP: &http.Client{}}
	sub, bearer, err := d.ResolveKey(context.Background(), "+15550109999")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sub != "kanidm-sub-1" || bearer != "oauth-bearer" {
		t.Fatalf("unexpected resolve: %q %q (want oauth bearer)", sub, bearer)
	}
}

func TestHTTPDirectory_DenyWhenNoBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"user_sub": "kanidm-sub-1"})
	}))
	defer srv.Close()

	d := &HTTPDirectory{BaseURL: srv.URL, HTTP: &http.Client{}}
	if _, _, err := d.ResolveKey(context.Background(), "+15550109999"); err == nil {
		t.Fatal("expected deny when neither bearer present, got nil")
	}
}

func TestOpenAICaller_UnauthorizedIsSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "revoked", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &OpenAICaller{BaseURL: srv.URL, Model: "m", HTTP: &http.Client{}}
	_, err := c.PerformTask(context.Background(), "hi")
	if !errors.Is(err, ErrDownstreamUnauthorized) {
		t.Fatalf("401: got %v, want ErrDownstreamUnauthorized", err)
	}
}

func TestOpenAICaller_ForbiddenIsNotSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := &OpenAICaller{BaseURL: srv.URL, Model: "m", HTTP: &http.Client{}}
	_, err := c.PerformTask(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if errors.Is(err, ErrDownstreamUnauthorized) {
		t.Fatalf("403 must not trigger the 401 retry path: %v", err)
	}
}

// sequenceDirectory replays canned resolves in order (then repeats the last).
type sequenceDirectory struct {
	subs []string
	keys []string
	n    int
}

func (s *sequenceDirectory) ResolveKey(_ context.Context, _ string) (string, string, error) {
	i := s.n
	if i >= len(s.keys) {
		i = len(s.keys) - 1
	}
	s.n++
	return s.subs[i], s.keys[i], nil
}

func TestAuthHandler_RetriesOnceWithRotatedBearer(t *testing.T) {
	dir := &sequenceDirectory{
		subs: []string{"kanidm-sub-1", "kanidm-sub-1"},
		keys: []string{"old-bearer", "rotated-bearer"},
	}
	var downstreamCalls []string
	h := &AuthHandler{
		EdgeSecret: testEdgeSecret(),
		Directory:  dir,
		// FakeCaller errors are raw; wrap the 401 in the sentinel the same
		// way OpenAICaller does so the retry path triggers.
		NewCaller: func(bearer string) DownstreamCaller {
			downstreamCalls = append(downstreamCalls, bearer)
			if bearer == "old-bearer" {
				return &FakeCaller{Err: errors.Join(errors.New("downstream status 401"), ErrDownstreamUnauthorized)}
			}
			return &FakeCaller{Result: "ok after rotation"}
		},
	}
	tok := mintTestToken(t, testEdgeSecret(), "kanidm-sub-1", "+15550109999", "call-r1", "jti-r1", time.Now().Add(time.Hour))
	got, err := h.PerformTask(context.Background(), tok, "task")
	if err != nil {
		t.Fatalf("expected success after rotation, got %v", err)
	}
	if got != "ok after rotation" {
		t.Fatalf("unexpected result: %q", got)
	}
	if len(downstreamCalls) != 2 || downstreamCalls[0] != "old-bearer" || downstreamCalls[1] != "rotated-bearer" {
		t.Fatalf("downstream calls = %v, want [old-bearer rotated-bearer]", downstreamCalls)
	}
	if dir.n != 2 {
		t.Fatalf("resolves = %d, want 2 (initial + one retry)", dir.n)
	}
}

func TestAuthHandler_NoRetryWhenBearerUnchanged(t *testing.T) {
	dir := &sequenceDirectory{
		subs: []string{"kanidm-sub-1", "kanidm-sub-1"},
		keys: []string{"revoked-pat", "revoked-pat"},
	}
	var downstreamCalls int
	h := &AuthHandler{
		EdgeSecret: testEdgeSecret(),
		Directory:  dir,
		NewCaller: func(bearer string) DownstreamCaller {
			downstreamCalls++
			return &FakeCaller{Err: errors.Join(errors.New("downstream status 401"), ErrDownstreamUnauthorized)}
		},
	}
	tok := mintTestToken(t, testEdgeSecret(), "kanidm-sub-1", "+15550109999", "call-r2", "jti-r2", time.Now().Add(time.Hour))
	if _, err := h.PerformTask(context.Background(), tok, "task"); err == nil {
		t.Fatal("expected deny for revoked bearer, got nil")
	}
	if downstreamCalls != 1 {
		t.Fatalf("downstream calls = %d, want 1 (no retry on identical bearer)", downstreamCalls)
	}
}
