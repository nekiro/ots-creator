// Package app connects the editing core to the Wails frontend: bound
// services, events and the binary resource handler.
package app

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nekiro/ots-creator/internal/project"
)

// ErrNoProject is returned when an operation needs an open client.
var ErrNoProject = errors.New("no client is open")

// EventProjectChanged is emitted with a State after every change.
const EventProjectChanged = "project:changed"

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
}

// Session owns the currently open project.
type Session struct {
	mu     sync.RWMutex
	p      *project.Project
	rev    atomic.Uint64
	notify Notifier
	// onSaved is called after a client is opened or compiled to disk.
	onSaved func(project.Info)
}

// NewSession returns an empty session. notify may be nil.
func NewSession(notify Notifier) *Session {
	if notify == nil {
		notify = func(string, any) {}
	}
	s := &Session{notify: notify}
	// Resource URLs carry the revision and are cached as immutable, also on
	// disk by the webview; starting from the launch time keeps URLs of one
	// run from matching cached images of an earlier run. Milliseconds stay
	// exact as JavaScript numbers.
	s.rev.Store(uint64(time.Now().UnixMilli()))
	return s
}

// Project returns the open project.
func (s *Session) Project() (*project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.p == nil {
		return nil, ErrNoProject
	}
	return s.p, nil
}

// Set replaces the open project (nil closes it).
func (s *Session) Set(p *project.Project) {
	s.mu.Lock()
	s.p = p
	s.mu.Unlock()
	if p != nil {
		p.TakeDelta() // a new project replaces everything anyway
	}
	s.rev.Add(1)
	st := s.State()
	st.Delta = &project.Delta{All: true}
	s.notify(EventProjectChanged, st)
}

// State returns the current snapshot.
func (s *Session) State() State {
	s.mu.RLock()
	p := s.p
	s.mu.RUnlock()
	st := State{Rev: s.rev.Load()}
	if p != nil {
		st.Open = true
		st.Info = p.Info()
	}
	return st
}

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

// Changed bumps the revision and notifies the frontend.
func (s *Session) Changed() {
	s.rev.Add(1)
	st := s.State()
	d := project.Delta{All: true}
	if p, err := s.Project(); err == nil {
		d = p.TakeDelta()
	}
	st.Delta = &d
	s.notify(EventProjectChanged, st)
}
