package taskmaster

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

const ToolName = "perform_task"

func CallPerformTask(ctx context.Context, caller DownstreamCaller, rawInput []byte) (string, error) {
	return CallPerformTaskWithLogger(ctx, nil, caller, rawInput)
}

// CallPerformTaskWithLogger parses raw input and delegates, logging
// input bytes and outcome only — never task content.
func CallPerformTaskWithLogger(ctx context.Context, l *slog.Logger, caller DownstreamCaller, rawInput []byte) (string, error) {
	base := loggerOrDefault(l)
	log := logWithRequestID(base, ctx)
	start := time.Now()
	log.Debug("mcp.call.start", "input_bytes", len(rawInput))
	var in struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal(rawInput, &in); err != nil {
		log.Warn("mcp.parse_failed",
			"input_bytes", len(rawInput),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", err
	}
	// Pass base (untagged): HandlePerformTaskWithLogger tags request_id
	// itself from ctx, keeping a single request_id key per line.
	return HandlePerformTaskWithLogger(ctx, base, caller, in.Task)
}
