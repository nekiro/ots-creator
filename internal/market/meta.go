// Package market reads and shares entries of the object market: objects
// (OBD files) and sprite packs (PNG grids) that users publish for others.
//
// Entries are public files, written through the market API:
//
//	index.json                  every entry (Index)
//	entries/{id}/thing.obd      object entries
//	entries/{id}/preview.gif    their animated preview, rendered by the app
//	entries/{id}/sprites.png    sprite pack entries
package market

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Entry kinds.
const (
	KindObject  = "object"
	KindSprites = "sprites"
)

// File names inside an entry folder.
const (
	ObjectFile  = "thing.obd"
	SpritesFile = "sprites.png"
	PreviewFile = "preview.gif"
	EntriesDir  = "entries"
	IndexFile   = "index.json"
)

// Limits, also checked by the market API.
const (
	MaxName         = 64
	MaxAuthor       = 32
	MaxDescription  = 500
	MaxTags         = 8
	MaxTag          = 24
	MaxObjectBytes  = 2 << 20
	MaxPackBytes    = 4 << 20
	MaxPreviewBytes = 1 << 20
	MaxPackSprites  = 1024
)

// Licenses an author can pick.
var Licenses = []string{"CC0-1.0", "CC-BY-4.0", "CC-BY-SA-4.0", "OTS-free"}

// Meta is what the author describes. Author is a free nickname.
type Meta struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags"`
	Description string   `json:"description,omitempty"`
	License     string   `json:"license"`
	Author      string   `json:"author"`
	// SpriteSize is the sprite edge of the entry (32 or 64).
	SpriteSize int `json:"spriteSize"`
}

var (
	idPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,62}[a-z0-9]$`)
	tagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	categories = []string{"item", "outfit", "effect", "missile"}
)

// ValidID reports whether id can name an entry folder.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// NormalizeTags lowercases, trims and dedupes tags; spaces become dashes.
func NormalizeTags(tags []string) []string {
	out := []string{}
	for _, t := range tags {
		t = strings.Join(strings.Fields(strings.ToLower(t)), "-")
		if t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func plain(s string, max int) bool {
	return s != "" && s == strings.TrimSpace(s) && utf8.RuneCountInString(s) <= max && !strings.ContainsFunc(s, unicode.IsControl)
}

// Validate checks the fields an author fills in.
func (m *Meta) Validate() error {
	var errs []error
	if !plain(m.Name, MaxName) {
		errs = append(errs, fmt.Errorf("name must be 1-%d characters without surrounding spaces", MaxName))
	}
	if !plain(m.Author, MaxAuthor) {
		errs = append(errs, fmt.Errorf("author must be 1-%d characters without surrounding spaces", MaxAuthor))
	}
	switch m.Kind {
	case KindObject:
		if !slices.Contains(categories, m.Category) {
			errs = append(errs, fmt.Errorf("category %q is not one of %v", m.Category, categories))
		}
	case KindSprites:
		if m.Category != "" {
			errs = append(errs, errors.New("sprite packs have no category"))
		}
	default:
		errs = append(errs, fmt.Errorf("kind %q is not %q or %q", m.Kind, KindObject, KindSprites))
	}
	if m.SpriteSize != 32 && m.SpriteSize != 64 {
		errs = append(errs, fmt.Errorf("sprite size %d is not 32 or 64", m.SpriteSize))
	}
	if len(m.Tags) > MaxTags {
		errs = append(errs, fmt.Errorf("more than %d tags", MaxTags))
	}
	for _, t := range m.Tags {
		if len(t) > MaxTag || !tagPattern.MatchString(t) {
			errs = append(errs, fmt.Errorf("tag %q must be lowercase letters, digits and dashes, at most %d", t, MaxTag))
		}
	}
	if len(NormalizeTags(m.Tags)) != len(m.Tags) {
		errs = append(errs, errors.New("tags repeat"))
	}
	if utf8.RuneCountInString(m.Description) > MaxDescription || !utf8.ValidString(m.Description) {
		errs = append(errs, fmt.Errorf("description is longer than %d characters", MaxDescription))
	}
	if !slices.Contains(Licenses, m.License) {
		errs = append(errs, fmt.Errorf("license %q is not one of %v", m.License, Licenses))
	}
	return errors.Join(errs...)
}

// Entry is one index row.
type Entry struct {
	ID string `json:"id"`
	Meta
	Info
	Created string `json:"created"`
	File    string `json:"file"`
	Preview string `json:"preview"`
	Bytes   int    `json:"bytes"`
}

// Index is index.json.
type Index struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

// IndexVersion is the newest index format the app reads.
const IndexVersion = 1
