//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() { fmt.Fprintln(os.Stderr, "Claude Eden 桌面仅支持 Windows 10 x64"); os.Exit(1) }
