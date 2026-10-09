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

	mu   sync.Mutex
	seen map[string]time.Time
}

func (h *AuthHandler) PerformTask(ctx context.Context, edgeToken, task string) (string, error) {
	id, err := VerifyEdgeToken(h.EdgeSecret, edgeToken)
	if err != nil {
		return "", err
	}
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
		return "", errors.New("deny")
	}
	h.seen[id.JTI] = now.Add(10 * time.Minute)
	h.mu.Unlock()
	sub, key, err := h.Directory.ResolveKey(ctx, id.Phone)
	if err != nil {
		return "", fmt.Errorf("deny: %w", err)
	}
	if sub != id.UserSub || key == "" {
		return "", errors.New("deny")
	}
	return HandlePerformTask(ctx, h.NewCaller(key), task)
}
