// A test file may fork desktoken (a test drives the real CLI); the exec guard skips it.
// It is never compiled (testdata).
package planted

import "os/exec"

var _ = exec.Command("desktoken", "worker")
