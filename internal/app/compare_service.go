package app

import (
	"errors"

	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// CompareService opens a second client (B) next to the open one (A),
// compares them and copies things in both directions.
type CompareService struct {
	s *Session
}

// NewCompareService returns the service.
func NewCompareService(s *Session) *CompareService { return &CompareService{s: s} }

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
	cs.s.SetOther(p)
	return cs.s.OtherState(), nil
}

// Close closes the second client.
func (cs *CompareService) Close() { cs.s.SetOther(nil) }

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
	if toOther {
		cs.s.OtherChanged()
	} else {
		cs.s.Changed()
	}
	return res, nil
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
