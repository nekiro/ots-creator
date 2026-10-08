package app

import (
	"context"
	"errors"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

type fakeUpdater struct {
	release *updater.Release
	calls   []string
	failDL  bool
}

func (f *fakeUpdater) Check(context.Context) (*updater.Release, error) {
	f.calls = append(f.calls, "check")
	return f.release, nil
}

func (f *fakeUpdater) DownloadAndInstall(context.Context) error {
	f.calls = append(f.calls, "download")
	if f.failDL {
		return errors.New("checksum mismatch")
	}
	return nil
}

func (f *fakeUpdater) Restart(context.Context) error {
	f.calls = append(f.calls, "restart")
	return nil
}

func TestUpdateServiceDisabledWithoutUpdater(t *testing.T) {
	s := NewUpdateService("dev", func() Updater { return nil })
	if _, err := s.Check(); !errors.Is(err, ErrUpdatesDisabled) {
		t.Fatalf("check: %v", err)
	}
	if err := s.Install(); !errors.Is(err, ErrUpdatesDisabled) {
		t.Fatalf("install: %v", err)
	}
}

func TestUpdateServiceCheck(t *testing.T) {
	f := &fakeUpdater{}
	s := NewUpdateService("1.0.0", func() Updater { return f })

	info, err := s.Check()
	if err != nil || info.Available || info.Current != "1.0.0" {
		t.Fatalf("up to date: %+v %v", info, err)
	}

	f.release = &updater.Release{Version: "1.1.0"}
	info, err = s.Check()
	if err != nil || !info.Available || info.Release.Version != "1.1.0" {
		t.Fatalf("available: %+v %v", info, err)
	}
}

func TestUpdateServiceInstallRestartsOnlyAfterDownload(t *testing.T) {
	f := &fakeUpdater{failDL: true}
	s := NewUpdateService("1.0.0", func() Updater { return f })
	if err := s.Install(); err == nil {
		t.Fatal("failed download must fail the install")
	}
	if len(f.calls) != 1 || f.calls[0] != "download" {
		t.Fatalf("must not restart after a failed download: %v", f.calls)
	}

	f.failDL = false
	f.calls = nil
	if err := s.Install(); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || f.calls[1] != "restart" {
		t.Fatalf("calls %v", f.calls)
	}
}
