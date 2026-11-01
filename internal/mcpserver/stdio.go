package mcpserver

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// standaloneNote is added to the instructions of the stdio server.
const standaloneNote = `

When the editor is not running (or its MCP server is off), the tools work without a window: the client and maps are opened in this process, nothing shows in the editor and save_map is the only way to keep the work.`

// RunStdio serves MCP over stdin and stdout, for agents that start the app
// as a command ("otscreator mcp"). While the editor runs with its MCP server
// on (liveURL returns its address), tool calls are passed to it, so the
// edits show live there; otherwise this process edits the maps itself,
// without a window. Once it holds a client or map of its own it keeps the
// calls, so the work is not split between two editors.
func (s *Server) RunStdio(ctx context.Context, liveURL func() string) error {
	server, live := s.newStdioServer(liveURL)
	defer live.close()
	return server.Run(ctx, &mcp.StdioTransport{})
}

// newStdioServer returns the server of RunStdio and its link to the editor.
func (s *Server) newStdioServer(liveURL func() string) (*mcp.Server, *liveEditor) {
	s.standalone = true
	server := s.newMCPServer()
	live := &liveEditor{url: liveURL, version: s.version}
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			call, ok := request.(*mcp.CallToolRequest)
			if !ok || s.holdsWork() {
				return next(ctx, method, request)
			}
			if result, handled, err := live.call(ctx, call.Params); handled {
				return result, err
			}
			return next(ctx, method, request)
		}
	})
	return server, live
}

// holdsWork reports whether this process has a client or map open.
func (s *Server) holdsWork() bool {
	return s.projects.State().Open || s.maps.ActiveMap() != 0
}

// liveEditor passes tool calls to the MCP server of the running editor.
type liveEditor struct {
	url     func() string
	version string

	mu      sync.Mutex
	session *mcp.ClientSession
	address string
}

// call runs a tool in the editor. handled is false when the editor cannot
// be reached, so the call is to run here.
func (l *liveEditor) call(ctx context.Context, params *mcp.CallToolParamsRaw) (result *mcp.CallToolResult, handled bool, err error) {
	session := l.connect(ctx)
	if session == nil {
		return nil, false, nil
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Meta: params.Meta, Name: params.Name, Arguments: params.Arguments})
	if err == nil || ctx.Err() != nil {
		return result, true, err
	}
	// When the editor is gone (the connection failed) the call never
	// reached it and runs here; otherwise the error is reported.
	l.drop(session)
	if l.connect(ctx) == nil {
		return nil, false, nil
	}
	return nil, true, err
}

// connect returns a session with the editor (nil when it does not run).
func (l *liveEditor) connect(ctx context.Context) *mcp.ClientSession {
	address := l.url()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.session != nil && l.address == address {
		return l.session
	}
	if l.session != nil {
		l.session.Close()
		l.session = nil
	}
	if address == "" {
		return nil
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "ots-creator-stdio", Version: l.version}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: address, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return nil
	}
	l.session, l.address = session, address
	return session
}

// drop forgets a session whose connection failed.
func (l *liveEditor) drop(session *mcp.ClientSession) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.session == session {
		l.session.Close()
		l.session = nil
	}
}

func (l *liveEditor) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.session != nil {
		l.session.Close()
		l.session = nil
	}
}
