package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/app"
	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// testSession connects an MCP client to a server on a client with items
// 100 (a ground) and 101 (a common item).
func testSession(t *testing.T) (*mcp.ClientSession, *[]Event) {
	t.Helper()
	session := app.NewSession(nil)
	proj := project.New(client.FindByValue(1098)[0], client.Features{})
	for range 2 {
		if _, err := proj.AddThing(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	ground, _ := proj.Thing(thing.CategoryItem, 100)
	ground.Props.Ground = true
	if err := proj.UpdateThing(ground); err != nil {
		t.Fatal(err)
	}
	session.Set(proj)
	maps := app.NewMapService(session, app.NewMapStore(filepath.Join(t.TempDir(), "mapeditor.json")))
	var events []Event
	notify := func(name string, data any) {
		if name == EventMap {
			events = append(events, data.(Event))
		}
	}
	server := New(maps, app.NewProjectService(session), notify, "test")
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.newMCPServer().Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession, &events
}

// call runs a tool and returns its text; failing tools fail the test
// unless wantError.
func call(t *testing.T, session *mcp.ClientSession, name string, args any, wantError bool) (string, *mcp.CallToolResult) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			text.WriteString(textContent.Text)
		}
	}
	if result.IsError != wantError {
		t.Fatalf("%s: error %v: %s", name, result.IsError, text.String())
	}
	return text.String(), result
}

func TestTools(t *testing.T) {
	session, events := testSession(t)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) < 40 {
		t.Fatalf("%d tools", len(tools.Tools))
	}

	call(t, session, "draw", map[string]any{"kind": "raw", "item": 100, "points": []any{map[string]any{"x": 10, "y": 10, "z": 7}}}, true)
	text, _ := call(t, session, "new_map", map[string]any{"width": 256, "height": 256}, false)
	var created mapSummary
	if err := json.Unmarshal([]byte(text), &created); err != nil || created.ID == 0 {
		t.Fatalf("new_map: %s", text)
	}
	if len(*events) != 1 || (*events)[0].Kind != EventOpened {
		t.Fatalf("events %+v", *events)
	}

	text, _ = call(t, session, "draw", map[string]any{"kind": "raw", "item": 100, "rect": map[string]any{"from": map[string]any{"x": 10, "y": 10, "z": 7}, "to": map[string]any{"x": 14, "y": 12, "z": 7}}}, false)
	var result EditResult
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Count != 15 || result.MapID != created.ID {
		t.Fatalf("draw: %s", text)
	}
	if last := (*events)[len(*events)-1]; last.Kind != EventChange || len(last.Change.Chunks) != 1 {
		t.Fatalf("draw event %+v", last)
	}

	grid := map[string]any{
		"origin": map[string]any{"x": 20, "y": 20, "z": 7},
		"rows":   []string{"ab", " a"},
		"legend": []any{
			map[string]any{"symbol": "a", "brushes": []any{map[string]any{"kind": "raw", "item": 100}}},
			map[string]any{"symbol": "b", "brushes": []any{map[string]any{"kind": "raw", "item": 100}, map[string]any{"kind": "raw", "item": 101}}},
		},
	}
	text, _ = call(t, session, "paint_grid", grid, false)
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Count != 4 {
		t.Fatalf("paint_grid: %s", text)
	}
	// One undo step reverts the whole grid.
	call(t, session, "undo", map[string]any{}, false)
	text, _ = call(t, session, "describe_area", map[string]any{"x": 20, "y": 20, "z": 7, "width": 2, "height": 2}, false)
	if !strings.Contains(text, ": 0 tiles") {
		t.Fatalf("after undo: %s", text)
	}
	call(t, session, "redo", map[string]any{}, false)
	text, _ = call(t, session, "describe_area", map[string]any{"x": 20, "y": 20, "z": 7, "width": 2, "height": 2}, false)
	if !strings.Contains(text, "21,20: 100 | [0]101") || !strings.Contains(text, ": 3 tiles") {
		t.Fatalf("describe_area: %s", text)
	}

	grid["rows"] = []string{"x"}
	if text, _ = call(t, session, "paint_grid", grid, true); !strings.Contains(text, "symbols not in the legend: x") {
		t.Fatalf("unknown symbol: %s", text)
	}

	call(t, session, "add_item", map[string]any{"pos": map[string]any{"x": 10, "y": 10, "z": 7}, "item": 101, "actionId": 1234}, false)
	text, _ = call(t, session, "get_tile", map[string]any{"pos": map[string]any{"x": 10, "y": 10, "z": 7}}, false)
	if !strings.Contains(text, `"actionId":1234`) {
		t.Fatalf("get_tile: %s", text)
	}
	text, _ = call(t, session, "find_items", map[string]any{"kind": "action"}, false)
	if !strings.Contains(text, `"x":10`) {
		t.Fatalf("find_items: %s", text)
	}

	_, image := call(t, session, "render_area", map[string]any{"x": 8, "y": 8, "z": 7, "width": 10, "height": 8}, false)
	if picture, ok := image.Content[0].(*mcp.ImageContent); !ok || picture.MIMEType != "image/png" || len(picture.Data) == 0 {
		t.Fatalf("render_area: %+v", image.Content)
	}
	call(t, session, "render_minimap", map[string]any{"x": 0, "y": 0, "z": 7, "width": 64, "height": 64}, false)

	text, _ = call(t, session, "save_town", map[string]any{"name": "Thais", "temple": map[string]any{"x": 12, "y": 11, "z": 7}}, false)
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Count != 1 {
		t.Fatalf("save_town: %s", text)
	}
	text, _ = call(t, session, "save_house", map[string]any{"name": "Shop", "townId": 1}, false)
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Count == 0 {
		t.Fatalf("save_house: %s", text)
	}
	call(t, session, "draw", map[string]any{"kind": "house", "house": result.Count, "points": []any{map[string]any{"x": 11, "y": 11, "z": 7}}}, false)
	text, _ = call(t, session, "get_house_tiles", map[string]any{"id": result.Count}, false)
	if !strings.Contains(text, `"x":11`) {
		t.Fatalf("house tiles: %s", text)
	}

	call(t, session, "close_map", map[string]any{}, true)
	path := filepath.Join(t.TempDir(), "agent.otbm")
	call(t, session, "save_map", map[string]any{"path": path}, false)
	text, _ = call(t, session, "get_status", map[string]any{}, false)
	if !strings.Contains(text, "agent.otbm") || strings.Contains(text, `"unsavedChanges":true`) {
		t.Fatalf("status: %s", text)
	}
	call(t, session, "close_map", map[string]any{}, false)
	text, _ = call(t, session, "open_map", map[string]any{"path": path}, false)
	if !strings.Contains(text, `"tiles":18`) {
		t.Fatalf("open_map: %s", text)
	}
}

func TestShapesAndTerrain(t *testing.T) {
	session, _ := testSession(t)
	call(t, session, "new_map", map[string]any{"width": 256, "height": 256}, false)
	pos := func(x, y int) map[string]any { return map[string]any{"x": x, "y": y, "z": 7} }

	// A right triangle with legs of 4 tiles: 5+4+3+2+1 tiles.
	text, _ := call(t, session, "draw", map[string]any{"kind": "raw", "item": 100, "polygon": []any{pos(10, 10), pos(14, 10), pos(10, 14)}}, false)
	var result EditResult
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Count != 15 {
		t.Fatalf("polygon: %s", text)
	}
	// The sea only goes where there is no ground yet.
	text, _ = call(t, session, "draw", map[string]any{"kind": "raw", "item": 101, "onlyEmpty": true, "rect": map[string]any{"from": pos(10, 10), "to": pos(14, 14)}}, false)
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Count != 10 {
		t.Fatalf("onlyEmpty: %s", text)
	}
	text, _ = call(t, session, "terrain_grid", map[string]any{"x": 10, "y": 10, "z": 7, "width": 5, "height": 5}, false)
	if !strings.Contains(text, "10 iiiii") || !strings.Contains(text, "14 i____") {
		t.Fatalf("terrain_grid: %s", text)
	}

	text, _ = call(t, session, "draw", map[string]any{"kind": "raw", "item": 100, "blob": map[string]any{"center": pos(100, 100), "radiusX": 10, "seed": 3}}, false)
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Count < 200 || result.Count > 500 {
		t.Fatalf("blob: %s", text)
	}

	// Unknown creatures are not placed: the result says where.
	text, _ = call(t, session, "draw", map[string]any{"kind": "creature", "creature": "rat", "points": []any{pos(100, 100), pos(5, 5)}}, false)
	if err := json.Unmarshal([]byte(text), &result); err != nil || len(result.Skipped) != 1 || result.Skipped[0].X != 5 {
		t.Fatalf("skipped: %s", text)
	}

	call(t, session, "set_spawn", map[string]any{"pos": pos(101, 101), "radius": 4}, false)
	text, _ = call(t, session, "describe_area", map[string]any{"x": 95, "y": 95, "z": 7, "width": 10, "height": 10, "only": "creatures"}, false)
	if !strings.Contains(text, ": 2 tiles") || !strings.Contains(text, "spawn r4") {
		t.Fatalf("describe_area creatures: %s", text)
	}
}

func TestTerrainRules(t *testing.T) {
	session, _ := testSession(t)
	call(t, session, "new_map", map[string]any{"width": 256, "height": 256}, false)
	pos := func(x, y int) map[string]any { return map[string]any{"x": x, "y": y, "z": 7} }
	// Sample: a 6x6 ground square whose west column has item 101 (a "wall").
	call(t, session, "draw", map[string]any{"kind": "raw", "item": 100, "rect": map[string]any{"from": pos(10, 10), "to": pos(15, 15)}}, false)
	call(t, session, "draw", map[string]any{"kind": "raw", "item": 101, "rect": map[string]any{"from": pos(10, 10), "to": pos(10, 15)}}, false)
	text, _ := call(t, session, "learn_terrain", map[string]any{
		"name": "test", "classes": []any{map[string]any{"name": "land", "grounds": []int{100}}},
		"samples": []any{map[string]any{"x": 8, "y": 8, "z": 7, "width": 10, "height": 10}},
	}, false)
	if !strings.Contains(text, `"tiles":36`) {
		t.Fatalf("learn: %s", text)
	}
	text, _ = call(t, session, "paint_terrain", map[string]any{
		"rules": "test", "origin": pos(50, 50), "rows": []string{"####", "####", "####", "####"},
		"legend": []any{map[string]any{"symbol": "#", "class": "land"}},
	}, false)
	if !strings.Contains(text, `"tiles":16`) || strings.Contains(text, "unseen") {
		t.Fatalf("paint: %s", text)
	}
	// The west column has nothing west of it, like the sample's: it gets 101.
	text, _ = call(t, session, "describe_area", map[string]any{"x": 50, "y": 50, "z": 7, "width": 4, "height": 4, "only": "items"}, false)
	if !strings.Contains(text, ": 4 tiles") || !strings.Contains(text, "50,51: 100 | [0]101") {
		t.Fatalf("painted: %s", text)
	}
	text, _ = call(t, session, "list_terrain_rules", map[string]any{}, false)
	if !strings.Contains(text, `"name":"test"`) {
		t.Fatalf("list: %s", text)
	}
}
