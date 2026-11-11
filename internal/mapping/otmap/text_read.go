package otmap

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/nekiro/ots-creator/internal/core/otitems"
)

var ErrNotOTMAP = errors.New("not an OTMAP map")

// cursor reads one line of an OTMAP file.
type cursor struct {
	file string
	line int
	text []byte
	at   int
}

func (c *cursor) errorf(format string, args ...any) error {
	return fmt.Errorf("%s:%d:%d: %s", c.file, c.line, c.at+1, fmt.Sprintf(format, args...))
}

func (c *cursor) end() bool { return c.at >= len(c.text) }

func (c *cursor) peek() byte {
	if c.end() {
		return 0
	}
	return c.text[c.at]
}

func (c *cursor) skipSpace() bool {
	start := c.at
	for !c.end() && (c.text[c.at] == ' ' || c.text[c.at] == '\t') {
		c.at++
	}
	return c.at > start
}

// separator requires spaces before the next token of the line.
func (c *cursor) separator() error {
	if !c.skipSpace() {
		return c.errorf("expected a space")
	}
	return nil
}

func (c *cursor) expect(char byte) error {
	if c.peek() != char {
		return c.errorf("expected %q", char)
	}
	c.at++
	return nil
}

func (c *cursor) accept(char byte) bool {
	if c.peek() == char {
		c.at++
		return true
	}
	return false
}

func (c *cursor) name() (string, error) {
	start := c.at
	for !c.end() {
		char := c.text[c.at]
		if !(char >= 'a' && char <= 'z' || c.at > start && (char >= '0' && char <= '9' || char == '_')) {
			break
		}
		c.at++
	}
	if c.at == start {
		return "", c.errorf("expected a name")
	}
	return string(c.text[start:c.at]), nil
}

func (c *cursor) uint(limit uint64) (uint64, error) {
	start := c.at
	for !c.end() && c.text[c.at] >= '0' && c.text[c.at] <= '9' {
		c.at++
	}
	if c.at == start {
		return 0, c.errorf("expected a number")
	}
	value, err := strconv.ParseUint(string(c.text[start:c.at]), 10, 64)
	if err != nil || value > limit {
		c.at = start
		return 0, c.errorf("number out of range (0-%d)", limit)
	}
	return value, nil
}

func (c *cursor) int(low, high int64) (int64, error) {
	start := c.at
	c.accept('-')
	if _, err := c.uint(math.MaxUint64); err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(string(c.text[start:c.at]), 10, 64)
	if err != nil || value < low || value > high {
		c.at = start
		return 0, c.errorf("number out of range (%d-%d)", low, high)
	}
	return value, nil
}

func (c *cursor) u16() (uint16, error) {
	value, err := c.uint(math.MaxUint16)
	return uint16(value), err
}

func (c *cursor) u32() (uint32, error) {
	value, err := c.uint(math.MaxUint32)
	return uint32(value), err
}

func (c *cursor) position() (Position, error) {
	x, err := c.u16()
	if err != nil {
		return Position{}, err
	}
	if err := c.expect(','); err != nil {
		return Position{}, err
	}
	y, err := c.u16()
	if err != nil {
		return Position{}, err
	}
	if err := c.expect(','); err != nil {
		return Position{}, err
	}
	z, err := c.uint(MaxFloor)
	return Position{x, y, uint8(z)}, err
}

func (c *cursor) coords() (Position, error) {
	x, err := c.u16()
	if err != nil {
		return Position{}, err
	}
	if err := c.separator(); err != nil {
		return Position{}, err
	}
	y, err := c.u16()
	if err != nil {
		return Position{}, err
	}
	if err := c.separator(); err != nil {
		return Position{}, err
	}
	z, err := c.uint(MaxFloor)
	return Position{x, y, uint8(z)}, err
}

func (c *cursor) quoted() (string, error) {
	if err := c.expect('"'); err != nil {
		return "", err
	}
	var out []byte
	for {
		if c.end() {
			return "", c.errorf("unterminated string")
		}
		char := c.text[c.at]
		c.at++
		switch {
		case char == '"':
			return string(out), nil
		case char == '\\':
			escape := c.peek()
			c.at++
			switch escape {
			case '\\', '"':
				out = append(out, escape)
			case 'n':
				out = append(out, '\n')
			case 'x':
				if c.at+2 > len(c.text) {
					return "", c.errorf("broken \\x escape")
				}
				value, err := strconv.ParseUint(string(c.text[c.at:c.at+2]), 16, 8)
				if err != nil {
					return "", c.errorf("broken \\x escape")
				}
				out = append(out, byte(value))
				c.at += 2
			default:
				c.at--
				return "", c.errorf("unknown escape")
			}
		case char < 0x20:
			c.at--
			return "", c.errorf("control character in a string")
		default:
			out = append(out, char)
		}
	}
}

func (c *cursor) finish() error {
	c.skipSpace()
	if !c.end() {
		return c.errorf("unexpected %q", c.peek())
	}
	return nil
}

// eachLine calls fn for every line that is not blank. Lines may end with
// CRLF.
func eachLine(file string, data []byte, fn func(c *cursor) error) error {
	line := 0
	for len(data) > 0 {
		line++
		text := data
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			text, data = data[:i], data[i+1:]
		} else {
			data = nil
		}
		text = bytes.TrimSuffix(text, []byte{'\r'})
		if len(bytes.Trim(text, " \t")) == 0 {
			continue
		}
		if err := fn(&cursor{file: file, line: line, text: text}); err != nil {
			return err
		}
	}
	return nil
}

// worldFile is what the world file says besides the map itself.
type worldFile struct {
	clientIDs bool
	hasSize   bool
	attrs     map[string]CustomAttr
}

func readWorld(path string, data []byte, gameMap *Map) (*worldFile, []string, error) {
	file := filepath.Base(path)
	world := &worldFile{attrs: map[string]CustomAttr{}}
	var warnings []string
	header, hasItems := false, false
	waypointNames := map[string]bool{}
	zoneIDs := map[uint16]bool{}
	attrCodes := map[byte]bool{}
	err := eachLine(file, data, func(c *cursor) error {
		if !header {
			if string(c.text) != worldHeader {
				if bytes.HasPrefix(c.text, []byte("otmap ")) {
					return c.errorf("unsupported version %q", c.text[len("otmap "):])
				}
				return fmt.Errorf("%w: %s has no %q header", ErrNotOTMAP, file, worldHeader)
			}
			header = true
			return nil
		}
		keyword, err := c.name()
		if err != nil {
			return err
		}
		if err := c.separator(); err != nil {
			return err
		}
		switch keyword {
		case "items":
			kind, err := c.name()
			if err != nil {
				return err
			}
			if kind != itemsServer && kind != itemsClient {
				return c.errorf("items must be %q or %q", itemsServer, itemsClient)
			}
			world.clientIDs, hasItems = kind == itemsClient, true
		case "otb":
			major, err := c.u32()
			if err != nil {
				return err
			}
			if err := c.separator(); err != nil {
				return err
			}
			minor, err := c.u32()
			if err != nil {
				return err
			}
			gameMap.ItemsMajor, gameMap.ItemsMinor = major, minor
		case "size":
			width, err := c.u16()
			if err != nil {
				return err
			}
			if err := c.separator(); err != nil {
				return err
			}
			height, err := c.u16()
			if err != nil {
				return err
			}
			gameMap.Width, gameMap.Height, world.hasSize = width, height, true
		case "description":
			text, err := c.quoted()
			if err != nil {
				return err
			}
			if gameMap.Description != "" {
				text = gameMap.Description + "\n" + text
			}
			gameMap.Description = text
		case "spawnformat":
			format, err := c.name()
			if err != nil {
				return err
			}
			switch SpawnFormat(format) {
			case SpawnsSingle, SpawnsSeparate:
				gameMap.SpawnFormat = SpawnFormat(format)
			default:
				return c.errorf("spawnformat must be %q or %q", SpawnsSingle, SpawnsSeparate)
			}
		case "attribute":
			if err := readAttributeDecl(c, world, gameMap, attrCodes); err != nil {
				return err
			}
		case "town":
			if err := readTown(c, gameMap); err != nil {
				return err
			}
		case "house":
			if err := readHouse(c, gameMap); err != nil {
				return err
			}
		case "zone":
			id, err := c.u16()
			if err != nil {
				return err
			}
			if err := c.separator(); err != nil {
				return err
			}
			name, err := c.quoted()
			if err != nil {
				return err
			}
			if zoneIDs[id] {
				c.at = 0
				return c.errorf("zone %d defined twice", id)
			}
			zoneIDs[id] = true
			gameMap.Zones = append(gameMap.Zones, Zone{ID: id, Name: name})
		case "waypoint":
			name, err := c.quoted()
			if err != nil {
				return err
			}
			if err := c.separator(); err != nil {
				return err
			}
			pos, err := c.position()
			if err != nil {
				return err
			}
			if waypointNames[name] {
				warnings = append(warnings, fmt.Sprintf("%s:%d: waypoint %q defined twice", file, c.line, name))
			}
			waypointNames[name] = true
			gameMap.Waypoints = append(gameMap.Waypoints, &Waypoint{Name: name, Pos: pos})
		default:
			c.at = 0
			return c.errorf("unknown record %q", keyword)
		}
		return c.finish()
	})
	if err != nil {
		return nil, nil, err
	}
	if !header {
		return nil, nil, fmt.Errorf("%w: %s is empty", ErrNotOTMAP, file)
	}
	if !hasItems {
		return nil, nil, fmt.Errorf("%s: no items record", file)
	}
	for _, house := range gameMap.Houses {
		if _, ok := gameMap.Towns[house.TownID]; !ok && house.TownID != 0 {
			warnings = append(warnings, fmt.Sprintf("%s: house %d is in town %d, which is not defined", file, house.ID, house.TownID))
		}
	}
	slices.SortFunc(gameMap.Attributes, func(left, right CustomAttr) int { return cmp.Compare(left.Code, right.Code) })
	return world, warnings, nil
}

func readAttributeDecl(c *cursor, world *worldFile, gameMap *Map, codes map[byte]bool) error {
	code, err := c.uint(math.MaxUint8)
	if err != nil {
		return err
	}
	if code <= attrZoneFile || code == attrAttributeMap {
		return c.errorf("attribute %d is a standard OTBM attribute", code)
	}
	if err := c.separator(); err != nil {
		return err
	}
	key, err := c.name()
	if err != nil {
		return err
	}
	if err := c.separator(); err != nil {
		return err
	}
	valueType, err := c.name()
	if err != nil {
		return err
	}
	if _, ok := rawValueSize(valueType); !ok {
		return c.errorf("unknown attribute type %q", valueType)
	}
	name := key
	if c.skipSpace() && !c.end() {
		if name, err = c.quoted(); err != nil {
			return err
		}
	}
	if standardAttrNames[key] {
		return c.errorf("%q is a standard attribute", key)
	}
	if _, ok := world.attrs[key]; ok || codes[byte(code)] {
		c.at = 0
		return c.errorf("attribute %d %q declared twice", code, key)
	}
	attr := CustomAttr{Code: byte(code), Name: name, Type: valueType}
	codes[byte(code)] = true
	world.attrs[key] = attr
	gameMap.Attributes = append(gameMap.Attributes, attr)
	return nil
}

func readTown(c *cursor, gameMap *Map) error {
	id, err := c.u32()
	if err != nil {
		return err
	}
	if err := c.separator(); err != nil {
		return err
	}
	name, err := c.quoted()
	if err != nil {
		return err
	}
	if err := c.separator(); err != nil {
		return err
	}
	if err := expectKey(c, "temple"); err != nil {
		return err
	}
	temple, err := c.position()
	if err != nil {
		return err
	}
	if _, ok := gameMap.Towns[id]; ok {
		c.at = 0
		return c.errorf("town %d defined twice", id)
	}
	gameMap.Towns[id] = &Town{ID: id, Name: name, Temple: temple}
	return nil
}

func expectKey(c *cursor, want string) error {
	start := c.at
	key, err := c.name()
	if err != nil {
		return err
	}
	if key != want {
		c.at = start
		return c.errorf("expected %s=", want)
	}
	return c.expect('=')
}

func readHouse(c *cursor, gameMap *Map) error {
	id, err := c.u32()
	if err != nil {
		return err
	}
	if err := c.separator(); err != nil {
		return err
	}
	name, err := c.quoted()
	if err != nil {
		return err
	}
	house := &House{ID: id, Name: name}
	if err := c.separator(); err != nil {
		return err
	}
	if err := expectKey(c, "town"); err != nil {
		return err
	}
	if house.TownID, err = c.u32(); err != nil {
		return err
	}
	if err := c.separator(); err != nil {
		return err
	}
	if err := expectKey(c, "exit"); err != nil {
		return err
	}
	if house.Exit, err = c.position(); err != nil {
		return err
	}
	for c.skipSpace() && !c.end() {
		key, err := c.name()
		if err != nil {
			return err
		}
		if key == "guildhall" {
			house.Guildhall = true
			continue
		}
		if err := c.expect('='); err != nil {
			return err
		}
		switch key {
		case "rent":
			rent, err := c.int(math.MinInt32, math.MaxInt32)
			if err != nil {
				return err
			}
			house.Rent = int32(rent)
		case "beds":
			beds, err := c.uint(math.MaxInt32)
			if err != nil {
				return err
			}
			house.Beds = int32(beds)
		case "clientid":
			if house.ClientID, err = c.u32(); err != nil {
				return err
			}
		default:
			return c.errorf("unknown house attribute %q", key)
		}
	}
	if _, ok := gameMap.Houses[id]; ok {
		c.at = 0
		return c.errorf("house %d defined twice", id)
	}
	gameMap.Houses[id] = house
	return nil
}

type positionedTile struct {
	pos  Position
	tile *Tile
}

type sectorResult struct {
	tiles    []positionedTile
	warnings []string
	err      error
}

// sectorParser reads one sector file.
type sectorParser struct {
	file             string
	sectorX, sectorY uint16
	attrs            map[string]CustomAttr
	tiles            map[Position]*Tile
	order            []Position
	written          map[Position]bool
	areas            map[Position]uint8
	lastKey          [5]int
	unsorted         bool
}

const (
	areaMonster uint8 = 1 << 0
	areaNPC     uint8 = 1 << 1
)

func readSector(path string, sectorX, sectorY uint16, attrs map[string]CustomAttr) sectorResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return sectorResult{err: err}
	}
	p := &sectorParser{
		file: filepath.Join(sectorsDirName, filepath.Base(path)), sectorX: sectorX, sectorY: sectorY, attrs: attrs,
		tiles: map[Position]*Tile{}, written: map[Position]bool{}, areas: map[Position]uint8{},
	}
	header := false
	err = eachLine(p.file, data, func(c *cursor) error {
		if !header {
			if string(c.text) != sectorHeader {
				return c.errorf("expected the header %q", sectorHeader)
			}
			header = true
			return nil
		}
		if char := c.peek(); char >= '0' && char <= '9' {
			return p.tile(c)
		}
		keyword, err := c.name()
		if err != nil {
			return err
		}
		switch keyword {
		case "spawn", "npcspawn":
			return p.area(c, keyword == "npcspawn")
		case "monster", "npc":
			return p.creature(c, keyword == "npc")
		}
		c.at = 0
		return c.errorf("unknown record %q", keyword)
	})
	if err != nil {
		return sectorResult{err: err}
	}
	result := sectorResult{tiles: make([]positionedTile, 0, len(p.order))}
	if p.unsorted {
		result.warnings = append(result.warnings, fmt.Sprintf("%s: records are not in order", p.file))
	}
	for _, pos := range p.order {
		tile := p.tiles[pos]
		if len(tile.Items) == 0 {
			tile.Items = nil
		}
		tile.Compact()
		result.tiles = append(result.tiles, positionedTile{pos, tile})
	}
	return result
}

// checkOrder notes records that are not in the canonical order.
func (p *sectorParser) checkOrder(section int, pos Position, sub int) {
	key := [5]int{section, int(pos.Z), int(pos.Y), int(pos.X), sub}
	if slices.Compare(key[:], p.lastKey[:]) < 0 {
		p.unsorted = true
	}
	p.lastKey = key
}

func (p *sectorParser) at(c *cursor) (Position, *Tile, error) {
	start := c.at
	pos, err := c.coords()
	if err != nil {
		return pos, nil, err
	}
	if pos.X>>ChunkBits != p.sectorX || pos.Y>>ChunkBits != p.sectorY {
		c.at = start
		return pos, nil, c.errorf("%d,%d,%d is outside sector %d_%d", pos.X, pos.Y, pos.Z, p.sectorX, p.sectorY)
	}
	tile := p.tiles[pos]
	if tile == nil {
		tile = &Tile{}
		p.tiles[pos] = tile
		p.order = append(p.order, pos)
	}
	return pos, tile, nil
}

func (p *sectorParser) tile(c *cursor) error {
	start := c.at
	pos, tile, err := p.at(c)
	if err != nil {
		return err
	}
	if p.written[pos] {
		c.at = start
		return c.errorf("tile %d,%d,%d written twice", pos.X, pos.Y, pos.Z)
	}
	p.written[pos] = true
	p.checkOrder(0, pos, 0)
	if err := c.separator(); err != nil {
		return err
	}
	if c.accept('[') {
		if err := p.tileAttrs(c, tile); err != nil {
			return err
		}
		if err := c.separator(); err != nil {
			return err
		}
	}
	if !c.accept('-') {
		if tile.Ground, err = p.item(c, 0); err != nil {
			return err
		}
	}
	for {
		hadSpace := c.skipSpace()
		if c.end() {
			return nil
		}
		if !hadSpace {
			return c.errorf("expected a space")
		}
		item, err := p.item(c, 0)
		if err != nil {
			return err
		}
		tile.Items = append(tile.Items, item)
	}
}

func (p *sectorParser) tileAttrs(c *cursor, tile *Tile) error {
	for {
		c.skipSpace()
		if c.accept(']') {
			return nil
		}
		key, err := c.name()
		if err != nil {
			return err
		}
		if flag, ok := tileFlag(key); ok {
			tile.Flags |= flag
			continue
		}
		if err := c.expect('='); err != nil {
			return err
		}
		switch key {
		case "flags":
			flags, err := c.u32()
			if err != nil {
				return err
			}
			tile.Flags |= flags
		case "house":
			if tile.HouseID, err = c.u32(); err != nil {
				return err
			}
		case "zone":
			for {
				zone, err := c.u16()
				if err != nil {
					return err
				}
				tile.EnsureExt().Zones = append(tile.EnsureExt().Zones, zone)
				if !c.accept(',') {
					break
				}
			}
		default:
			return c.errorf("unknown tile attribute %q", key)
		}
	}
}

func tileFlag(name string) (uint32, bool) {
	for _, named := range tileFlagNames {
		if named.name == name {
			return named.flag, true
		}
	}
	return 0, false
}

func (p *sectorParser) item(c *cursor, depth int) (Item, error) {
	if depth > maxContentsDepth {
		return Item{}, c.errorf("containers nested deeper than %d", maxContentsDepth)
	}
	id, err := c.uint(math.MaxUint16)
	if err != nil {
		return Item{}, err
	}
	if id == 0 {
		return Item{}, c.errorf("item id 0")
	}
	item := Item{ID: uint16(id)}
	if c.accept(':') {
		if item.Subtype, err = c.u16(); err != nil {
			return Item{}, err
		}
	}
	if c.accept('(') {
		if err := p.itemAttrs(c, &item); err != nil {
			return Item{}, err
		}
	}
	if c.accept('{') {
		for {
			c.skipSpace()
			if c.accept('}') {
				break
			}
			content, err := p.item(c, depth+1)
			if err != nil {
				return Item{}, err
			}
			item.Ensure().Items = append(item.Ensure().Items, content)
		}
	}
	item.Compact()
	return item, nil
}

func (p *sectorParser) itemAttrs(c *cursor, item *Item) error {
	for {
		c.skipSpace()
		if c.accept(')') {
			return nil
		}
		if c.accept('@') {
			if err := readMapAttr(c, item); err != nil {
				return err
			}
			continue
		}
		key, err := c.name()
		if err != nil {
			return err
		}
		if err := c.expect('='); err != nil {
			return err
		}
		if err := p.itemAttr(c, item, key); err != nil {
			return err
		}
	}
}

func (p *sectorParser) itemAttr(c *cursor, item *Item, key string) error {
	ext := item.Ensure()
	var err error
	switch key {
	case "aid":
		ext.ActionID, err = c.u16()
	case "uid":
		ext.UniqueID, err = c.u16()
	case "depot":
		ext.DepotID, err = c.u16()
	case "door":
		var door uint64
		door, err = c.uint(math.MaxUint8)
		ext.DoorID = uint8(door)
	case "dest":
		var dest Position
		dest, err = c.position()
		ext.Dest = &dest
	case "text":
		ext.Text, err = c.quoted()
	case "desc":
		ext.Description, err = c.quoted()
	default:
		code, valueType, ok := byte(0), "", false
		for _, standard := range rawAttrNames {
			if standard.name == key {
				code, valueType, ok = standard.code, standard.valueType, true
			}
		}
		if !ok {
			var attr CustomAttr
			if attr, ok = p.attrs[key]; ok {
				code, valueType = attr.Code, attr.Type
			}
		}
		if !ok {
			return c.errorf("unknown attribute %q", key)
		}
		data, err := readRawValue(c, valueType)
		if err != nil {
			return err
		}
		ext.Raw = append(ext.Raw, RawAttr{Code: code, Data: data})
	}
	return err
}

func readRawValue(c *cursor, valueType string) ([]byte, error) {
	switch valueType {
	case "string":
		text, err := c.quoted()
		if err != nil {
			return nil, err
		}
		if len(text) > math.MaxUint16 {
			return nil, c.errorf("string longer than %d bytes", math.MaxUint16)
		}
		return append(binary.LittleEndian.AppendUint16(nil, uint16(len(text))), text...), nil
	case "i32":
		value, err := c.int(math.MinInt32, math.MaxInt32)
		return binary.LittleEndian.AppendUint32(nil, uint32(int32(value))), err
	case "u8":
		value, err := c.uint(math.MaxUint8)
		return []byte{byte(value)}, err
	case "u16":
		value, err := c.uint(math.MaxUint16)
		return binary.LittleEndian.AppendUint16(nil, uint16(value)), err
	case "u32":
		value, err := c.uint(math.MaxUint32)
		return binary.LittleEndian.AppendUint32(nil, uint32(value)), err
	default: // u64
		value, err := c.uint(math.MaxUint64)
		return binary.LittleEndian.AppendUint64(nil, value), err
	}
}

func readMapAttr(c *cursor, item *Item) error {
	var attr Attr
	var err error
	if c.peek() == '"' {
		attr.Key, err = c.quoted()
	} else {
		attr.Key, err = c.name()
	}
	if err != nil {
		return err
	}
	if err := c.expect('='); err != nil {
		return err
	}
	switch char := c.peek(); {
	case char == '"':
		attr.Type = AttrString
		attr.Str, err = c.quoted()
	case char == 't' || char == 'f':
		start := c.at
		word, _ := c.name()
		if word != "true" && word != "false" {
			c.at = start
			return c.errorf("expected a value")
		}
		attr.Type, attr.Value = AttrBoolean, word == "true"
	default:
		start := c.at
		for !c.end() && bytes.IndexByte([]byte("+-.0123456789eEnaif"), c.text[c.at]) >= 0 {
			c.at++
		}
		token := string(c.text[start:c.at])
		float := len(token) > 1 && token[len(token)-1] == 'f' && token != "inf" && token != "-inf"
		if float {
			token = token[:len(token)-1]
		}
		switch {
		case token == "nan" || token == "inf" || token == "-inf" || bytes.ContainsAny([]byte(token), ".eE"):
			parsed := token
			switch token {
			case "nan":
				parsed = "NaN"
			case "inf":
				parsed = "+Inf"
			case "-inf":
				parsed = "-Inf"
			}
			bitSize := 64
			attr.Type = AttrDouble
			if float {
				bitSize, attr.Type = 32, AttrFloat
			}
			attr.Num, err = strconv.ParseFloat(parsed, bitSize)
		case float:
			err = errors.New("bad number")
		default:
			attr.Type = AttrInteger
			var value int64
			value, err = strconv.ParseInt(token, 10, 32)
			attr.Int = int32(value)
		}
		if err != nil {
			c.at = start
			return c.errorf("bad number %q", token)
		}
	}
	if err != nil {
		return err
	}
	item.Ensure().Attrs = append(item.Ensure().Attrs, attr)
	return nil
}

func (p *sectorParser) area(c *cursor, npc bool) error {
	if err := c.separator(); err != nil {
		return err
	}
	start := c.at
	pos, tile, err := p.at(c)
	if err != nil {
		return err
	}
	kind, sub := areaMonster, 0
	if npc {
		kind, sub = areaNPC, 1
	}
	if p.areas[pos]&kind != 0 {
		c.at = start
		return c.errorf("two spawn areas of one kind at %d,%d,%d", pos.X, pos.Y, pos.Z)
	}
	p.areas[pos] |= kind
	p.checkOrder(1, pos, sub)
	spawn := &Spawn{Radius: 1}
	for c.skipSpace() && !c.end() {
		key, err := c.name()
		if err != nil {
			return err
		}
		if key != "radius" {
			return c.errorf("unknown spawn attribute %q", key)
		}
		if err := c.expect('='); err != nil {
			return err
		}
		radius, err := c.uint(math.MaxInt32)
		if err != nil {
			return err
		}
		spawn.Radius = int32(max(radius, 1))
	}
	if !c.end() {
		return c.errorf("unexpected %q", c.peek())
	}
	if npc {
		tile.EnsureExt().SpawnNPC = spawn
	} else {
		tile.EnsureExt().Spawn = spawn
	}
	return nil
}

func (p *sectorParser) creature(c *cursor, npc bool) error {
	if err := c.separator(); err != nil {
		return err
	}
	pos, tile, err := p.at(c)
	if err != nil {
		return err
	}
	p.checkOrder(2, pos, 0)
	if err := c.separator(); err != nil {
		return err
	}
	creature := Creature{NPC: npc}
	if creature.Name, err = c.quoted(); err != nil {
		return err
	}
	for c.skipSpace() && !c.end() {
		key, err := c.name()
		if err != nil {
			return err
		}
		if err := c.expect('='); err != nil {
			return err
		}
		switch key {
		case "spawntime":
			spawnTime, err := c.int(math.MinInt32, math.MaxInt32)
			if err != nil {
				return err
			}
			creature.SpawnTime = int32(spawnTime)
		case "dir":
			start := c.at
			name, err := c.name()
			if err != nil {
				return err
			}
			direction := slices.Index(directionNames, name)
			if direction < 0 {
				c.at = start
				return c.errorf("direction must be n, e, s or w")
			}
			creature.Direction = uint8(direction)
		case "weight":
			weight, err := c.uint(math.MaxUint8)
			if err != nil {
				return err
			}
			creature.Weight = uint8(weight)
		default:
			return c.errorf("unknown creature attribute %q", key)
		}
	}
	if !c.end() {
		return c.errorf("unexpected %q", c.peek())
	}
	tile.EnsureExt().Creatures = append(tile.EnsureExt().Creatures, creature)
	return nil
}

func loadOTMAP(path string, items Items, progress Progress) (*Map, []string, error) {
	progress(StageRead, 0, 1)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	gameMap := New(0, 0)
	world, warnings, err := readWorld(path, data, gameMap)
	if err != nil {
		return nil, nil, err
	}
	if db, ok := items.(*otitems.DB); ok && world.clientIDs != (db.Mode == otitems.ModeClient) {
		if world.clientIDs {
			warnings = append(warnings, "the map uses client item ids, but the loaded items use server ids")
		} else {
			warnings = append(warnings, "the map uses server item ids, but the loaded items use client ids")
		}
	}
	type sectorFile struct {
		path             string
		sectorX, sectorY uint16
	}
	var files []sectorFile
	sectorsDir := filepath.Join(filepath.Dir(path), sectorsDirName)
	entries, err := os.ReadDir(sectorsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != sectorExt {
			continue
		}
		sectorX, sectorY, ok := parseSectorFileName(name)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("%s: not a sector file name, skipped", name))
			continue
		}
		files = append(files, sectorFile{filepath.Join(sectorsDir, name), sectorX, sectorY})
	}
	// Sectors are parsed on all cores and put into the map here, one at a
	// time: the map is not safe for concurrent use. The channel bounds how
	// many parsed sectors wait.
	workers := max(min(len(files), runtime.GOMAXPROCS(0)), 1)
	results := make(chan sectorResult, workers)
	var next atomic.Int64
	var stop atomic.Bool
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				index := int(next.Add(1) - 1)
				if index >= len(files) {
					return
				}
				file := files[index]
				results <- readSector(file.path, file.sectorX, file.sectorY, world.attrs)
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	var firstErr error
	done := 0
	for result := range results {
		done++
		progress(StageTiles, done, len(files))
		if firstErr != nil {
			continue
		}
		if result.err != nil {
			firstErr = result.err
			stop.Store(true)
			continue
		}
		warnings = append(warnings, result.warnings...)
		for _, placed := range result.tiles {
			gameMap.SetTile(placed.pos, placed.tile)
		}
	}
	if firstErr != nil {
		return nil, nil, firstErr
	}
	progress(StageFiles, 1, 1)
	for _, chunk := range gameMap.chunks {
		chunk.clip()
	}
	warnings = append(warnings, checkReferences(gameMap)...)
	if !world.hasSize {
		if _, _, maxX, maxY, ok := gameMap.Bounds(); ok {
			gameMap.Width, gameMap.Height = uint16(min(maxX+1, math.MaxUint16)), uint16(min(maxY+1, math.MaxUint16))
		}
	}
	gameMap.ClearModified()
	return gameMap, warnings, nil
}

// checkReferences warns about houses and zones that tiles use but the world
// file does not define. Like OTBM, a missing house gets a default record.
func checkReferences(gameMap *Map) []string {
	var warnings []string
	zones := map[uint16]bool{}
	for _, zone := range gameMap.Zones {
		zones[zone.ID] = true
	}
	missingHouses := map[uint32]bool{}
	missingZones := map[uint16]bool{}
	for _, chunk := range gameMap.chunks {
		// House and zone tiles are never packed.
		chunk.eachPtr(func(tile *Tile) {
			if tile.HouseID != 0 && gameMap.Houses[tile.HouseID] == nil {
				missingHouses[tile.HouseID] = true
			}
			for _, zone := range tile.Zones() {
				if !zones[zone] {
					missingZones[zone] = true
				}
			}
		})
	}
	for _, id := range slices.Sorted(maps.Keys(missingHouses)) {
		gameMap.Houses[id] = &House{ID: id, Name: fmt.Sprintf("House #%d", id)}
		warnings = append(warnings, fmt.Sprintf("house %d is used by tiles but not defined", id))
	}
	for _, id := range slices.Sorted(maps.Keys(missingZones)) {
		warnings = append(warnings, fmt.Sprintf("zone %d is used by tiles but not defined", id))
	}
	return warnings
}
