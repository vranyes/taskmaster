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

func dialTestServer(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	clientTrans, serverTrans := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
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
	t.Cleanup(func() { cs.Close() })
	if err := <-serverErr; err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { ss.Close() })
	return cs
}

func testAuthHandler() (*AuthHandler, string) {
	secret := []byte("test-edge-secret-32-bytes-long!!")
	h := &AuthHandler{
		EdgeSecret: secret,
		Directory:  &FakeDirectory{Sub: "kanidm-sub-1", Key: "user-1-key"},
		NewCaller: func(apiKey string) DownstreamCaller {
			return &FakeCaller{Result: "You have 3 emails. The urgent one is from Ana."}
		},
	}
	tok, err := MintEdgeToken(secret, EdgeClaims{
		UserSub: "kanidm-sub-1", Phone: "+15550109999",
		CallID: "call-mcp-1", JTI: "jti-mcp-1", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		panic(err)
	}
	return h, tok
}

func TestAuthMCPServer_PerformTaskWithToken(t *testing.T) {
	h, tok := testAuthHandler()
	cs := dialTestServer(t, NewAuthMCPServer(h))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      ToolName,
		Arguments: map[string]any{"task": "summarize my inbox", "edge_token": tok},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatal("expected success, got error result")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if tc.Text != "You have 3 emails. The urgent one is from Ana." {
		t.Fatalf("unexpected text: %q", tc.Text)
	}
}

func TestAuthMCPServer_RejectsMissingToken(t *testing.T) {
	h, _ := testAuthHandler()
	cs := dialTestServer(t, NewAuthMCPServer(h))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      ToolName,
		Arguments: map[string]any{"task": "summarize my inbox"},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("expected model-visible error for missing token, got success")
	}
}
