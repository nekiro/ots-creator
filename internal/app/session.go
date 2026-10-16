// Package app connects the editing core to the Wails frontend: bound
// services, events and the binary resource handler.
package app

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nekiro/ots-creator/internal/core/project"
)

// ErrNoProject is returned when an operation needs an open client.
var ErrNoProject = errors.New("no client is open")

// ErrNoOther is returned when an operation needs a second client.
var ErrNoOther = errors.New("no second client is open")

// EventProjectChanged is emitted with a State after every change.
const EventProjectChanged = "project:changed"

// EventOtherChanged is emitted with a State after every change of the
// second client (see CompareService).
const EventOtherChanged = "compare:changed"

// Notifier delivers events to the frontend.
type Notifier func(name string, data any)

// State is the project snapshot sent to the frontend.
type State struct {
	Open bool         `json:"open"`
	Info project.Info `json:"info"`
	// Rev increases on every change.
	Rev uint64 `json:"rev"`
	// Delta lists what changed with this revision, so the frontend only
	// reloads those thumbnails and sprites. It is nil in snapshots that are
	// not change events (treat as "all").
	Delta *project.Delta `json:"delta"`
	// OTBError is set by Open when the items.otb remembered for the client
	// could not be read; the client is open without it.
	OTBError string `json:"otbError,omitempty"`
}

// clientSlot is one open client of the session: the main one or the
// second one it is compared with.
type clientSlot struct {
	project *project.Project
	// missing is the error when no client is open in the slot.
	missing error
	// event is emitted with the State of the slot when it changes.
	event string
}

// Session owns the currently open project and an optional second client
// it is compared with.
type Session struct {
	mu    sync.RWMutex
	main  clientSlot
	other clientSlot
	// rev is shared by both clients, so resource URLs of both stay unique.
	rev    atomic.Uint64
	notify Notifier
	// canceled stops the running parallel operation (see Cancel).
	canceled atomic.Bool
	// exporting counts running parallel operations (exports).
	exporting atomic.Int32
	// onSaved is called after a client is opened or compiled to disk.
	onSaved func(project.Info)
	// otbPaths remembers the items.otb of each client (nil: not kept).
	otbPaths otbPaths
}

// otbPaths stores which items.otb is kept in sync with a client.
type otbPaths interface {
	ItemsOTB(client string) string
	SetItemsOTB(client, otb string) error
}

// NewSession returns an empty session. notify may be nil.
func NewSession(notify Notifier) *Session {
	if notify == nil {
		notify = func(string, any) {}
	}
	s := &Session{
		notify: notify,
		main:   clientSlot{missing: ErrNoProject, event: EventProjectChanged},
		other:  clientSlot{missing: ErrNoOther, event: EventOtherChanged},
	}
	// Resource URLs carry the revision and are cached as immutable, also on
	// disk by the webview; starting from the launch time keeps URLs of one
	// run from matching cached images of an earlier run. Milliseconds stay
	// exact as JavaScript numbers.
	s.rev.Store(uint64(time.Now().UnixMilli()))
	return s
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

// setClient replaces the client of a slot (nil closes it).
func (s *Session) setClient(slot *clientSlot, p *project.Project) {
	s.mu.Lock()
	slot.project = p
	s.mu.Unlock()
	if p != nil {
		p.TakeDelta() // a new client replaces everything anyway
	}
	s.rev.Add(1)
	state := s.state(slot)
	state.Delta = &project.Delta{All: true}
	s.notify(slot.event, state)
}

func (s *Session) state(slot *clientSlot) State {
	s.mu.RLock()
	p := slot.project
	s.mu.RUnlock()
	state := State{Rev: s.rev.Load()}
	if p != nil {
		state.Open = true
		state.Info = p.Info()
	}
	return state
}

// changed bumps the revision and notifies the frontend about a slot.
func (s *Session) changed(slot *clientSlot) {
	s.rev.Add(1)
	state := s.state(slot)
	delta := project.Delta{All: true}
	if p, err := s.client(slot); err == nil {
		delta = p.TakeDelta()
	}
	state.Delta = &delta
	s.notify(slot.event, state)
}

// Project returns the open project.
func (s *Session) Project() (*project.Project, error) { return s.client(&s.main) }

// Set replaces the open project (nil closes it).
func (s *Session) Set(p *project.Project) { s.setClient(&s.main, p) }

// State returns the current snapshot.
func (s *Session) State() State { return s.state(&s.main) }

// Changed bumps the revision and notifies the frontend.
func (s *Session) Changed() { s.changed(&s.main) }

// Other returns the second client.
func (s *Session) Other() (*project.Project, error) { return s.client(&s.other) }

// SetOther replaces the second client (nil closes it).
func (s *Session) SetOther(p *project.Project) { s.setClient(&s.other, p) }

// OtherState returns the snapshot of the second client.
func (s *Session) OtherState() State { return s.state(&s.other) }

// OtherChanged bumps the revision and notifies the frontend about the
// second client.
func (s *Session) OtherChanged() { s.changed(&s.other) }

// saved reports that the open client now lives at its dat/spr paths.
func (s *Session) saved() {
	p, err := s.Project()
	if err != nil || s.onSaved == nil {
		return
	}
	if info := p.Info(); info.DatPath != "" {
		s.onSaved(info)
	}
}
