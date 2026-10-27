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

const (
	KindObject  = "object"
	KindSprites = "sprites"
)

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
	SpriteSize  int      `json:"spriteSize"`
}

var (
	idPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,62}[a-z0-9]$`)
	tagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	categories = []string{"item", "outfit", "effect", "missile"}
)

func ValidID(id string) bool { return idPattern.MatchString(id) }

func NormalizeTags(tags []string) []string {
	out := []string{}
	for _, tag := range tags {
		tag = strings.Join(strings.Fields(strings.ToLower(tag)), "-")
		if tag != "" && !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}
	return out
}

func plain(text string, maxRunes int) bool {
	return text != "" && text == strings.TrimSpace(text) && utf8.RuneCountInString(text) <= maxRunes && !strings.ContainsFunc(text, unicode.IsControl)
}

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
	for _, tag := range m.Tags {
		if len(tag) > MaxTag || !tagPattern.MatchString(tag) {
			errs = append(errs, fmt.Errorf("tag %q must be lowercase letters, digits and dashes, at most %d", tag, MaxTag))
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

type Entry struct {
	ID string `json:"id"`
	Meta
	Info
	Created string `json:"created"`
	File    string `json:"file"`
	Preview string `json:"preview"`
	Bytes   int    `json:"bytes"`
}

type Index struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

const IndexVersion = 1
