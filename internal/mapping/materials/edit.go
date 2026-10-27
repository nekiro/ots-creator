package materials

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (m *Materials) SaveBorder(border Border) uint32 {
	if border.ID == 0 {
		border.ID = m.NextBorderID()
	}
	if current := m.Border(border.ID); current != nil {
		*current = border
	} else {
		m.Borders = append(m.Borders, &border)
	}
	return border.ID
}

// SaveBrush creates or replaces a brush; oldName is its current name (""
// for a new brush). tilesets, when not nil, are the collections the brush is
// in afterwards: missing ones are created, others no longer list it.
func (m *Materials) SaveBrush(oldName string, brush Brush, tilesets []string) error {
	brush.Name = strings.TrimSpace(brush.Name)
	if brush.Name == "" {
		return errors.New("the brush needs a name")
	}
	renamed := !strings.EqualFold(oldName, brush.Name)
	if renamed && m.Brush(brush.Name) != nil {
		return fmt.Errorf("a brush named %q exists", brush.Name)
	}
	if oldName != "" && m.Brush(oldName) != nil {
		if renamed {
			m.RenameBrush(oldName, brush.Name)
		}
		*m.Brush(brush.Name) = brush
	} else {
		m.Brushes = append(m.Brushes, &brush)
	}
	if tilesets != nil {
		m.setBrushTilesets(&brush, tilesets)
	}
	return nil
}

func (m *Materials) setBrushTilesets(brush *Brush, tilesets []string) {
	isBrush := func(name string) bool { return strings.EqualFold(name, brush.Name) }
	wanted := map[string]bool{}
	for _, name := range tilesets {
		if name = strings.TrimSpace(name); name != "" {
			wanted[strings.ToLower(name)] = true
			if m.Tileset(name) == nil {
				m.Tilesets = append(m.Tilesets, &Tileset{Name: name})
			}
		}
	}
	for _, tileset := range m.Tilesets {
		tileset.Terrain = slices.DeleteFunc(tileset.Terrain, isBrush)
		tileset.Doodads = slices.DeleteFunc(tileset.Doodads, isBrush)
		if !wanted[strings.ToLower(tileset.Name)] {
			continue
		}
		if brush.Type == TypeDoodad {
			tileset.Doodads = append(tileset.Doodads, brush.Name)
		} else {
			tileset.Terrain = append(tileset.Terrain, brush.Name)
		}
	}
}

func (m *Materials) SaveTileset(oldName string, tileset Tileset) error {
	tileset.Name = strings.TrimSpace(tileset.Name)
	if tileset.Name == "" {
		return errors.New("the tileset needs a name")
	}
	tileset.Auto = false
	if current := m.Tileset(oldName); oldName != "" && current != nil {
		*current = tileset
	} else if m.Tileset(tileset.Name) != nil {
		return fmt.Errorf("a tileset named %q exists", tileset.Name)
	} else {
		m.Tilesets = append(m.Tilesets, &tileset)
	}
	return nil
}

// RemoveTileset deletes a tileset; its brushes stay.
func (m *Materials) RemoveTileset(name string) {
	m.Tilesets = slices.DeleteFunc(m.Tilesets, func(tileset *Tileset) bool { return strings.EqualFold(tileset.Name, name) })
}

func (m *Materials) AddRawItems(tilesetName string, items []uint16) {
	tileset := m.Tileset(tilesetName)
	if tileset == nil {
		tileset = &Tileset{Name: tilesetName}
		m.Tilesets = append(m.Tilesets, tileset)
	}
	tileset.Raw = appendMissingIDs(tileset.Raw, items)
}

func (m *Materials) SaveCreature(creature CreatureType) error {
	if strings.TrimSpace(creature.Name) == "" {
		return errors.New("the creature needs a name")
	}
	if current := m.Creature(creature.Name); current != nil {
		*current = creature
	} else {
		m.Creatures = append(m.Creatures, &creature)
	}
	return nil
}
