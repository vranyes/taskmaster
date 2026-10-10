package taskmaster

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// HashForLog returns a short stable hash for correlating an identity
// (user sub, phone, jti) in logs without recording the raw value.
// PLAN.md: logs carry user-id hash only — no raw phones, keys, OTPs, content.
func HashForLog(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}

func loggerOrDefault(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}

// requestIDKey carries the per-HTTP-request correlation ID through
// context.Context so every stage (http -> mcp tool -> auth -> handler ->
// downstream) can tag its logs with the same request_id.
type requestIDKey struct{}

// ContextWithRequestID returns a context carrying id.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request ID, or "" when absent.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// newRequestID mints a 16-hex-char correlation ID.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

// logWithRequestID returns l.With("request_id", id) when ctx carries one,
// otherwise l unchanged. Never logs anything itself.
func logWithRequestID(l *slog.Logger, ctx context.Context) *slog.Logger {
	if id := RequestIDFromContext(ctx); id != "" {
		return l.With("request_id", id)
	}
	return l
}

// remoteAddrHash returns a stable hash of the client IP (port stripped)
// so logs can correlate callers without recording raw IPs.
func remoteAddrHash(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	return HashForLog(host)
}

// SetupDefaultLogger installs a JSON slog default honoring
// TASKMASTER_LOG_LEVEL (debug|info|warn|error, default info).
// Safe to call once from main; library code must use loggerOrDefault
// so tests that build handlers without a Logger still work.
func SetupDefaultLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("TASKMASTER_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	l := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(l)
	return l
}

// statusRecorder captures the status code and bytes written for HTTP
// request logging. It forwards Flush so SSE / streaming handlers (MCP
// streamable GET) keep working when wrapped.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Flush() {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// WithRequestLogging wraps h with method/path/status/duration logs.
// It mints (or honors X-Request-ID) a per-request correlation ID, stashes
// it in the request context, and tags every log with request_id so later
// stages (tool/auth/handler/downstream) correlate.
//
// Identity: the HTTP edge has no verified identity yet (the edge JWT lives
// inside the MCP payload), so this logs transport facts only — never
// bodies, tokens, or keys. Caller identity (hashed) is attached by the
// auth layer under the same request_id. The client IP is logged as
// remote_addr_hash only.
func WithRequestLogging(l *slog.Logger, h http.Handler) http.Handler {
	log := loggerOrDefault(l)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		// Cap a client-supplied ID so a giant header can't bloat logs.
		if len(requestID) > 64 {
			requestID = requestID[:64]
		}
		ctx := ContextWithRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)
		reqLog := log.With("request_id", requestID)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"proto", r.Proto,
			"host", r.Host,
			"content_length", r.ContentLength,
		}
		if q := r.URL.RawQuery; q != "" {
			// Keys only — values may carry PII/tokens.
			attrs = append(attrs, "query_keys", queryKeys(r), "query_bytes", len(q))
		}
		if ct := r.Header.Get("Content-Type"); ct != "" {
			attrs = append(attrs, "content_type", ct)
		}
		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, "user_agent", ua)
		}
		if h := remoteAddrHash(r); h != "" {
			attrs = append(attrs, "remote_addr_hash", h)
		}
		// MCP session header identifies a streamable session; hash it.
		if s := r.Header.Get("Mcp-Session-Id"); s != "" {
			attrs = append(attrs, "mcp_session_hash", HashForLog(s))
		} else if s := r.Header.Get("mcp-session-id"); s != "" {
			attrs = append(attrs, "mcp_session_hash", HashForLog(s))
		}
		reqLog.Debug("http.request.start", attrs...)
		// Echo the ID back so callers can correlate client-side.
		w.Header().Set("X-Request-ID", requestID)
		rec := &statusRecorder{ResponseWriter: w}
		h.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		done := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"response_bytes", rec.bytes,
		}
		if status >= 500 {
			reqLog.Warn("http.request.done", done...)
		} else {
			reqLog.Info("http.request.done", done...)
		}
	})
}

// queryKeys returns sorted-ish unique query parameter names for logging
// without values.
func queryKeys(r *http.Request) []string {
	q := r.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	return keys
}
