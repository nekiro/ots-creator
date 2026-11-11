package otmap

import (
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// OTMAP is the text map format (docs/otmap.md): a world file and one file
// per 32x32 sector in the sectors directory next to it.

const (
	OTMAPExt         = ".otmap"
	sectorExt        = ".sector"
	sectorsDirName   = "sectors"
	worldHeader      = "otmap 1"
	sectorHeader     = "otmap-sector 1"
	itemsServer      = "server"
	itemsClient      = "client"
	maxContentsDepth = 64
)

// IsOTMAP reports whether path names an OTMAP world file.
func IsOTMAP(path string) bool { return strings.EqualFold(filepath.Ext(path), OTMAPExt) }

func sectorFileName(sectorX, sectorY uint16) string {
	return strconv.Itoa(int(sectorX)) + "_" + strconv.Itoa(int(sectorY)) + sectorExt
}

func parseSectorFileName(name string) (sectorX, sectorY uint16, ok bool) {
	base, found := strings.CutSuffix(name, sectorExt)
	if !found {
		return 0, 0, false
	}
	left, right, found := strings.Cut(base, "_")
	if !found {
		return 0, 0, false
	}
	x, errX := strconv.ParseUint(left, 10, 16)
	y, errY := strconv.ParseUint(right, 10, 16)
	if errX != nil || errY != nil || x > 0xFFFF>>ChunkBits || y > 0xFFFF>>ChunkBits {
		return 0, 0, false
	}
	return uint16(x), uint16(y), true
}

var tileFlagNames = []struct {
	name string
	flag uint32
}{
	{"pz", FlagPZ}, {"nopvp", FlagNoPVP}, {"nologout", FlagNoLogout}, {"pvpzone", FlagPVPZone}, {"refresh", FlagRefresh},
}

var namedTileFlags = FlagPZ | FlagNoPVP | FlagNoLogout | FlagPVPZone | FlagRefresh

var directionNames = []string{"n", "e", "s", "w"}

// rawAttrNames are the standard OTBM attributes kept verbatim in
// ItemExt.Raw, in the order they are written.
var rawAttrNames = []struct {
	code      byte
	name      string
	valueType string
}{
	{attrWrittenBy, "writer", "string"},
	{attrWrittenDate, "written", "u32"},
	{attrDuration, "duration", "u32"},
	{attrDecayingState, "decay", "u8"},
	{attrSleeperGUID, "sleeper", "u32"},
	{attrSleepStart, "sleepstart", "u32"},
}

// standardAttrNames are the item attribute keys of the format; declared
// attributes cannot use them.
var standardAttrNames = map[string]bool{
	"aid": true, "uid": true, "depot": true, "door": true, "dest": true, "text": true, "desc": true,
	"writer": true, "written": true, "duration": true, "decay": true, "sleeper": true, "sleepstart": true,
}

// attrKeys gives every declared attribute its key in sector files: the
// name made an identifier, unique, and not a standard key.
func attrKeys(declared []CustomAttr) map[byte]string {
	keys := make(map[byte]string, len(declared))
	used := map[string]bool{}
	for _, attr := range declared {
		key := attrKey(attr.Name)
		if key == "" || standardAttrNames[key] || used[key] {
			key = fmt.Sprintf("attr_%d", attr.Code)
		}
		used[key] = true
		keys[attr.Code] = key
	}
	return keys
}

func attrKey(name string) string {
	var out strings.Builder
	underscore := false
	for _, char := range strings.ToLower(name) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9' && out.Len() > 0:
			if underscore && out.Len() > 0 {
				out.WriteByte('_')
			}
			underscore = false
			out.WriteRune(char)
		default:
			underscore = true
		}
	}
	return out.String()
}

func validName(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		if char := name[i]; !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_') {
			return false
		}
	}
	return true
}

// appendQuoted writes a string. Valid UTF-8 stays as it is; other bytes
// (Latin-1 text of old servers) become \xHH, so every byte string reads back
// unchanged.
func appendQuoted(out []byte, text string) []byte {
	out = append(out, '"')
	for i := 0; i < len(text); {
		char := text[i]
		switch {
		case char == '"' || char == '\\':
			out = append(out, '\\', char)
			i++
			continue
		case char == '\n':
			out = append(out, '\\', 'n')
			i++
			continue
		case char < 0x20 || char == 0x7F:
			out = appendHexByte(out, char)
			i++
			continue
		case char < utf8.RuneSelf:
			out = append(out, char)
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			out = appendHexByte(out, char)
		} else {
			out = append(out, text[i:i+size]...)
		}
		i += size
	}
	return append(out, '"')
}

func appendHexByte(out []byte, value byte) []byte {
	const digits = "0123456789ABCDEF"
	return append(out, '\\', 'x', digits[value>>4], digits[value&15])
}

func appendPosition(out []byte, pos Position) []byte {
	out = strconv.AppendUint(out, uint64(pos.X), 10)
	out = append(out, ',')
	out = strconv.AppendUint(out, uint64(pos.Y), 10)
	out = append(out, ',')
	return strconv.AppendUint(out, uint64(pos.Z), 10)
}

// appendFloat writes the shortest form that reads back to the same value,
// always with a dot or an exponent so it does not read as an integer.
func appendFloat(out []byte, value float64, bitSize int) []byte {
	switch {
	case math.IsNaN(value):
		return append(out, "nan"...)
	case math.IsInf(value, 1):
		return append(out, "inf"...)
	case math.IsInf(value, -1):
		return append(out, "-inf"...)
	}
	start := len(out)
	out = strconv.AppendFloat(out, value, 'g', -1, bitSize)
	if !strings.ContainsAny(string(out[start:]), ".e") {
		out = append(out, ".0"...)
	}
	return out
}

// rawValueSize is the payload size of a raw attribute type; -1 is a
// uint16-prefixed string.
func rawValueSize(valueType string) (int, bool) {
	size, ok := customAttrSizes[valueType]
	return size, ok
}

// appendRawValue writes the payload of a raw attribute as text.
func appendRawValue(out []byte, valueType string, data []byte) ([]byte, error) {
	size, ok := rawValueSize(valueType)
	if !ok {
		return out, fmt.Errorf("unknown attribute type %q", valueType)
	}
	if size < 0 {
		if len(data) < 2 || int(binary.LittleEndian.Uint16(data)) != len(data)-2 {
			return out, fmt.Errorf("broken %s value", valueType)
		}
		return appendQuoted(out, string(data[2:])), nil
	}
	if len(data) != size {
		return out, fmt.Errorf("%s value of %d bytes", valueType, len(data))
	}
	switch valueType {
	case "u8":
		return strconv.AppendUint(out, uint64(data[0]), 10), nil
	case "u16":
		return strconv.AppendUint(out, uint64(binary.LittleEndian.Uint16(data)), 10), nil
	case "u32":
		return strconv.AppendUint(out, uint64(binary.LittleEndian.Uint32(data)), 10), nil
	case "i32":
		return strconv.AppendInt(out, int64(int32(binary.LittleEndian.Uint32(data))), 10), nil
	default: // u64
		return strconv.AppendUint(out, binary.LittleEndian.Uint64(data), 10), nil
	}
}
