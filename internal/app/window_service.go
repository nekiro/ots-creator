package app

import "sync/atomic"

// EventCloseRequested is emitted when the user closes the window while
// there are changes that would be lost or an export runs. The frontend asks for confirmation
// and calls WindowService.Quit to close anyway.
const EventCloseRequested = "app:close-requested"

// EventFilesDropped carries a FilesDropped for files dropped onto the window.
const EventFilesDropped = "app:files-dropped"

// FilesDropped lists dropped files and the id of the drop target element
// (the closest element with data-file-drop-target).
type FilesDropped struct {
	Paths  []string `json:"paths"`
	Target string   `json:"target"`
}

// WindowService guards the main window against closing with unsaved work.
type WindowService struct {
	s         *Session
	close     func()
	unapplied atomic.Bool
	force     atomic.Bool
}

// NewWindowService returns the guard. close closes the main window.
func NewWindowService(s *Session, close func()) *WindowService {
	return &WindowService{s: s, close: close}
}

// SetUnapplied tells the backend whether the editor holds edits that were
// not applied to the client yet.
func (ws *WindowService) SetUnapplied(v bool) { ws.unapplied.Store(v) }

// Quit closes the window without asking.
func (ws *WindowService) Quit() {
	ws.force.Store(true)
	ws.close()
}

// BlockClose returns the hook for the window closing event: it reports
// whether the close must wait for the user and then emits
// EventCloseRequested. It is a function, not a method, so it is not bound
// to the frontend.
func BlockClose(ws *WindowService) func() bool {
	return func() bool {
		if ws.force.Load() || !ws.unsaved() {
			return false
		}
		ws.s.notify(EventCloseRequested, nil)
		return true
	}
}

func (ws *WindowService) unsaved() bool {
	if ws.unapplied.Load() || ws.s.exporting.Load() > 0 {
		return true
	}
	if st := ws.s.OtherState(); st.Open && st.Info.Changed {
		return true
	}
	st := ws.s.State()
	return st.Open && st.Info.Changed
}
