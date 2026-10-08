package taskmaster

import (
	"context"
	"encoding/json"
)

const ToolName = "perform_task"

func CallPerformTask(ctx context.Context, caller DownstreamCaller, rawInput []byte) (string, error) {
	var in struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return "", err
	}
	return HandlePerformTask(ctx, caller, in.Task)
}
