package materials

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/formats/xmltree"

	"github.com/nekiro/ots-creator/internal/core/otitems"
)

const (
	// maxScanFiles limits how many files a creature scan reads.
	maxScanFiles = 50000
	// maxScanFileSize skips bigger files in a creature scan.
	maxScanFileSize = 2 << 20
	// xmlSniffBytes is how much of an XML file is checked for a creature
	// root before parsing it.
	xmlSniffBytes = 512
)

var (
	reMonsterType = regexp.MustCompile(`Game\.createMonsterType\(\s*"([^"]+)"`)
	reNpcType     = regexp.MustCompile(`Game\.createNpcType\(\s*"([^"]+)"`)
	reNpcVar      = regexp.MustCompile(`(?:internalNpcName|npcName)\s*=\s*"([^"]+)"`)
	reNpcConfig   = regexp.MustCompile(`npcConfig\.name\s*=\s*"([^"]+)"`)
	reOutfit      = regexp.MustCompile(`(?s)\.outfit\s*=\s*\{(.*?)\}`)
	reKeyValue    = regexp.MustCompile(`(\w+)\s*=\s*(\d+)`)
)

// ScanCreatures finds monsters and NPCs in a server data folder: TFS XML
// (monster/*.xml, npc/*.xml) and Lua scripts (revscripts). A registry
// (monsters.xml, npcs.xml) names its creatures itself, as the server does:
// a file may hold another name than the one the map spawns.
func ScanCreatures(dir string) ([]*CreatureType, error) {
	var out []*CreatureType
	seen := map[string]bool{}
	add := func(creature *CreatureType) {
		key := strings.ToLower(creature.Name)
		if creature.Name == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, creature)
	}
	// xmlFiles holds the creature of each xml file by its cleaned lower case
	// path, for the registries resolved after the walk.
	xmlFiles := map[string]*CreatureType{}
	var registries []string
	fileCount := 0
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if path != dir && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "world" || name == "logs") {
				return filepath.SkipDir
			}
			return nil
		}
		fileCount++
		if fileCount > maxScanFiles {
			return filepath.SkipAll
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".xml" && ext != ".lua" {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > maxScanFileSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if ext == ".xml" {
			if isCreatureRegistry(data) {
				registries = append(registries, path)
			} else if creature := xmlCreature(data); creature != nil {
				xmlFiles[strings.ToLower(filepath.Clean(path))] = creature
				add(creature)
			}
			return nil
		}
		for _, creature := range luaCreatures(string(data)) {
			add(creature)
		}
		return nil
	})
	for _, registry := range registries {
		for _, creature := range registryCreatures(registry, xmlFiles) {
			add(creature)
		}
	}
	slices.SortFunc(out, func(left, right *CreatureType) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return out, err
}

// isCreatureRegistry reports whether data is a TFS monsters.xml or npcs.xml.
func isCreatureRegistry(data []byte) bool {
	head := strings.ToLower(string(data[:min(len(data), xmlSniffBytes)]))
	return strings.Contains(head, "<monsters") || strings.Contains(head, "<npcs")
}

// registryCreatures lists the entries of a registry under their registered
// names, with the outfit of the file each one points to.
func registryCreatures(path string, xmlFiles map[string]*CreatureType) []*CreatureType {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	root, err := xmltree.Parse(data)
	if err != nil || (root.Name != "monsters" && root.Name != "npcs") {
		return nil
	}
	var out []*CreatureType
	for _, node := range root.Children {
		name, file := node.Str("name"), node.Str("file")
		if name == "" || file == "" {
			continue
		}
		creature := xmlFiles[strings.ToLower(filepath.Clean(filepath.Join(filepath.Dir(path), file)))]
		if creature == nil || strings.EqualFold(creature.Name, name) {
			continue
		}
		registered := *creature
		registered.Name = name
		out = append(out, &registered)
	}
	return out
}

func xmlCreature(data []byte) *CreatureType {
	// Cheap check before parsing every xml file of a server.
	head := strings.ToLower(string(data[:min(len(data), xmlSniffBytes)]))
	if !strings.Contains(head, "<monster") && !strings.Contains(head, "<npc") {
		return nil
	}
	root, err := xmltree.Parse(data)
	if err != nil || (root.Name != "monster" && root.Name != "npc") || root.Str("name") == "" {
		return nil
	}
	creature := &CreatureType{Name: root.Str("name"), NPC: root.Name == "npc"}
	for _, node := range root.Children {
		if node.Name != "look" {
			continue
		}
		creature.Outfit = Outfit{
			LookType: node.U16("type"), Head: uint8(node.Int("head")), Body: uint8(node.Int("body")),
			Legs: uint8(node.Int("legs")), Feet: uint8(node.Int("feet")), Addons: uint8(node.Int("addons")),
			Mount: node.U16("mount"), LookItem: node.U16("typeex"),
		}
	}
	return creature
}

func luaCreatures(script string) []*CreatureType {
	var out []*CreatureType
	outfit := func() Outfit {
		var result Outfit
		match := reOutfit.FindStringSubmatch(script)
		if match == nil {
			return result
		}
		for _, pair := range reKeyValue.FindAllStringSubmatch(match[1], -1) {
			value, _ := strconv.Atoi(pair[2])
			switch strings.ToLower(pair[1]) {
			case "looktype":
				result.LookType = uint16(value)
			case "lookhead":
				result.Head = uint8(value)
			case "lookbody":
				result.Body = uint8(value)
			case "looklegs":
				result.Legs = uint8(value)
			case "lookfeet":
				result.Feet = uint8(value)
			case "lookaddons":
				result.Addons = uint8(value)
			case "lookmount":
				result.Mount = uint16(value)
			case "looktypeex":
				result.LookItem = uint16(value)
			}
		}
		return result
	}
	if match := reMonsterType.FindStringSubmatch(script); match != nil {
		out = append(out, &CreatureType{Name: match[1], Outfit: outfit()})
		return out
	}
	name := ""
	for _, pattern := range []*regexp.Regexp{reNpcType, reNpcVar, reNpcConfig} {
		if match := pattern.FindStringSubmatch(script); match != nil {
			name = match[1]
			break
		}
	}
	if name != "" && (strings.Contains(script, "createNpcType") || strings.Contains(script, "npcConfig")) {
		out = append(out, &CreatureType{Name: name, NPC: true, Outfit: outfit()})
	}
	return out
}

// AutoTilesets groups every item by its flags, so a client without brush
// definitions still has an organized palette.
func AutoTilesets(db *otitems.DB) []*Tileset {
	groups := []struct {
		name string
		test func(itemType *otitems.Type) bool
	}{
		{"Grounds", func(itemType *otitems.Type) bool { return itemType.IsGround() }},
		{"Borders", func(itemType *otitems.Type) bool { return itemType.TopOrder == 1 }},
		{"Walls", func(itemType *otitems.Type) bool { return itemType.TopOrder == 2 && !itemType.IsDoor() }},
		{"Doors", func(itemType *otitems.Type) bool { return itemType.IsDoor() }},
		{"Stairs & holes", func(itemType *otitems.Type) bool {
			return itemType.Flags.Has(otitems.FlagFloorChange) || itemType.IsTeleport()
		}},
		{"Containers", func(itemType *otitems.Type) bool { return itemType.IsContainer() }},
		{"Fluids & splashes", func(itemType *otitems.Type) bool {
			return itemType.Flags&(otitems.FlagSplash|otitems.FlagFluidContainer) != 0
		}},
		{"Top items", func(itemType *otitems.Type) bool { return itemType.TopOrder == 3 }},
		{"Light sources", func(itemType *otitems.Type) bool {
			return itemType.Flags.Has(otitems.FlagLight) && !itemType.Flags.Has(otitems.FlagPickupable)
		}},
		{"Blocking decoration", func(itemType *otitems.Type) bool {
			return itemType.Flags.Has(otitems.FlagUnpassable) && !itemType.Flags.Has(otitems.FlagPickupable)
		}},
		{"Decoration", func(itemType *otitems.Type) bool { return !itemType.Flags.Has(otitems.FlagPickupable) }},
		{"Pickupable items", func(itemType *otitems.Type) bool { return itemType.Flags.Has(otitems.FlagPickupable) }},
	}
	sets := make([]*Tileset, len(groups))
	for i, group := range groups {
		sets[i] = &Tileset{Name: group.name, Auto: true}
	}
	db.Each(func(itemType *otitems.Type) {
		if itemType.Flags.Has(otitems.FlagMeta) || itemType.ClientID == 0 {
			return
		}
		for i, group := range groups {
			if group.test(itemType) {
				sets[i].Raw = append(sets[i].Raw, itemType.ID)
				return
			}
		}
	})
	return slices.DeleteFunc(sets, func(tileset *Tileset) bool { return len(tileset.Raw) == 0 })
}
