package app

import (
	"errors"
	"path/filepath"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/formats/itemsxml"
	"github.com/nekiro/ots-creator/internal/fsutil"
)

type ProjectService struct {
	session *Session
}

func NewProjectService(session *Session) *ProjectService { return &ProjectService{session: session} }

func (ps *ProjectService) Cancel() { ps.session.Cancel() }

func (ps *ProjectService) Versions() []client.Version { return client.Versions() }

// StartupWarnings returns the problems found while starting, such as a
// damaged settings file that was replaced with defaults, once.
func (ps *ProjectService) StartupWarnings() []string { return ps.session.takeWarnings() }

func (ps *ProjectService) State() State { return ps.session.State() }

func (ps *ProjectService) Inspect(path string) (ClientFiles, error) {
	files, err := inspectClient(path)
	if err == nil {
		files.OTBPath = ps.otbPath(files.DatPath)
	}
	return files, err
}

type OpenRequest struct {
	Format   project.Format  `json:"format"`
	DatPath  string          `json:"datPath"`
	SprPath  string          `json:"sprPath"`
	Version  *client.Version `json:"version"`
	Features client.Features `json:"features"`
	// OTBPath is the items.otb to keep in sync with the client: nil uses
	// the one remembered for it, "" none. It is remembered for the client.
	OTBPath *string `json:"otbPath"`
}

func openProject(req OpenRequest) (*project.Project, error) {
	if req.Format == project.FormatAssets {
		return project.OpenAssets(req.DatPath)
	}
	return project.Open(project.OpenOptions{DatPath: req.DatPath, SprPath: req.SprPath, Version: req.Version, Features: req.Features})
}

func (ps *ProjectService) Open(req OpenRequest) (State, error) {
	proj, err := openProject(req)
	if err != nil {
		return ps.session.State(), err
	}
	// The items.otb remembered for the client is optional: a missing or
	// broken file is reported, never stops the client from opening.
	var otbErr error
	path := ps.otbPath(proj.Info().DatPath)
	if req.OTBPath != nil {
		path = *req.OTBPath
		otbErr = ps.rememberOTB(proj.Info().DatPath, path)
	}
	if path != "" {
		if _, err := proj.AttachOTB(path); err != nil {
			otbErr = err
		}
	}
	// So is the items.xml remembered for it.
	var xmlErr error
	if path := ps.session.rememberedXML(proj.Info().DatPath); path != "" {
		_, xmlErr = proj.AttachXML(path)
	}
	ps.session.Set(proj)
	ps.session.saved()
	state := ps.session.State()
	if otbErr != nil {
		state.OTBError = otbErr.Error()
	}
	if xmlErr != nil {
		state.XMLError = xmlErr.Error()
	}
	return state, nil
}

func (ps *ProjectService) otbPath(client string) string { return ps.session.rememberedOTB(client) }

func (ps *ProjectService) rememberOTB(client, otb string) error {
	return ps.session.rememberOTB(client, otb)
}

// AttachItemsOTB attaches the items.otb of the server to the client:
// compiling then updates it too, and maps on server ids read their ids with
// it. It is remembered for the client.
func (ps *ProjectService) AttachItemsOTB(path string) (State, error) {
	err := ps.session.attachItemsOTB(path)
	return ps.session.State(), err
}

func (ps *ProjectService) DetachItemsOTB() (State, error) {
	err := ps.session.attachItemsOTB("")
	return ps.session.State(), err
}

func (ps *ProjectService) ItemsOTBPreview() (project.OTBReport, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return project.OTBReport{}, err
	}
	return proj.OTBPreview()
}

// ItemServerIDs returns the server ids the attached items.otb gives the
// client items once compiled.
func (ps *ProjectService) ItemServerIDs(clientIDs []uint32) (map[uint32]uint16, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return nil, err
	}
	return proj.OTBServerIDs(clientIDs)
}

// AttachItemsXML attaches the items.xml of the server to the client: the
// items show and edit its data, and compiling writes the edits and an entry
// for every new item. It is remembered for the client.
func (ps *ProjectService) AttachItemsXML(path string) (State, error) {
	err := ps.session.attachItemsXML(path)
	return ps.session.State(), err
}

func (ps *ProjectService) DetachItemsXML() (State, error) {
	err := ps.session.attachItemsXML("")
	return ps.session.State(), err
}

// FindItemsXML looks for the items.xml of the server next to the attached
// items.otb and around the client; "" when there is none.
func (ps *ProjectService) FindItemsXML() string {
	proj, err := ps.session.Project()
	if err != nil {
		return ""
	}
	info := proj.Info()
	starts := []string{info.DatPath}
	if info.ItemsOTB != nil {
		starts = []string{info.ItemsOTB.Path, info.DatPath}
	}
	for _, start := range starts {
		if found := detectServerData(start).XMLPath; found != "" {
			return found
		}
	}
	return ""
}

// FindItemsOTB returns the items.otb next to the attached items.xml, ""
// when there is none.
func (ps *ProjectService) FindItemsOTB() string {
	proj, err := ps.session.Project()
	if err != nil {
		return ""
	}
	if xml := proj.XML(); xml != nil {
		if path := filepath.Join(filepath.Dir(xml.Path), "items.otb"); fsutil.FileExists(path) {
			return path
		}
	}
	return ""
}

func (ps *ProjectService) ItemsXMLPreview() (project.XMLReport, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return project.XMLReport{}, err
	}
	return proj.XMLPreview()
}

// SetItemsXMLMapping stores the attribute map of the attached items.xml:
// the properties of the client items compiling writes to it as attributes,
// for servers on client ids.
func (ps *ProjectService) SetItemsXMLMapping(mapping []project.XMLMapping) (State, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return ps.session.State(), err
	}
	if err := proj.SetXMLMapping(mapping); err != nil {
		return ps.session.State(), err
	}
	ps.session.Changed()
	return ps.session.State(), nil
}

// ItemXML returns the items.xml entry of a client item.
func (ps *ProjectService) ItemXML(clientID uint32) (project.XMLItem, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return project.XMLItem{}, err
	}
	return proj.ItemXML(clientID)
}

// SetItemXML edits the items.xml entry of a client item; compiling writes
// the file.
func (ps *ProjectService) SetItemXML(clientID uint32, item itemsxml.Item) error {
	proj, err := ps.session.Project()
	if err != nil {
		return err
	}
	if err := proj.SetItemXML(clientID, item); err != nil {
		return err
	}
	ps.session.Changed()
	return nil
}

func (ps *ProjectService) New(version client.Version, features client.Features) State {
	ps.session.Set(project.New(version, features))
	return ps.session.State()
}

func (ps *ProjectService) Close() {
	ps.session.Set(nil)
}

type CompileRequest struct {
	Format    project.Format  `json:"format"`
	DatPath   string          `json:"datPath"`
	SprPath   string          `json:"sprPath"`
	Version   client.Version  `json:"version"`
	Features  client.Features `json:"features"`
	WriteOTFI bool            `json:"writeOtfi"`
}

func (ps *ProjectService) Compile() (project.CompileStats, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return project.CompileStats{}, err
	}
	info := proj.Info()
	if info.DatPath == "" || (info.Format != project.FormatAssets && info.SprPath == "") {
		return project.CompileStats{}, errors.New("the client has never been saved, use Compile As")
	}
	return ps.CompileAs(currentFilesRequest(info))
}

func currentFilesRequest(info project.Info) CompileRequest {
	return CompileRequest{Format: info.Format, DatPath: info.DatPath, SprPath: info.SprPath, Version: info.Version, Features: info.Features}
}

func (req CompileRequest) options(progress func(done, total int)) project.CompileOptions {
	return project.CompileOptions{Format: req.Format, DatPath: req.DatPath, SprPath: req.SprPath, Version: req.Version, Features: req.Features, WriteOTFI: req.WriteOTFI, Progress: progress}
}

// CompileAs saves to new files, optionally with another version/features.
// For an asset folder it reports the sheets it removed.
func (ps *ProjectService) CompileAs(req CompileRequest) (project.CompileStats, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return project.CompileStats{}, err
	}
	var stats project.CompileStats
	options := req.options(ps.session.reporter("Compiling"))
	options.Stats = &stats
	err = proj.Compile(options)
	ps.session.Changed()
	if err == nil {
		ps.session.saved()
		// Compiled to new files: the items.otb and items.xml stay with
		// the client.
		if otb := proj.OTB(); otb != nil {
			_ = ps.rememberOTB(proj.Info().DatPath, otb.Path)
		}
		if xml := proj.XML(); xml != nil {
			_ = ps.session.rememberXML(proj.Info().DatPath, xml.Path)
		}
	}
	return stats, err
}

// Warnings lists properties that would be lost when compiling to a format
// and version.
func (ps *ProjectService) Warnings(format project.Format, version client.Version) ([]string, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return nil, err
	}
	return proj.Warnings(format, version), nil
}

func (ps *ProjectService) Undo() (string, error) {
	return ps.history((*project.Project).Undo)
}

func (ps *ProjectService) Redo() (string, error) {
	return ps.history((*project.Project).Redo)
}

func (ps *ProjectService) history(step func(*project.Project) string) (string, error) {
	proj, err := ps.session.Project()
	if err != nil {
		return "", err
	}
	label := step(proj)
	if label != "" {
		ps.session.Changed()
	}
	return label, nil
}
