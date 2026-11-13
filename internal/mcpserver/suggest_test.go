package mcpserver

import (
	"strings"
	"testing"
)

func TestSuggestTools(t *testing.T) {
	session, _ := testSession(t)
	call(t, session, "new_map", map[string]any{"width": 64, "height": 64}, false)
	rect := map[string]any{"from": map[string]any{"x": 10, "y": 10, "z": 7}, "to": map[string]any{"x": 20, "y": 20, "z": 7}}
	call(t, session, "draw", map[string]any{"kind": "raw", "item": 100, "rect": rect}, false)
	var points []any
	for x := 10; x <= 20; x += 2 {
		points = append(points, map[string]any{"x": x, "y": 12, "z": 7})
	}
	call(t, session, "draw", map[string]any{"kind": "raw", "item": 101, "points": points}, false)

	text, _ := call(t, session, "suggest_items", map[string]any{"pos": map[string]any{"x": 15, "y": 15, "z": 7}}, false)
	if !strings.Contains(text, `"item":101`) {
		t.Fatalf("suggest_items: %s", text)
	}
	call(t, session, "suggest_items", map[string]any{"pos": map[string]any{"x": 15, "y": 15, "z": 7}, "kind": "raw"}, true)

	text, _ = call(t, session, "variants", map[string]any{"item": 101}, false)
	if !strings.HasPrefix(text, `{"variants":[{"item":101`) {
		t.Fatalf("variants: %s", text)
	}
	call(t, session, "variants", map[string]any{"item": 0}, true)

	call(t, session, "swap_item", map[string]any{"pos": map[string]any{"x": 10, "y": 12, "z": 7}, "index": 0, "item": 100}, false)
	text, _ = call(t, session, "get_tile", map[string]any{"pos": map[string]any{"x": 10, "y": 12, "z": 7}}, false)
	if strings.Contains(text, `"id":101`) {
		t.Fatalf("swap_item: %s", text)
	}
	call(t, session, "swap_item", map[string]any{"pos": map[string]any{"x": 11, "y": 12, "z": 7}, "index": 0, "item": 100}, true)

	text, _ = call(t, session, "complete_object", map[string]any{"pos": map[string]any{"x": 12, "y": 12, "z": 7}, "item": 101, "place": true}, false)
	if !strings.Contains(text, `"missing":[]`) {
		t.Fatalf("complete_object: %s", text)
	}
}
