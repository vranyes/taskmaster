package taskmaster

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type performTaskArgs struct {
	Task string `json:"task" jsonschema:"the task to perform"`
}

func NewMCPServer(caller DownstreamCaller) *mcp.Server {
	return NewMCPServerWithLogger(nil, caller)
}

// NewMCPServerWithLogger wires the tool with structured logs.
// Logger may be nil. Only task_bytes/result_bytes/duration are logged.
// When ctx carries a request ID (HTTP path), tool logs are tagged with it.
func NewMCPServerWithLogger(l *slog.Logger, caller DownstreamCaller) *mcp.Server {
	log := loggerOrDefault(l)
	server := mcp.NewServer(&mcp.Implementation{Name: "taskmaster"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolName,
		Description: "Perform a task and return simple readable text",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args performTaskArgs) (*mcp.CallToolResult, any, error) {
		start := time.Now()
		callLog := logWithRequestID(log, ctx)
		callLog.Debug("mcp.tool.start", "tool", ToolName, "task_bytes", len(args.Task))
		result, err := HandlePerformTaskWithLogger(ctx, log, caller, args.Task)
		if err != nil {
			callLog.Warn("mcp.tool.error",
				"tool", ToolName,
				"task_bytes", len(args.Task),
				"duration_ms", time.Since(start).Milliseconds(),
				"error", err.Error(),
			)
			return nil, nil, err
		}
		callLog.Info("mcp.tool.done",
			"tool", ToolName,
			"task_bytes", len(args.Task),
			"result_bytes", len(result),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil, nil
	})
	return server
}

type performAuthTaskArgs struct {
	Task      string `json:"task" jsonschema:"the task to perform"`
	EdgeToken string `json:"edge_token" jsonschema:"per-call edge JWT minted by voice-bridge"`
}

func NewAuthMCPServer(h *AuthHandler) *mcp.Server {
	log := h.logger()
	server := mcp.NewServer(&mcp.Implementation{Name: "taskmaster"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolName,
		Description: "Perform a task and return simple readable text",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args performAuthTaskArgs) (*mcp.CallToolResult, any, error) {
		start := time.Now()
		// Edge token / task content never logged; AuthHandler logs hashes + sizes.
		// Log the token hash + sizes here so the tool span correlates with
		// auth.* logs under the same request_id even when verification fails.
		callLog := logWithRequestID(log, ctx)
		startAttrs := []any{"tool", ToolName, "task_bytes", len(args.Task), "token_bytes", len(args.EdgeToken)}
		if args.EdgeToken != "" {
			startAttrs = append(startAttrs, "token_hash", HashForLog(args.EdgeToken))
		}
		callLog.Debug("mcp.tool.start", startAttrs...)
		result, err := h.PerformTask(ctx, args.EdgeToken, args.Task)
		if err != nil {
			errAttrs := []any{
				"tool", ToolName,
				"task_bytes", len(args.Task),
				"token_bytes", len(args.EdgeToken),
				"duration_ms", time.Since(start).Milliseconds(),
				"error", err.Error(),
			}
			if args.EdgeToken != "" {
				errAttrs = append(errAttrs, "token_hash", HashForLog(args.EdgeToken))
			}
			callLog.Warn("mcp.tool.error", errAttrs...)
			return nil, nil, err
		}
		doneAttrs := []any{
			"tool", ToolName,
			"task_bytes", len(args.Task),
			"result_bytes", len(result),
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if args.EdgeToken != "" {
			doneAttrs = append(doneAttrs, "token_hash", HashForLog(args.EdgeToken))
		}
		callLog.Info("mcp.tool.done", doneAttrs...)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil, nil
	})
	return server
}

func NewAuthHTTPHandler(h *AuthHandler) http.Handler {
	return WithRequestLogging(h.logger(), mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return NewAuthMCPServer(h)
	}, nil))
}

func NewHTTPHandler(caller DownstreamCaller) http.Handler {
	return NewHTTPHandlerWithLogger(nil, caller)
}

// NewHTTPHandlerWithLogger wraps the streamable handler with request logs.
func NewHTTPHandlerWithLogger(l *slog.Logger, caller DownstreamCaller) http.Handler {
	return WithRequestLogging(l, mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return NewMCPServerWithLogger(l, caller)
	}, nil))
}
