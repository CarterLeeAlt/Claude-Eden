package paths

import "syscall"

// UnixSocketPathMax includes the terminating NUL in the host's sockaddr_un.
const UnixSocketPathMax = len(syscall.RawSockaddrUnix{}.Path)

// SSHControlSocketMaxName reserves OpenSSH's temporary 16-character suffix.
const SSHControlSocketMaxName = "ssh.sock.1234567890123456"
