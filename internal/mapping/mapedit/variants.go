package mapedit

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// maxVariants limits the items of a variant family.
const maxVariants = 48

// maxRotations limits the walk of a rotateTo chain.
const maxRotations = 8

// Variants returns the items an item can be swapped for, the item itself
// first: the items of its brush in the same place (ground, wall alignment,
// door state, carpet or table part, doodad singles), its rotations and the
// items with the same name and stack order. The family of a member can
// differ, so callers step through the family of the item they started at.
func (e *Editor) Variants(id uint16) []uint16 {
	itemType := e.itemType(id)
	if itemType == nil {
		return nil
	}
	family := []uint16{id}
	add := func(candidate uint16) {
		if len(family) < maxVariants && candidate != 0 && !slices.Contains(family, candidate) && e.itemType(candidate) != nil {
			family = append(family, candidate)
		}
	}
	for _, candidate := range e.Brushes.siblings(id) {
		add(candidate)
	}
	for next, steps := itemType.RotateTo, 0; next != 0 && next != id && steps < maxRotations; steps++ {
		add(next)
		rotated := e.itemType(next)
		if rotated == nil {
			break
		}
		next = rotated.RotateTo
	}
	name := strings.ToLower(strings.TrimSpace(itemType.Name))
	if name != "" {
		var named []uint16
		e.Items.Each(func(other *otitems.Type) {
			if other.ID != id && other.IsGround() == itemType.IsGround() && other.TopOrder == itemType.TopOrder &&
				!other.Flags.Has(otitems.FlagMeta) && strings.ToLower(strings.TrimSpace(other.Name)) == name {
				named = append(named, other.ID)
			}
		})
		slices.Sort(named)
		for _, candidate := range named {
			add(candidate)
		}
	}
	return family
}

// siblings returns the items that take the place of an item in its brush.
func (b *Brushes) siblings(id uint16) []uint16 {
	info := b.info[id]
	if info == nil {
		return nil
	}
	switch {
	case info.ground != nil:
		return info.ground.items.ids
	case info.wall != nil && info.door:
		var doors []uint16
		for _, door := range info.wall.nodes[info.wallAlign].doors {
			if door.open == info.doorOpen {
				doors = append(doors, door.id)
			}
		}
		return doors
	case info.wall != nil:
		return info.wall.nodes[info.wallAlign].items.ids
	case info.carpet != nil:
		return info.carpet.parts[info.carpetAlign].ids
	case info.table != nil:
		return info.table.parts[info.tableAlign].ids
	case info.doodad != nil:
		var singles []uint16
		for _, alt := range info.doodad.alts {
			for _, single := range alt.singles {
				singles = append(singles, single.item.ID)
			}
		}
		if slices.Contains(singles, id) {
			return singles
		}
	}
	return nil
}

// ItemsOf returns the items a brush places, by brush name and palette kind.
func (b *Brushes) ItemsOf(name, kind string) []uint16 {
	key := lower(name)
	var ids []uint16
	switch kind {
	case "ground":
		if ground := b.grounds[key]; ground != nil {
			ids = append(ids, ground.items.ids...)
		}
	case "wall", "decoration":
		if wall := b.walls[key]; wall != nil {
			for _, node := range wall.nodes {
				ids = append(ids, node.items.ids...)
			}
		}
	case "carpet":
		if carpet := b.carpets[key]; carpet != nil {
			for _, part := range carpet.parts {
				ids = append(ids, part.ids...)
			}
		}
	case "table":
		if table := b.tables[key]; table != nil {
			for _, part := range table.parts {
				ids = append(ids, part.ids...)
			}
		}
	case "doodad":
		if doodad := b.doodads[key]; doodad != nil {
			for id := range doodad.owned {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// IsBorder reports whether an item is a border piece of a ground brush or
// stacks as a border.
func (b *Brushes) IsBorder(id uint16) bool { return b.isBorder(id) }

// SwapItem replaces an item by another one, keeping its attributes.
// Swaps with the same non-zero stroke merge into one undo step.
func (e *Editor) SwapItem(ref ItemRef, id uint16, stroke int64) error {
	if e.itemType(id) == nil {
		return fmt.Errorf("unknown item %d", id)
	}
	current := e.Map.Tile(ref.Pos)
	if current == nil {
		return ErrNoItem
	}
	if _, _, _, err := itemAt(current, ref); err != nil {
		return err
	}
	if ref.Index < 0 && len(ref.Path) == 0 && !e.isGround(id) {
		return errors.New("only a ground can replace the ground")
	}
	tx := e.Begin("Swap item", stroke)
	defer tx.Commit()
	item, _, _, err := itemAt(tx.Edit(ref.Pos), ref)
	if err != nil {
		return err
	}
	item.ID = id
	return nil
}

// mixPrefix names the doodad brushes made from suggestions.
const mixPrefix = "suggested mix "

// mixDensity is the share of tiles (of 100) a mix places objects on.
const mixDensity = 30

// AddMix makes a doodad brush that scatters the given items with their
// chances, for this session only: it is not saved and does not claim its
// items (picking an item from the map still gives its own brush). It
// returns the brush name.
func (b *Brushes) AddMix(chances []materials.Chance) (string, error) {
	alt := &doodadAlt{ownedItems: map[uint16]bool{}}
	for _, chance := range chances {
		if chance.ID == 0 || b.items.Get(chance.ID) == nil || alt.ownedItems[chance.ID] {
			continue
		}
		weight := max(chance.Chance, 1)
		alt.singles = append(alt.singles, doodadSingle{chance: weight, item: materials.ItemSpec{ID: chance.ID}})
		alt.singleTotal += weight
		alt.ownedItems[chance.ID] = true
	}
	if len(alt.singles) == 0 {
		return "", errors.New("the mix has no known items")
	}
	alt.hasSingleObject = true
	b.mixes++
	b.nextID++
	name := fmt.Sprintf("%s%d", mixPrefix, b.mixes)
	b.doodads[lower(name)] = &DoodadBrush{
		id: b.nextID, Name: name, look: alt.singles[0].item.ID, alts: []*doodadAlt{alt}, owned: alt.ownedItems,
		options: materials.DoodadOptions{Draggable: true, Thickness: mixDensity, ThicknessCeiling: 100},
	}
	return name, nil
}

// PlacedItem is an item to put on a tile.
type PlacedItem struct {
	Pos  otmap.Position `json:"pos"`
	Item uint16         `json:"item"`
}

// AddItems puts items on tiles in one undo step; items with the same
// non-zero stroke as the last step join it.
func (e *Editor) AddItems(placed []PlacedItem, stroke int64) error {
	for _, item := range placed {
		if e.itemType(item.Item) == nil {
			return fmt.Errorf("unknown item %d", item.Item)
		}
	}
	tx := e.Begin("Add items", stroke)
	defer tx.Commit()
	for _, item := range placed {
		e.addItem(tx.Edit(item.Pos), e.newItem(item.Item))
	}
	return nil
}
