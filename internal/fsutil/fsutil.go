package fsutil

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	FileMode fs.FileMode = 0o644
	DirMode  fs.FileMode = 0o755
)

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

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

// WriteJSON writes value as JSON indented with indent to path, creating
// its folder, with WriteAtomic. An empty path writes nothing.
func WriteJSON(path string, value any, indent string) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(value, "", indent)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), DirMode); err != nil {
		return err
	}
	return WriteAtomic(path, data)
}

// AppendOtherPaths appends to out the entries of list whose path (given by
// pathOf) is not the same file as path, keeping their order. It trims
// recent file lists.
func AppendOtherPaths[T any](out, list []T, path string, pathOf func(T) string) []T {
	for _, entry := range list {
		if !SamePath(pathOf(entry), path) {
			out = append(out, entry)
		}
	}
	return out
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

func SamePath(first, second string) bool {
	return PathKey(first) == PathKey(second)
}

func TrimExt(name string) string { return strings.TrimSuffix(name, filepath.Ext(name)) }

// EnsureExt returns path ending with ext (".otbm"), adding it when path
// has another extension or none.
func EnsureExt(path, ext string) string {
	if strings.EqualFold(filepath.Ext(path), ext) {
		return path
	}
	return path + ext
}
