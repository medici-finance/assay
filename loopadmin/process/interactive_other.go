//go:build !unix

package process

import "os/exec"

func newInteractiveProcessTree(cmd *exec.Cmd) (processTree, error) { return newProcessTree(cmd) }
