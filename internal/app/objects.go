package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/otobj"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Object file formats for export (settings.Settings.ObjectFormat).
const (
	ObjectOTOBJ = "otobj" // native format, see docs/otobj.md
	ObjectOBD   = "obd"   // ObjectBuilder, version 3
)

// Generator names this program in written OTOBJ files; main adds the
// version.
var Generator = "OTS Creator"

// isObjectFile reports whether path is an object file (.otobj or .obd).
func isObjectFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == otobj.Ext || ext == ".obd"
}

// readObject reads an .otobj or .obd file, chosen by the extension.
func readObject(path string) (*obd.Data, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d *obd.Data
	if strings.EqualFold(filepath.Ext(path), otobj.Ext) {
		d, err = otobj.Decode(data)
	} else {
		d, err = obd.Decode(data)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return d, nil
}

// encodeObject writes d in format (ObjectOTOBJ or ObjectOBD) and returns
// the data and the file extension.
func encodeObject(d *obd.Data, format string) ([]byte, string, error) {
	if format == ObjectOBD {
		data, err := obd.Encode(d)
		return data, ".obd", err
	}
	data, err := otobj.Encode(d, otobj.Options{Generator: Generator})
	return data, otobj.Ext, err
}

// ObjectFile is a decoded object file (.otobj or .obd) for previewing.
// Sprite ids of Thing are rewritten to 1..len(Sprites); Sprites[id-1]
// holds RGBA pixels.
type ObjectFile struct {
	Path string `json:"path"`
	// Format is ObjectOTOBJ or ObjectOBD; Version the version of that format.
	Format        string       `json:"format"`
	Version       int          `json:"version"`
	ClientVersion uint16       `json:"clientVersion"`
	SpriteSize    int          `json:"spriteSize"`
	Thing         *thing.Thing `json:"thing"`
	Sprites       [][]byte     `json:"sprites"`
}

// newObjectFile prepares decoded object data for the frontend.
func newObjectFile(path string, data *obd.Data) *ObjectFile {
	format := ObjectOBD
	if strings.EqualFold(filepath.Ext(path), otobj.Ext) {
		format = ObjectOTOBJ
	}
	file := &ObjectFile{
		Path: path, Format: format, Version: data.Version, ClientVersion: data.ClientVersion,
		SpriteSize: data.SpriteSize, Thing: data.Thing.Clone(), Sprites: [][]byte{},
	}
	for groupIndex, group := range file.Thing.FrameGroups {
		for slot := range group.Sprites {
			file.Sprites = append(file.Sprites, data.Sprites[groupIndex][slot].Pixels)
			group.Sprites[slot] = uint32(len(file.Sprites))
		}
	}
	return file
}

// exportObject writes one thing of p into dir as {category}_{id}.{ext}.
func exportObject(p *project.Project, c thing.Category, id uint32, dir, format string) (string, error) {
	d, err := p.ExportOBD(c, id, obd.Version3)
	if err != nil {
		return "", err
	}
	data, ext, err := encodeObject(d, format)
	if err != nil {
		return "", fmt.Errorf("%s %d: %w", c, id, err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%d%s", c, id, ext))
	return path, os.WriteFile(path, data, fsutil.FileMode)
}
