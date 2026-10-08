package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/imaging"
)

// Kinds of files the viewer lists.
const (
	ViewerOBD   = "obd"
	ViewerImage = "image"
)

// ViewerEntry is one file the viewer can show.
type ViewerEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

// ImageFile is an image decoded for the viewer, re-encoded as PNG.
type ImageFile struct {
	Path   string `json:"path"`
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	PNG    []byte `json:"png"`
}

// ViewerService lists and reads files for the OBD viewer. It works without
// an open client.
type ViewerService struct{}

func viewerKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".obd":
		return ViewerOBD
	case ".png", ".bmp", ".gif", ".jpg", ".jpeg":
		return ViewerImage
	}
	return ""
}

// List returns the OBD files and images in dir, OBD files first, each
// group sorted by name with numbers in natural order.
func (*ViewerService) List(dir string) ([]ViewerEntry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []ViewerEntry{}
	for _, de := range des {
		kind := viewerKind(de.Name())
		if de.IsDir() || kind == "" {
			continue
		}
		var size int64
		if info, err := de.Info(); err == nil {
			size = info.Size()
		}
		out = append(out, ViewerEntry{Path: filepath.Join(dir, de.Name()), Name: de.Name(), Kind: kind, Size: size})
	}
	slices.SortFunc(out, func(a, b ViewerEntry) int {
		if a.Kind != b.Kind {
			if a.Kind == ViewerOBD {
				return -1
			}
			return 1
		}
		return naturalCompare(a.Name, b.Name)
	})
	return out, nil
}

// ReadImage decodes an image file.
func (*ViewerService) ReadImage(path string) (*ImageFile, error) {
	if viewerKind(path) != ViewerImage {
		return nil, fmt.Errorf("%s is not an image", filepath.Base(path))
	}
	img, err := imaging.Load(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	data, err := imaging.EncodePNG(img)
	if err != nil {
		return nil, err
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	return &ImageFile{Path: path, Format: format, Width: img.Rect.Dx(), Height: img.Rect.Dy(), PNG: data}, nil
}

// naturalCompare orders "item_9" before "item_10", ignoring case.
func naturalCompare(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if len(na) != len(nb) {
				return len(na) - len(nb)
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return int(a[0]) - int(b[0])
		}
		a, b = a[1:], b[1:]
	}
	return len(a) - len(b)
}

func digits(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}
