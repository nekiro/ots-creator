//go:build !windows

package app

import "errors"

func setClipboardFiles(paths []string, text string) error {
	return errors.ErrUnsupported
}

func clipboardFiles() ([]string, error) {
	return nil, errors.ErrUnsupported
}
