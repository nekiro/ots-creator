package app

import (
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/settings"
)

func TestCompiledClientsBecomeRecent(t *testing.T) {
	s, ps, _, ss, _ := newEnv(t)
	st, _ := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	svc := NewSettingsService(st, s)
	ps.New(v1098(), client.Features{})
	if len(svc.Get().Recent) != 0 {
		t.Fatal("unsaved client must not be recent")
	}
	dir := t.TempDir()
	dat := filepath.Join(dir, "Tibia.dat")
	if _, err := ps.CompileAs(CompileRequest{DatPath: dat, SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: ps.State().Info.Features}); err != nil {
		t.Fatal(err)
	}
	r := svc.Get().Recent
	if len(r) != 1 || r[0].DatPath != dat || r[0].Version.Value != 1098 {
		t.Fatalf("recent %+v", r)
	}

	ids, err := ss.AddPixels([][]byte{make([]byte, 32*32*4), solidPx(32)})
	if err != nil || len(ids) != 1 {
		t.Fatalf("add pixels %v %v", ids, err)
	}
	res, err := ss.Optimize(project.OptimizeOptions{Duplicates: true, Unused: true, Empty: true})
	if err != nil || res.After != 0 || res.Unused != 1 {
		t.Fatalf("optimize %+v %v", res, err)
	}
}

func solidPx(size int) []byte {
	px := make([]byte, size*size*4)
	for i := 3; i < len(px); i += 4 {
		px[i] = 255
	}
	return px
}

func TestMCPCommandNamesTheAppImage(t *testing.T) {
	mount := filepath.Join(t.TempDir(), ".mount_otscreator")
	image := filepath.Join(t.TempDir(), "otscreator-linux-amd64.AppImage")
	t.Setenv("APPDIR", mount)
	t.Setenv("APPIMAGE", image)
	if got := appImageOr(filepath.Join(mount, "usr", "bin", "otscreator")); got != image {
		t.Fatalf("inside the mount: %q, want %q", got, image)
	}
	// A child started from the AppImage inherits APPIMAGE but runs elsewhere.
	other := filepath.Join(t.TempDir(), "otscreator")
	if got := appImageOr(other); got != other {
		t.Fatalf("outside the mount: %q, want %q", got, other)
	}
}
