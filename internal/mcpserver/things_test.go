package mcpserver

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nekiro/ots-creator/internal/core/imaging"
)

// writeSheet writes a 32x64 PNG: one sprite with two animation frames.
func writeSheet(t *testing.T) string {
	t.Helper()
	sheet := image.NewNRGBA(image.Rect(0, 0, 32, 64))
	for y := range 64 {
		for x := 8; x < 24; x++ {
			sheet.SetNRGBA(x, y, color.NRGBA{uint8(y * 4), 120, 60, 255})
		}
	}
	data, err := imaging.EncodePNG(sheet)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sheet.png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestThingTools(t *testing.T) {
	session, _ := testSession(t)
	sheet := writeSheet(t)

	text, _ := call(t, session, "free_ids", map[string]any{"from": 100, "count": 2}, false)
	if !strings.Contains(text, "102") {
		t.Fatalf("free_ids: %s", text)
	}
	call(t, session, "create_thing", map[string]any{"id": 100, "sheet": sheet}, true)
	call(t, session, "create_thing", map[string]any{"id": 102, "sheet": sheet, "category": "nothing"}, true)

	text, _ = call(t, session, "create_thing", map[string]any{"id": 102, "sheet": sheet, "template": 101, "name": "training wand", "durationMs": 300, "props": map[string]any{"pickupable": true}}, false)
	var created thingView
	if err := json.Unmarshal([]byte(text), &created); err != nil {
		t.Fatalf("create_thing: %v: %s", err, text)
	}
	if created.Name != "training wand" || len(created.FrameGroups) != 1 {
		t.Fatalf("create_thing: %s", text)
	}
	group := created.FrameGroups[0]
	if group.Frames != 2 || group.PatternX != 1 || len(group.Durations) != 2 || group.Durations[0].Min != 300 || created.Props["pickupable"] != true {
		t.Fatalf("created layout: %s", text)
	}

	text, _ = call(t, session, "find_things", map[string]any{"flag": "pickupable"}, false)
	if !strings.Contains(text, `"id":102`) {
		t.Fatalf("find_things: %s", text)
	}

	_, result := call(t, session, "render_things", map[string]any{"ids": []int{101, 102}, "scale": 2}, false)
	picture, ok := result.Content[0].(*mcp.ImageContent)
	if !ok || picture.MIMEType != "image/png" || len(picture.Data) == 0 {
		t.Fatalf("render_things: %+v", result.Content)
	}
	call(t, session, "render_things", map[string]any{"ids": []int{}}, true)

	call(t, session, "update_thing", map[string]any{"ids": []int{102}, "name": "renamed", "durationMs": 150, "props": map[string]any{"stackable": true}}, false)
	text, _ = call(t, session, "get_thing", map[string]any{"id": 102}, false)
	if !strings.Contains(text, `"name":"renamed"`) || !strings.Contains(text, `"stackable":true`) || !strings.Contains(text, `"min":150`) {
		t.Fatalf("update_thing: %s", text)
	}
	call(t, session, "update_thing", map[string]any{"ids": []int{102}, "props": map[string]any{"noSuchFlag": true}}, true)

	call(t, session, "client_undo", map[string]any{}, false)
	text, _ = call(t, session, "get_thing", map[string]any{"id": 102}, false)
	if strings.Contains(text, `"name":"renamed"`) {
		t.Fatalf("client_undo: %s", text)
	}
	call(t, session, "client_redo", map[string]any{}, false)

	call(t, session, "import_sheet", map[string]any{"id": 101, "sheet": sheet}, false)

	exported := filepath.Join(t.TempDir(), "exported.png")
	text, _ = call(t, session, "export_sheet", map[string]any{"id": 102, "path": exported}, false)
	if !strings.Contains(text, "32x64 px") || !strings.Contains(text, `"frames":2`) {
		t.Fatalf("export_sheet: %s", text)
	}
	img, err := imaging.Load(exported)
	if err != nil || img.Rect.Dx() != 32 || img.Rect.Dy() != 64 || img.NRGBAAt(0, 0).A != 0 || img.NRGBAAt(10, 4).A != 255 {
		t.Fatalf("exported sheet: %v %v", err, img.Rect)
	}
	call(t, session, "import_sheet", map[string]any{"id": 101, "sheet": exported, "group": 0}, false)
	call(t, session, "import_sheet", map[string]any{"id": 101, "sheet": exported, "group": 3}, true)
	call(t, session, "export_sheet", map[string]any{"id": 102, "path": exported, "background": "green"}, true)
	// Without an items.otb the result tells why there is no server id.
	text, _ = call(t, session, "get_thing", map[string]any{"id": 102}, false)
	if !strings.Contains(text, "serverIdNote") {
		t.Fatalf("server id note: %s", text)
	}

	text, _ = call(t, session, "duplicate_things", map[string]any{"ids": []int{102}, "at": 103}, false)
	if !strings.Contains(text, `"ids":[103]`) {
		t.Fatalf("duplicate_things at: %s", text)
	}
	call(t, session, "duplicate_things", map[string]any{"ids": []int{102}, "at": 103}, true)
	call(t, session, "duplicate_things", map[string]any{"ids": []int{102}, "at": 103, "replace": true}, false)
	text, _ = call(t, session, "duplicate_things", map[string]any{"ids": []int{102}}, false)
	if !strings.Contains(text, `"ids":[104]`) {
		t.Fatalf("duplicate_things: %s", text)
	}
	text, _ = call(t, session, "get_thing", map[string]any{"id": 104}, false)
	if !strings.Contains(text, `"stackable":true`) {
		t.Fatalf("copy: %s", text)
	}

	text, _ = call(t, session, "update_thing", map[string]any{"where": map[string]any{"flag": "stackable"}, "props": map[string]any{"hangable": true}}, false)
	if !strings.Contains(text, `"changed":3`) {
		t.Fatalf("update_thing where: %s", text)
	}
	call(t, session, "update_thing", map[string]any{"ids": []int{102}, "where": map[string]any{"flag": "stackable"}}, true)

	text, _ = call(t, session, "move_things", map[string]any{"ids": []int{104}, "at": 101}, false)
	if !strings.Contains(text, `"ids":[101]`) {
		t.Fatalf("move_things: %s", text)
	}
	call(t, session, "remove_things", map[string]any{"ids": []int{101, 103}}, false)
	text, _ = call(t, session, "find_things", map[string]any{"flag": "stackable"}, false)
	if !strings.Contains(text, `"total":1`) {
		t.Fatalf("after remove_things: %s", text)
	}

	// The test client was never written to disk, so it has no files to save to.
	call(t, session, "save_client", map[string]any{}, true)
}
