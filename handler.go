package taskmaster

import (
	"context"
	"log/slog"
	"time"
)

const DefaultTimeout = 30 * time.Second

func withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, DefaultTimeout)
}

func HandlePerformTask(ctx context.Context, caller DownstreamCaller, task string) (string, error) {
	return HandlePerformTaskWithLogger(ctx, nil, caller, task)
}

// HandlePerformTaskWithLogger is the logged core. Logger may be nil (uses slog.Default()).
// It logs sizes, durations and outcome only — never task/result content.
// When ctx carries a request ID (see ContextWithRequestID) every log is
// tagged with it; when the caller passes an identity-enriched logger
// (auth layer does), downstream logs inherit user_sub_hash/call_id.
func HandlePerformTaskWithLogger(ctx context.Context, l *slog.Logger, caller DownstreamCaller, task string) (string, error) {
	log := logWithRequestID(loggerOrDefault(l), ctx)
	start := time.Now()
	taskBytes := len(task)
	log.Debug("handler.start", "task_bytes", taskBytes)
	if err := ValidateTask(task); err != nil {
		log.Warn("handler.validation_failed",
			"stage", "task",
			"task_bytes", taskBytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", err
	}
	log.Debug("handler.validation.task.ok", "task_bytes", taskBytes)
	log.Debug("handler.downstream.start", "task_bytes", taskBytes)
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()
	result, err := caller.PerformTask(ctx, task)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		log.Warn("handler.downstream.error",
			"task_bytes", taskBytes,
			"duration_ms", duration,
			"error", err.Error(),
		)
		return "", err
	}
	log.Debug("handler.downstream.done",
		"task_bytes", taskBytes,
		"result_bytes", len(result),
		"duration_ms", duration,
	)
	if err := ValidateResult(result); err != nil {
		log.Warn("handler.validation_failed",
			"stage", "result",
			"task_bytes", taskBytes,
			"result_bytes", len(result),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return "", err
	}
	log.Info("handler.done",
		"task_bytes", taskBytes,
		"result_bytes", len(result),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return result, nil
}
