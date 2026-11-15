package vmname

import (
	"path/filepath"

	"github.com/sylvinus/agent-vm/internal/paths"
)

// HomeLimit fits the longest Lima socket in this profile's UNIX socket path.
// An Eden-managed storage link keeps the original logical home after relocation.
func HomeLimit(home string, logical bool) Limit {
	real, rest := home, ""
	for !logical {
		if p, err := filepath.EvalSymlinks(real); err == nil {
			real = filepath.Join(p, rest)
			break
		}
		parent := filepath.Dir(real)
		if parent == real {
			real = home
			break
		}
		rest = filepath.Join(filepath.Base(real), rest)
		real = parent
	}
	// <home>/<name>/<socket>, leaving room for the terminating NUL.
	n := paths.UnixSocketPathMax - 1 - len(real) - len("/") - len("/"+paths.SSHControlSocketMaxName)
	return Limit(max(1, min(n, 76)))
}
