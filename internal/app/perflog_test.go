package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePerfLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	for _, line := range []string{" draw 3ms \n", "draw 4ms"} {
		recorder := httptest.NewRecorder()
		writePerfLog(recorder, httptest.NewRequest(http.MethodPost, "/res/perflog", strings.NewReader(line)), dir)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status %d", recorder.Code)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, perfLogFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], " draw 3ms") || !strings.HasSuffix(lines[1], " draw 4ms") {
		t.Fatalf("log = %q", data)
	}
}
