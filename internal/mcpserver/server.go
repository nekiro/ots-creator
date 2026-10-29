// Package mcpserver lets AI agents edit maps through the Model Context
// Protocol: a local server (streamable HTTP on 127.0.0.1) whose tools call
// the map editor of the running app. Edits go through the same services as
// the editor's own, so they show in its window and can be undone there.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
	"github.com/nekiro/ots-creator/internal/settings"
)

// Path is the URL path of the MCP endpoint.
const Path = "/mcp"

// EventMap is emitted with an Event when an agent changed the maps, so the
// editor window shows it.
const EventMap = "mcp:map"

// Kinds of Event.
const (
	// EventChange: Change of map ID is to be applied.
	EventChange = "change"
	// EventOpened: map ID was created or opened; the editor shows it.
	EventOpened = "opened"
	// EventClosed: map ID was closed.
	EventClosed = "closed"
	// EventData: the item data or brushes were reloaded.
	EventData = "data"
	// EventShow: the editor centers its view on Pos of map ID.
	EventShow = "show"
)

// Event tells the editor window what an agent did.
type Event struct {
	Kind   string          `json:"kind"`
	ID     int             `json:"id"`
	Change *app.MapChange  `json:"change,omitempty"`
	Pos    *otmap.Position `json:"pos,omitempty"`
}

// shutdownTimeout bounds waiting for running requests when the server stops.
const shutdownTimeout = 2 * time.Second

// Server is the MCP server of the app. It is started and stopped by the
// settings (Apply).
type Server struct {
	maps     *app.MapService
	projects *app.ProjectService
	notify   app.Notifier
	version  string

	mu       sync.Mutex
	listener *http.Server
	address  string
	// stroke numbers the undo steps of batches (see nextStroke).
	stroke int64
	// terrain keeps the rules of learn_terrain.
	terrain ruleStore
}

// New returns a stopped server. notify may be nil.
func New(maps *app.MapService, projects *app.ProjectService, notify app.Notifier, version string) *Server {
	if notify == nil {
		notify = func(string, any) {}
	}
	return &Server{maps: maps, projects: projects, notify: notify, version: version, terrain: ruleStore{dir: maps.SettingsDir()}}
}

// Handler returns the HTTP handler of the MCP endpoint.
func (s *Server) Handler() http.Handler {
	server := s.newMCPServer()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	mux := http.NewServeMux()
	mux.Handle(Path, http.NewCrossOriginProtection().Handler(handler))
	return mux
}

// Apply starts the server on port when enabled (restarting it when the
// port changed) and stops it otherwise.
func (s *Server) Apply(enabled bool, port int) error {
	if port <= 0 || port > 0xFFFF {
		port = settings.DefaultMCPPort
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	s.mu.Lock()
	defer s.mu.Unlock()
	if enabled && s.listener != nil && s.address == address {
		return nil
	}
	if s.listener != nil {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		_ = s.listener.Shutdown(ctx)
		cancel()
		s.listener, s.address = nil, ""
	}
	if !enabled {
		return nil
	}
	socket, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("MCP server: %w", err)
	}
	listener := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	s.listener, s.address = listener, address
	go func() {
		if err := listener.Serve(socket); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("MCP server: %v", err)
		}
	}()
	return nil
}

// URL returns the address agents connect to ("" when stopped).
func (s *Server) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return "http://" + s.address + Path
}

// nextStroke returns a stroke number of its own: the edits of one batch
// share it and become one undo step. Agents count down from -1, the
// editor's own strokes are positive.
func (s *Server) nextStroke() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stroke--
	return s.stroke
}

func (s *Server) newMCPServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "ots-creator", Title: "OTS Creator map editor", Version: s.version}, &mcp.ServerOptions{
		Instructions: instructions,
		Capabilities: &mcp.ServerCapabilities{},
	})
	s.addSessionTools(server)
	s.addCatalogTools(server)
	s.addViewTools(server)
	s.addTerrainTools(server)
	s.addTerrainRuleTools(server)
	s.addDrawTools(server)
	s.addItemTools(server)
	s.addWorldTools(server)
	s.addSelectionTools(server)
	return server
}

const instructions = `OTS Creator edits Open Tibia maps (OTBM) with the sprites of an open client. The edits show live in the editor window and can be undone there.

Coordinates: x grows east, y grows south, z is the floor: 7 is the ground level, 0-6 are above it, 8-15 underground. Every tool takes mapId; 0 or none means the map shown in the editor.

Workflow:
1. get_status: is a client open, which maps are open. Without a client use open_client; without a map use new_map or open_map.
2. Find what to draw with: list_brushes / get_tileset for brushes (grounds, walls, doodads...), search_items for single items, list_creatures for monsters and NPCs.
3. Draw: draw (one brush), draw_batch (many brushes, one undo step) or paint_grid (a text plan of a whole area). Ground brushes add borders between grounds and walls connect by themselves.
4. Look at the result with render_area (an image of the area), terrain_grid (exact tile positions as text) or describe_area / get_tile, and fix what is wrong (undo, draw with erase, remove_item).
5. save_map writes the map; nothing is written to disk before.

When the brushes do not fit the client, paint with terrain rules instead: count_grounds and learn_terrain on an area of the map in the wanted style, then paint_terrain plans of terrain classes; the borders, walls and decoration come from the sample.

Prefer brushes over raw items: they choose the right border, wall and carpet pieces. Paint grounds first, then walls, then doors, then decoration and items.`

// errNoMap is returned when a tool needs a map and none is open.
var errNoMap = errors.New("no map is open: use new_map or open_map")

// mapID resolves a mapId argument: 0 is the map shown in the editor.
func (s *Server) mapID(id int) (int, error) {
	if id != 0 {
		return id, nil
	}
	if active := s.maps.ActiveMap(); active != 0 {
		return active, nil
	}
	return 0, errNoMap
}

// changed shows an edit in the editor window (failed edits return no
// change).
func (s *Server) changed(id int, change app.MapChange) {
	if change.Info.ID == 0 {
		return
	}
	s.notify(EventMap, Event{Kind: EventChange, ID: id, Change: &change})
}

// EditResult is what edit tools return.
type EditResult struct {
	MapID int `json:"mapId"`
	// Count is the number of tiles or items the edit changed (or the id of
	// what it created).
	Count     int      `json:"count"`
	Selection int      `json:"selection"`
	Changed   bool     `json:"unsavedChanges"`
	Undo      string   `json:"undo,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	// Skipped are points where a creature or item could not be placed.
	Skipped []Pos `json:"skipped,omitempty"`
}

func editResult(id int, change app.MapChange) EditResult {
	return EditResult{MapID: id, Count: change.Count, Selection: change.Info.Selection, Changed: change.Info.Changed, Undo: change.Info.UndoLabel, Warnings: change.Warnings}
}

// edit runs an edit of a map, shows it in the editor and returns its
// result.
func (s *Server) edit(mapID int, run func(id int) (app.MapChange, error)) (any, error) {
	id, err := s.mapID(mapID)
	if err != nil {
		return nil, err
	}
	change, err := run(id)
	s.changed(id, change)
	if err != nil {
		return nil, err
	}
	return editResult(id, change), nil
}

// mergeChanges joins the changes of several edits of one map: the chunks
// of all, the state after the last.
func mergeChanges(changes []app.MapChange) app.MapChange {
	if len(changes) == 0 {
		return app.MapChange{}
	}
	merged := changes[len(changes)-1]
	seen := map[uint32]bool{}
	merged.Chunks, merged.Count, merged.Warnings = nil, 0, nil
	for _, change := range changes {
		for _, key := range change.Chunks {
			if !seen[key] {
				seen[key] = true
				merged.Chunks = append(merged.Chunks, key)
			}
		}
		merged.All = merged.All || change.All
		merged.Meta = merged.Meta || change.Meta
		merged.Count += change.Count
		merged.Warnings = append(merged.Warnings, change.Warnings...)
	}
	return merged
}

// Pos is a map position of tool arguments.
type Pos struct {
	X int `json:"x" jsonschema:"x, grows east"`
	Y int `json:"y" jsonschema:"y, grows south"`
	Z int `json:"z" jsonschema:"floor: 7 ground level, 0-6 above, 8-15 underground"`
}

func (pos Pos) position() (otmap.Position, error) {
	if pos.X < 0 || pos.Y < 0 || pos.X > 0xFFFF || pos.Y > 0xFFFF || pos.Z < 0 || pos.Z > otmap.MaxFloor {
		return otmap.Position{}, fmt.Errorf("position %d,%d,%d is out of range (x and y 0-65535, z 0-15)", pos.X, pos.Y, pos.Z)
	}
	return otmap.Position{X: uint16(pos.X), Y: uint16(pos.Y), Z: uint8(pos.Z)}, nil
}

func positions(list []Pos) ([]otmap.Position, error) {
	out := make([]otmap.Position, 0, len(list))
	for _, pos := range list {
		position, err := pos.position()
		if err != nil {
			return nil, err
		}
		out = append(out, position)
	}
	return out, nil
}

// addTool registers a tool whose handler returns a JSON result (no output
// schema: results stay out of the tool list).
func addTool[In any](server *mcp.Server, name, description string, handler func(ctx context.Context, input In) (any, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
		result, err := handler(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		if content, ok := result.(*mcp.CallToolResult); ok {
			return content, nil, nil
		}
		return nil, result, nil
	})
}
