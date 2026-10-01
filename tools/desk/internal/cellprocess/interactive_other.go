//go:build !unix

package cellprocess

import "os/exec"

func newInteractiveProcessTree(cmd *exec.Cmd) (processTree, error) { return newProcessTree(cmd) }
