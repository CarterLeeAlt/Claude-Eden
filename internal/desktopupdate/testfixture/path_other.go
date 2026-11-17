//go:build !windows

package main

import "path/filepath"

func canonicalExecutable(path string) (string, error) {
	return filepath.Abs(path)
}
