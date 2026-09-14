package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// insideExecutor reverses one inside apply step for real, leaving the target
// as it was before the effect (component-model.md §4: "running the inverse
// MUST leave the target as it was before the effect"). It is registered ONLY
// for components whose install target is a concrete, adopter-repo-relative
// path today.
//
// Several inside components declared in this tree (the plugin skills, the
// hook wiring, the statusgen/desk-tools binaries, the CI workflow file) install
// into a location that is either harness-defined (§9's assay.harness adapter,
// not landed until a later brief) or determined by installer flags that
// are not recorded in the manifest (deskinstall's --dest-dir). Guessing either
// path here would be exactly the "best-effort" reverse the brief's ground
// rules forbid ("If a step's reverse cannot be written without deleting
// something Assay did not create... never a best-effort delete"). Disabling
// one of those components therefore refuses with a named reason instead of
// silently doing nothing or deleting the wrong thing (disable.go's
// "no registered inverse" path) — an honest could-not-check, not a rounded-up
// success.
type insideExecutor func(root string) error

func execKey(component, stepID string) string { return component + "#" + stepID }

var insideExecutors = map[string]insideExecutor{
	execKey("assay/streams-scaffold", "streams-tree"):     streamsScaffoldInverse,
	execKey("assay/main-guard", "hooks-path"):             mainGuardInverse,
	execKey("assay/registers-scaffold", "registers-tree"): registersScaffoldInverse,
}

// streamsReadmeName is the one file assay/streams-scaffold's streams-tree step
// creates (component.yaml: "create docs/streams/ with a README and the
// per-stream layout"). The inverse removes exactly this file — never the
// whole docs/streams/ directory, which an adopter may already have populated
// with their own stream content before or after install (threat model:
// "an inverse deletes a file Assay did not create").
const streamsReadmeName = "README.md"

func streamsScaffoldInverse(root string) error {
	dir := filepath.Join(root, "docs", "streams")
	readme := filepath.Join(dir, streamsReadmeName)
	if err := removeIfExists(readme); err != nil {
		return fmt.Errorf("removing %s: %w", readme, err)
	}
	// Best-effort tidy: only succeeds if the directories are now empty, which
	// is exactly the case where Assay created them and nothing else has been
	// placed there since. A non-empty directory (adopter content) is left
	// alone — os.Remove on it fails and that failure is ignored.
	removeDirIfEmpty(dir)
	removeDirIfEmpty(filepath.Join(root, "docs"))
	return nil
}

// registersScaffoldFiles are the three register files assay/registers-scaffold
// creates (component.yaml: "create the FINDINGS / INTAKE / RETRO registers
// under docs/streams/").
var registersScaffoldFiles = []string{"FINDINGS.md", "INTAKE.md", "RETRO.md"}

func registersScaffoldInverse(root string) error {
	dir := filepath.Join(root, "docs", "streams")
	for _, name := range registersScaffoldFiles {
		p := filepath.Join(dir, name)
		if err := removeIfExists(p); err != nil {
			return fmt.Errorf("removing %s: %w", p, err)
		}
	}
	removeDirIfEmpty(dir)
	return nil
}

// mainGuardHookPath is the pre-commit hook assay/main-guard installs
// (component.yaml: "install .githooks/pre-commit and set core.hooksPath").
const mainGuardHookRelPath = ".githooks/pre-commit"

func mainGuardInverse(root string) error {
	// Unset core.hooksPath FIRST: if we removed the hook file first and the
	// unset step then failed, a half-reversed tree would have core.hooksPath
	// pointing at a directory whose hook is gone — a different, silently
	// broken state than either "still installed" or "fully reversed".
	cmd := exec.Command("git", "-C", root, "config", "--unset", "core.hooksPath")
	if out, err := cmd.CombinedOutput(); err != nil {
		// `git config --unset` exits 5 when the key is already absent — an
		// idempotent no-op (component-model.md §7: disabling an
		// already-disabled entry must not error), not a failure.
		var ee *exec.ExitError
		if !(errors.As(err, &ee) && ee.ExitCode() == 5) {
			return fmt.Errorf("git config --unset core.hooksPath: %v: %s", err, string(out))
		}
	}
	hook := filepath.Join(root, filepath.FromSlash(mainGuardHookRelPath))
	if err := removeIfExists(hook); err != nil {
		return fmt.Errorf("removing %s: %w", hook, err)
	}
	removeDirIfEmpty(filepath.Dir(hook))
	return nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// removeDirIfEmpty removes dir only if it is now empty; a non-empty dir or an
// already-absent one are both fine outcomes, so the error is discarded.
func removeDirIfEmpty(dir string) {
	_ = os.Remove(dir)
}
