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

type Version struct {
	Value        uint16 `json:"value"`
	Name         string `json:"name"`
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

func ParseVersions(data []byte) ([]Version, error) {
	var doc xmlVersions
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse versions: %w", err)
	}
	out := make([]Version, 0, len(doc.Versions))
	for _, entry := range doc.Versions {
		value, err := strconv.ParseUint(entry.Value, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("version %q: bad value: %w", entry.String, err)
		}
		datSignature, err := strconv.ParseUint(entry.Dat, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("version %q: bad dat signature: %w", entry.String, err)
		}
		sprSignature, err := strconv.ParseUint(entry.Spr, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("version %q: bad spr signature: %w", entry.String, err)
		}
		out = append(out, Version{Value: uint16(value), Name: entry.String, DatSignature: uint32(datSignature), SprSignature: uint32(sprSignature)})
	}
	return out, nil
}

var (
	builtinOnce sync.Once
	builtin     []Version
)

func Versions() []Version {
	builtinOnce.Do(func() {
		parsed, err := ParseVersions(versionsXML)
		if err != nil {
			panic(err) // embedded data is covered by tests
		}
		builtin = parsed
	})
	return slices.Clone(builtin)
}

func FindBySignatures(datSignature, sprSignature uint32) (Version, bool) {
	for _, version := range Versions() {
		if version.DatSignature == datSignature && version.SprSignature == sprSignature {
			return version, true
		}
	}
	return Version{}, false
}

func FindByValue(value uint16) []Version {
	var out []Version
	for _, version := range Versions() {
		if version.Value == value {
			out = append(out, version)
		}
	}
	return out
}
