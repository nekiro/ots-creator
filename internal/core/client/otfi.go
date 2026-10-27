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

func ParseOTFI(data []byte) (OTFI, error) {
	var result OTFI
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inNode := false
	found := false
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Text()
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
			return result, fmt.Errorf("otfi line %d: expected key: value", line)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch key {
		case "extended":
			result.Features.Extended = parseBool(value)
		case "transparency":
			result.Features.Transparency = parseBool(value)
		case "frame-durations":
			result.Features.ImprovedAnimations = parseBool(value)
		case "frame-groups":
			result.Features.FrameGroups = parseBool(value)
		case "metadata-file":
			result.MetadataFile = value
		case "sprites-file":
			result.SpritesFile = value
		case "sprite-size":
			size, err := strconv.Atoi(value)
			if err != nil || size <= 0 {
				return result, fmt.Errorf("otfi line %d: bad sprite-size %q", line, value)
			}
			result.Features.SpriteSize = size
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if !found {
		return result, fmt.Errorf("otfi: missing DatSpr node")
	}
	if result.Features.SpriteSize == 0 {
		result.Features.SpriteSize = DefaultSpriteSize
	}
	return result, nil
}

func parseBool(text string) bool {
	value, _ := strconv.ParseBool(text)
	return value
}

func (o OTFI) Marshal() []byte {
	var builder strings.Builder
	builder.WriteString("DatSpr\n")
	fmt.Fprintf(&builder, "  extended: %t\n", o.Features.Extended)
	fmt.Fprintf(&builder, "  transparency: %t\n", o.Features.Transparency)
	fmt.Fprintf(&builder, "  frame-durations: %t\n", o.Features.ImprovedAnimations)
	fmt.Fprintf(&builder, "  frame-groups: %t\n", o.Features.FrameGroups)
	if o.MetadataFile != "" {
		fmt.Fprintf(&builder, "  metadata-file: %s\n", o.MetadataFile)
	}
	if o.SpritesFile != "" {
		fmt.Fprintf(&builder, "  sprites-file: %s\n", o.SpritesFile)
	}
	if o.Features.SpriteSize != 0 && o.Features.SpriteSize != DefaultSpriteSize {
		fmt.Fprintf(&builder, "  sprite-size: %d\n", o.Features.SpriteSize)
	}
	return []byte(builder.String())
}
