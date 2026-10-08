package assets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// CatalogFile is the index of an asset folder.
const CatalogFile = "catalog-content.json"

// Sheet is one sprite sheet of the catalog.
type Sheet struct {
	File       string `json:"file"`
	SpriteType int    `json:"spritetype"`
	First      uint32 `json:"firstspriteid"`
	Last       uint32 `json:"lastspriteid"`
	Area       int    `json:"area"`
}

// Count returns the number of sprites in the sheet.
func (s Sheet) Count() int { return int(s.Last-s.First) + 1 }

// Catalog is a parsed catalog-content.json. Entries other than sprite
// sheets and the appearances file (static data, map, ...) are kept as is.
type Catalog struct {
	Appearances string
	// Sheets are ordered by first sprite id.
	Sheets []Sheet
	others []json.RawMessage
}

type entryHead struct {
	Type string `json:"type"`
	File string `json:"file"`
}

// ParseCatalog parses catalog-content.json.
func ParseCatalog(data []byte) (*Catalog, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", CatalogFile, err)
	}
	c := &Catalog{}
	for _, r := range raw {
		var h entryHead
		if err := json.Unmarshal(r, &h); err != nil {
			return nil, fmt.Errorf("%s: %w", CatalogFile, err)
		}
		switch h.Type {
		case "appearances":
			c.Appearances = h.File
		case "sprite":
			var s Sheet
			if err := json.Unmarshal(r, &s); err != nil {
				return nil, fmt.Errorf("%s: %w", CatalogFile, err)
			}
			if _, _, ok := SpriteDims(s.SpriteType); !ok || s.Last < s.First || s.Count() > SheetCapacity(s.SpriteType) {
				return nil, fmt.Errorf("%s: bad sprite sheet entry %s", CatalogFile, s.File)
			}
			c.Sheets = append(c.Sheets, s)
		default:
			c.others = append(c.others, r)
		}
	}
	if c.Appearances == "" {
		return nil, fmt.Errorf("%s: no appearances file", CatalogFile)
	}
	slices.SortFunc(c.Sheets, func(a, b Sheet) int { return int(a.First) - int(b.First) })
	for i := 1; i < len(c.Sheets); i++ {
		if c.Sheets[i].First <= c.Sheets[i-1].Last {
			return nil, fmt.Errorf("%s: sprite sheets %s and %s overlap", CatalogFile, c.Sheets[i-1].File, c.Sheets[i].File)
		}
	}
	return c, nil
}

// ReadCatalog reads the catalog of an asset folder.
func ReadCatalog(dir string) (*Catalog, error) {
	data, err := os.ReadFile(filepath.Join(dir, CatalogFile))
	if err != nil {
		return nil, err
	}
	return ParseCatalog(data)
}

// Find returns the index of the sheet holding a sprite id.
func (c *Catalog) Find(id uint32) (int, bool) {
	i, _ := slices.BinarySearchFunc(c.Sheets, id, func(s Sheet, id uint32) int {
		switch {
		case s.Last < id:
			return -1
		case s.First > id:
			return 1
		}
		return 0
	})
	if i < len(c.Sheets) && c.Sheets[i].First <= id && id <= c.Sheets[i].Last {
		return i, true
	}
	return 0, false
}

// NextSpriteID returns the first unused sprite id after every sheet.
func (c *Catalog) NextSpriteID() uint32 {
	if len(c.Sheets) == 0 {
		return 0
	}
	return c.Sheets[len(c.Sheets)-1].Last + 1
}

// Files lists every file the catalog references.
func (c *Catalog) Files() []string {
	out := []string{c.Appearances}
	for _, r := range c.others {
		var h entryHead
		if json.Unmarshal(r, &h) == nil && h.File != "" {
			out = append(out, h.File)
		}
	}
	for _, s := range c.Sheets {
		out = append(out, s.File)
	}
	return out
}

// Marshal encodes the catalog in the layout of the official file: the
// appearances entry, the other entries, then the sprite sheets.
func (c *Catalog) Marshal() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("[\n")
	write := func(v any) error {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		if b.Len() > 2 {
			b.WriteString(", \n")
		}
		b.Write(data)
		return nil
	}
	if err := write(entryHead{Type: "appearances", File: c.Appearances}); err != nil {
		return nil, err
	}
	for _, r := range c.others {
		if err := write(r); err != nil {
			return nil, err
		}
	}
	for _, s := range c.Sheets {
		if err := write(struct {
			Type string `json:"type"`
			Sheet
		}{"sprite", s}); err != nil {
			return nil, err
		}
	}
	b.WriteString("\n]\n")
	return b.Bytes(), nil
}

// FindDir returns the asset folder for a path: the folder itself, its
// catalog file, or a Tibia install folder holding packages/Tibia/assets.
func FindDir(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		if strings.EqualFold(filepath.Base(path), CatalogFile) {
			return filepath.Dir(path), nil
		}
		path = filepath.Dir(path)
	}
	for _, dir := range []string{path, filepath.Join(path, "assets"), filepath.Join(path, "Tibia", "assets"), filepath.Join(path, "packages", "Tibia", "assets")} {
		if st, err := os.Stat(filepath.Join(dir, CatalogFile)); err == nil && !st.IsDir() {
			return dir, nil
		}
	}
	return "", errors.New("no " + CatalogFile + " found")
}

// With returns a copy of the catalog with another appearances file and
// extra sprite sheets (with ids after every existing sheet).
func (c *Catalog) With(appearances string, sheets []Sheet) *Catalog {
	out := &Catalog{Appearances: appearances, others: c.others}
	out.Sheets = append(slices.Clone(c.Sheets), sheets...)
	return out
}

// ClientVersion reads the client version ("15.33") from the package.json
// of the Tibia package that holds an asset folder. It returns "" when there
// is none.
func ClientVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(dir), "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	parts := strings.SplitN(pkg.Version, ".", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}
