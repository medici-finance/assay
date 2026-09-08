package untrustcorpus

import (
	"os"
	"path/filepath"
	"strings"
)

// root.go — where the corpus table lives, DERIVED rather than typed.
//
// The brief's contract is that the root is derived, never hand-typed on the command
// line: a consumer (the deterministic scan, the no-egress sandbox, CI) asks the tool for
// the root, it does not paste a path. This mirrors the App-credential home resolver
// (confighome.go): the MECHANISM is shipped and generic, and a deployment supplies the
// VALUE through an environment knob, with a shipped default that is a complete
// configuration on its own.

// EnvCorpusRoot puts a directory at the head of the resolution: set it to relocate the
// table (e.g. into the house-private twin) without editing any tool.
const EnvCorpusRoot = "ASSAY_UNTRUST_CORPUS_ROOT"

// shippedRoot is the in-repo location of the table relative to the desk-tools module
// root (tools/desk), where the tool is normally invoked from. It ships with the tool so
// the resolver is a complete configuration with the env knob unset.
const shippedRoot = "internal/deskkit/untrustcorpus/testdata"

// DefaultRoot resolves the corpus root without the caller typing a path: the
// ASSAY_UNTRUST_CORPUS_ROOT directory when set, else the shipped in-repo location.
func DefaultRoot() string {
	if v := strings.TrimSpace(os.Getenv(EnvCorpusRoot)); v != "" {
		return expandHomePath(v)
	}
	return shippedRoot
}

// expandHomePath expands a leading ~ to the user's home directory.
func expandHomePath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
