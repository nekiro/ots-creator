package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	return path, os.WriteFile(path, data, 0o644)
}
