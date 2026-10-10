package app

import (
	"os"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DialogService exposes native file dialogs to the frontend. Each dialog
// is named by a key and opens in the folder it last confirmed.
type DialogService struct {
	st *settings.Store
}

// NewDialogService returns the service; st remembers the dialog folders.
func NewDialogService(st *settings.Store) *DialogService {
	return &DialogService{st: st}
}

// FileFilter is one entry of a dialog filter list, e.g. {"Client files", "*.dat;*.spr;*.otfi"}.
type FileFilter struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

// lastDir returns the folder the dialog last confirmed while it still
// exists; "" leaves the start folder to the system.
func (ds *DialogService) lastDir(key string) string {
	if ds.st == nil {
		return ""
	}
	dir := ds.st.LastDir(key)
	if fi, err := os.Stat(dir); dir == "" || err != nil || !fi.IsDir() {
		return ""
	}
	return dir
}

func (ds *DialogService) remember(key, dir string) {
	if ds.st != nil && dir != "" {
		_ = ds.st.SetLastDir(key, dir)
	}
}

// OpenFiles shows an open dialog. It returns no paths when cancelled.
func (ds *DialogService) OpenFiles(key, title string, filters []FileFilter, multiple bool) ([]string, error) {
	d := application.Get().Dialog.OpenFile().SetTitle(title).CanChooseFiles(true).SetDirectory(ds.lastDir(key))
	for _, f := range filters {
		d.AddFilter(f.Name, f.Pattern)
	}
	var paths []string
	if multiple {
		ps, err := d.PromptForMultipleSelection()
		if err != nil {
			return nil, err
		}
		paths = ps
	} else {
		p, err := d.PromptForSingleSelection()
		if err != nil || p == "" {
			return nil, err
		}
		paths = []string{p}
	}
	if len(paths) > 0 {
		ds.remember(key, filepath.Dir(paths[0]))
	}
	return paths, nil
}

// PickDirectory shows a directory chooser. It returns "" when cancelled.
func (ds *DialogService) PickDirectory(key, title string) (string, error) {
	dir, err := application.Get().Dialog.OpenFile().SetTitle(title).CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true).SetDirectory(ds.lastDir(key)).PromptForSingleSelection()
	if err == nil {
		ds.remember(key, dir)
	}
	return dir, err
}

// SaveFile shows a save dialog. It returns "" when cancelled.
func (ds *DialogService) SaveFile(key, title, filename string, filters []FileFilter) (string, error) {
	d := application.Get().Dialog.SaveFile().SetMessage(title).SetFilename(filename).CanCreateDirectories(true).SetDirectory(ds.lastDir(key))
	for _, f := range filters {
		d.AddFilter(f.Name, f.Pattern)
	}
	path, err := d.PromptForSingleSelection()
	if err == nil && path != "" {
		ds.remember(key, filepath.Dir(path))
	}
	return path, err
}
