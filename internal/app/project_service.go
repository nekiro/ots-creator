package app

import (
	"errors"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/project"
)

// ProjectService opens, creates and compiles clients.
type ProjectService struct {
	session *Session
}

// NewProjectService returns the service.
func NewProjectService(s *Session) *ProjectService { return &ProjectService{session: s} }

// Cancel stops the running export; it then returns ErrCanceled.
func (ps *ProjectService) Cancel() { ps.session.Cancel() }

// Versions returns all known client versions.
func (ps *ProjectService) Versions() []client.Version { return client.Versions() }

// DefaultFeatures returns the features forced by a version.
func (ps *ProjectService) DefaultFeatures(version uint16) client.Features {
	return client.DefaultFeatures(version)
}

// State returns the current project state.
func (ps *ProjectService) State() State { return ps.session.State() }

// Inspect looks at a client directory or any of its dat, spr or otfi files
// and finds the matching files, signatures, version and OTFI features.
func (ps *ProjectService) Inspect(path string) (ClientFiles, error) {
	files, err := inspectClient(path)
	if err == nil {
		files.OTBPath = ps.otbPath(files.DatPath)
	}
	return files, err
}

// OpenRequest selects the client to open.
type OpenRequest struct {
	// Format FormatAssets opens the asset folder in DatPath.
	Format   project.Format  `json:"format"`
	DatPath  string          `json:"datPath"`
	SprPath  string          `json:"sprPath"`
	Version  *client.Version `json:"version"`
	Features client.Features `json:"features"`
	// OTBPath is the items.otb to keep in sync with the client: nil uses
	// the one remembered for it, "" none. It is remembered for the client.
	OTBPath *string `json:"otbPath"`
}

// openProject loads the client a request selects.
func openProject(req OpenRequest) (*project.Project, error) {
	if req.Format == project.FormatAssets {
		return project.OpenAssets(req.DatPath)
	}
	return project.Open(project.OpenOptions{DatPath: req.DatPath, SprPath: req.SprPath, Version: req.Version, Features: req.Features})
}

// Open loads a client and makes it the current project.
func (ps *ProjectService) Open(req OpenRequest) (State, error) {
	p, err := openProject(req)
	if err != nil {
		return ps.session.State(), err
	}
	// The items.otb remembered for the client is optional: a missing or
	// broken file is reported, never stops the client from opening.
	var otbErr error
	path := ps.otbPath(p.Info().DatPath)
	if req.OTBPath != nil {
		path = *req.OTBPath
		otbErr = ps.rememberOTB(p.Info().DatPath, path)
	}
	if path != "" {
		if _, err := p.AttachOTB(path); err != nil {
			otbErr = err
		}
	}
	ps.session.Set(p)
	ps.session.saved()
	st := ps.session.State()
	if otbErr != nil {
		st.OTBError = otbErr.Error()
	}
	return st, nil
}

func (ps *ProjectService) otbPath(client string) string {
	if ps.session.otbPaths == nil || client == "" {
		return ""
	}
	return ps.session.otbPaths.ItemsOTB(client)
}

func (ps *ProjectService) rememberOTB(client, otb string) error {
	if ps.session.otbPaths == nil || client == "" {
		return nil
	}
	return ps.session.otbPaths.SetItemsOTB(client, otb)
}

// AttachItemsOTB keeps an items.otb in sync with the client: compiling
// then updates it too. It is remembered for the client.
func (ps *ProjectService) AttachItemsOTB(path string) (State, error) {
	p, err := ps.session.Project()
	if err != nil {
		return ps.session.State(), err
	}
	if _, err := p.AttachOTB(path); err != nil {
		return ps.session.State(), err
	}
	ps.session.Changed()
	return ps.session.State(), ps.rememberOTB(p.Info().DatPath, path)
}

// DetachItemsOTB stops keeping the items.otb in sync and forgets it.
func (ps *ProjectService) DetachItemsOTB() (State, error) {
	p, err := ps.session.Project()
	if err != nil {
		return ps.session.State(), err
	}
	p.DetachOTB()
	ps.session.Changed()
	return ps.session.State(), ps.rememberOTB(p.Info().DatPath, "")
}

// ItemsOTBPreview reports what compiling would change in the attached
// items.otb.
func (ps *ProjectService) ItemsOTBPreview() (project.OTBReport, error) {
	p, err := ps.session.Project()
	if err != nil {
		return project.OTBReport{}, err
	}
	return p.OTBPreview()
}

// New creates an empty client.
func (ps *ProjectService) New(v client.Version, f client.Features) State {
	ps.session.Set(project.New(v, f))
	return ps.session.State()
}

// Close closes the current client.
func (ps *ProjectService) Close() {
	ps.session.Set(nil)
}

// CompileRequest selects the output of a compilation.
type CompileRequest struct {
	// Format FormatAssets writes an asset folder to DatPath.
	Format    project.Format  `json:"format"`
	DatPath   string          `json:"datPath"`
	SprPath   string          `json:"sprPath"`
	Version   client.Version  `json:"version"`
	Features  client.Features `json:"features"`
	WriteOTFI bool            `json:"writeOtfi"`
}

// Compile saves to the current files with the current settings.
func (ps *ProjectService) Compile() (project.CompileStats, error) {
	p, err := ps.session.Project()
	if err != nil {
		return project.CompileStats{}, err
	}
	info := p.Info()
	if info.DatPath == "" || (info.Format != project.FormatAssets && info.SprPath == "") {
		return project.CompileStats{}, errors.New("the client has never been saved, use Compile As")
	}
	return ps.CompileAs(CompileRequest{Format: info.Format, DatPath: info.DatPath, SprPath: info.SprPath, Version: info.Version, Features: info.Features})
}

// CompileAs saves to new files, optionally with another version/features.
// For an asset folder it reports the sheets it removed.
func (ps *ProjectService) CompileAs(req CompileRequest) (project.CompileStats, error) {
	p, err := ps.session.Project()
	if err != nil {
		return project.CompileStats{}, err
	}
	var stats project.CompileStats
	err = p.Compile(project.CompileOptions{Format: req.Format, DatPath: req.DatPath, SprPath: req.SprPath, Version: req.Version, Features: req.Features, WriteOTFI: req.WriteOTFI, Progress: ps.session.reporter("Compiling"), Stats: &stats})
	ps.session.Changed()
	if err == nil {
		ps.session.saved()
		// Compiled to new files: the items.otb stays with the client.
		if otb := p.OTB(); otb != nil {
			_ = ps.rememberOTB(p.Info().DatPath, otb.Path)
		}
	}
	return stats, err
}

// Warnings lists properties that would be lost when compiling to a format
// and version.
func (ps *ProjectService) Warnings(format project.Format, v client.Version) ([]string, error) {
	p, err := ps.session.Project()
	if err != nil {
		return nil, err
	}
	return p.Warnings(format, v), nil
}

// Undo reverts the last edit and returns its label.
func (ps *ProjectService) Undo() (string, error) {
	return ps.history((*project.Project).Undo)
}

// Redo re-applies the last undone edit and returns its label.
func (ps *ProjectService) Redo() (string, error) {
	return ps.history((*project.Project).Redo)
}

// history runs Undo or Redo and returns the label of the step ("" when
// there was none).
func (ps *ProjectService) history(step func(*project.Project) string) (string, error) {
	p, err := ps.session.Project()
	if err != nil {
		return "", err
	}
	label := step(p)
	if label != "" {
		ps.session.Changed()
	}
	return label, nil
}
