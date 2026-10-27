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

const CatalogFile = "catalog-content.json"

type Sheet struct {
	File       string `json:"file"`
	SpriteType int    `json:"spritetype"`
	First      uint32 `json:"firstspriteid"`
	Last       uint32 `json:"lastspriteid"`
	Area       int    `json:"area"`
}

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

func ParseCatalog(data []byte) (*Catalog, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", CatalogFile, err)
	}
	catalog := &Catalog{}
	for _, entry := range raw {
		var head entryHead
		if err := json.Unmarshal(entry, &head); err != nil {
			return nil, fmt.Errorf("%s: %w", CatalogFile, err)
		}
		switch head.Type {
		case "appearances":
			catalog.Appearances = head.File
		case "sprite":
			var sheet Sheet
			if err := json.Unmarshal(entry, &sheet); err != nil {
				return nil, fmt.Errorf("%s: %w", CatalogFile, err)
			}
			if _, _, ok := SpriteDims(sheet.SpriteType); !ok || sheet.Last < sheet.First || sheet.Count() > SheetCapacity(sheet.SpriteType) {
				return nil, fmt.Errorf("%s: bad sprite sheet entry %s", CatalogFile, sheet.File)
			}
			catalog.Sheets = append(catalog.Sheets, sheet)
		default:
			catalog.others = append(catalog.others, entry)
		}
	}
	if catalog.Appearances == "" {
		return nil, fmt.Errorf("%s: no appearances file", CatalogFile)
	}
	slices.SortFunc(catalog.Sheets, func(left, right Sheet) int { return int(left.First) - int(right.First) })
	for i := 1; i < len(catalog.Sheets); i++ {
		if catalog.Sheets[i].First <= catalog.Sheets[i-1].Last {
			return nil, fmt.Errorf("%s: sprite sheets %s and %s overlap", CatalogFile, catalog.Sheets[i-1].File, catalog.Sheets[i].File)
		}
	}
	return catalog, nil
}

func ReadCatalog(dir string) (*Catalog, error) {
	data, err := os.ReadFile(filepath.Join(dir, CatalogFile))
	if err != nil {
		return nil, err
	}
	return ParseCatalog(data)
}

func (c *Catalog) Find(id uint32) (int, bool) {
	return findSheet(c.Sheets, id)
}

func findSheet(sheets []Sheet, id uint32) (int, bool) {
	i, found := slices.BinarySearchFunc(sheets, id, func(sheet Sheet, id uint32) int {
		switch {
		case sheet.Last < id:
			return -1
		case sheet.First > id:
			return 1
		}
		return 0
	})
	if !found {
		return 0, false
	}
	return i, true
}

// NextSpriteID returns the first unused sprite id after every sheet. Ids
// start at 1: clients read sprite id 0 as "no sprite".
func (c *Catalog) NextSpriteID() uint32 {
	if len(c.Sheets) == 0 {
		return 1
	}
	return c.Sheets[len(c.Sheets)-1].Last + 1
}

func (c *Catalog) Files() []string {
	out := []string{c.Appearances}
	for _, entry := range c.others {
		var head entryHead
		if json.Unmarshal(entry, &head) == nil && head.File != "" {
			out = append(out, head.File)
		}
	}
	for _, sheet := range c.Sheets {
		out = append(out, sheet.File)
	}
	return out
}

// Marshal encodes the catalog in the layout of the official file: the
// appearances entry, the other entries, then the sprite sheets.
func (c *Catalog) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("[\n")
	write := func(entry any) error {
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return err
		}
		if buf.Len() > 2 {
			buf.WriteString(", \n")
		}
		buf.Write(data)
		return nil
	}
	if err := write(entryHead{Type: "appearances", File: c.Appearances}); err != nil {
		return nil, err
	}
	for _, entry := range c.others {
		if err := write(entry); err != nil {
			return nil, err
		}
	}
	for _, sheet := range c.Sheets {
		if err := write(struct {
			Type string `json:"type"`
			Sheet
		}{"sprite", sheet}); err != nil {
			return nil, err
		}
	}
	buf.WriteString("\n]\n")
	return buf.Bytes(), nil
}

// FindDir returns the asset folder for a path: the folder itself, its
// catalog file, or a Tibia install folder holding packages/Tibia/assets.
func FindDir(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		if strings.EqualFold(filepath.Base(path), CatalogFile) {
			return filepath.Dir(path), nil
		}
		path = filepath.Dir(path)
	}
	for _, dir := range []string{path, filepath.Join(path, "assets"), filepath.Join(path, "Tibia", "assets"), filepath.Join(path, "packages", "Tibia", "assets")} {
		if info, err := os.Stat(filepath.Join(dir, CatalogFile)); err == nil && !info.IsDir() {
			return dir, nil
		}
	}
	return "", errors.New("no " + CatalogFile + " found")
}

func (c *Catalog) With(appearances string, sheets []Sheet) *Catalog {
	return &Catalog{Appearances: appearances, Sheets: slices.Clone(sheets), others: c.others}
}

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
