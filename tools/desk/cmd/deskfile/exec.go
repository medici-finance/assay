package main

import (
	"os/exec"
)

// execCommand is the seam through which deskfile starts a child process. Since the write-verbs-C
// migration the ONLY child it starts is `desktoken` (the identity layer, in github.go); the forge
// reads and writes go through the resolved deskkit.Forge, not a `gh` subprocess. Production binds
// it to exec.Command; tests wrap it to record argv.
var execCommand = exec.Command
