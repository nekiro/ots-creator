package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.bin")
	if err := WriteAtomic(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("second")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "second" {
		t.Fatalf("read %q, %v", data, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left: %v", entries)
	}
	if err := WriteAtomic(filepath.Join(path, "below-a-file"), nil); err == nil {
		t.Fatal("write below a file succeeded")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	os.WriteFile(src, []byte("data"), FileMode)
	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "data" {
		t.Fatalf("copy = %q", data)
	}
	os.WriteFile(filepath.Join(dir, "other"), []byte("other"), FileMode)
	if err := CopyFile(filepath.Join(dir, "other"), dst); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "data" {
		t.Fatal("existing destination was replaced")
	}
	if err := CopyFile(filepath.Join(dir, "missing"), filepath.Join(dir, "new")); err == nil {
		t.Fatal("copy of a missing file succeeded")
	}
}

func TestExistsAndReadOptional(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	os.WriteFile(file, []byte("x"), FileMode)
	if !FileExists(file) || FileExists(dir) || FileExists(filepath.Join(dir, "missing")) {
		t.Fatal("FileExists")
	}
	if !DirExists(dir) || DirExists(file) {
		t.Fatal("DirExists")
	}
	if data, err := ReadOptional(""); data != nil || err != nil {
		t.Fatal("empty path")
	}
	if data, err := ReadOptional(filepath.Join(dir, "missing")); data != nil || err != nil {
		t.Fatal("missing file")
	}
	if data, err := ReadOptional(file); string(data) != "x" || err != nil {
		t.Fatal("existing file")
	}
}

func TestSamePath(t *testing.T) {
	if !SamePath("a/b/../c", "a/c") {
		t.Fatal("cleaned paths differ")
	}
	if got := SamePath("Dir/File", "dir/file"); got != (runtime.GOOS == "windows") {
		t.Fatalf("case insensitive = %v on %s", got, runtime.GOOS)
	}
	if SamePath("a", "b") {
		t.Fatal("different paths match")
	}
}

func TestExtensions(t *testing.T) {
	if TrimExt("map.otbm") != "map" || TrimExt("dir.v2/map") != "dir.v2/map" {
		t.Fatal("TrimExt")
	}
	if EnsureExt("map", ".otbm") != "map.otbm" || EnsureExt("map.OTBM", ".otbm") != "map.OTBM" ||
		EnsureExt("map.txt", ".otbm") != "map.txt.otbm" {
		t.Fatal("EnsureExt")
	}
}
