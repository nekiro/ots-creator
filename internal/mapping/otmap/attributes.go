package otmap

import (
	"encoding/json"
	"fmt"
)

// CustomAttr is an item attribute a server adds to OTBM, declared in the
// map's attributes file (<map>-attributes.json) so the editor keeps it:
//
//	[{"code": 28, "name": "Chest quest number", "type": "u16"}]
type CustomAttr struct {
	Code byte   `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

const attrsFileSuffix = "-attributes.json"

var customAttrSizes = map[string]int{
	"u8": 1, "u16": 2, "u32": 4, "i32": 4, "u64": 8, "string": -1,
}

// ReadCustomAttrs parses an attributes file into the valid declarations.
// Bad entries are skipped with a warning.
func ReadCustomAttrs(data []byte) ([]CustomAttr, []string, error) {
	var attrs []CustomAttr
	if err := json.Unmarshal(data, &attrs); err != nil {
		return nil, nil, err
	}
	var valid []CustomAttr
	var warnings []string
	codes := map[byte]bool{}
	for _, attr := range attrs {
		_, ok := customAttrSizes[attr.Type]
		switch {
		case !ok:
			warnings = append(warnings, fmt.Sprintf("attribute %d: unknown type %q", attr.Code, attr.Type))
		case attr.Code <= attrZoneFile || attr.Code == attrAttributeMap:
			warnings = append(warnings, fmt.Sprintf("attribute %d: the code is a standard OTBM attribute", attr.Code))
		case codes[attr.Code]:
			warnings = append(warnings, fmt.Sprintf("attribute %d: declared twice", attr.Code))
		default:
			codes[attr.Code] = true
			valid = append(valid, attr)
		}
	}
	return valid, warnings, nil
}

// customSizes gives the payload size of every declared attribute by code
// (see rawSizes).
func customSizes(attrs []CustomAttr) map[byte]int {
	sizes := make(map[byte]int, len(attrs))
	for _, attr := range attrs {
		sizes[attr.Code] = customAttrSizes[attr.Type]
	}
	return sizes
}

// WriteCustomAttrs encodes declarations as an attributes file.
func WriteCustomAttrs(attrs []CustomAttr) ([]byte, error) {
	data, err := json.MarshalIndent(attrs, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
