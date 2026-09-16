package main

import "os/exec"

// execCommand is the single seam through which the ONE subprocess this tool runs flows:
// `desktoken <role> --repo <slug>`, the session-role token mint. Production binds it to
// exec.Command; tests replace it so no real App credential is needed. desklabel invokes
// no forge CLI (`gh`, `glab`) on any path — every forge read and write goes through the
// resolved deskkit.Forge.
var execCommand = exec.Command
