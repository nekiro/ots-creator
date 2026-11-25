package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nekiro/ots-creator/internal/fsutil"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/itemsxml"
)

// An items.xml can be attached to a client. Its ids are the ids of the
// server: server ids when an items.otb is attached too (it maps them to the
// items of the client), else client ids. The items show its data and can
// edit it; compiling writes the edits, an entry for every new item and, on
// client ids, moves the entries of moved items and writes the attributes the
// attribute map gives new and changed items. Entries that do not change
// keep their bytes.

type itemsXML struct {
	path string
	file *itemsxml.File
	// stamp tells when the file changed on disk: it is read again then.
	stamp fileStamp
	// mapping is the attribute map (see XMLMapping), kept in a file next
	// to the items.xml.
	mapping      []XMLMapping
	mappingStamp fileStamp
}

type fileStamp struct {
	modified time.Time
	size     int64
}

func stampOf(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{modified: info.ModTime(), size: info.Size()}
}

// XMLMapping writes a property of the client item to items.xml as an
// attribute, for servers on client ids that read from items.xml what an
// items.otb would tell them. Client is the JSON name of the property
// ("unpassable", "groundSpeed", "market.name"), Key the attribute key. A
// flag that is set writes Value ("1" when empty); a number or text writes
// itself unless Value is given. A flag that is off, 0 and "" write nothing.
// Not turns it around for servers whose attribute is the opposite of the
// client flag ("moveable" of "unmoveable"): Value ("1" when empty) is
// written while the property is off, 0 or "". Aliases are other keys the
// server reads as Key ("blocking" of "blocksolid"): an entry that has one
// keeps it instead of getting Key next to it.
type XMLMapping struct {
	Client  string   `json:"client"`
	Key     string   `json:"key"`
	Value   string   `json:"value,omitempty"`
	Not     bool     `json:"not,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

type xmlMappingFile struct {
	Attributes []XMLMapping `json:"attributes"`
}

// xmlMappingPath is the attribute map of an items.xml: items.attributes.json
// next to items.xml.
func xmlMappingPath(xmlPath string) string {
	return fsutil.TrimExt(xmlPath) + ".attributes.json"
}

type XMLInfo struct {
	Path string `json:"path"`
	// Items is the number of item ids the file describes.
	Items int `json:"items"`
	// ServerIDs tells that the ids of the file are server ids, translated
	// with the attached items.otb; they are client ids without one.
	ServerIDs   bool         `json:"serverIds"`
	MappingPath string       `json:"mappingPath"`
	Mapping     []XMLMapping `json:"mapping"`
}

type XMLReport struct {
	// Edited counts entries edited in the editor, Added entries of new
	// items, Updated entries whose mapped attributes follow changed client
	// items and Moved entries that follow moved client items.
	Edited  int `json:"edited"`
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Moved   int `json:"moved"`
	// Skipped counts edited entries of items the items.otb gives no
	// server id.
	Skipped int  `json:"skipped"`
	Written bool `json:"written"`
	// Error is set when the file could not be updated (the client files
	// were still written).
	Error string `json:"error,omitempty"`
}

func (r XMLReport) Changed() bool { return r.Edited+r.Added+r.Updated+r.Moved > 0 }

// XMLItem is the items.xml entry of a client item.
type XMLItem struct {
	// ID is the id of the item in items.xml once compiled; 0 when the
	// items.otb gives the item no server id.
	ID uint16 `json:"id"`
	// Exists tells that the file on disk has the entry.
	Exists bool          `json:"exists"`
	Item   itemsxml.Item `json:"item"`
}

func readItemsXML(path string) (*itemsxml.File, fileStamp, error) {
	stamp := stampOf(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, stamp, err
	}
	file, err := itemsxml.Parse(data)
	return file, stamp, err
}

// AttachXML reads an items.xml and shows and edits it with the client from
// now on. It replaces an attached one, with the edits made to it.
func (p *Project) AttachXML(path string) (XMLInfo, error) {
	file, stamp, err := readItemsXML(path)
	if err != nil {
		return XMLInfo{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.xml = &itemsXML{path: path, file: file, stamp: stamp}
	p.xmlEdits = nil
	p.xml.loadMapping()
	return p.xmlInfo(), nil
}

func (p *Project) DetachXML() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.xml, p.xmlEdits = nil, nil
}

func (p *Project) XML() *XMLInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.xml == nil {
		return nil
	}
	info := p.xmlInfo()
	return &info
}

func (p *Project) xmlInfo() XMLInfo {
	mapping := append([]XMLMapping{}, p.xml.mapping...)
	return XMLInfo{Path: p.xml.path, Items: p.xml.file.Items(), ServerIDs: p.otb != nil, MappingPath: xmlMappingPath(p.xml.path), Mapping: mapping}
}

var errNoXML = errors.New("no items.xml is attached")

// loadMapping reads the attribute map; a missing or broken file is no map.
func (x *itemsXML) loadMapping() {
	path := xmlMappingPath(x.path)
	x.mappingStamp = stampOf(path)
	x.mapping = nil
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var stored xmlMappingFile
	if json.Unmarshal(data, &stored) == nil {
		x.mapping = cleanMapping(stored.Attributes)
	}
}

func cleanMapping(mapping []XMLMapping) []XMLMapping {
	out := []XMLMapping{}
	for _, row := range mapping {
		row.Client, row.Key = strings.TrimSpace(row.Client), strings.TrimSpace(row.Key)
		var aliases []string
		for _, alias := range row.Aliases {
			if alias = strings.TrimSpace(alias); alias != "" && !strings.EqualFold(alias, row.Key) {
				aliases = append(aliases, alias)
			}
		}
		row.Aliases = aliases
		if row.Client != "" && row.Key != "" {
			out = append(out, row)
		}
	}
	return out
}

// refreshXML reads the items.xml and its attribute map again when they
// changed on disk (servers are edited in text editors meanwhile). A file
// that can no longer be read stays as it was read last.
func (p *Project) refreshXML() {
	if p.xml == nil {
		return
	}
	if stamp := stampOf(p.xml.path); stamp != p.xml.stamp {
		if file, stamp, err := readItemsXML(p.xml.path); err == nil {
			p.xml.file, p.xml.stamp = file, stamp
		}
	}
	if stampOf(xmlMappingPath(p.xml.path)) != p.xml.mappingStamp {
		p.xml.loadMapping()
	}
}

// SetXMLMapping stores the attribute map next to the items.xml; an empty
// map removes the file.
func (p *Project) SetXMLMapping(mapping []XMLMapping) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.xml == nil {
		return errNoXML
	}
	mapping = cleanMapping(mapping)
	for _, row := range mapping {
		if _, ok := propertyValue(&thing.Properties{}, row.Client); !ok {
			return fmt.Errorf("unknown client property %q", row.Client)
		}
	}
	path := xmlMappingPath(p.xml.path)
	if len(mapping) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else {
		data, err := json.MarshalIndent(xmlMappingFile{Attributes: mapping}, "", "\t")
		if err != nil {
			return err
		}
		if err := fsutil.WriteAtomic(path, append(data, '\n')); err != nil {
			return err
		}
	}
	p.xml.mapping, p.xml.mappingStamp = mapping, stampOf(path)
	return nil
}

// propertyValue reads a property of a client item by its JSON name, a path
// for nested ones ("market.name"). ok is false for an unknown name.
func propertyValue(props *thing.Properties, path string) (value reflect.Value, ok bool) {
	value = reflect.ValueOf(props).Elem()
	for _, name := range strings.Split(path, ".") {
		if value.Kind() != reflect.Struct {
			return value, false
		}
		index, found := jsonFields(value.Type())[name]
		if !found {
			return value, false
		}
		value = value.Field(index)
	}
	switch value.Kind() {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value, true
	}
	return value, false
}

var jsonFieldCache = map[reflect.Type]map[string]int{}

func init() {
	// Filled once: the lookups under the project lock only read.
	var fill func(kind reflect.Type)
	fill = func(kind reflect.Type) {
		fields := map[string]int{}
		jsonFieldCache[kind] = fields
		for i := 0; i < kind.NumField(); i++ {
			field := kind.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			fields[name] = i
			if field.Type.Kind() == reflect.Struct {
				fill(field.Type)
			}
		}
	}
	fill(reflect.TypeOf(thing.Properties{}))
}

func jsonFields(kind reflect.Type) map[string]int { return jsonFieldCache[kind] }

// mappedAttributes returns what the attribute map writes for a client item:
// attribute keys (lower case) to values, "" with false for the mapped
// attributes the item does not have.
func mappedAttributes(mapping []XMLMapping, object *thing.Thing) map[string]mappedValue {
	out := make(map[string]mappedValue, len(mapping))
	for _, row := range mapping {
		written := mappedValue{key: row.Key}
		if object != nil {
			if value, ok := propertyValue(&object.Props, row.Client); ok && value.IsZero() == row.Not {
				written.set = true
				switch {
				case row.Value != "":
					written.value = row.Value
				case row.Not || value.Kind() == reflect.Bool:
					written.value = "1"
				case value.Kind() == reflect.String:
					written.value = value.String()
				case value.CanInt():
					written.value = strconv.FormatInt(value.Int(), 10)
				default:
					written.value = strconv.FormatUint(value.Uint(), 10)
				}
			}
		}
		// Several properties can write one key: the first that is set does,
		// and the key has the aliases of all of them.
		existing, ok := out[strings.ToLower(row.Key)]
		written.aliases = append(existing.aliases, row.Aliases...)
		if ok && existing.set {
			existing.aliases = written.aliases
			written = existing
		}
		out[strings.ToLower(row.Key)] = written
	}
	return out
}

type mappedValue struct {
	key, value string
	set        bool
	aliases    []string
}

func (written mappedValue) names(key string) bool {
	return strings.EqualFold(key, written.key) || slices.ContainsFunc(written.aliases, func(alias string) bool { return strings.EqualFold(alias, key) })
}

// sameXMLValue tells whether a server reads two attribute values the same:
// "1", "true" and "yes" are one value of a flag.
func sameXMLValue(a, b string) bool {
	truthy := func(text string) bool {
		switch strings.ToLower(text) {
		case "1", "true", "yes":
			return true
		}
		return false
	}
	return a == b || (truthy(a) && truthy(b))
}

// apply makes the entry say what the map writes: the attribute with the key
// or an alias of it gets the value (one that reads the same is kept as it
// is written), or is removed when the item does not have it.
func (written mappedValue) apply(item *itemsxml.Item) {
	if !written.set {
		item.Attributes = slices.DeleteFunc(item.Attributes, func(attribute itemsxml.Attribute) bool { return written.names(attribute.Key) })
		return
	}
	for i := range item.Attributes {
		if written.names(item.Attributes[i].Key) {
			if !sameXMLValue(item.Attributes[i].Value, written.value) {
				item.Attributes[i].Value = written.value
			}
			return
		}
	}
	item.Attributes = append(item.Attributes, itemsxml.Attribute{Key: written.key, Value: written.value})
}

// diskID returns the id on disk of the item that is at a client id now.
func (p *Project) diskID(id uint32) uint32 {
	for from, to := range p.moves {
		if to == id {
			return from
		}
	}
	return id
}

// baseItem returns the item with an id on disk, nil when the files on disk
// have none there.
func (p *Project) baseItem(diskID uint32) *thing.Thing {
	i := int(diskID) - int(thing.CategoryItem.MinID())
	if i < 0 || i >= len(p.base) {
		return nil
	}
	return p.base[i]
}

// xmlDiskEntry returns the entry the file on disk has for the item with a
// client id on disk, and its id in the file (0: the items.otb does not map
// the item).
func (p *Project) xmlDiskEntry(diskID uint32) (*itemsxml.Item, uint16) {
	if diskID > maxServerID {
		return nil, 0
	}
	id := uint16(diskID)
	if p.otb != nil {
		id = p.otb.servers[uint16(diskID)]
	}
	if id == 0 {
		return nil, 0
	}
	item, _ := p.xml.file.Get(id)
	return item, id
}

// ItemXML returns the items.xml entry of a client item, with the edits made
// to it.
func (p *Project) ItemXML(clientID uint32) (XMLItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.xml == nil {
		return XMLItem{}, errNoXML
	}
	if p.things.Get(thing.CategoryItem, clientID) == nil {
		return XMLItem{}, fmt.Errorf("item %d does not exist", clientID)
	}
	p.refreshXML()
	disk := p.diskID(clientID)
	onDisk, id := p.xmlDiskEntry(disk)
	view := XMLItem{ID: id, Exists: onDisk != nil}
	switch {
	case p.otb == nil:
		view.ID = 0
		if clientID <= maxServerID {
			view.ID = uint16(clientID)
		}
	case id == 0:
		// A new item: the server id compiling will give it.
		if synced, _, err := p.syncOTB(); err == nil && clientID <= maxServerID {
			view.ID = serverIDs(synced.Items)[uint16(clientID)]
		}
	}
	if edited := p.xmlEdits[disk]; edited != nil {
		onDisk = edited
	}
	if onDisk != nil {
		view.Item = *onDisk.Clone()
	}
	normalizeXMLItem(&view.Item)
	return view, nil
}

// normalizeXMLItem gives the lists of an item to the UI as lists, never
// null.
func normalizeXMLItem(item *itemsxml.Item) {
	if item.Fields == nil {
		item.Fields = []itemsxml.Field{}
	}
	item.Attributes = normalizeXMLAttributes(item.Attributes)
}

func normalizeXMLAttributes(attributes []itemsxml.Attribute) []itemsxml.Attribute {
	out := make([]itemsxml.Attribute, 0, len(attributes))
	for _, attribute := range attributes {
		attribute.Key = strings.TrimSpace(attribute.Key)
		if attribute.Key == "" {
			continue
		}
		attribute.Children = normalizeXMLAttributes(attribute.Children)
		out = append(out, attribute)
	}
	return out
}

// SetItemXML edits the items.xml entry of a client item. The file is
// written when the client is compiled.
func (p *Project) SetItemXML(clientID uint32, item itemsxml.Item) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.xml == nil {
		return errNoXML
	}
	if p.things.Get(thing.CategoryItem, clientID) == nil {
		return fmt.Errorf("item %d does not exist", clientID)
	}
	p.refreshXML()
	edit := p.record(fmt.Sprintf("Edit items.xml of item %d", clientID))
	if p.editXML(edit, clientID, &item) {
		edit.commit()
	}
	return nil
}

// editXML makes an entry the edit of an item and records it; false when the
// item has that entry already. The caller holds the lock.
func (p *Project) editXML(edit *recorder, clientID uint32, item *itemsxml.Item) bool {
	edited := item.Clone()
	normalizeXMLItem(edited)
	disk := p.diskID(clientID)
	onDisk, _ := p.xmlDiskEntry(disk)
	if onDisk == nil {
		onDisk = &itemsxml.Item{}
	}
	normalizeXMLItem(onDisk)
	if edited.Equal(onDisk) {
		edited = nil // as the file has it: nothing to write
	}
	before := p.xmlEdits[disk]
	if before == edited || (before != nil && edited != nil && before.Equal(edited)) {
		return false
	}
	edit.entry.xml = append(edit.entry.xml, xmlChange{id: disk, before: before, after: edited})
	p.setXMLEdit(disk, edited)
	return true
}

// PasteXMLAttributes gives the attributes copied from one entry to the
// entries of other items, as one undo step, and returns how many entries
// changed. The names of the entries stay. On client ids the attributes of the
// attribute map stay as well: the client says those for every item.
func (p *Project) PasteXMLAttributes(clientIDs []uint32, attributes []itemsxml.Attribute) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.xml == nil {
		return 0, errNoXML
	}
	p.refreshXML()
	var mapped map[string]mappedValue
	if p.otb == nil {
		mapped = mappedAttributes(p.xml.mapping, nil)
	}
	isMapped := func(attribute itemsxml.Attribute) bool {
		for _, written := range mapped {
			if written.names(attribute.Key) {
				return true
			}
		}
		return false
	}
	edit := p.record("Paste items.xml attributes")
	changed := 0
	for _, clientID := range clientIDs {
		if p.things.Get(thing.CategoryItem, clientID) == nil {
			continue
		}
		disk := p.diskID(clientID)
		entry := p.xmlEdits[disk]
		if entry == nil {
			entry, _ = p.xmlDiskEntry(disk)
		}
		if entry == nil {
			entry = &itemsxml.Item{}
		}
		pasted := entry.Clone()
		pasted.Attributes = nil
		for _, attribute := range entry.Attributes {
			if isMapped(attribute) {
				pasted.Attributes = append(pasted.Attributes, attribute)
			}
		}
		for _, attribute := range attributes {
			if !isMapped(attribute) {
				pasted.Attributes = append(pasted.Attributes, attribute)
			}
		}
		if p.editXML(edit, clientID, pasted) {
			changed++
		}
	}
	if changed > 0 {
		edit.commit()
	}
	return changed, nil
}

func (p *Project) setXMLEdit(diskID uint32, item *itemsxml.Item) {
	if item == nil {
		delete(p.xmlEdits, diskID)
		return
	}
	if p.xmlEdits == nil {
		p.xmlEdits = map[uint32]*itemsxml.Item{}
	}
	p.xmlEdits[diskID] = item
}

// xmlName is the name items.xml gives a client item, "" without one.
func (p *Project) xmlName(clientID uint32) string {
	if p.xml == nil {
		return ""
	}
	disk := clientID
	if len(p.moves) > 0 {
		disk = p.diskID(clientID)
	}
	if edited := p.xmlEdits[disk]; edited != nil {
		return edited.Name
	}
	if item, _ := p.xmlDiskEntry(disk); item != nil {
		return item.Name
	}
	return ""
}

func (p *Project) XMLPreview() (XMLReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.xml == nil {
		return XMLReport{}, errNoXML
	}
	p.refreshXML()
	_, report := p.planXML()
	return report, nil
}

// planXML returns what compiling changes in the attached items.xml. The
// caller holds the lock.
func (p *Project) planXML() (itemsxml.Changes, XMLReport) {
	var report XMLReport
	changes := itemsxml.Changes{Items: map[uint16]*itemsxml.Item{}}
	file := p.xml.file
	current := func(disk uint32) uint32 {
		if to, ok := p.moves[disk]; ok {
			return to
		}
		return disk
	}
	// finalID gives the id in the written file of the item at a client id.
	finalID := func(clientID uint32) uint16 {
		if clientID > maxServerID {
			return 0
		}
		return uint16(clientID)
	}
	// planned holds the items (ids on disk) whose edits are planned already.
	var planned map[uint32]bool
	if p.otb != nil {
		synced, _, err := p.syncOTB()
		if err != nil {
			report.Error = err.Error()
			return changes, report
		}
		servers := serverIDs(synced.Items)
		finalID = func(clientID uint32) uint16 {
			if clientID > maxServerID {
				return 0
			}
			return servers[uint16(clientID)]
		}
		// The new server items are the ones the sync appended.
		for _, item := range synced.Items[len(p.otb.file.Items):] {
			if id := item.ServerID(); !file.Has(id) {
				object := p.things.Get(thing.CategoryItem, uint32(item.ClientID()))
				changes.Items[id] = &itemsxml.Item{Name: thingName(object)}
				report.Added++
			}
		}
	} else {
		changes.Moves = map[uint16]uint16{}
		for from, to := range p.moves {
			if from > maxServerID || to > maxServerID || from == to {
				continue
			}
			changes.Moves[uint16(from)] = uint16(to)
			if file.Has(uint16(from)) {
				report.Moved++
			}
		}
		planned = p.planClientItems(changes, &report)
	}
	for disk, edited := range p.xmlEdits {
		id := finalID(current(disk))
		if id == 0 {
			report.Skipped++
			continue
		}
		if planned[disk] {
			continue
		}
		// The edited entry of a new item is the entry added for it.
		if _, added := changes.Items[id]; !added {
			report.Edited++
		}
		changes.Items[id] = edited.Clone()
	}
	return changes, report
}

// planClientItems adds what the client items change in an items.xml on
// client ids: an entry for every new item, and the mapped attributes of the
// items whose properties changed. Both start from the entry as edited in the
// editor; it returns the items (ids on disk) whose edits it planned.
func (p *Project) planClientItems(changes itemsxml.Changes, report *XMLReport) map[uint32]bool {
	planned := map[uint32]bool{}
	mapping := p.xml.mapping
	onDisk := invertMoves(p.moves)
	first, last := thing.CategoryItem.MinID(), min(p.things.MaxID(thing.CategoryItem), maxServerID)
	for id := first; id <= last; id++ {
		object := p.things.Get(thing.CategoryItem, id)
		if object == nil {
			continue
		}
		disk := id
		if original, ok := onDisk[id]; ok {
			disk = original
		}
		base := p.baseItem(disk)
		if base == object || disk > maxServerID {
			continue
		}
		existing, _ := p.xml.file.Get(uint16(disk))
		edited := p.xmlEdits[disk].Clone()
		after := mappedAttributes(mapping, object)
		if base == nil {
			// A new item. An entry the file already has for its id is kept.
			if existing != nil {
				continue
			}
			item := edited
			if item == nil {
				item = &itemsxml.Item{Name: thingName(object)}
				for _, row := range mapping {
					if written := after[strings.ToLower(row.Key)]; written.set {
						written.apply(item)
					}
				}
			}
			changes.Items[uint16(id)] = item
			planned[disk] = true
			report.Added++
			continue
		}
		if len(mapping) == 0 {
			continue
		}
		before := mappedAttributes(mapping, base)
		item := edited
		if item == nil {
			item = existing.Clone()
		}
		if item == nil {
			item = &itemsxml.Item{}
		}
		changed := false
		for _, row := range mapping {
			key := strings.ToLower(row.Key)
			was, now := before[key], after[key]
			if was.set == now.set && was.value == now.value {
				continue
			}
			changed = true
			now.apply(item)
		}
		if changed && (existing == nil || !item.Equal(existing)) {
			changes.Items[uint16(id)] = item
			planned[disk] = true
			report.Updated++
		}
	}
	return planned
}

func thingName(object *thing.Thing) string {
	if object == nil {
		return ""
	}
	return object.Name
}

// prepareXML plans the attached items.xml before a compilation and returns
// the bytes to write (nil when the file stays as it is). Errors go into the
// report: an items.xml never stops the client from being written.
func (p *Project) prepareXML() ([]byte, *XMLReport) {
	if p.xml == nil {
		return nil, nil
	}
	p.refreshXML()
	changes, report := p.planXML()
	if report.Error != "" {
		return nil, &report
	}
	data := p.xml.file.Encode(changes)
	if bytes.Equal(data, p.xml.file.Data()) {
		return nil, &report
	}
	return data, &report
}

// writeXML writes what prepareXML returned, after the client files. The
// edits are in the file then.
func (p *Project) writeXML(data []byte, report *XMLReport) {
	if report == nil || report.Error != "" {
		return
	}
	if data == nil {
		p.xmlEdits = nil
		return
	}
	file, err := itemsxml.Parse(data)
	if err == nil {
		err = fsutil.WriteAtomic(p.xml.path, data)
	}
	if err != nil {
		report.Error = err.Error()
		return
	}
	p.xml.file, p.xml.stamp = file, stampOf(p.xml.path)
	p.xmlEdits = nil
	report.Written = true
}
