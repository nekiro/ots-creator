package app

import (
	"context"
	"errors"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// UpdateRepository is the GitHub repository that publishes releases.
const UpdateRepository = "nekiro/ots-creator"

// UpdateChecksums names the release asset with SHA-256 sums of the other
// assets (sha256sum format). The release workflow uploads it.
const UpdateChecksums = "checksums.txt"

// ErrUpdatesDisabled is returned by development builds, which have no version.
var ErrUpdatesDisabled = errors.New("updates are disabled in development builds")

// Updater is the part of the Wails updater (app.Updater) the service uses.
type Updater interface {
	Check(ctx context.Context) (*updater.Release, error)
	DownloadAndInstall(ctx context.Context) error
	Restart(ctx context.Context) error
}

// UpdateInfo is the result of an update check.
type UpdateInfo struct {
	Current   string           `json:"current"`
	Available bool             `json:"available"`
	Release   *updater.Release `json:"release"`
}

// UpdateService exposes the Wails updater to the frontend. Progress is
// reported through the standard updater events (wails:updater:*).
type UpdateService struct {
	version string
	get     func() Updater
}

// NewUpdateService returns the service for the running version. get returns
// the updater, or nil while updates are disabled; it is a function because
// the Wails updater only exists once the application has been created.
func NewUpdateService(version string, get func() Updater) *UpdateService {
	return &UpdateService{version: version, get: get}
}

func (s *UpdateService) updater() (Updater, error) {
	if s.get == nil {
		return nil, ErrUpdatesDisabled
	}
	u := s.get()
	if u == nil {
		return nil, ErrUpdatesDisabled
	}
	return u, nil
}

// Version returns the running version ("dev" for local builds).
func (s *UpdateService) Version() string { return s.version }

// Check asks GitHub for a newer release.
func (s *UpdateService) Check() (UpdateInfo, error) {
	info := UpdateInfo{Current: s.version}
	u, err := s.updater()
	if err != nil {
		return info, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := u.Check(ctx)
	if err != nil {
		return info, err
	}
	info.Release = r
	info.Available = r != nil
	return info, nil
}

// Install downloads and verifies the release found by Check, swaps the
// executable and restarts into the new version.
func (s *UpdateService) Install() error {
	u, err := s.updater()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := u.DownloadAndInstall(ctx); err != nil {
		return err
	}
	return u.Restart(ctx)
}
