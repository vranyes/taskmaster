package taskmaster

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const edgeAudience = "taskmaster"

type EdgeClaims struct {
	UserSub string
	Phone   string
	CallID  string
	// JTI is a unique nonce per mint, not per call: one call may
	// delegate several tasks, each with its own token. Replay
	// protection keys on JTI; CallID stays as the audit claim.
	JTI       string
	ExpiresAt time.Time
}

type EdgeIdentity struct {
	UserSub string
	Phone   string
	CallID  string
	JTI     string
}

func MintEdgeToken(secret []byte, c EdgeClaims) (string, error) {
	// Fail closed: without a caller-supplied nonce two mints over
	// identical claims produce byte-identical tokens, and the
	// replay window becomes the expiry instead of one use.
	if c.JTI == "" {
		return "", errors.New("deny: empty jti")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]any{
		"sub": c.UserSub, "phone": c.Phone, "jti": c.JTI,
		"call_id": c.CallID, "aud": edgeAudience, "exp": c.ExpiresAt.Unix(),
	})
	if err != nil {
		return "", err
	}
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyEdgeToken(secret []byte, tok string) (EdgeIdentity, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return EdgeIdentity{}, errors.New("deny")
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(got, want) != 1 {
		return EdgeIdentity{}, errors.New("deny")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return EdgeIdentity{}, errors.New("deny")
	}
	var p struct {
		Sub    string `json:"sub"`
		Phone  string `json:"phone"`
		Jti    string `json:"jti"`
		CallID string `json:"call_id"`
		Aud    string `json:"aud"`
		Exp    int64  `json:"exp"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return EdgeIdentity{}, errors.New("deny")
	}
	if p.Aud != edgeAudience || p.Sub == "" || p.Jti == "" || p.CallID == "" {
		return EdgeIdentity{}, errors.New("deny")
	}
	if time.Now().Unix() > p.Exp {
		return EdgeIdentity{}, errors.New("deny")
	}
	return EdgeIdentity{UserSub: p.Sub, Phone: p.Phone, CallID: p.CallID, JTI: p.Jti}, nil
}

type Directory interface {
	ResolveKey(ctx context.Context, phone string) (userSub, apiKey string, err error)
}

type FakeDirectory struct {
	Sub string
	Key string
	Err error
}

func (f *FakeDirectory) ResolveKey(ctx context.Context, phone string) (string, string, error) {
	if f.Err != nil {
		return "", "", f.Err
	}
	return f.Sub, f.Key, nil
}

type AuthHandler struct {
	EdgeSecret []byte
	Directory  Directory
	NewCaller  func(apiKey string) DownstreamCaller
	Logger     *slog.Logger

	mu   sync.Mutex
	seen map[string]time.Time
}

func (h *AuthHandler) logger() *slog.Logger { return loggerOrDefault(h.Logger) }

func (h *AuthHandler) PerformTask(ctx context.Context, edgeToken, task string) (string, error) {
	base := h.logger()
	log := logWithRequestID(base, ctx)
	start := time.Now()
	// Never log edgeToken, phone, keys, or task content — sizes and hashes only.
	tokenBytes := len(edgeToken)
	var tokenHash string
	if edgeToken != "" {
		tokenHash = HashForLog(edgeToken)
	}
	log.Debug("auth.request.start",
		"task_bytes", len(task),
		"token_bytes", tokenBytes,
	)
	id, err := VerifyEdgeToken(h.EdgeSecret, edgeToken)
	if err != nil {
		attrs := []any{
			"reason", "verify_failed",
			"task_bytes", len(task),
			"token_bytes", tokenBytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		}
		if tokenHash != "" {
			attrs = append(attrs, "token_hash", tokenHash)
		}
		log.Warn("auth.edge_token.invalid", attrs...)
		return "", err
	}
	attrs := []any{
		"user_sub_hash", HashForLog(id.UserSub),
		"phone_hash", HashForLog(id.Phone),
		"call_id", id.CallID,
		"jti_hash", HashForLog(id.JTI),
		"task_bytes", len(task),
		"token_hash", HashForLog(edgeToken),
	}
	log.Debug("auth.edge_token.verified", attrs...)
	// Identity-enriched loggers. reqLog is for auth's own span logs;
	// downstreamLog is identity-only (no request_id): HandlePerformTask-
	// WithLogger tags request_id from ctx itself, so passing a pre-tagged
	// logger would emit duplicate request_id keys.
	identity := base.With(
		"user_sub_hash", HashForLog(id.UserSub),
		"phone_hash", HashForLog(id.Phone),
		"call_id", id.CallID,
		"jti_hash", HashForLog(id.JTI),
	)
	log = logWithRequestID(identity, ctx)
	downstreamLog := identity
	now := time.Now()
	h.mu.Lock()
	if h.seen == nil {
		h.seen = map[string]time.Time{}
	}
	for jti, exp := range h.seen {
		if !exp.After(now) {
			delete(h.seen, jti)
		}
	}
	// Single-use binds the nonce, not the call: one call may mint
	// several tokens (one per delegation), each usable once.
	if exp, ok := h.seen[id.JTI]; ok && exp.After(now) {
		h.mu.Unlock()
		log.Warn("auth.edge_token.replay",
			"task_bytes", len(task),
			"reason", "replay",
		)
		return "", errors.New("deny")
	}
	h.seen[id.JTI] = now.Add(10 * time.Minute)
	h.mu.Unlock()
	dirStart := time.Now()
	log.Debug("auth.directory.start", "task_bytes", len(task))
	sub, key, err := h.Directory.ResolveKey(ctx, id.Phone)
	if err != nil {
		log.Warn("auth.directory.error",
			"task_bytes", len(task),
			"duration_ms", time.Since(start).Milliseconds(),
			"directory_ms", time.Since(dirStart).Milliseconds(),
			"error", err.Error(),
		)
		return "", fmt.Errorf("deny: %w", err)
	}
	if sub != id.UserSub || key == "" {
		log.Warn("auth.directory.mismatch",
			"task_bytes", len(task),
			"duration_ms", time.Since(start).Milliseconds(),
			"directory_ms", time.Since(dirStart).Milliseconds(),
			"reason", "sub_mismatch_or_empty_key",
		)
		return "", errors.New("deny")
	}
	log.Debug("auth.directory.resolved",
		"directory_ms", time.Since(dirStart).Milliseconds(),
	)
	log.Debug("auth.downstream.start", "task_bytes", len(task))
	result, err := HandlePerformTaskWithLogger(ctx, downstreamLog, h.NewCaller(key), task)
	if err != nil {
		log.Warn("auth.perform_task.error",
			"task_bytes", len(task),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", err
	}
	log.Info("auth.perform_task.done",
		"task_bytes", len(task),
		"result_bytes", len(result),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return result, nil
}
