// Package fsutil holds the small file system helpers shared by the
// packages that read and write client, map and settings files.
package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// FileMode is the permission of every file the app writes.
	FileMode fs.FileMode = 0o644
	// DirMode is the permission of every directory the app creates.
	DirMode fs.FileMode = 0o755
)

// FileExists reports whether path is an existing regular file.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// DirExists reports whether path is an existing directory.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ReadOptional reads a file, treating "" and a missing file as no data.
func ReadOptional(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// WriteAtomic writes data to a temporary file next to path and renames it
// over path, so readers never see a half written file.
func WriteAtomic(path string, data []byte) error {
	return writeTemp(path, func(tmp *os.File) error {
		_, err := tmp.Write(data)
		return err
	})
}

// CopyFile copies src to dst. An existing dst is kept: callers copy content
// addressed files, so a file with the same name has the same content. Files
// are never changed in place (every write goes to a new file that is renamed
// over), so a hard link is a safe and much faster copy on one volume.
func CopyFile(src, dst string) error {
	if FileExists(dst) {
		return nil
	}
	if os.Link(src, dst) == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeTemp(dst, func(tmp *os.File) error {
		_, err := io.Copy(tmp, in)
		return err
	})
}

func writeTemp(path string, write func(*os.File) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	err = write(tmp)
	if err == nil {
		err = tmp.Chmod(FileMode)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}

// PathKey returns a key that is equal for every spelling of one path that
// SamePath matches: cleaned, and lower case where file names ignore case.
func PathKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

// SamePath reports whether two paths name the same file.
func SamePath(first, second string) bool {
	return PathKey(first) == PathKey(second)
}

// TrimExt returns name without its extension.
func TrimExt(name string) string { return strings.TrimSuffix(name, filepath.Ext(name)) }

// EnsureExt returns path ending with ext (".otbm"), adding it when path
// has another extension or none.
func EnsureExt(path, ext string) string {
	if strings.EqualFold(filepath.Ext(path), ext) {
		return path
	}
	return path + ext
}
