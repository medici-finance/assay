package gitcore

import (
	"crypto"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/hash"
)

// ObjectFormat reads the local repository's declared hash format through the same
// configuration parser as Open. It reads the common config for linked worktrees.
// It must run before Open: this pinned go-git selects its hash implementation at
// build time, so one binary cannot read SHA-1 and SHA-256 repositories together.
// Only a positively observed mismatch permits a caller's native compatibility
// path; an unreadable/invalid configuration remains an error.
func ObjectFormat(dir string) (string, error) {
	top, err := Toplevel(dir)
	if err != nil {
		return "", err
	}
	common, err := CommonDir(top)
	if err != nil {
		return "", err
	}
	f, err := os.Open(filepath.Join(common, "config"))
	if err != nil {
		return "", err
	}
	cfg, readErr := config.ReadConfig(f)
	closeErr := f.Close()
	if readErr != nil {
		return "", readErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	// v5.19.3's Unmarshal preserves this in Raw but does not populate the typed
	// Extensions.ObjectFormat field. Use its existing case-insensitive codec.
	format := strings.ToLower(strings.TrimSpace(cfg.Raw.Section("extensions").Option("objectformat")))
	if format == "" {
		format = "sha1"
	}
	if format != "sha1" && format != "sha256" {
		return "", fmt.Errorf("gitcore: unsupported object format %q", format)
	}
	return format, nil
}

// BuildObjectFormat names the format this build of go-git can represent.
func BuildObjectFormat() string {
	if hash.CryptoType == crypto.SHA256 {
		return "sha256"
	}
	return "sha1"
}
