package taskmaster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func httptestChatServer(t *testing.T, gotModel, gotTask *string, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		*gotModel = req.Model
		if len(req.Messages) > 0 {
			*gotTask = req.Messages[len(req.Messages)-1].Content
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": reply}},
			},
		})
	}))
}

func TestOpenAICaller_ReturnsContent(t *testing.T) {
	var gotModel, gotTask string
	srv := httptestChatServer(t, &gotModel, &gotTask, "You have 3 emails. The urgent one is from Ana.")
	defer srv.Close()

	c := &OpenAICaller{
		BaseURL: srv.URL,
		Model:   "test-model",
		HTTP:    &http.Client{},
	}
	got, err := c.PerformTask(context.Background(), "summarize my inbox")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "You have 3 emails. The urgent one is from Ana." {
		t.Fatalf("unexpected result: %q", got)
	}
	if gotModel != "test-model" {
		t.Fatalf("unexpected model: %q", gotModel)
	}
	if gotTask != "summarize my inbox" {
		t.Fatalf("unexpected task sent: %q", gotTask)
	}
}

func TestOpenAICaller_ErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "overloaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := &OpenAICaller{BaseURL: srv.URL, Model: "m", HTTP: &http.Client{}}
	if _, err := c.PerformTask(context.Background(), "summarize my inbox"); err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

func TestOpenAICaller_ErrorsOnNoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	c := &OpenAICaller{BaseURL: srv.URL, Model: "m", HTTP: &http.Client{}}
	if _, err := c.PerformTask(context.Background(), "summarize my inbox"); err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

func TestOpenAICaller_SendsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := &OpenAICaller{BaseURL: srv.URL, Model: "m", APIKey: "secret", HTTP: &http.Client{}}
	if _, err := c.PerformTask(context.Background(), "hi"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
}
