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

const EventFilesDropped = "app:files-dropped"

// FilesDropped lists dropped files and the id of the drop target element
// (the closest element with data-file-drop-target).
type FilesDropped struct {
	Paths  []string `json:"paths"`
	Target string   `json:"target"`
}

type WindowService struct {
	session *Session
	Maps    *MapService
	close   func()
	// OpenWindow opens another window of the app at a frontend URL (set by
	// main). closed is called when it closes; it returns what focuses it.
	OpenWindow func(url, title string, width, height int, closed func()) (focus func())
	playMu     sync.Mutex
	focusPlay  func()
	unapplied  atomic.Bool
	force      atomic.Bool
}

func NewWindowService(session *Session, closeWindow func()) *WindowService {
	return &WindowService{session: session, close: closeWindow}
}

// SetUnapplied tells the backend whether the editor holds edits that were
// not applied to the client yet.
func (ws *WindowService) SetUnapplied(unapplied bool) { ws.unapplied.Store(unapplied) }

const EventPlayMove = "play:move"

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

func (ws *WindowService) PlayWindowOpen() bool {
	ws.playMu.Lock()
	defer ws.playMu.Unlock()
	return ws.focusPlay != nil
}

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
