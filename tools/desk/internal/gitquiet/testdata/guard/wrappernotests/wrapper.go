package wrappernotests

import "os/exec"

// Init launches git, but the package has no tests, so no test binary runs it.
func Init(dir string) error { return exec.Command("git", "init", dir).Run() }
