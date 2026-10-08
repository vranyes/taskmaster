package taskmaster

import (
	"context"
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
	if err := ValidateTask(task); err != nil {
		return "", err
	}
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()
	result, err := caller.PerformTask(ctx, task)
	if err != nil {
		return "", err
	}
	if err := ValidateResult(result); err != nil {
		return "", err
	}
	return result, nil
}
