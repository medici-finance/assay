package main

// A linked verifier home has a .git file pointing outside the container mount.
// Transport commit objects, not that pointer or the host's writable Git directory.
// The imported repository is disposable and used ONLY for fail-first baselines;
// head rows still run on the unchanged, tree-digested admitted mount.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/medici-finance/assay/gitcore"
)

func exportContainerHistory(home, out, source string) error {
	var err error
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if _, inside := relWithin(home, out); inside {
		return fmt.Errorf("history pack must be outside the admitted home")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		return fmt.Errorf("history pack output must be a new file")
	}
	if !isObjectID(source) {
		return fmt.Errorf("history export needs the admitted full source commit")
	}
	// The shared in-process library exports reachable objects, without other
	// branches, local config, hooks, alternates, remotes or credentials.
	tmp, err := os.CreateTemp(filepath.Dir(out), ".verifyrun-history-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	err = gitcore.WriteHistoryPack(home, source, tmp)
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Publish without overwriting a path created since the initial check.
	if err := os.Link(name, out); err != nil {
		return fmt.Errorf("publishing new history pack: %w", err)
	}
	return nil
}

// importContainerHistory accepts a pack only when the admitted source and all
// its reachable objects are present. origin/main is anchored to that source so
// the ordinary ancestry and brief-at-base checks cannot silently degrade.
func importContainerHistory(home, brief, pack, source string) (string, string, func(), error) {
	noop := func() {}
	if !isObjectID(source) {
		return "", "", noop, fmt.Errorf("history import needs the admitted full source commit")
	}
	rel, inside := relWithin(home, brief)
	if !inside {
		return "", "", noop, fmt.Errorf("history import currently requires a home-relative brief")
	}
	if _, inside := relWithin(home, pack); inside {
		return "", "", noop, fmt.Errorf("history pack must be outside the admitted home")
	}
	info, err := os.Stat(pack)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", noop, fmt.Errorf("history pack must be a readable regular file outside the home")
	}
	pack, err = filepath.Abs(pack)
	if err != nil {
		return "", "", noop, err
	}
	tmp, err := os.MkdirTemp("", "verifyrun-history-")
	if err != nil {
		return "", "", noop, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	if _, inside := relWithin(home, tmp); inside {
		cleanup()
		return "", "", noop, fmt.Errorf("history scratch must be outside the admitted home; move TMPDIR")
	}
	repo := filepath.Join(tmp, "history")
	in, err := os.Open(pack)
	if err != nil {
		cleanup()
		return "", "", noop, err
	}
	err = gitcore.ReadHistoryPack(repo, source, in)
	closeErr := in.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return "", "", noop, err
	}
	// The mapped brief need not exist in the metadata-only repository. Resolve
	// the existing parent first (not the absent leaf), including macOS /var links.
	repo = canonicalPath(repo)
	return repo, filepath.Join(repo, filepath.FromSlash(rel)), cleanup, nil
}
