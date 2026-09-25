package main

// custody.go — writing a freshly minted token to disk under the ruled custody model
// (the recorded custody ruling, option 2).
//
// LAYER 1 — CREATED RESTRICTED. The file is created owner-only from the instant it exists:
// createRestricted (custody_{unix,windows}.go) opens a NEW file with O_EXCL and mode 0600 on
// unix, and on Windows with CREATE_NEW and a protected owner-only DACL passed IN the create
// call — never created open and tightened afterwards, so a crash between create and check
// never leaves a readable token on disk. The token is written to a temp file in the target
// directory and renamed into place, so an existing, possibly looser file at the target is
// REPLACED (its access list goes with it) rather than truncated and rewritten in place under
// its old permissions.
//
// LAYER 2 — VERIFIED AS WRITTEN. provision.go then calls deskkit.ClassifyCustodyOwnerOnly on
// the file as it now exists — the access list READ BACK from the filesystem, a different
// signal from the intent expressed at creation — before the path is reported as usable. A
// definite failure stops the run; an inconclusive read-back warns and continues.
//
// LAYER 3 (not here, unchanged): every desk verb re-runs the owner-only custody check when it
// READS the token.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// createRestrictedFunc creates a NEW file at path — failing if anything exists there — that
// is owner-only from the moment it exists.
type createRestrictedFunc func(path string) (*os.File, error)

// writeTokenFile writes token to <dir>/gitlab-<role>.token through a restricted temp file and
// an atomic rename. It returns the final path. No error it returns formats the token.
func writeTokenFile(create createRestrictedFunc, dir, role, token string) (string, error) {
	final := filepath.Join(dir, tokenFileName(role))
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", fmt.Errorf("generate a temp name in %s: %v", dir, err)
	}
	tmp := filepath.Join(dir, "."+tokenFileName(role)+"."+hex.EncodeToString(rnd[:])+".tmp")
	f, err := create(tmp)
	if err != nil {
		return "", fmt.Errorf("create %s owner-only: %v", tmp, err)
	}
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write([]byte(token)); err != nil {
		_ = f.Close()
		cleanup()
		return "", fmt.Errorf("write %s: %v", tmp, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return "", fmt.Errorf("sync %s: %v", tmp, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("close %s: %v", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		cleanup()
		return "", fmt.Errorf("rename %s into place at %s: %v", tmp, final, err)
	}
	return final, nil
}
