package mcpserver

import (
	"strings"
	"testing"
)

func TestCheckTools(t *testing.T) {
	session, _ := testSession(t)
	call(t, session, "new_map", map[string]any{"width": 64, "height": 64}, false)
	rect := map[string]any{"from": map[string]any{"x": 10, "y": 10, "z": 7}, "to": map[string]any{"x": 20, "y": 12, "z": 7}}
	call(t, session, "draw", map[string]any{"kind": "raw", "item": 100, "rect": rect}, false)

	text, _ := call(t, session, "find_path", map[string]any{"from": map[string]any{"x": 10, "y": 10, "z": 7}, "to": map[string]any{"x": 20, "y": 12, "z": 7}}, false)
	if !strings.Contains(text, `"found":true`) || !strings.Contains(text, `"steps":10`) {
		t.Fatalf("find_path: %s", text)
	}
	call(t, session, "find_path", map[string]any{"from": map[string]any{"x": 10, "y": 10, "z": 7}, "to": map[string]any{"x": 30, "y": 30, "z": 7}}, true)

	call(t, session, "save_waypoint", map[string]any{"name": "lost", "pos": map[string]any{"x": 40, "y": 40, "z": 7}}, false)
	text, _ = call(t, session, "validate_map", map[string]any{}, false)
	if !strings.Contains(text, `"waypoint":1`) || !strings.Contains(text, "lost") {
		t.Fatalf("validate_map: %s", text)
	}
}
