// Package client describes client versions and their feature switches.
package client

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"slices"
	"strconv"
	"sync"
)

//go:embed versions.xml
var versionsXML []byte

// Version is one known client release.
type Version struct {
	Value        uint16 `json:"value"` // e.g. 1098
	Name         string `json:"name"`  // e.g. "10.98"
	DatSignature uint32 `json:"datSignature"`
	SprSignature uint32 `json:"sprSignature"`
}

func (v Version) String() string { return v.Name }

type xmlVersions struct {
	Versions []struct {
		Value  string `xml:"value,attr"`
		String string `xml:"string,attr"`
		Dat    string `xml:"dat,attr"`
		Spr    string `xml:"spr,attr"`
	} `xml:"version"`
}

// ParseVersions parses a versions.xml document in the ObjectBuilder format.
func ParseVersions(data []byte) ([]Version, error) {
	var doc xmlVersions
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse versions: %w", err)
	}
	out := make([]Version, 0, len(doc.Versions))
	for _, x := range doc.Versions {
		value, err := strconv.ParseUint(x.Value, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("version %q: bad value: %w", x.String, err)
		}
		dat, err := strconv.ParseUint(x.Dat, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("version %q: bad dat signature: %w", x.String, err)
		}
		spr, err := strconv.ParseUint(x.Spr, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("version %q: bad spr signature: %w", x.String, err)
		}
		out = append(out, Version{Value: uint16(value), Name: x.String, DatSignature: uint32(dat), SprSignature: uint32(spr)})
	}
	return out, nil
}

var (
	builtinOnce sync.Once
	builtin     []Version
)

// Versions returns the built-in version table.
func Versions() []Version {
	builtinOnce.Do(func() {
		v, err := ParseVersions(versionsXML)
		if err != nil {
			panic(err) // embedded data is covered by tests
		}
		builtin = v
	})
	return slices.Clone(builtin)
}

// FindBySignatures returns the first version that matches both signatures.
func FindBySignatures(dat, spr uint32) (Version, bool) {
	for _, v := range Versions() {
		if v.DatSignature == dat && v.SprSignature == spr {
			return v, true
		}
	}
	return Version{}, false
}

// FindByValue returns every version with the given numeric value.
func FindByValue(value uint16) []Version {
	var out []Version
	for _, v := range Versions() {
		if v.Value == value {
			out = append(out, v)
		}
	}
	return out
}
