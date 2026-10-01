//go:build !unix && !windows

package cellprocess

import (
	"errors"
	"os/exec"
)

func newProcessTree(*exec.Cmd) (processTree, error) {
	return nil, errors.New("bounded process-tree execution is unsupported on this platform")
}
