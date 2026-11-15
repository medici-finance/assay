// Package preparation calls the existing Python preparation domain with typed inputs.
// The image-owned launcher remains the owner of retained identity and history.
package preparation

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

//go:embed core.py
var core string

// Options contains only the settings declared by the preparation command.
type Options struct {
	Prepare         bool   `json:"prepare"`
	Readback        bool   `json:"readback"`
	Work            string `json:"work"`
	Cell            string `json:"cell"`
	Role            string `json:"role"`
	AgentVersion    string `json:"agent_version"`
	Model           string `json:"model"`
	Agent           string `json:"agent"`
	VersionManifest string `json:"version_manifest"`
	Instructions    string `json:"instructions"`
	Workdir         string `json:"workdir"`
}

// Run retains the legacy JSON, diagnostic and exit-code contract. The embedded
// domain has no argument parser; it receives data and calls the image initializer.
func Run(ctx context.Context, options Options, stdout, stderr io.Writer) int {
	raw, err := json.Marshal(options)
	if err != nil {
		fmt.Fprintln(stderr, "held: compatibility preparation inputs unavailable")
		return 78
	}
	// -I keeps the caller's working directory (the writable work volume) and
	// PYTHON* variables off the import path; -c alone would put the cwd first.
	child := exec.CommandContext(ctx, "python3", "-I", "-B", "-c", core+"\nraise SystemExit(main())\n")
	child.Stdin = bytes.NewReader(raw)
	child.Stdout, child.Stderr = stdout, stderr
	if err := child.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() >= 0 {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, "held: compatibility preparation interpreter unavailable")
		return 78
	}
	return 0
}
