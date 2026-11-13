package mcpserver

import (
	"strings"
	"testing"
)

func TestMaterialTools(t *testing.T) {
	session, _ := testSession(t)
	call(t, session, "new_map", map[string]any{"width": 64, "height": 64}, false)

	call(t, session, "save_border", map[string]any{"border": map[string]any{"edges": map[string]any{"up": 101}}}, true)
	text, _ := call(t, session, "save_border", map[string]any{"border": map[string]any{"edges": map[string]any{"n": 101, "s": 101, "e": 101, "w": 101, "cnw": 999}}}, false)
	if !strings.Contains(text, `"count":`) || !strings.Contains(text, "999") {
		t.Fatalf("save_border: %s", text)
	}
	text, _ = call(t, session, "get_borders", map[string]any{"item": 999}, false)
	if !strings.Contains(text, `"cnw":999`) {
		t.Fatalf("get_borders: %s", text)
	}
	call(t, session, "get_borders", map[string]any{}, true)

	brush := map[string]any{"name": "test sand", "type": "ground", "items": []any{map[string]any{"id": 100, "chance": 10}}, "borders": []any{map[string]any{"align": "outer", "border": 4242}}}
	call(t, session, "save_brush", map[string]any{"brush": brush}, true)
	brush["borders"] = []any{map[string]any{"align": "sideways", "border": 1}}
	call(t, session, "save_brush", map[string]any{"brush": brush}, true)
	brush["borders"] = nil
	brush["type"] = "lake"
	call(t, session, "save_brush", map[string]any{"brush": brush}, true)
	brush["type"] = "ground"
	text, _ = call(t, session, "save_brush", map[string]any{"brush": brush, "tilesets": []string{"agent"}}, false)
	if !strings.Contains(text, `"name":"test sand"`) || strings.Contains(text, "warnings") {
		t.Fatalf("save_brush: %s", text)
	}
	call(t, session, "draw", map[string]any{"kind": "ground", "name": "test sand", "points": []any{map[string]any{"x": 5, "y": 5, "z": 7}}}, false)
	text, _ = call(t, session, "get_tile", map[string]any{"pos": map[string]any{"x": 5, "y": 5, "z": 7}}, false)
	if !strings.Contains(text, "100") {
		t.Fatalf("drawn with the new brush: %s", text)
	}
	text, _ = call(t, session, "list_brushes", map[string]any{"tileset": "agent"}, false)
	if !strings.Contains(text, "test sand") {
		t.Fatalf("brush tileset: %s", text)
	}

	brush["name"] = "renamed sand"
	call(t, session, "save_brush", map[string]any{"brush": brush, "oldName": "test sand"}, false)
	call(t, session, "get_brush", map[string]any{"name": "test sand"}, true)

	call(t, session, "add_to_tileset", map[string]any{"name": "agent", "items": []int{999}}, true)
	text, _ = call(t, session, "add_to_tileset", map[string]any{"name": "agent", "items": []int{101}}, false)
	if !strings.Contains(text, `"count":1`) {
		t.Fatalf("add_to_tileset: %s", text)
	}

	call(t, session, "save_creature", map[string]any{"creature": map[string]any{"name": " "}}, true)
	call(t, session, "save_creature", map[string]any{"creature": map[string]any{"name": "Sand Crab", "outfit": map[string]any{"lookType": 128}}}, false)
	text, _ = call(t, session, "list_creatures", map[string]any{"query": "sand crab"}, false)
	if !strings.Contains(text, "Sand Crab") {
		t.Fatalf("save_creature: %s", text)
	}

	call(t, session, "remove_brush", map[string]any{"name": "renamed sand"}, false)
	call(t, session, "remove_brush", map[string]any{"name": "renamed sand"}, true)
}
