package materials

import (
	"strings"

	"github.com/nekiro/ots-creator/internal/xmltree"
)

func (im *importer) tileset(node *xmltree.Node) {
	name := node.Str("name")
	if name == "" {
		return
	}
	tileset := im.materials.Tileset(name)
	if tileset == nil {
		tileset = &Tileset{Name: name}
		im.materials.Tilesets = append(im.materials.Tilesets, tileset)
	}
	for _, category := range node.Children {
		raw := strings.HasSuffix(category.Name, "_and_raw") || category.Name == "raw"
		base := strings.TrimSuffix(category.Name, "_and_raw")
		for _, child := range category.Children {
			switch child.Name {
			case "brush":
				brushName := child.Str("name")
				if brushName == "" {
					continue
				}
				if base == "doodad" {
					tileset.Doodads = appendMissing(tileset.Doodads, []string{brushName})
				} else {
					tileset.Terrain = appendMissing(tileset.Terrain, []string{brushName})
				}
			case "item":
				from, to := idRange(child)
				for id := from; id <= to && id != 0; id++ {
					switch {
					case base == "items" || base == "collections":
						tileset.Items = appendMissingIDs(tileset.Items, []uint16{uint16(id)})
						if raw {
							tileset.Raw = appendMissingIDs(tileset.Raw, []uint16{uint16(id)})
						}
					default:
						tileset.Raw = appendMissingIDs(tileset.Raw, []uint16{uint16(id)})
					}
				}
			}
		}
	}
}
