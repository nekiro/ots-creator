package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DialogService exposes native file dialogs to the frontend.
type DialogService struct{}

// FileFilter is one entry of a dialog filter list, e.g. {"Client files", "*.dat;*.spr;*.otfi"}.
type FileFilter struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

// OpenFiles shows an open dialog. It returns no paths when cancelled.
func (*DialogService) OpenFiles(title string, filters []FileFilter, multiple bool) ([]string, error) {
	d := application.Get().Dialog.OpenFile().SetTitle(title).CanChooseFiles(true)
	for _, f := range filters {
		d.AddFilter(f.Name, f.Pattern)
	}
	if multiple {
		return d.PromptForMultipleSelection()
	}
	p, err := d.PromptForSingleSelection()
	if err != nil || p == "" {
		return nil, err
	}
	return []string{p}, nil
}

// PickDirectory shows a directory chooser. It returns "" when cancelled.
func (*DialogService) PickDirectory(title string) (string, error) {
	return application.Get().Dialog.OpenFile().SetTitle(title).CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true).PromptForSingleSelection()
}

// SaveFile shows a save dialog. It returns "" when cancelled.
func (*DialogService) SaveFile(title, filename string, filters []FileFilter) (string, error) {
	d := application.Get().Dialog.SaveFile().SetMessage(title).SetFilename(filename).CanCreateDirectories(true)
	for _, f := range filters {
		d.AddFilter(f.Name, f.Pattern)
	}
	return d.PromptForSingleSelection()
}
