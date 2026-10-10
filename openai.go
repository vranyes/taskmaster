package taskmaster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type OpenAICaller struct {
	BaseURL string
	Model   string
	APIKey  string
	HTTP    *http.Client
	Logger  *slog.Logger
}

func (c *OpenAICaller) PerformTask(ctx context.Context, task string) (string, error) {
	log := logWithRequestID(loggerOrDefault(c.Logger), ctx)
	start := time.Now()
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	body, _ := json.Marshal(map[string]any{
		"model": c.Model,
		"messages": []map[string]any{
			{"role": "user", "content": task},
		},
	})
	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Warn("downstream.request_build_failed",
			"model", c.Model,
			"task_bytes", len(task),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	// Never log task content, API keys, or response content.
	log.Debug("downstream.request.start",
		"model", c.Model,
		"task_bytes", len(task),
		"request_bytes", len(body),
	)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Warn("downstream.request.error",
			"model", c.Model,
			"task_bytes", len(task),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warn("downstream.request.bad_status",
			"model", c.Model,
			"task_bytes", len(task),
			"status", resp.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return "", fmt.Errorf("downstream status %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Warn("downstream.decode_failed",
			"model", c.Model,
			"task_bytes", len(task),
			"status", resp.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", err
	}
	if len(out.Choices) == 0 {
		log.Warn("downstream.no_choices",
			"model", c.Model,
			"task_bytes", len(task),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return "", fmt.Errorf("downstream returned no choices")
	}
	content := out.Choices[0].Message.Content
	log.Info("downstream.request.done",
		"model", c.Model,
		"task_bytes", len(task),
		"result_bytes", len(content),
		"status", resp.StatusCode,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return content, nil
}
