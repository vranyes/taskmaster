package taskmaster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPDirectory_ResolvesKey(t *testing.T) {
	var gotAuth, gotReveal, gotPhone string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPhone = r.URL.Query().Get("phone")
		gotReveal = r.URL.Query().Get("reveal")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"user_sub": "kanidm-sub-1", "api_key": "user-1-key",
		})
	}))
	defer srv.Close()

	d := &HTTPDirectory{BaseURL: srv.URL, Secret: "taskmaster-secret", HTTP: &http.Client{}}
	sub, key, err := d.ResolveKey(context.Background(), "+15550109999")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sub != "kanidm-sub-1" || key != "user-1-key" {
		t.Fatalf("unexpected resolve: %q %q", sub, key)
	}
	if gotAuth != "Bearer taskmaster-secret" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotPhone != "+15550109999" || gotReveal != "key" {
		t.Fatalf("unexpected query: phone=%q reveal=%q", gotPhone, gotReveal)
	}
}

func TestHTTPDirectory_DenyOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"deny"}`))
	}))
	defer srv.Close()

	d := &HTTPDirectory{BaseURL: srv.URL, Secret: "s", HTTP: &http.Client{}}
	if _, _, err := d.ResolveKey(context.Background(), "+15550000000"); err == nil {
		t.Fatal("expected deny on 404, got nil")
	}
}
