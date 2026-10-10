package app

import (
	"errors"
	"fmt"
	"sync"

	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// CompareService opens a second client (B) next to the open one (A),
// compares them and copies things in both directions.
type CompareService struct {
	s *Session

	mu sync.Mutex
	// before holds the state of copied things in their target from before
	// the first copy, so Revert can put each one back.
	before map[copyKey]*thing.Thing
}

type copyKey struct {
	p  *project.Project
	c  thing.Category
	id uint32
}

// NewCompareService returns the service.
func NewCompareService(s *Session) *CompareService {
	return &CompareService{s: s, before: map[copyKey]*thing.Thing{}}
}

func (cs *CompareService) forget() {
	cs.mu.Lock()
	clear(cs.before)
	cs.mu.Unlock()
}

// State returns the state of the second client.
func (cs *CompareService) State() State { return cs.s.OtherState() }

// Open loads the second client.
func (cs *CompareService) Open(req OpenRequest) (State, error) {
	var p *project.Project
	var err error
	if req.Format == project.FormatAssets {
		p, err = project.OpenAssets(req.DatPath)
	} else {
		p, err = project.Open(project.OpenOptions{DatPath: req.DatPath, SprPath: req.SprPath, Version: req.Version, Features: req.Features})
	}
	if err != nil {
		return cs.s.OtherState(), err
	}
	cs.forget()
	cs.s.SetOther(p)
	return cs.s.OtherState(), nil
}

// Thing returns one thing of the second client.
func (cs *CompareService) Thing(c thing.Category, id uint32) (*thing.Thing, error) {
	p, err := cs.s.Other()
	if err != nil {
		return nil, err
	}
	return p.Thing(c, id)
}

// Close closes the second client.
func (cs *CompareService) Close() {
	cs.forget()
	cs.s.SetOther(nil)
}

func (cs *CompareService) both() (a, b *project.Project, err error) {
	if a, err = cs.s.Project(); err != nil {
		return nil, nil, err
	}
	if b, err = cs.s.Other(); err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

// Diff compares one category of the open client (A) with the second (B).
func (cs *CompareService) Diff(c thing.Category) (project.DiffResult, error) {
	a, b, err := cs.both()
	if err != nil {
		return project.DiffResult{}, err
	}
	return project.Diff(a, b, c)
}

// Transfer copies things between the clients: from B into A, or from A
// into B with toOther. See project.Transfer for appendNew.
func (cs *CompareService) Transfer(toOther bool, c thing.Category, ids []uint32, appendNew bool) (project.TransferResult, error) {
	a, b, err := cs.both()
	if err != nil {
		return project.TransferResult{}, err
	}
	if len(ids) == 0 {
		return project.TransferResult{}, errors.New("nothing to copy")
	}
	dst, src := a, b
	if toOther {
		dst, src = b, a
	}
	res, err := project.Transfer(dst, src, c, ids, appendNew)
	if err != nil {
		return res, err
	}
	cs.mu.Lock()
	for id, t := range res.Before {
		k := copyKey{dst, c, id}
		if _, ok := cs.before[k]; !ok {
			cs.before[k] = t
		}
	}
	cs.mu.Unlock()
	if toOther {
		cs.s.OtherChanged()
	} else {
		cs.s.Changed()
	}
	return res, nil
}

// Revert puts copied things back as they were before they were copied, in
// A, or in B with toOther. It is one undoable edit in that client.
func (cs *CompareService) Revert(toOther bool, c thing.Category, ids []uint32) error {
	a, b, err := cs.both()
	if err != nil {
		return err
	}
	dst := a
	if toOther {
		dst = b
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	before := map[uint32]*thing.Thing{}
	for _, id := range ids {
		t, ok := cs.before[copyKey{dst, c, id}]
		if !ok {
			return fmt.Errorf("%s %d was not copied", c, id)
		}
		before[id] = t
	}
	if err := dst.RestoreThings(c, before); err != nil {
		return err
	}
	for _, id := range ids {
		delete(cs.before, copyKey{dst, c, id})
	}
	if toOther {
		cs.s.OtherChanged()
	} else {
		cs.s.Changed()
	}
	return nil
}

// Undo reverts the last edit of the second client and returns its label.
func (cs *CompareService) Undo() (string, error) {
	p, err := cs.s.Other()
	if err != nil {
		return "", err
	}
	label := p.Undo()
	if label != "" {
		cs.s.OtherChanged()
	}
	return label, nil
}

// Compile saves the second client to its files with its settings.
func (cs *CompareService) Compile() error {
	p, err := cs.s.Other()
	if err != nil {
		return err
	}
	info := p.Info()
	err = p.Compile(project.CompileOptions{Format: info.Format, DatPath: info.DatPath, SprPath: info.SprPath, Version: info.Version, Features: info.Features})
	cs.s.OtherChanged()
	return err
}
