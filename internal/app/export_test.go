package app

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestParallelCancel(t *testing.T) {
	s := NewSession(nil)
	var ran atomic.Int64
	err := s.parallel("test", 100000, func(i int) error {
		if ran.Add(1) == 1 {
			s.Cancel()
		}
		return nil
	})
	if !errors.Is(err, ErrCanceled) || ran.Load() == 100000 {
		t.Fatalf("err %v after %d", err, ran.Load())
	}
	// The next operation starts uncanceled.
	if err := s.parallel("test", 10, func(int) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestExportAll(t *testing.T) {
	dir := t.TempDir()
	_, ps, ts, ss, rec := newEnv(t)
	ps.New(v1098(), client.Features{})
	img := filepath.Join(dir, "red.png")
	writePNG(t, img, 32, 32, [4]byte{200, 10, 10, 255})
	if _, err := ss.ImportImages([]string{img}); err != nil {
		t.Fatal(err)
	}
	ts.Add(thing.CategoryItem)
	it, _ := ts.Get(thing.CategoryItem, 101)
	it.FrameGroups[0].Sprites[0] = 1
	if err := ts.Update(it); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "objects")
	sum, err := ts.ExportAll(out, true, ObjectOBD)
	if err != nil || sum.Files != 1 || sum.Skipped != 4 {
		t.Fatalf("objects %+v %v", sum, err)
	}
	if _, err := os.Stat(filepath.Join(out, "items", "item_101.obd")); err != nil {
		t.Fatal(err)
	}
	if sum, err = ts.ExportAll(out, false, ObjectOTOBJ); err != nil || sum.Files != 5 {
		t.Fatalf("all objects %+v %v", sum, err)
	}

	out = filepath.Join(dir, "sprites")
	os.Mkdir(out, 0o755)
	sum, err = ss.ExportAll(out, "png", true)
	if err != nil || sum.Files != 1 || sum.Skipped != 0 {
		t.Fatalf("sprites %+v %v", sum, err)
	}
	if rec.events[len(rec.events)-1] != EventProgress {
		t.Fatalf("events %v", rec.events)
	}
}

func TestExportAllSheets(t *testing.T) {
	dir := t.TempDir()
	_, ps, _, ss, _ := newEnv(t)
	ps.New(v1098(), client.Features{})
	img := filepath.Join(dir, "tiles.png")
	writePNG(t, img, 32*5, 32, [4]byte{200, 10, 10, 255}) // 5 sprites
	if _, err := ss.ImportImages([]string{img}); err != nil {
		t.Fatal(err)
	}
	sum, err := ss.ExportAllSheets(dir, "png", 2, 2, true)
	if err != nil || sum.Files != 2 {
		t.Fatalf("%+v %v", sum, err)
	}
	last, err := imaging.Load(filepath.Join(dir, "sprites_5-5.png"))
	if err != nil {
		t.Fatal(err)
	}
	// The last sheet holds one sprite: one row, full width.
	if last.Rect.Dx() != 64 || last.Rect.Dy() != 32 || last.Pix[3] != 255 || last.Pix[(32*4)+3] != 0 {
		t.Fatalf("last sheet %v", last.Rect)
	}
	if _, err := ss.ExportAllSheets(dir, "png", 0, 4, true); err == nil {
		t.Fatal("bad size accepted")
	}
}
