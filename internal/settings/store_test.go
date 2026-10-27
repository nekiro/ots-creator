package settings

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	defaults := Defaults()
	if !defaults.CheckUpdates || defaults.ReopenLast || defaults.ObjectFormat != "otobj" || defaults.ExportFormat != "png" {
		t.Fatalf("defaults %+v", defaults)
	}
	if defaults.Recent == nil || defaults.MarketShared == nil || defaults.LastDirs == nil || defaults.ItemsOTB == nil {
		t.Fatal("defaults must have empty, non-nil collections")
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Skip("no user config dir:", err)
	}
	if filepath.Base(path) != "settings.json" || filepath.Base(filepath.Dir(path)) != "OTS Creator" {
		t.Fatalf("path %q", path)
	}
}

func TestNormalize(t *testing.T) {
	settings := Settings{SheetBackground: "pink", ExportFormat: "bmp", ObjectFormat: "obd", ListColumns: 1}
	for range MaxRecent + 3 {
		settings.Recent = append(settings.Recent, Recent{DatPath: "x"})
	}
	settings.normalize()
	if settings.SheetBackground != BackgroundMagenta || settings.ExportFormat != "bmp" || settings.ObjectFormat != "obd" {
		t.Fatalf("formats %+v", settings)
	}
	if settings.ListColumns != MinListColumns || len(settings.Recent) != MaxRecent {
		t.Fatalf("columns %d, recent %d", settings.ListColumns, len(settings.Recent))
	}
	if settings.MarketShared == nil || settings.LastDirs == nil || settings.ItemsOTB == nil {
		t.Fatal("normalize must create the maps")
	}

	transparent := Settings{SheetBackground: BackgroundTransparent, ObjectFormat: "bogus"}
	transparent.normalize()
	if transparent.SheetBackground != BackgroundTransparent || transparent.ObjectFormat != "otobj" || transparent.ListColumns != DefaultListColumns {
		t.Fatalf("normalized %+v", transparent)
	}
}

func TestMarketSharedAndAuthor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, _ := Open(path)
	if err := store.SetMarketShared("entry-1", "token-1"); err != nil {
		t.Fatal(err)
	}
	store.SetMarketShared("entry-2", "token-2")
	store.SetMarketShared("entry-2", "") // forgets it
	if err := store.SetMarketAuthor("nick"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMarketAuthor("nick"); err != nil { // unchanged
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Get()
	if got.MarketAuthor != "nick" || len(got.MarketShared) != 1 || got.MarketShared["entry-1"] != "token-1" {
		t.Fatalf("market %q %v", got.MarketAuthor, got.MarketShared)
	}
	got.MarketShared["entry-3"] = "token-3"
	if len(reopened.Get().MarketShared) != 1 {
		t.Fatal("Get must not share its maps with the store")
	}
}

func TestItemsOTB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, _ := Open(path)
	clientPath := filepath.Join("clients", "1098", "Tibia.dat")
	if err := store.SetItemsOTB("", "ignored.otb"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetItemsOTB(clientPath, "items.otb"); err != nil {
		t.Fatal(err)
	}
	reopened, _ := Open(path)
	if got := reopened.ItemsOTB(clientPath); got != "items.otb" {
		t.Fatalf("items.otb = %q", got)
	}
	if got := reopened.ItemsOTB(strings.ToUpper(clientPath)); got != "items.otb" && got != "" {
		t.Fatalf("other case = %q", got)
	}
	if err := reopened.SetItemsOTB(clientPath, ""); err != nil {
		t.Fatal(err)
	}
	if got := reopened.ItemsOTB(clientPath); got != "" {
		t.Fatalf("after forget = %q", got)
	}
}

func TestInMemoryStore(t *testing.T) {
	// A store without a path works but saves nothing.
	store := &Store{settings: Defaults()}
	if err := store.SetLastDir("key", "dir"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLastDir("", "dir"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddRecent(Recent{}); err != nil {
		t.Fatal(err)
	}
	if store.LastDir("key") != "dir" || len(store.Get().Recent) != 0 {
		t.Fatalf("store %+v", store.Get())
	}
}
