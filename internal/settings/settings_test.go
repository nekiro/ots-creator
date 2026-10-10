package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsWhenMissing(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "none", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s := st.Get(); !s.CheckUpdates || s.SheetBackground != BackgroundMagenta || s.ExportFormat != "png" || s.ListColumns != DefaultListColumns || s.Recent == nil {
		t.Fatalf("defaults %+v", s)
	}
}

func TestRoundTripAndRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "settings.json")
	st, _ := Open(path)
	s := st.Get()
	s.CheckUpdates = false
	s.ExportFormat = "bogus"
	s.ListColumns = 99
	s.Recent = nil // ignored by Update
	if err := st.Update(s); err != nil {
		t.Fatal(err)
	}
	for i := range MaxRecent + 2 {
		st.AddRecent(Recent{DatPath: filepath.Join("c", string(rune('a'+i)), "Tibia.dat")})
	}
	st.AddRecent(Recent{DatPath: filepath.Join("c", "c", "Tibia.dat")})

	re, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := re.Get()
	if got.CheckUpdates || got.ExportFormat != "png" || got.ListColumns != MaxListColumns {
		t.Fatalf("prefs %+v", got)
	}
	if len(got.Recent) != MaxRecent || got.Recent[0].DatPath != filepath.Join("c", "c", "Tibia.dat") {
		t.Fatalf("recent %+v", got.Recent)
	}
	for _, r := range got.Recent[1:] {
		if r.DatPath == got.Recent[0].DatPath {
			t.Fatal("duplicate recent entry")
		}
	}

	re.RemoveRecent(got.Recent[0].DatPath)
	if n := len(re.Get().Recent); n != MaxRecent-1 {
		t.Fatalf("after remove %d", n)
	}
	re.RemoveRecent("")
	if n := len(re.Get().Recent); n != 0 {
		t.Fatalf("after clear %d", n)
	}
}

func TestCorruptFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte("{nope"), 0o644)
	st, err := Open(path)
	if err == nil {
		t.Fatal("corrupt file must report an error")
	}
	if !st.Get().CheckUpdates {
		t.Fatal("store must fall back to defaults")
	}
}

func TestLastDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, _ := Open(path)
	if err := st.SetLastDir("openClient", filepath.Join("c", "client")); err != nil {
		t.Fatal(err)
	}
	s := st.Get()
	s.LastDirs = nil // ignored by Update
	if err := st.Update(s); err != nil {
		t.Fatal(err)
	}
	re, _ := Open(path)
	if got := re.LastDir("openClient"); got != filepath.Join("c", "client") {
		t.Fatalf("openClient = %q", got)
	}
	if got := re.LastDir("exportSprites"); got != "" {
		t.Fatalf("exportSprites = %q", got)
	}
}
