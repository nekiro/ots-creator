package fsutil

import (
	"encoding/json"
	"errors"
	"fmt"
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
	// PrivateFileMode is for files that hold passwords or tokens.
	PrivateFileMode fs.FileMode = 0o600
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
	return writeTemp(path, FileMode, func(tmp *os.File) error {
		_, err := tmp.Write(data)
		return err
	})
}

// WriteJSON writes value as JSON indented with indent to path, creating
// its folder, with WriteAtomic. An empty path writes nothing.
func WriteJSON(path string, value any, indent string) error {
	return writeJSON(path, value, indent, FileMode)
}

// WriteJSONPrivate is WriteJSON for files that hold passwords or tokens:
// only the user can read the file.
func WriteJSONPrivate(path string, value any, indent string) error {
	return writeJSON(path, value, indent, PrivateFileMode)
}

func writeJSON(path string, value any, indent string, mode os.FileMode) error {
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
	return writeTemp(path, mode, func(tmp *os.File) error {
		_, err := tmp.Write(data)
		return err
	})
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
	return writeTemp(dst, FileMode, func(tmp *os.File) error {
		_, err := io.Copy(tmp, in)
		return err
	})
}

func writeTemp(path string, mode os.FileMode, write func(*os.File) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	err = write(tmp)
	if err == nil {
		err = tmp.Chmod(mode)
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

// ReadJSON reads the JSON file at path into value. A missing file is no
// error and leaves value alone. A file that is not valid JSON is moved
// aside, so the next save does not overwrite what a person may still
// recover from it; the error tells where it went.
func ReadJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return SetAside(path, err)
	}
	return nil
}

// SetAside moves the damaged file at path to path + ".bad" and returns the
// error to show for it; cause is what is wrong with the file.
func SetAside(path string, cause error) error {
	name := filepath.Base(path)
	if err := os.Rename(path, path+".bad"); err != nil {
		return fmt.Errorf("%s is damaged and its defaults are used (%w)", name, cause)
	}
	return fmt.Errorf("%s was damaged and its defaults are used; the old file is kept as %s.bad (%w)", name, name, cause)
}
