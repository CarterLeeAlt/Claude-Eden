package paths

import (
	"syscall"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

// UnixSocketPathMax includes the terminating NUL in the host's sockaddr_un.
const UnixSocketPathMax = len(syscall.RawSockaddrUnix{}.Path)

// SSHControlSocketMaxName reserves OpenSSH's temporary 16-character suffix.
const SSHControlSocketMaxName = filenames.LongestSock
