package mcpserver

import (
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
)

// mapCount returns the number of open maps get_status reports.
func mapCount(t *testing.T, text string) int {
	t.Helper()
	var status statusResult
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("get_status: %s", text)
	}
	return len(status.Maps)
}

func TestStdioPassesCallsToEditor(t *testing.T) {
	editor := testServer(t, true, nil)
	web := httptest.NewServer(editor.Handler())
	defer web.Close()

	var mu sync.Mutex
	address := web.URL + Path
	liveURL := func() string {
		mu.Lock()
		defer mu.Unlock()
		return address
	}
	setAddress := func(value string) {
		mu.Lock()
		defer mu.Unlock()
		address = value
	}

	standalone := testServer(t, false, nil)
	server, live := standalone.newStdioServer(liveURL)
	t.Cleanup(live.close)
	session := connect(t, server)

	// The editor runs: the map is made there (here there is no client).
	call(t, session, "new_map", map[string]any{"width": 256, "height": 256}, false)
	if editor.maps.ActiveMap() == 0 || standalone.maps.ActiveMap() != 0 {
		t.Fatalf("map made in the wrong process: editor %d, standalone %d", editor.maps.ActiveMap(), standalone.maps.ActiveMap())
	}
	if text, _ := call(t, session, "get_status", struct{}{}, false); mapCount(t, text) != 1 {
		t.Fatalf("get_status of the editor: %s", text)
	}

	// The editor's server is off: the calls run here.
	setAddress("")
	if text, _ := call(t, session, "get_status", struct{}{}, false); mapCount(t, text) != 0 {
		t.Fatalf("get_status without the editor: %s", text)
	}

	// The editor is closed while connected: the calls run here.
	setAddress(web.URL + Path)
	call(t, session, "get_status", struct{}{}, false)
	web.Close()
	if text, _ := call(t, session, "get_status", struct{}{}, false); mapCount(t, text) != 0 {
		t.Fatalf("get_status after the editor closed: %s", text)
	}
	call(t, session, "new_map", map[string]any{}, true)
}

func TestStdioKeepsOwnWork(t *testing.T) {
	editor := testServer(t, true, nil)
	web := httptest.NewServer(editor.Handler())
	defer web.Close()
	editorSession := connect(t, editor.newMCPServer())
	call(t, editorSession, "new_map", map[string]any{"width": 256, "height": 256}, false)

	// A client is open here: the calls stay, though the editor runs.
	standalone := testServer(t, true, nil)
	server, live := standalone.newStdioServer(func() string { return web.URL + Path })
	t.Cleanup(live.close)
	session := connect(t, server)
	if text, _ := call(t, session, "get_status", struct{}{}, false); mapCount(t, text) != 0 {
		t.Fatalf("get_status went to the editor: %s", text)
	}
}
