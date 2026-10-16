package otmap

import (
	"github.com/nekiro/ots-creator/internal/formats/xmltree"
)

// ReadZones applies a zones.xml.
func ReadZones(gameMap *Map, data []byte) error {
	doc, err := xmltree.Parse(data)
	if err != nil {
		return err
	}
	for _, node := range doc.Children {
		if node.Name == "zone" {
			gameMap.Zones = append(gameMap.Zones, Zone{ID: uint16(node.Int("zoneid")), Name: node.Attrs["name"]})
		}
	}
	return nil
}

// WriteZones returns a zones.xml.
func WriteZones(gameMap *Map) []byte {
	w := newXMLWriter()
	w.elem("zones", nil, false)
	for _, zone := range gameMap.Zones {
		w.elem("zone", []xmlAttr{{"name", zone.Name}, {"zoneid", itoa(zone.ID)}}, true)
	}
	w.end("zones")
	return w.buf.Bytes()
}
