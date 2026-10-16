package otmap

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/formats/xmltree"
)

// ReadHouses applies a houses.xml to the map.
func ReadHouses(m *Map, data []byte) ([]string, error) {
	doc, err := xmltree.Parse(data)
	if err != nil {
		return nil, err
	}
	var warnings []string
	for _, node := range doc.Children {
		if node.Name != "house" {
			continue
		}
		id := uint32(node.Int("houseid"))
		if id == 0 {
			warnings = append(warnings, "house without an id discarded")
			continue
		}
		house := m.Houses[id]
		if house == nil {
			house = &House{ID: id}
			m.Houses[id] = house
		}
		house.Name = node.Attrs["name"]
		if house.Name == "" {
			house.Name = fmt.Sprintf("House #%d", id)
		}
		house.Exit = Position{uint16(node.Int("entryx")), uint16(node.Int("entryy")), uint8(node.Int("entryz"))}
		house.Rent = int32(node.Int("rent"))
		house.TownID = uint32(node.Int("townid"))
		house.Guildhall = strings.EqualFold(node.Attrs["guildhall"], "true") || node.Attrs["guildhall"] == "1"
		house.ClientID = uint32(node.Int("clientid"))
		house.Beds = int32(node.Int("beds"))
	}
	return warnings, nil
}

// HouseSizes counts the tiles of every house.
func HouseSizes(m *Map) map[uint32]int {
	sizes := map[uint32]int{}
	for _, chunk := range m.chunks {
		// House tiles are never packed.
		chunk.eachPtr(func(tile *Tile) {
			if tile.HouseID != 0 {
				sizes[tile.HouseID]++
			}
		})
	}
	return sizes
}

// WriteHouses returns a houses.xml.
func WriteHouses(m *Map) []byte {
	w := newXMLWriter()
	w.elem("houses", nil, false)
	sizes := HouseSizes(m)
	for _, house := range m.SortedHouses() {
		attrs := []xmlAttr{
			{"name", house.Name}, {"houseid", itoa(house.ID)},
			{"entryx", itoa(house.Exit.X)}, {"entryy", itoa(house.Exit.Y)}, {"entryz", itoa(house.Exit.Z)},
			{"rent", itoa(house.Rent)},
		}
		if house.Guildhall {
			attrs = append(attrs, xmlAttr{"guildhall", "true"})
		}
		attrs = append(attrs, xmlAttr{"townid", itoa(house.TownID)}, xmlAttr{"size", strconv.Itoa(sizes[house.ID])})
		if house.ClientID != 0 {
			attrs = append(attrs, xmlAttr{"clientid", itoa(house.ClientID)})
		}
		if house.Beds != 0 {
			attrs = append(attrs, xmlAttr{"beds", itoa(house.Beds)})
		}
		w.elem("house", attrs, true)
	}
	w.end("houses")
	return w.buf.Bytes()
}
