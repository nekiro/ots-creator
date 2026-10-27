package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/obd"
	"github.com/nekiro/ots-creator/internal/formats/otobj"
	"github.com/nekiro/ots-creator/internal/fsutil"
)

const (
	ObjectOTOBJ = "otobj" // native format, see docs/otobj.md
	ObjectOBD   = "obd"   // ObjectBuilder, version 3
)

// Generator names this program in written OTOBJ files; main adds the
// version.
var Generator = "OTS Creator"

func isObjectFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == otobj.Ext || ext == ".obd"
}

func readObject(path string) (*obd.Data, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var objectData *obd.Data
	if strings.EqualFold(filepath.Ext(path), otobj.Ext) {
		objectData, err = otobj.Decode(data)
	} else {
		objectData, err = obd.Decode(data)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return objectData, nil
}

func encodeObject(objectData *obd.Data, format string) ([]byte, string, error) {
	if format == ObjectOBD {
		data, err := obd.Encode(objectData)
		return data, ".obd", err
	}
	data, err := otobj.Encode(objectData, otobj.Options{Generator: Generator})
	return data, otobj.Ext, err
}

// ObjectFile is a decoded object file (.otobj or .obd) for previewing.
// Sprite ids of Thing are rewritten to 1..len(Sprites); Sprites[id-1]
// holds RGBA pixels.
type ObjectFile struct {
	Path          string       `json:"path"`
	Format        string       `json:"format"`
	Version       int          `json:"version"`
	ClientVersion uint16       `json:"clientVersion"`
	SpriteSize    int          `json:"spriteSize"`
	Thing         *thing.Thing `json:"thing"`
	Sprites       [][]byte     `json:"sprites"`
}

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

func exportObject(proj *project.Project, category thing.Category, id uint32, dir, format string) (string, error) {
	objectData, err := proj.ExportOBD(category, id, obd.Version3)
	if err != nil {
		return "", err
	}
	data, ext, err := encodeObject(objectData, format)
	if err != nil {
		return "", fmt.Errorf("%s %d: %w", category, id, err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%d%s", category, id, ext))
	return path, os.WriteFile(path, data, fsutil.FileMode)
}
