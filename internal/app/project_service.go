package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekiro/ots-creator/internal/assets"
	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/project"
)

// ProjectService opens, creates and compiles clients.
type ProjectService struct {
	s *Session
}

// NewProjectService returns the service.
func NewProjectService(s *Session) *ProjectService { return &ProjectService{s: s} }

// Versions returns all known client versions.
func (ps *ProjectService) Versions() []client.Version { return client.Versions() }

// DefaultFeatures returns the features forced by a version.
func (ps *ProjectService) DefaultFeatures(version uint16) client.Features {
	return client.DefaultFeatures(version)
}

// State returns the current project state.
func (ps *ProjectService) State() State { return ps.s.State() }

// ClientFiles describes a dat/spr pair found on disk before opening it.
type ClientFiles struct {
	// Format is FormatAssets for a Tibia 12+ asset folder; DatPath is then
	// the folder and Detected its version.
	Format       project.Format  `json:"format"`
	DatPath      string          `json:"datPath"`
	SprPath      string          `json:"sprPath"`
	DatSignature uint32          `json:"datSignature"`
	SprSignature uint32          `json:"sprSignature"`
	Detected     *client.Version `json:"detected"`
	Features     client.Features `json:"features"`
	HasOTFI      bool            `json:"hasOtfi"`
}

// Inspect looks at a client directory or any of its dat, spr or otfi files
// and finds the matching files, signatures, version and OTFI features.
func (ps *ProjectService) Inspect(path string) (ClientFiles, error) {
	var cf ClientFiles
	if dir, err := assets.FindDir(path); err == nil {
		v := project.AssetsVersion(dir)
		return ClientFiles{Format: project.FormatAssets, DatPath: dir, Detected: &v, Features: client.DefaultFeatures(v.Value)}, nil
	}
	cf.Format = project.FormatDat
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		found, err := findClientFile(path)
		if err != nil {
			return cf, err
		}
		path = found
	}
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	cf.DatPath = filepath.Join(dir, base+".dat")
	cf.SprPath = filepath.Join(dir, base+".spr")
	if otfiBytes, err := os.ReadFile(filepath.Join(dir, base+".otfi")); err == nil {
		o, err := client.ParseOTFI(otfiBytes)
		if err != nil {
			return cf, err
		}
		cf.HasOTFI = true
		cf.Features = o.Features
		if o.MetadataFile != "" {
			cf.DatPath = filepath.Join(dir, o.MetadataFile)
		}
		if o.SpritesFile != "" {
			cf.SprPath = filepath.Join(dir, o.SpritesFile)
		}
	}
	var err error
	if cf.DatSignature, err = readSignature(cf.DatPath); err != nil {
		return cf, err
	}
	if cf.SprSignature, err = readSignature(cf.SprPath); err != nil {
		return cf, err
	}
	if v, ok := client.FindBySignatures(cf.DatSignature, cf.SprSignature); ok {
		cf.Detected = &v
		if !cf.HasOTFI {
			cf.Features = client.DefaultFeatures(v.Value)
		}
	}
	if cf.Features.SpriteSize == 0 {
		cf.Features.SpriteSize = client.DefaultSpriteSize
	}
	return cf, nil
}

// findClientFile picks the client inside dir: an .otfi first (it names its
// dat and spr), then a .dat, preferring the Tibia base name.
func findClientFile(dir string) (string, error) {
	for _, ext := range []string{".otfi", ".dat"} {
		if p := filepath.Join(dir, "Tibia"+ext); fileExists(p) {
			return p, nil
		}
		matches, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
		if len(matches) > 0 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("no .dat or .otfi file in %s", dir)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func readSignature(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var b [4]byte
	if _, err := f.Read(b[:]); err != nil {
		return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24, nil
}

// OpenRequest selects the client to open.
type OpenRequest struct {
	// Format FormatAssets opens the asset folder in DatPath.
	Format   project.Format  `json:"format"`
	DatPath  string          `json:"datPath"`
	SprPath  string          `json:"sprPath"`
	Version  *client.Version `json:"version"`
	Features client.Features `json:"features"`
}

// Open loads a client and makes it the current project.
func (ps *ProjectService) Open(req OpenRequest) (State, error) {
	var p *project.Project
	var err error
	if req.Format == project.FormatAssets {
		p, err = project.OpenAssets(req.DatPath)
	} else {
		p, err = project.Open(project.OpenOptions{DatPath: req.DatPath, SprPath: req.SprPath, Version: req.Version, Features: req.Features})
	}
	if err != nil {
		return ps.s.State(), err
	}
	ps.s.Set(p)
	ps.s.saved()
	return ps.s.State(), nil
}

// New creates an empty client.
func (ps *ProjectService) New(v client.Version, f client.Features) State {
	ps.s.Set(project.New(v, f))
	return ps.s.State()
}

// Close closes the current client.
func (ps *ProjectService) Close() {
	ps.s.Set(nil)
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
func (ps *ProjectService) Compile() error {
	p, err := ps.s.Project()
	if err != nil {
		return err
	}
	info := p.Info()
	if info.DatPath == "" || (info.Format != project.FormatAssets && info.SprPath == "") {
		return errors.New("the client has never been saved, use Compile As")
	}
	return ps.CompileAs(CompileRequest{Format: info.Format, DatPath: info.DatPath, SprPath: info.SprPath, Version: info.Version, Features: info.Features})
}

// CompileAs saves to new files, optionally with another version/features.
func (ps *ProjectService) CompileAs(req CompileRequest) error {
	p, err := ps.s.Project()
	if err != nil {
		return err
	}
	err = p.Compile(project.CompileOptions{Format: req.Format, DatPath: req.DatPath, SprPath: req.SprPath, Version: req.Version, Features: req.Features, WriteOTFI: req.WriteOTFI})
	ps.s.Changed()
	if err == nil {
		ps.s.saved()
	}
	return err
}

// Warnings lists properties that would be lost when compiling to a format
// and version.
func (ps *ProjectService) Warnings(format project.Format, v client.Version) ([]string, error) {
	p, err := ps.s.Project()
	if err != nil {
		return nil, err
	}
	return p.Warnings(format, v), nil
}

// Undo reverts the last edit and returns its label.
func (ps *ProjectService) Undo() (string, error) {
	p, err := ps.s.Project()
	if err != nil {
		return "", err
	}
	label := p.Undo()
	if label != "" {
		ps.s.Changed()
	}
	return label, nil
}

// Redo re-applies the last undone edit and returns its label.
func (ps *ProjectService) Redo() (string, error) {
	p, err := ps.s.Project()
	if err != nil {
		return "", err
	}
	label := p.Redo()
	if label != "" {
		ps.s.Changed()
	}
	return label, nil
}
