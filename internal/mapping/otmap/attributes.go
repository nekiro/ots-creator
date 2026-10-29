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

var customAttrSizes = map[string]int{
	"u8": 1, "u16": 2, "u32": 4, "i32": 4, "u64": 8, "string": -1,
}

// ReadCustomAttrs parses an attributes file into payload sizes by code (see
// rawSizes). Bad entries are skipped with a warning.
func ReadCustomAttrs(data []byte) (map[byte]int, []string, error) {
	var attrs []CustomAttr
	if err := json.Unmarshal(data, &attrs); err != nil {
		return nil, nil, err
	}
	sizes := map[byte]int{}
	var warnings []string
	for _, attr := range attrs {
		size, ok := customAttrSizes[attr.Type]
		switch {
		case !ok:
			warnings = append(warnings, fmt.Sprintf("attribute %d: unknown type %q", attr.Code, attr.Type))
		case attr.Code <= attrZoneFile || attr.Code == attrAttributeMap:
			warnings = append(warnings, fmt.Sprintf("attribute %d: the code is a standard OTBM attribute", attr.Code))
		default:
			sizes[attr.Code] = size
		}
	}
	return sizes, warnings, nil
}
