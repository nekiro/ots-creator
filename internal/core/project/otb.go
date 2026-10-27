package project

import (
	"bytes"
	"fmt"
	"os"
	"slices"

	"github.com/nekiro/ots-creator/internal/fsutil"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/itemsotb"
)

// An items.otb can be attached to a client. It is never edited directly:
// compiling brings it in line with the client's items. Server items follow
// moved client items, items whose client data changed get it reloaded, and
// client items past the last mapped one get new server ids.

type itemsOTB struct {
	path string
	file *itemsotb.File
	// data is the file as it is on disk; it is only rewritten when the
	// synced file encodes differently.
	data []byte
	info OTBInfo
}

func newItemsOTB(path string, file *itemsotb.File, data []byte) *itemsOTB {
	otb := &itemsOTB{path: path, file: file, data: data}
	major, minor, build := file.Version()
	otb.info = OTBInfo{Path: path, Version: fmt.Sprintf("%d.%d.%d", major, minor, build), Items: len(file.Items)}
	for _, item := range file.Items {
		otb.info.MaxServerID = max(otb.info.MaxServerID, item.ServerID())
		otb.info.MaxClientID = max(otb.info.MaxClientID, item.ClientID())
	}
	return otb
}

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

type OTBReport struct {
	// Remapped counts server items whose client item was moved to another id.
	Remapped int `json:"remapped"`
	// Updated counts server items whose client data was reloaded and changed.
	Updated int `json:"updated"`
	// Added counts new server items for client items past the last mapped one.
	Added int `json:"added"`
	// Missing counts server items whose client item no longer exists.
	Missing int  `json:"missing"`
	Written bool `json:"written"`
	// Error is set when the file could not be updated (the client files
	// were still written).
	Error string `json:"error,omitempty"`
}

func (r OTBReport) Changed() bool { return r.Remapped+r.Updated+r.Added > 0 }

// AttachOTB reads an items.otb and keeps it in sync with the client from
// now on. It replaces an attached one.
func (p *Project) AttachOTB(path string) (OTBInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OTBInfo{}, err
	}
	file, err := itemsotb.Parse(data)
	if err != nil {
		return OTBInfo{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.otb = newItemsOTB(path, file, data)
	return p.otb.info, nil
}

func (p *Project) DetachOTB() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.otb = nil
}

func (p *Project) OTB() *OTBInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.otb == nil {
		return nil
	}
	info := p.otb.info
	return &info
}

func (p *Project) OTBPreview() (OTBReport, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.otb == nil {
		return OTBReport{}, fmt.Errorf("no items.otb is attached")
	}
	_, report, err := p.syncOTB()
	return report, err
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
	for from, current := range p.moves {
		taken[current] = true
		if to, ok := step[current]; ok {
			current = to
		}
		next[from] = current
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

func invertMoves(moves map[uint32]uint32) map[uint32]uint32 {
	out := make(map[uint32]uint32, len(moves))
	for from, to := range moves {
		out[to] = from
	}
	return out
}

// otbSpriteHash tells whether sprite hashes can be computed: ItemEditor
// hashes 32x32 spr sprites only.
func (p *Project) otbSpriteHash() bool {
	return p.format == FormatDat && p.sprites.size == itemsotb.SpriteSize
}

// maxServerID is the highest server id an items.otb can hold. Client ids
// are 16 bit there too.
const maxServerID = 0xFFFF

type otbSync struct {
	project *Project
	report  OTBReport
	// onDisk maps current item ids to their ids on disk.
	onDisk  map[uint32]uint32
	changed map[uint32]bool
	hashes  map[uint32]*[16]byte
}

// syncOTB returns a copy of the attached items.otb brought in line with the
// client's items. The caller holds the lock.
func (p *Project) syncOTB() (*itemsotb.File, OTBReport, error) {
	source := p.otb.file
	synced := &itemsotb.File{}
	*synced = *source
	synced.Items = make([]*itemsotb.Item, len(source.Items))
	syncer := &otbSync{project: p, onDisk: invertMoves(p.moves), changed: map[uint32]bool{}, hashes: map[uint32]*[16]byte{}}
	maxServer, maxClient, err := syncer.updateItems(source.Items, synced.Items)
	if err != nil {
		return nil, syncer.report, err
	}
	if synced.Items, err = syncer.appendItems(synced.Items, maxServer, maxClient); err != nil {
		return nil, syncer.report, err
	}
	return synced, syncer.report, nil
}

// updateItems copies items into synced, remapping the server items whose
// client item moved and reloading the ones whose client data changed. It
// returns the highest server id and the highest mapped client id.
func (s *otbSync) updateItems(items, synced []*itemsotb.Item) (maxServer, maxClient uint16, err error) {
	for i, item := range items {
		synced[i] = item
		maxServer = max(maxServer, item.ServerID())
		clientID := uint32(item.ClientID())
		if item.Deprecated() || clientID == 0 {
			continue
		}
		currentID := clientID
		if movedTo, ok := s.project.moves[clientID]; ok {
			currentID = movedTo
		}
		if currentID > maxServerID {
			s.report.Missing++
			continue
		}
		maxClient = max(maxClient, uint16(currentID))
		object := s.project.things.Get(thing.CategoryItem, currentID)
		if object == nil {
			s.report.Missing++
			continue
		}
		moved := currentID != clientID
		if !moved && !s.isChanged(currentID) {
			continue
		}
		updated := item.Clone()
		if moved {
			updated.SetClientID(uint16(currentID))
			s.report.Remapped++
		}
		if err := s.reload(updated, object); err != nil {
			return 0, 0, err
		}
		if !moved && updated.Equal(item) {
			continue
		}
		if !moved {
			s.report.Updated++
		}
		synced[i] = updated
	}
	return maxServer, maxClient, nil
}

// appendItems adds new server items for the client items past the last
// mapped one.
func (s *otbSync) appendItems(items []*itemsotb.Item, maxServer, maxClient uint16) ([]*itemsotb.Item, error) {
	last := s.project.things.MaxID(thing.CategoryItem)
	for id := uint32(maxClient) + 1; id <= last && id <= maxServerID; id++ {
		if maxServer == maxServerID {
			return nil, fmt.Errorf("items.otb has no free server id for client item %d", id)
		}
		object := s.project.things.Get(thing.CategoryItem, id)
		if object == nil {
			continue
		}
		maxServer++
		item := itemsotb.NewItem(maxServer, uint16(id))
		if err := s.reload(item, object); err != nil {
			return nil, err
		}
		items = append(items, item)
		s.report.Added++
	}
	return items, nil
}

// isChanged reports whether the client data of item id differs from the
// one on disk: another thing in its slot or an edited sprite.
func (s *otbSync) isChanged(id uint32) bool {
	if changed, ok := s.changed[id]; ok {
		return changed
	}
	category := thing.CategoryItem
	object := s.project.things.Get(category, id)
	diskID := id
	if original, ok := s.onDisk[id]; ok {
		diskID = original
	}
	changed := true
	if i := int(diskID) - int(category.MinID()); i >= 0 && i < len(s.project.base) && s.project.base[i] == object {
		changed = slices.ContainsFunc(object.SpriteIDs(), s.project.sprites.edited)
	}
	s.changed[id] = changed
	return changed
}

// spriteHash returns the ItemEditor sprite hash of an item, or nil when it
// cannot be computed.
func (s *otbSync) spriteHash(object *thing.Thing) (*[16]byte, error) {
	if !s.project.otbSpriteHash() || len(object.FrameGroups) == 0 {
		return nil, nil
	}
	if digest, ok := s.hashes[object.ID]; ok {
		return digest, nil
	}
	group := object.FrameGroups[0]
	digest, err := itemsotb.SpriteHash(group, s.project.sprites.transparent, func(i int) ([]byte, error) {
		return s.project.sprites.compressed(group.Sprites[i])
	})
	if err != nil {
		return nil, fmt.Errorf("item %d: %w", object.ID, err)
	}
	s.hashes[object.ID] = &digest
	return &digest, nil
}

func (s *otbSync) reload(item *itemsotb.Item, object *thing.Thing) error {
	digest, err := s.spriteHash(object)
	if err != nil {
		return err
	}
	item.Reload(object, digest)
	return nil
}

// prepareOTB syncs the attached items.otb before a compilation and returns
// the bytes to write (nil when the file stays as it is). Errors go into the
// report: an items.otb never stops the client from being written.
func (p *Project) prepareOTB() (*itemsotb.File, []byte, *OTBReport) {
	if p.otb == nil {
		return nil, nil, nil
	}
	file, report, err := p.syncOTB()
	if err != nil {
		report.Error = err.Error()
		return nil, nil, &report
	}
	data, err := file.Encode()
	if err != nil {
		report.Error = err.Error()
		return nil, nil, &report
	}
	if bytes.Equal(data, p.otb.data) {
		return nil, nil, &report
	}
	return file, data, &report
}

// writeOTB writes what prepareOTB returned, after the client files.
func (p *Project) writeOTB(file *itemsotb.File, data []byte, report *OTBReport) {
	if report == nil || data == nil {
		return
	}
	if err := fsutil.WriteAtomic(p.otb.path, data); err != nil {
		report.Error = err.Error()
		return
	}
	p.otb = newItemsOTB(p.otb.path, file, data)
	report.Written = true
}
