package taskmaster

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type performTaskArgs struct {
	Task string `json:"task" jsonschema:"the task to perform"`
}

func NewMCPServer(caller DownstreamCaller) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "taskmaster"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolName,
		Description: "Perform a task and return simple readable text",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args performTaskArgs) (*mcp.CallToolResult, any, error) {
		result, err := HandlePerformTask(ctx, caller, args.Task)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil, nil
	})
	return server
}

func NewHTTPHandler(caller DownstreamCaller) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return NewMCPServer(caller)
	}, nil)
}
