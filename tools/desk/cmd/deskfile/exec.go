package main

import (
	"os/exec"
)

// execCommand is the seam through which deskfile would start a child process. It starts none in
// production: the forge reads and writes go through the resolved deskkit.Forge (write-verbs-C),
// and the role credential is read in process through deskkit.WriteVerbRoleCredential (#5457),
// not by forking `desktoken`. Production binds it to exec.Command; the test harness wraps it to
// record argv, so a child process reintroduced here shows up in the recorded calls.
var execCommand = exec.Command
