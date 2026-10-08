package wrapper

import "os/exec"

// Init runs git through the package's own launcher, the shape of a package whose only
// "git" literal sits in non-test code its tests call.
func Init(dir string) error { return exec.Command("git", "init", dir).Run() }
