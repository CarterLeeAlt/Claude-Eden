//go:build windows

package main

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func canonicalExecutable(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	buffer := make([]uint16, 32768)
	length, err := windows.GetFinalPathNameByHandle(windows.Handle(file.Fd()), &buffer[0], uint32(len(buffer)), 0)
	if err != nil {
		return "", err
	}
	result := windows.UTF16ToString(buffer[:length])
	return strings.TrimPrefix(result, `\\?\`), nil
}
