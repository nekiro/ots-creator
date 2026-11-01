package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mcpserver"
	"github.com/nekiro/ots-creator/internal/settings"
)

// mcpCommand is the argument that runs the stdio MCP server instead of the
// window ("otscreator mcp"); agents start the app with it.
const mcpCommand = app.MCPCommand

// runMCP serves MCP over stdin and stdout. The calls go to the running
// editor when its MCP server is on, otherwise this process edits the maps
// without a window. stdout carries the protocol, so logs go to stderr.
func runMCP() int {
	log.SetOutput(os.Stderr)
	app.Generator = "OTS Creator " + version
	session := app.NewSession(nil)
	maps := app.NewMapService(session, app.OpenMapStore())
	agents := mcpserver.New(maps, app.NewProjectService(session), nil, version)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := agents.RunStdio(ctx, liveMCPURL); err != nil && ctx.Err() == nil {
		log.Printf("MCP server stopped: %v", err)
		return 1
	}
	return 0
}

// liveMCPURL returns the address of the editor's MCP server as the settings
// have it ("" when it is off). The settings are read on every call, so a
// change in the editor applies at once.
func liveMCPURL() string {
	path, err := settings.DefaultPath()
	if err != nil {
		return ""
	}
	store, _ := settings.Open(path)
	current := store.Get()
	if !current.MCPServer {
		return ""
	}
	return "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(current.MCPPort)) + mcpserver.Path
}
