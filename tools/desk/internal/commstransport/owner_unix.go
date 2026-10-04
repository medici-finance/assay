//go:build unix

package commstransport

import (
	"fmt"
	"os"
	"syscall"
)

func ownedByCurrentUser(path string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("comms endpoint owner cannot be established: %s", path)
	}
	if int(st.Uid) != os.Getuid() {
		return fmt.Errorf("comms endpoint %s is not owned by the current user", path)
	}
	return nil
}
