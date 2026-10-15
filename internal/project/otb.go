package project

import (
	"bytes"
	"fmt"
	"os"
	"slices"

	"github.com/nekiro/ots-creator/internal/itemsotb"
	"github.com/nekiro/ots-creator/internal/thing"
)

// An items.otb can be attached to a client. It is never edited directly:
// compiling brings it in line with the client's items. Server items follow
// moved client items, items whose client data changed get it reloaded, and
// client items past the last mapped one get new server ids.

// itemsOTB is the attached items.otb.
type itemsOTB struct {
	path string
	file *itemsotb.File
	// data is the file as it is on disk; it is only rewritten when the
	// synced file encodes differently.
	data []byte
	info OTBInfo
}

func newItemsOTB(path string, f *itemsotb.File, data []byte) *itemsOTB {
	o := &itemsOTB{path: path, file: f, data: data}
	major, minor, build := f.Version()
	o.info = OTBInfo{Path: path, Version: fmt.Sprintf("%d.%d.%d", major, minor, build), Items: len(f.Items)}
	for _, it := range f.Items {
		o.info.MaxServerID = max(o.info.MaxServerID, it.ServerID())
		o.info.MaxClientID = max(o.info.MaxClientID, it.ClientID())
	}
	return o
}

// OTBInfo describes the attached items.otb.
type OTBInfo struct {
	Path string `json:"path"`
	// Version is "major.minor.build"; minor is the client version id.
	Version string `json:"version"`
	// Items is the number of server items, MaxServerID and MaxClientID the
	// highest ids they use.
	Items       int    `json:"items"`
	MaxServerID uint16 `json:"maxServerId"`
	MaxClientID uint16 `json:"maxClientId"`
}

// OTBReport tells what syncing the items.otb changes (or changed).
type OTBReport struct {
	// Remapped counts server items whose client item was moved to another id.
	Remapped int `json:"remapped"`
	// Updated counts server items whose client data was reloaded and changed.
	Updated int `json:"updated"`
	// Added counts new server items for client items past the last mapped one.
	Added int `json:"added"`
	// Missing counts server items whose client item no longer exists.
	Missing int `json:"missing"`
	// Written is set when the file was rewritten.
	Written bool `json:"written"`
	// Error is set when the file could not be updated (the client files
	// were still written).
	Error string `json:"error,omitempty"`
}

// Changed reports whether the sync changes the file.
func (r OTBReport) Changed() bool { return r.Remapped+r.Updated+r.Added > 0 }

// AttachOTB reads an items.otb and keeps it in sync with the client from
// now on. It replaces an attached one.
func (p *Project) AttachOTB(path string) (OTBInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OTBInfo{}, err
	}
	f, err := itemsotb.Parse(data)
	if err != nil {
		return OTBInfo{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.otb = newItemsOTB(path, f, data)
	return p.otb.info, nil
}

// DetachOTB stops syncing the items.otb.
func (p *Project) DetachOTB() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.otb = nil
}

// OTB returns the attached items.otb, or nil.
func (p *Project) OTB() *OTBInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.otb == nil {
		return nil
	}
	in := p.otb.info
	return &in
}

// OTBPreview reports what compiling would change in the items.otb.
func (p *Project) OTBPreview() (OTBReport, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.otb == nil {
		return OTBReport{}, fmt.Errorf("no items.otb is attached")
	}
	_, rep, err := p.syncOTB()
	return rep, err
}

// resetBase records the items as they are on disk; syncing the items.otb
// compares the client with them.
func (p *Project) resetBase() {
	p.base = slices.Clone(p.things.Things[thing.CategoryItem])
	p.moves = nil
}

// applyMoves records that the items at the keys of step moved to its
// values (p.moves maps ids on disk to current ids).
func (p *Project) applyMoves(step map[uint32]uint32) {
	if len(step) == 0 {
		return
	}
	next := make(map[uint32]uint32, len(p.moves)+len(step))
	taken := make(map[uint32]bool, len(p.moves))
	for from, cur := range p.moves {
		taken[cur] = true
		if to, ok := step[cur]; ok {
			cur = to
		}
		next[from] = cur
	}
	for from, to := range step {
		if !taken[from] {
			next[from] = to
		}
	}
	for from, to := range next {
		if from == to {
			delete(next, from)
		}
	}
	p.moves = next
}

func invertMoves(m map[uint32]uint32) map[uint32]uint32 {
	out := make(map[uint32]uint32, len(m))
	for from, to := range m {
		out[to] = from
	}
	return out
}

// otbSpriteHash tells whether sprite hashes can be computed: ItemEditor
// hashes 32x32 spr sprites only.
func (p *Project) otbSpriteHash() bool {
	return p.format == FormatDat && p.sprites.size == itemsotb.SpriteSize
}

// syncOTB returns a copy of the attached items.otb brought in line with the
// client's items. The caller holds the lock.
func (p *Project) syncOTB() (*itemsotb.File, OTBReport, error) {
	var rep OTBReport
	src := p.otb.file
	f := &itemsotb.File{}
	*f = *src
	f.Items = make([]*itemsotb.Item, len(src.Items))

	cat := thing.CategoryItem
	last := p.things.MaxID(cat)
	disk := invertMoves(p.moves) // current id -> id on disk
	changed := map[uint32]bool{}
	isChanged := func(id uint32) bool {
		if v, ok := changed[id]; ok {
			return v
		}
		t := p.things.Get(cat, id)
		from := id
		if d, ok := disk[id]; ok {
			from = d
		}
		v := true
		if i := int(from) - int(cat.MinID()); i >= 0 && i < len(p.base) && p.base[i] == t {
			v = slices.ContainsFunc(t.SpriteIDs(), p.sprites.edited)
		}
		changed[id] = v
		return v
	}
	hashes := map[uint32]*[16]byte{}
	hashOf := func(t *thing.Thing) (*[16]byte, error) {
		if !p.otbSpriteHash() || len(t.FrameGroups) == 0 {
			return nil, nil
		}
		if h, ok := hashes[t.ID]; ok {
			return h, nil
		}
		g := t.FrameGroups[0]
		h, err := itemsotb.SpriteHash(g, p.sprites.transparent, func(i int) ([]byte, error) {
			return p.sprites.compressed(g.Sprites[i])
		})
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", t.ID, err)
		}
		hashes[t.ID] = &h
		return &h, nil
	}
	reload := func(it *itemsotb.Item, t *thing.Thing) error {
		h, err := hashOf(t)
		if err != nil {
			return err
		}
		it.Reload(t, h)
		return nil
	}

	var maxServer, maxClient uint16
	for i, it := range src.Items {
		f.Items[i] = it
		maxServer = max(maxServer, it.ServerID())
		cid := uint32(it.ClientID())
		if it.Deprecated() || cid == 0 {
			continue
		}
		cur := cid
		if to, ok := p.moves[cid]; ok {
			cur = to
		}
		if cur > 0xFFFF {
			rep.Missing++
			continue
		}
		maxClient = max(maxClient, uint16(cur))
		t := p.things.Get(cat, cur)
		if t == nil {
			rep.Missing++
			continue
		}
		moved := cur != cid
		if !moved && !isChanged(cur) {
			continue
		}
		n := it.Clone()
		if moved {
			n.SetClientID(uint16(cur))
			rep.Remapped++
		}
		if err := reload(n, t); err != nil {
			return nil, rep, err
		}
		if !moved && n.Equal(it) {
			continue
		}
		if !moved {
			rep.Updated++
		}
		f.Items[i] = n
	}

	for id := uint32(maxClient) + 1; id <= last && id <= 0xFFFF; id++ {
		if maxServer == 0xFFFF {
			return nil, rep, fmt.Errorf("items.otb has no free server id for client item %d", id)
		}
		t := p.things.Get(cat, id)
		if t == nil {
			continue
		}
		maxServer++
		it := itemsotb.NewItem(maxServer, uint16(id))
		if err := reload(it, t); err != nil {
			return nil, rep, err
		}
		f.Items = append(f.Items, it)
		rep.Added++
	}
	return f, rep, nil
}

// prepareOTB syncs the attached items.otb before a compilation and returns
// the bytes to write (nil when the file stays as it is). Errors go into the
// report: an items.otb never stops the client from being written.
func (p *Project) prepareOTB() (*itemsotb.File, []byte, *OTBReport) {
	if p.otb == nil {
		return nil, nil, nil
	}
	f, rep, err := p.syncOTB()
	if err != nil {
		rep.Error = err.Error()
		return nil, nil, &rep
	}
	data, err := f.Encode()
	if err != nil {
		rep.Error = err.Error()
		return nil, nil, &rep
	}
	if bytes.Equal(data, p.otb.data) {
		return nil, nil, &rep
	}
	return f, data, &rep
}

// writeOTB writes what prepareOTB returned, after the client files.
func (p *Project) writeOTB(f *itemsotb.File, data []byte, rep *OTBReport) {
	if rep == nil || data == nil {
		return
	}
	if err := writeAtomic(p.otb.path, data); err != nil {
		rep.Error = err.Error()
		return
	}
	p.otb = newItemsOTB(p.otb.path, f, data)
	rep.Written = true
}
