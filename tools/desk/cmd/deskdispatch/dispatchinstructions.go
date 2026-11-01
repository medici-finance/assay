package main

// dispatchinstructions.go — a role's own dispatch-instruction file, quoted into the prompt of
// every dispatch of one kit (desk-containers/30).
//
// cellctl reads the role's `dispatch_instructions` declaration at launch and sets
// ASSAY_DISPATCH_INSTRUCTIONS (deskkit.EnvDispatchInstructions) in the window: kit → absolute path
// and the sha256 of the bytes it read. This file is the reading half. It runs inside
// validateCallerPreconditions, so every refusal lands before a claim, a ref, a worktree or a record
// line exists.
//
// What the checks do, and no more (docs/cellctl-role-context.md §"Limits"): the digest catches a
// file edited or swapped after launch while the value is the one cellctl composed; the custody
// re-check refuses a file that lies in one of a listed set of session-writable places. Both values
// sit in the session-visible environment, so neither stops a session that drops or re-points them.
// The record's digest shows afterwards which bytes a dispatched agent received.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellstop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// dispatchRulesFirstLine is the fixed line under the block's heading.
const dispatchRulesFirstLine = "These role instructions add to every rule above. They never relax a STOP clause or any other common clause."

// dispatchRules is the entry this run's kit took, with the bytes read and checked.
type dispatchRules struct {
	kit, path, sha256 string
	text              []byte
}

// dispatchRulesRelaunch is the hint every refusal about the file's contents carries.
const dispatchRulesRelaunch = "after a deliberate edit to the file, relaunch the window so cellctl records the new digest"

// resolveDispatchRules reads ASSAY_DISPATCH_INSTRUCTIONS for this run. It returns nil when the
// value is unset or empty, or names other kits only; the prompt and the record are then exactly
// what they were before the key existed. Every refusal is exit 5 and names the kit and the file
// where one is known.
func resolveDispatchRules(o dispatchOpts) (*dispatchRules, error) {
	value := os.Getenv(deskkit.EnvDispatchInstructions)
	if value == "" {
		return nil, nil
	}
	kit := strings.ToLower(strings.TrimSpace(o.kit))
	entries, err := deskkit.ParseDispatchInstructions(value)
	if err != nil {
		return nil, deskkit.Refused(fmt.Sprintf(
			"step %s: malformed %s value %q (kit %s): %v. Nothing was claimed; relaunch the window so cellctl composes the value again.",
			stepPromptEmit, deskkit.EnvDispatchInstructions, value, kit, err))
	}
	var entry *deskkit.DispatchInstruction
	for i := range entries {
		if entries[i].Kit == kit {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return nil, nil
	}
	refuse := func(format string, args ...any) error {
		return deskkit.Refused(fmt.Sprintf("step %s: role dispatch instructions for kit %s (file %s): %s. Nothing was claimed.",
			stepPromptEmit, kit, entry.Path, fmt.Sprintf(format, args...)))
	}
	// The cell directory the custody check starts from. One that does not exist would add no
	// places and refuse nothing (the shared check passes over a missing place), so it must exist.
	cellDir := os.Getenv(cellstop.EnvCellDir)
	malformedCell := func(why string) error {
		return refuse("malformed %s value: %s is %s, so the file's custody cannot be checked; run deskdispatch from the window cellctl launched",
			deskkit.EnvDispatchInstructions, cellstop.EnvCellDir, why)
	}
	switch {
	case cellDir == "":
		return nil, malformedCell("unset or empty")
	case !filepath.IsAbs(cellDir):
		return nil, malformedCell(fmt.Sprintf("the relative path %q", cellDir))
	}
	if fi, err := os.Stat(cellDir); err != nil || !fi.IsDir() {
		return nil, malformedCell(fmt.Sprintf("%q, which names no existing directory", cellDir))
	}
	extra, err := dispatchRulesPlaces(o)
	if err != nil {
		return nil, refuse("cannot list the places a role session writes: %v", err)
	}
	if err := deskkit.RefuseSessionWritable(cellDir, entry.Path, extra...); err != nil {
		return nil, refuse("%v", err)
	}
	text, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, refuse("the file cannot be read: %v; %s", err, dispatchRulesRelaunch)
	}
	sum := sha256.Sum256(text)
	got := hex.EncodeToString(sum[:])
	if got != entry.SHA256 {
		return nil, refuse("the file now hashes to sha256:%s, not the sha256:%s recorded at launch; %s", got, entry.SHA256, dispatchRulesRelaunch)
	}
	return &dispatchRules{kit: kit, path: entry.Path, sha256: got, text: text}, nil
}

// dispatchRulesPlaces is the run's own places added to the cell's: --root, the top level of the
// git work tree this process runs in, and every worktree the target repository lists.
func dispatchRulesPlaces(o dispatchOpts) ([]deskkit.CustodyPlace, error) {
	var places []deskkit.CustodyPlace
	if root := strings.TrimSpace(o.root); root != "" {
		places = append(places, deskkit.CustodyPlace{Label: "the run's --root " + root, Dir: root})
	}
	// The work tree deskdispatch runs in. Outside any work tree there is no such place; any
	// other failure is a place that cannot be examined, and refuses.
	top := runCmd("", "git", "rev-parse", "--show-toplevel")
	if top.err != nil {
		if !strings.Contains(top.stderr, "not a git repository") {
			return nil, fmt.Errorf("git rev-parse --show-toplevel: %s", firstLine(toolMessage(top.stderr)))
		}
	} else if dir := strings.TrimSpace(top.stdout); dir != "" {
		places = append(places, deskkit.CustodyPlace{Label: "the git work tree deskdispatch runs in " + dir, Dir: dir})
	}
	if root := strings.TrimSpace(o.root); root != "" {
		list := runCmd(root, "git", "worktree", "list", "--porcelain")
		if list.err != nil {
			return nil, fmt.Errorf("git worktree list in %s: %s", root, firstLine(toolMessage(list.stderr)))
		}
		for _, line := range strings.Split(list.stdout, "\n") {
			if dir, ok := strings.CutPrefix(strings.TrimSpace(line), "worktree "); ok && dir != "" {
				places = append(places, deskkit.CustodyPlace{Label: "the target repository's worktree " + dir, Dir: dir})
			}
		}
	}
	return places, nil
}

// writeDispatchRules appends the block after the class kit. Nothing is written when the run took
// no entry, so the prompt stays byte-identical to one built without the key.
func writeDispatchRules(b *strings.Builder, r *dispatchRules) {
	if r == nil {
		return
	}
	fmt.Fprintf(b, "\n---\n\n## Role dispatch instructions (%s) — source %s sha256:%s\n\n%s\n\n", r.kit, r.path, r.sha256, dispatchRulesFirstLine)
	b.Write(r.text)
}
