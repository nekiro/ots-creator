package client

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// OTFI is the OTClient feature file (*.otfi) that sits next to dat/spr files.
//
//	DatSpr
//	  extended: true
//	  transparency: false
//	  frame-durations: true
//	  frame-groups: true
//	  metadata-file: Tibia.dat
//	  sprites-file: Tibia.spr
//	  sprite-size: 32
type OTFI struct {
	Features     Features `json:"features"`
	MetadataFile string   `json:"metadataFile"`
	SpritesFile  string   `json:"spritesFile"`
}

// ParseOTFI parses the DatSpr node of an OTML document.
func ParseOTFI(data []byte) (OTFI, error) {
	var o OTFI
	sc := bufio.NewScanner(bytes.NewReader(data))
	inNode := false
	found := false
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indented := raw[0] == ' ' || raw[0] == '\t'
		if !indented {
			inNode = trimmed == "DatSpr"
			found = found || inNode
			continue
		}
		if !inNode {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return o, fmt.Errorf("otfi line %d: expected key: value", line)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch key {
		case "extended":
			o.Features.Extended = parseBool(value)
		case "transparency":
			o.Features.Transparency = parseBool(value)
		case "frame-durations":
			o.Features.ImprovedAnimations = parseBool(value)
		case "frame-groups":
			o.Features.FrameGroups = parseBool(value)
		case "metadata-file":
			o.MetadataFile = value
		case "sprites-file":
			o.SpritesFile = value
		case "sprite-size":
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return o, fmt.Errorf("otfi line %d: bad sprite-size %q", line, value)
			}
			o.Features.SpriteSize = n
		}
	}
	if err := sc.Err(); err != nil {
		return o, err
	}
	if !found {
		return o, fmt.Errorf("otfi: missing DatSpr node")
	}
	if o.Features.SpriteSize == 0 {
		o.Features.SpriteSize = DefaultSpriteSize
	}
	return o, nil
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}

// Marshal writes the OTFI document.
func (o OTFI) Marshal() []byte {
	var b strings.Builder
	b.WriteString("DatSpr\n")
	fmt.Fprintf(&b, "  extended: %t\n", o.Features.Extended)
	fmt.Fprintf(&b, "  transparency: %t\n", o.Features.Transparency)
	fmt.Fprintf(&b, "  frame-durations: %t\n", o.Features.ImprovedAnimations)
	fmt.Fprintf(&b, "  frame-groups: %t\n", o.Features.FrameGroups)
	if o.MetadataFile != "" {
		fmt.Fprintf(&b, "  metadata-file: %s\n", o.MetadataFile)
	}
	if o.SpritesFile != "" {
		fmt.Fprintf(&b, "  sprites-file: %s\n", o.SpritesFile)
	}
	if o.Features.SpriteSize != 0 && o.Features.SpriteSize != DefaultSpriteSize {
		fmt.Fprintf(&b, "  sprite-size: %d\n", o.Features.SpriteSize)
	}
	return []byte(b.String())
}
