package app

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nekiro/ots-creator/internal/core/project"
)

var ErrNoProject = errors.New("no client is open")

var ErrNoOther = errors.New("no second client is open")

const EventProjectChanged = "project:changed"

const EventOtherChanged = "compare:changed"

type Notifier func(name string, data any)

type State struct {
	Open bool         `json:"open"`
	Info project.Info `json:"info"`
	Rev  uint64       `json:"rev"`
	// Delta lists what changed with this revision, so the frontend only
	// reloads those thumbnails and sprites. It is nil in snapshots that are
	// not change events (treat as "all").
	Delta *project.Delta `json:"delta"`
	// OTBError is set by Open when the items.otb remembered for the client
	// could not be read; the client is open without it.
	OTBError string `json:"otbError,omitempty"`
}

type clientSlot struct {
	project *project.Project
	missing error
	event   string
}

type Session struct {
	mu    sync.RWMutex
	main  clientSlot
	other clientSlot
	// rev is shared by both clients, so resource URLs of both stay unique.
	rev       atomic.Uint64
	notify    Notifier
	canceled  atomic.Bool
	exporting atomic.Int32
	onSaved   func(project.Info)
	// otbPaths remembers the items.otb of each client (nil: not kept).
	otbPaths otbPaths
}

type otbPaths interface {
	ItemsOTB(client string) string
	SetItemsOTB(client, otb string) error
}

func NewSession(notify Notifier) *Session {
	if notify == nil {
		notify = func(string, any) {}
	}
	session := &Session{
		notify: notify,
		main:   clientSlot{missing: ErrNoProject, event: EventProjectChanged},
		other:  clientSlot{missing: ErrNoOther, event: EventOtherChanged},
	}
	// Resource URLs carry the revision and are cached as immutable, also on
	// disk by the webview; starting from the launch time keeps URLs of one
	// run from matching cached images of an earlier run. Milliseconds stay
	// exact as JavaScript numbers.
	session.rev.Store(uint64(time.Now().UnixMilli()))
	return session
}

// Cancel stops the running long operation (an export) after the files
// that are being written.
func (s *Session) Cancel() { s.canceled.Store(true) }

func (s *Session) client(slot *clientSlot) (*project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if slot.project == nil {
		return nil, slot.missing
	}
	return slot.project, nil
}

func (s *Session) setClient(slot *clientSlot, proj *project.Project) {
	s.mu.Lock()
	slot.project = proj
	s.mu.Unlock()
	if proj != nil {
		proj.TakeDelta() // a new client replaces everything anyway
	}
	s.rev.Add(1)
	state := s.state(slot)
	state.Delta = &project.Delta{All: true}
	s.notify(slot.event, state)
}

func (s *Session) state(slot *clientSlot) State {
	s.mu.RLock()
	proj := slot.project
	s.mu.RUnlock()
	state := State{Rev: s.rev.Load()}
	if proj != nil {
		state.Open = true
		state.Info = proj.Info()
	}
	return state
}

func (s *Session) changed(slot *clientSlot) {
	s.rev.Add(1)
	state := s.state(slot)
	delta := project.Delta{All: true}
	if proj, err := s.client(slot); err == nil {
		delta = proj.TakeDelta()
	}
	state.Delta = &delta
	s.notify(slot.event, state)
}

func (s *Session) Project() (*project.Project, error) { return s.client(&s.main) }

func (s *Session) Set(proj *project.Project) { s.setClient(&s.main, proj) }

func (s *Session) State() State { return s.state(&s.main) }

func (s *Session) Changed() { s.changed(&s.main) }

func (s *Session) Other() (*project.Project, error) { return s.client(&s.other) }

func (s *Session) SetOther(proj *project.Project) { s.setClient(&s.other, proj) }

func (s *Session) OtherState() State { return s.state(&s.other) }

func (s *Session) OtherChanged() { s.changed(&s.other) }

func (s *Session) saved() {
	proj, err := s.Project()
	if err != nil || s.onSaved == nil {
		return
	}
	if info := proj.Info(); info.DatPath != "" {
		s.onSaved(info)
	}
}
