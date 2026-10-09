package taskmaster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPDirectory struct {
	BaseURL string
	HTTP    *http.Client
}

func (d *HTTPDirectory) ResolveKey(ctx context.Context, phone string) (string, string, error) {
	httpClient := d.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	u := strings.TrimRight(d.BaseURL, "/") + "/resolve?phone=" +
		url.QueryEscape(phone) + "&reveal=key"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", err
	}
	// No bearer by design: in-cluster transport trusts NetworkPolicy,
	// not tokens. Key material is gated by network identity.
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("directory deny: status %d", resp.StatusCode)
	}
	var out struct {
		UserSub string `json:"user_sub"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	if out.UserSub == "" || out.APIKey == "" {
		return "", "", fmt.Errorf("directory deny: incomplete record")
	}
	return out.UserSub, out.APIKey, nil
}
