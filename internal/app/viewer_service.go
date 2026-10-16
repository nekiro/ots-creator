package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/imaging"
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
	case ".obd", ".otobj":
		return ViewerOBD
	case ".png", ".bmp", ".gif", ".jpg", ".jpeg":
		return ViewerImage
	}
	return ""
}

// List returns the OBD files and images in dir, OBD files first, each
// group sorted by name with numbers in natural order.
func (*ViewerService) List(dir string) ([]ViewerEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []ViewerEntry{}
	for _, file := range files {
		kind := viewerKind(file.Name())
		if file.IsDir() || kind == "" {
			continue
		}
		var size int64
		if info, err := file.Info(); err == nil {
			size = info.Size()
		}
		out = append(out, ViewerEntry{Path: filepath.Join(dir, file.Name()), Name: file.Name(), Kind: kind, Size: size})
	}
	slices.SortFunc(out, func(first, second ViewerEntry) int {
		if first.Kind != second.Kind {
			if first.Kind == ViewerOBD {
				return -1
			}
			return 1
		}
		return naturalCompare(first.Name, second.Name)
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
func naturalCompare(first, second string) int {
	first, second = strings.ToLower(first), strings.ToLower(second)
	for first != "" && second != "" {
		firstDigits, secondDigits := leadingDigits(first), leadingDigits(second)
		if firstDigits > 0 && secondDigits > 0 {
			firstNumber := strings.TrimLeft(first[:firstDigits], "0")
			secondNumber := strings.TrimLeft(second[:secondDigits], "0")
			if len(firstNumber) != len(secondNumber) {
				return len(firstNumber) - len(secondNumber)
			}
			if order := strings.Compare(firstNumber, secondNumber); order != 0 {
				return order
			}
			first, second = first[firstDigits:], second[secondDigits:]
			continue
		}
		if first[0] != second[0] {
			return int(first[0]) - int(second[0])
		}
		first, second = first[1:], second[1:]
	}
	return len(first) - len(second)
}

// leadingDigits counts the digits text starts with.
func leadingDigits(text string) int {
	count := 0
	for count < len(text) && text[count] >= '0' && text[count] <= '9' {
		count++
	}
	return count
}
