package main

// A linked verifier home has a .git file pointing outside the container mount.
// Transport commit objects, not that pointer or the host's writable Git directory.
// The imported repository is disposable and used ONLY for fail-first baselines;
// head rows still run on the unchanged, tree-digested admitted mount.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func exportContainerHistory(home, out, source string) error {
	var err error
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if _, inside := relWithin(home, out); inside {
		return fmt.Errorf("history bundle must be outside the admitted home")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		return fmt.Errorf("history bundle output must be a new file")
	}
	if !isObjectID(source) {
		return fmt.Errorf("history export needs the admitted full source commit")
	}
	if shallow, err := historyGit(home, "rev-parse", "--is-shallow-repository").Output(); err != nil || strings.TrimSpace(string(shallow)) != "false" {
		return fmt.Errorf("history export requires a complete, non-shallow repository")
	}
	head, err := historyGit(home, "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil || strings.TrimSpace(string(head)) != source {
		return fmt.Errorf("history export HEAD is not the admitted source")
	}
	// HEAD alone exports its complete reachable history, without other branches,
	// local config, hooks, alternates, remotes or credentials.
	tmp, err := os.CreateTemp(filepath.Dir(out), ".verifyrun-history-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	defer os.Remove(name)
	if raw, err := historyGit(home, "bundle", "create", name, "HEAD").CombinedOutput(); err != nil {
		return fmt.Errorf("exporting admitted history: %v: %s", err, strings.TrimSpace(string(raw)))
	}
	// Publish without overwriting a path created since the initial check.
	if err := os.Link(name, out); err != nil {
		return fmt.Errorf("publishing new history bundle: %w", err)
	}
	return nil
}

func containerHistoryGit(root string, args ...string) error {
	cmd := historyGit(root, args...)
	// A bundle carries objects, never execution configuration. Disable inherited
	// config and templates as well; no host credential/helper or hook is needed.
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_COUNT=0")
	if raw, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("isolated history: %v: %s", err, strings.TrimSpace(string(raw)))
	}
	return nil
}

// importContainerHistory admits no caller-supplied source identity. The bundle's
// fetched HEAD must equal the source already bound by the host admission receipt.
// Only objects reachable from that source are fetched; origin/main is anchored to
// it so the ordinary ancestry and brief-at-base checks cannot silently degrade.
func importContainerHistory(home, brief, bundle, source string) (string, string, func(), error) {
	noop := func() {}
	if !isObjectID(source) {
		return "", "", noop, fmt.Errorf("history import needs the admitted full source commit")
	}
	rel, inside := relWithin(home, brief)
	if !inside {
		return "", "", noop, fmt.Errorf("history import currently requires a home-relative brief")
	}
	if _, inside := relWithin(home, bundle); inside {
		return "", "", noop, fmt.Errorf("history bundle must be outside the admitted home")
	}
	info, err := os.Stat(bundle)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", noop, fmt.Errorf("history bundle must be a readable regular file outside the home")
	}
	bundle, err = filepath.Abs(bundle)
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
	steps := [][]string{
		{"init", "--template=", repo},
		{"-c", "protocol.file.allow=always", "-c", "fetch.fsckObjects=true", "fetch", "--no-tags", "--no-recurse-submodules", bundle, "HEAD:refs/heads/verification-source"},
		{"symbolic-ref", "HEAD", "refs/heads/verification-source"},
	}
	for i, args := range steps {
		dir := repo
		if i == 0 {
			dir = tmp
		}
		if err := containerHistoryGit(dir, args...); err != nil {
			cleanup()
			return "", "", noop, err
		}
	}
	head, err := historyGit(repo, "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil || strings.TrimSpace(string(head)) != source {
		cleanup()
		return "", "", noop, fmt.Errorf("history bundle HEAD is not the admitted source")
	}
	for _, args := range [][]string{{"fsck", "--connectivity-only", "--no-dangling"}, {"update-ref", remoteMainRef, source}} {
		if err := containerHistoryGit(repo, args...); err != nil {
			cleanup()
			return "", "", noop, err
		}
	}
	// The mapped brief need not exist in the metadata-only repository. Resolve
	// the existing parent first (not the absent leaf), including macOS /var links.
	repo = canonicalPath(repo)
	return repo, filepath.Join(repo, filepath.FromSlash(rel)), cleanup, nil
}
