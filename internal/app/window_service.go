package app

import (
	"fmt"
	"sync"
	"sync/atomic"
)

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
	session *Session
	// Maps reports unsaved maps.
	Maps  *MapService
	close func()
	// OpenWindow opens another window of the app at a frontend URL (set by
	// main). closed is called when it closes; it returns what focuses it.
	OpenWindow func(url, title string, width, height int, closed func()) (focus func())
	// playMu guards focusPlay, set while the play window is open.
	playMu    sync.Mutex
	focusPlay func()
	unapplied atomic.Bool
	force     atomic.Bool
}

// NewWindowService returns the guard. closeWindow closes the main window.
func NewWindowService(session *Session, closeWindow func()) *WindowService {
	return &WindowService{session: session, close: closeWindow}
}

// SetUnapplied tells the backend whether the editor holds edits that were
// not applied to the client yet.
func (ws *WindowService) SetUnapplied(unapplied bool) { ws.unapplied.Store(unapplied) }

// EventPlayMove carries a PlayMove to the open play window: the player
// starts again there.
const EventPlayMove = "play:move"

// PlayMove is where the play window's player starts again.
type PlayMove struct {
	Map int `json:"map"`
	X   int `json:"x"`
	Y   int `json:"y"`
	Z   int `json:"z"`
}

// OpenPlayWindow shows the play view of a map in its own window, the
// player at a position. There is one play window: when it is open, its
// player moves there and it comes to the front.
func (ws *WindowService) OpenPlayWindow(mapID, x, y, z int) {
	ws.playMu.Lock()
	focus := ws.focusPlay
	if focus == nil && ws.OpenWindow != nil {
		url := fmt.Sprintf("/?play=%d&x=%d&y=%d&z=%d", mapID, x, y, z)
		ws.focusPlay = ws.OpenWindow(url, "OTS Creator - Play", 1100, 860, func() {
			ws.playMu.Lock()
			ws.focusPlay = nil
			ws.playMu.Unlock()
		})
	}
	ws.playMu.Unlock()
	if focus != nil {
		ws.session.notify(EventPlayMove, PlayMove{Map: mapID, X: x, Y: y, Z: z})
		focus()
	}
}

// PlayWindowOpen reports whether the play window is open.
func (ws *WindowService) PlayWindowOpen() bool {
	ws.playMu.Lock()
	defer ws.playMu.Unlock()
	return ws.focusPlay != nil
}

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
		ws.session.notify(EventCloseRequested, nil)
		return true
	}
}

func (ws *WindowService) unsaved() bool {
	if ws.unapplied.Load() || ws.session.exporting.Load() > 0 {
		return true
	}
	if ws.Maps != nil && ws.Maps.AnyChanged() {
		return true
	}
	if other := ws.session.OtherState(); other.Open && other.Info.Changed {
		return true
	}
	state := ws.session.State()
	return state.Open && state.Info.Changed
}
