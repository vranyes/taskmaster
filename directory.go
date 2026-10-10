package taskmaster

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPDirectory struct {
	BaseURL string
	HTTP    *http.Client
	Logger  *slog.Logger
}

func (d *HTTPDirectory) ResolveKey(ctx context.Context, phone string) (string, string, error) {
	log := logWithRequestID(loggerOrDefault(d.Logger), ctx)
	start := time.Now()
	phoneHash := HashForLog(phone)
	log.Debug("directory.resolve.start", "phone_hash", phoneHash)
	httpClient := d.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	u := strings.TrimRight(d.BaseURL, "/") + "/resolve?phone=" +
		url.QueryEscape(phone) + "&reveal=key"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		log.Warn("directory.request_build_failed",
			"phone_hash", phoneHash,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", "", err
	}
	// No bearer by design: in-cluster transport trusts NetworkPolicy,
	// not tokens. Key material is gated by network identity.
	// Never log phone, user_sub, or api_key raw — hashes only.
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Warn("directory.request.error",
			"phone_hash", phoneHash,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warn("directory.resolve.deny",
			"phone_hash", phoneHash,
			"status", resp.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return "", "", fmt.Errorf("directory deny: status %d", resp.StatusCode)
	}
	var out struct {
		UserSub string `json:"user_sub"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Warn("directory.decode_failed",
			"phone_hash", phoneHash,
			"status", resp.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", "", err
	}
	if out.UserSub == "" || out.APIKey == "" {
		log.Warn("directory.resolve.incomplete",
			"phone_hash", phoneHash,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return "", "", fmt.Errorf("directory deny: incomplete record")
	}
	log.Info("directory.resolve.done",
		"phone_hash", phoneHash,
		"user_sub_hash", HashForLog(out.UserSub),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return out.UserSub, out.APIKey, nil
}
