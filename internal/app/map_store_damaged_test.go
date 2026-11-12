package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDamagedMapStoreIsKeptAndReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mapeditor.json")
	if err := os.WriteFile(path, []byte(`{"profiles": {`), 0o644); err != nil {
		t.Fatal(err)
	}
	session := NewSession(nil)
	NewMapService(session, NewMapStore(path))
	if warnings := session.takeWarnings(); len(warnings) != 1 {
		t.Fatalf("warnings: %v", warnings)
	}
	if len(session.takeWarnings()) != 0 {
		t.Fatal("a warning was reported twice")
	}
	kept, err := os.ReadFile(path + ".bad")
	if err != nil || string(kept) != `{"profiles": {` {
		t.Fatalf("the damaged file was not kept: %q, %v", kept, err)
	}
}
