package taskmaster

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServer_PerformTaskViaClient(t *testing.T) {
	fake := &FakeCaller{Result: "You have 3 emails. The urgent one is from Ana."}
	server := NewMCPServer(fake)

	clientTrans, serverTrans := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverErr := make(chan error, 1)
	var ss *mcp.ServerSession
	go func() {
		var err error
		ss, err = server.Connect(ctx, serverTrans, nil)
		serverErr <- err
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(
		ctx, clientTrans, nil,
	)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	if err := <-serverErr; err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      ToolName,
		Arguments: map[string]any{"task": "summarize my inbox"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if tc.Text != "You have 3 emails. The urgent one is from Ana." {
		t.Fatalf("unexpected text: %q", tc.Text)
	}
}

func TestMCPServer_RejectsEmptyTask(t *testing.T) {
	fake := &FakeCaller{Result: "never"}
	server := NewMCPServer(fake)

	clientTrans, serverTrans := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverErr := make(chan error, 1)
	var ss *mcp.ServerSession
	go func() {
		var err error
		ss, err = server.Connect(ctx, serverTrans, nil)
		serverErr <- err
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(
		ctx, clientTrans, nil,
	)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	if err := <-serverErr; err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      ToolName,
		Arguments: map[string]any{"task": "   "},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("expected model-visible error for empty task, got success")
	}
}

func TestHTTPHandler_PerformTask(t *testing.T) {
	fake := &FakeCaller{Result: "You have 3 emails. The urgent one is from Ana."}
	srv := httptest.NewServer(NewHTTPHandler(fake))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-http-client"}, nil).Connect(
		ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil,
	)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      ToolName,
		Arguments: map[string]any{"task": "summarize my inbox"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if tc.Text != "You have 3 emails. The urgent one is from Ana." {
		t.Fatalf("unexpected text: %q", tc.Text)
	}
}
