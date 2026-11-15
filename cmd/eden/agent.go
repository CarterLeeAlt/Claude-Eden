package main

import (
	"io"

	"github.com/sylvinus/agent-vm/internal/agentcli"
)

func runAgent(args []string, stdin io.Reader, stdout io.Writer) int {
	return agentcli.Main(args, stdin, stdout)
}
