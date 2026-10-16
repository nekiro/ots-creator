// Package otitems is the item type database of the map editor. Maps store
// server ids: with an items.otb (TFS and older servers) they map to client
// ids through it; without one (servers that use client ids
// directly) both ids are the same. Flags come from the otb when present and
// from the client (dat or appearances) otherwise; names come from items.xml
// or the client.
package otitems

import (
	"slices"
	"strconv"
	"strings"
)

// Mode tells how map item ids relate to client ids.
type Mode string

const (
	// ModeOTB maps server ids to client ids through items.otb.
	ModeOTB Mode = "otb"
	// ModeClient uses client ids as server ids (no items.otb).
	ModeClient Mode = "client"
)

// DB is the item database. It is read-only after loading.
type DB struct {
	Mode Mode `json:"mode"`
	// OTB versions, written into OTBM headers.
	MajorVersion uint32 `json:"majorVersion"`
	MinorVersion uint32 `json:"minorVersion"`
	BuildNumber  uint32 `json:"buildNumber"`
	items        []*Type
	byClient     map[uint16]uint16
}

// Get returns the type of a server id, or nil.
func (db *DB) Get(id uint16) *Type {
	if db == nil || int(id) >= len(db.items) {
		return nil
	}
	return db.items[id]
}

// MaxID returns the highest server id.
func (db *DB) MaxID() uint16 {
	if len(db.items) == 0 {
		return 0
	}
	return uint16(len(db.items) - 1)
}

// FromClient returns the first server id that shows a client id.
func (db *DB) FromClient(clientID uint16) (uint16, bool) {
	id, ok := db.byClient[clientID]
	return id, ok
}

// Each calls fn for every item type in id order.
func (db *DB) Each(fn func(*Type)) {
	for _, itemType := range db.items {
		if itemType != nil {
			fn(itemType)
		}
	}
}

// Count returns the number of item types.
func (db *DB) Count() int {
	count := 0
	for _, itemType := range db.items {
		if itemType != nil {
			count++
		}
	}
	return count
}

func (db *DB) set(itemType *Type) {
	for int(itemType.ID) >= len(db.items) {
		db.items = append(db.items, nil)
	}
	db.items[itemType.ID] = itemType
}

func (db *DB) index() {
	db.byClient = make(map[uint16]uint16, len(db.items))
	for _, itemType := range db.items {
		if itemType == nil || itemType.ClientID == 0 {
			continue
		}
		if _, ok := db.byClient[itemType.ClientID]; !ok {
			db.byClient[itemType.ClientID] = itemType.ID
		}
	}
}

// Search returns server ids whose name contains query (case-insensitive)
// or whose id equals query, at most limit.
func (db *DB) Search(query string, limit int) []uint16 {
	query = strings.ToLower(strings.TrimSpace(query))
	var out []uint16
	if query == "" {
		return out
	}
	if id, err := strconv.ParseUint(query, 10, 16); err == nil {
		if itemType := db.Get(uint16(id)); itemType != nil {
			out = append(out, itemType.ID)
		}
	}
	for _, itemType := range db.items {
		if itemType == nil || len(out) >= limit {
			continue
		}
		if strings.Contains(strings.ToLower(itemType.Name), query) && !slices.Contains(out, itemType.ID) {
			out = append(out, itemType.ID)
		}
	}
	return out
}

// NewDB returns a database with the given types.
func NewDB(mode Mode, types []*Type) *DB {
	db := &DB{Mode: mode}
	for _, itemType := range types {
		db.set(itemType)
	}
	db.index()
	return db
}

// MarkMeta marks ids as editor-only items (never saved), creating types for
// ids the database does not have.
func (db *DB) MarkMeta(ids []uint16) {
	for _, id := range ids {
		itemType := db.Get(id)
		if itemType == nil {
			itemType = &Type{ID: id, Name: "editor item"}
			db.set(itemType)
		}
		itemType.Flags |= FlagMeta
	}
}
