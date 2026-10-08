package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestViewerList(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"item_10.obd", "item_9.obd", "b.PNG", "a.bmp", "notes.txt"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	os.Mkdir(filepath.Join(dir, "sub.obd"), 0o755)
	var vs ViewerService
	list, err := vs.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list {
		names = append(names, e.Name+":"+e.Kind)
	}
	want := []string{"item_9.obd:obd", "item_10.obd:obd", "a.bmp:image", "b.PNG:image"}
	if len(names) != len(want) {
		t.Fatalf("got %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func TestViewerReadImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	writePNG(t, path, 5, 3, [4]byte{1, 2, 3, 255})
	var vs ViewerService
	img, err := vs.ReadImage(path)
	if err != nil || img.Width != 5 || img.Height != 3 || img.Format != "png" || len(img.PNG) == 0 {
		t.Fatalf("%+v %v", img, err)
	}
	if _, err := vs.ReadImage(filepath.Join(dir, "x.obd")); err == nil {
		t.Fatal("non-image must fail")
	}
}

func TestNaturalCompare(t *testing.T) {
	if naturalCompare("a2", "a10") >= 0 || naturalCompare("A10", "a9") <= 0 || naturalCompare("x", "x") != 0 || naturalCompare("a", "ab") >= 0 {
		t.Fatal("natural order")
	}
}
