package app

import (
	"errors"
	"fmt"
	"sync"

	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// CompareService opens a second client (B) next to the open one (A),
// compares them and copies things in both directions.
type CompareService struct {
	session *Session

	mu sync.Mutex
	// before holds the state of copied things in their target from before
	// the first copy, so Revert can put each one back.
	before map[copyKey]*thing.Thing
}

type copyKey struct {
	target   *project.Project
	category thing.Category
	id       uint32
}

var errNothingToCopy = errors.New("nothing to copy")

// NewCompareService returns the service.
func NewCompareService(session *Session) *CompareService {
	return &CompareService{session: session, before: map[copyKey]*thing.Thing{}}
}

func (cs *CompareService) forget() {
	cs.mu.Lock()
	clear(cs.before)
	cs.mu.Unlock()
}

// State returns the state of the second client.
func (cs *CompareService) State() State { return cs.session.OtherState() }

// Open loads the second client.
func (cs *CompareService) Open(req OpenRequest) (State, error) {
	proj, err := openProject(req)
	if err != nil {
		return cs.session.OtherState(), err
	}
	cs.forget()
	cs.session.SetOther(proj)
	return cs.session.OtherState(), nil
}

// Thing returns one thing of the second client.
func (cs *CompareService) Thing(category thing.Category, id uint32) (*thing.Thing, error) {
	other, err := cs.session.Other()
	if err != nil {
		return nil, err
	}
	return other.Thing(category, id)
}

// Close closes the second client.
func (cs *CompareService) Close() {
	cs.forget()
	cs.session.SetOther(nil)
}

// clients returns the client things are copied into and the one they come
// from: B into A, or A into B with toOther.
func (cs *CompareService) clients(toOther bool) (target, source *project.Project, err error) {
	clientA, err := cs.session.Project()
	if err != nil {
		return nil, nil, err
	}
	clientB, err := cs.session.Other()
	if err != nil {
		return nil, nil, err
	}
	if toOther {
		return clientB, clientA, nil
	}
	return clientA, clientB, nil
}

// changed tells the frontend that A, or B with toOther, changed.
func (cs *CompareService) changed(toOther bool) {
	if toOther {
		cs.session.OtherChanged()
	} else {
		cs.session.Changed()
	}
}

// Diff compares one category of the open client (A) with the second (B).
func (cs *CompareService) Diff(category thing.Category) (project.DiffResult, error) {
	clientA, clientB, err := cs.clients(false)
	if err != nil {
		return project.DiffResult{}, err
	}
	return project.Diff(clientA, clientB, category)
}

// Transfer copies things between the clients: from B into A, or from A
// into B with toOther. See project.Transfer for appendNew.
func (cs *CompareService) Transfer(toOther bool, category thing.Category, ids []uint32, appendNew bool) (project.TransferResult, error) {
	return cs.transfer(toOther, category, ids, func(target, source *project.Project) (project.TransferResult, error) {
		return project.Transfer(target, source, category, ids, appendNew)
	})
}

// TransferTo copies things between the clients like Transfer, ids[i] to
// targets[i] in the target client.
func (cs *CompareService) TransferTo(toOther bool, category thing.Category, ids, targets []uint32) (project.TransferResult, error) {
	return cs.transfer(toOther, category, ids, func(target, source *project.Project) (project.TransferResult, error) {
		return project.TransferTo(target, source, category, ids, targets)
	})
}

// transfer runs a copy and remembers the things it replaced for Revert.
func (cs *CompareService) transfer(toOther bool, category thing.Category, ids []uint32, copyThings func(target, source *project.Project) (project.TransferResult, error)) (project.TransferResult, error) {
	target, source, err := cs.clients(toOther)
	if err != nil {
		return project.TransferResult{}, err
	}
	if len(ids) == 0 {
		return project.TransferResult{}, errNothingToCopy
	}
	result, err := copyThings(target, source)
	if err != nil {
		return result, err
	}
	cs.mu.Lock()
	for id, previous := range result.Before {
		key := copyKey{target, category, id}
		if _, ok := cs.before[key]; !ok {
			cs.before[key] = previous
		}
	}
	cs.mu.Unlock()
	cs.changed(toOther)
	return result, nil
}

// TargetPlan is where TransferTo would put things.
type TargetPlan struct {
	IDs []uint32 `json:"ids"`
	// Taken are the target ids that hold objects, which the copies replace.
	Taken []uint32 `json:"taken"`
}

// planTargets returns count target ids in proj from id from on: free ids only
// (see project.FreeIDs), or consecutive ids.
func planTargets(proj *project.Project, category thing.Category, from uint32, count int, freeOnly bool) (TargetPlan, error) {
	if count <= 0 {
		return TargetPlan{IDs: []uint32{}, Taken: []uint32{}}, nil
	}
	if !category.Valid() {
		return TargetPlan{}, fmt.Errorf("invalid category %d", category)
	}
	from = max(from, category.MinID())
	if freeOnly {
		ids, err := proj.FreeIDs(category, from, count)
		return TargetPlan{IDs: ids, Taken: []uint32{}}, err
	}
	ids := make([]uint32, count)
	for i := range ids {
		ids[i] = from + uint32(i)
	}
	return TargetPlan{IDs: ids, Taken: proj.TakenIDs(category, ids)}, nil
}

// Revert puts copied things back as they were before they were copied, in
// A, or in B with toOther. It is one undoable edit in that client.
func (cs *CompareService) Revert(toOther bool, category thing.Category, ids []uint32) error {
	target, _, err := cs.clients(toOther)
	if err != nil {
		return err
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	before := map[uint32]*thing.Thing{}
	for _, id := range ids {
		previous, ok := cs.before[copyKey{target, category, id}]
		if !ok {
			return fmt.Errorf("%s %d was not copied", category, id)
		}
		before[id] = previous
	}
	if err := target.RestoreThings(category, before); err != nil {
		return err
	}
	for _, id := range ids {
		delete(cs.before, copyKey{target, category, id})
	}
	cs.changed(toOther)
	return nil
}

// Undo reverts the last edit of the second client and returns its label.
func (cs *CompareService) Undo() (string, error) {
	other, err := cs.session.Other()
	if err != nil {
		return "", err
	}
	label := other.Undo()
	if label != "" {
		cs.session.OtherChanged()
	}
	return label, nil
}

// Compile saves the second client to its files with its settings.
func (cs *CompareService) Compile() error {
	other, err := cs.session.Other()
	if err != nil {
		return err
	}
	err = other.Compile(currentFilesRequest(other.Info()).options(cs.session.reporter("Compiling B")))
	cs.session.OtherChanged()
	return err
}
