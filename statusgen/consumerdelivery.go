package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var consumerCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)

// runDeliveryConsumers evaluates the declared routes at a bounded historical
// delivery, including claims inherited from a separate authoring change. Ordinary
// branch mode retains its entry-inheritance protection for Evidence-only edits.
func runDeliveryConsumers(root, base, brief, delivery string) int {
	fail := func(err any) int {
		fmt.Fprintf(os.Stderr, "statusgen: --consumers --delivery: COULD-NOT-CHECK: %v\n", err)
		return 2
	}
	if brief == "" || !consumerCommit.MatchString(delivery) || !consumerCommit.MatchString(base) {
		return fail("requires a named --brief and full immutable commit SHAs for --delivery and --base")
	}
	git := func(args ...string) (string, error) {
		out, err := historyGit(root, args...).Output()
		return strings.TrimSpace(string(out)), err
	}
	prefix, err := git("rev-parse", "--show-prefix")
	if err != nil || prefix != "" {
		return fail("--root must name the repository root")
	}
	shallow, err := git("rev-parse", "--is-shallow-repository")
	if err != nil || shallow != "false" {
		return fail("complete local Git history is required (no fetch is attempted)")
	}
	actual, err := git("rev-parse", "--verify", delivery+"^{commit}")
	if err != nil || actual != delivery {
		return fail("delivery is not a commit in local history")
	}
	parent, err := git("rev-parse", "--verify", delivery+"^1")
	if err != nil || parent != base {
		return fail("base must be the delivery's first parent; wider or empty history ranges are refused")
	}
	if _, err := git("merge-base", "--is-ancestor", delivery, "HEAD"); err != nil {
		return fail("delivery must be an ancestor of the checkout's HEAD")
	}
	pathBytes, err := historyGit(root, "diff", "--name-only", "--no-renames", "-z", base, delivery, "--").Output()
	if err != nil || len(pathBytes) == 0 {
		return fail("delivery diff is absent or empty")
	}
	changed := map[string]bool{}
	for _, p := range splitNUL(pathBytes) {
		changed[p] = true
	}
	snapshot, err := os.MkdirTemp("", "consumers-delivery-*")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(snapshot)
	// Reuse the existing Git archive pipeline. No historical source is run,
	// no refs/worktree metadata are mutated, and local edits cannot replace inputs.
	// A tree archive has no commit context for export-subst. Check the complete
	// entry roster before extraction, so no symlink or submodule can escape the
	// snapshot or make an input silently disappear. export-ignore is detected below.
	repo := coverageRepo{root: root}
	entries, ok := repo.TreeModes(delivery)
	if !ok || len(entries) == 0 {
		return fail("cannot enumerate delivered tree entries")
	}
	for _, entry := range entries {
		if entry.Mode != "100644" && entry.Mode != "100755" {
			return fail("delivery snapshot contains a symlink or submodule: " + entry.Path)
		}
	}
	archive := historyGit(root, "archive", "--format=tar", delivery+"^{tree}")
	if err := materializeCommands(delivery, archive, exec.Command("tar", "-x", "-C", snapshot)); err != nil {
		return fail(err)
	}
	for _, entry := range entries {
		info, err := os.Lstat(filepath.Join(snapshot, filepath.FromSlash(entry.Path)))
		if err != nil || !info.Mode().IsRegular() {
			return fail("delivery archive omitted a tracked blob (for example export-ignore): " + entry.Path)
		}
	}
	streams, _, err := loadStreams(snapshot)
	if err != nil {
		return fail(err)
	}
	briefs := selectConsumerBriefs(snapshot, streams, changed, brief)
	if len(briefs) != 1 {
		return fail("brief must resolve uniquely in the delivered snapshot")
	}
	bf := briefs[0]
	if bf.ConsumersProse == "" && len(bf.Consumers) == 0 {
		return fail("delivered brief declares no consumer routes")
	}
	verdicts := corroborateBrief(snapshot, streams, changed, nil, bf)
	return emitConsumers(base+".."+delivery+" (immutable delivered snapshot)", 1, verdicts, nil, true)
}

// consumerRangeWhy uses the shared shell tokenizer and simple-command reader;
// quoted examples or flags on a neighbouring command cannot satisfy this check.
func consumerRangeWhy(cmd string, toks []shellTok, expect string) string {
	boundedRoots := map[string]bool{}
	for _, part := range splitSimpleCommands(toks) {
		words := cmdWords(part.toks)
		if len(words) == 0 || words[0] == "echo" || words[0] == "printf" {
			continue
		}
		// Only a parsed Git checkout, preceding this consumer invocation and
		// addressing the same --root, can describe an existing snapshot recipe.
		if filepath.Base(words[0]) == "git" {
			rootAt := "."
			checkout, detached := false, false
			for i, w := range words {
				if w == "-C" && i+1 < len(words) {
					rootAt = words[i+1]
				}
				if w == "checkout" {
					checkout = true
				}
				if w == "--detach" && i+1 < len(words) && words[i+1] != "" {
					detached = true
				}
			}
			if checkout && detached {
				boundedRoots[rootAt] = true
			}
		}
		flags := map[string]string{}
		consumes := false
		for i, w := range words {
			if w == "--consumers" {
				consumes = true
			}
			for _, flag := range []string{"--base", "--delivery", "--brief", "--root"} {
				if strings.HasPrefix(w, flag+"=") {
					flags[flag] = strings.TrimPrefix(w, flag+"=")
				}
				if w == flag && i+1 < len(words) && !strings.HasPrefix(words[i+1], "--") {
					flags[flag] = words[i+1]
				}
			}
		}
		if !consumes {
			continue
		}
		if flags["--base"] == "" {
			return "consumer command has no explicit --base (the default becomes HEAD on merged main)"
		}
		if flags["--delivery"] != "" && flags["--brief"] == "" {
			return "delivery command has no named --brief"
		}
		branchOnly := strings.Contains(strings.ToLower(expect), "pre-merge")
		rootAt := flags["--root"]
		if rootAt == "" {
			rootAt = "."
		}
		if flags["--delivery"] == "" && !branchOnly && !boundedRoots[rootAt] {
			return "consumer command has no immutable delivery snapshot (a fixed base alone still reads the latest tree and inherited claims)"
		}
	}
	return ""
}

// deriveConsumerDelivery names the immutable delivery commit of a merged brief
// when the caller supplied neither --base nor --delivery. The only source is the
// brief's COMMITTED Evidence: a literal `Verification-Table:` marker whose
// coordinates deliveryCoordinates (the protected-table closure's own reader)
// authenticates against the first-parent chain of HEAD. The CLI, the
// environment and the working tree never contribute a coordinate, and the
// pinned base is computed here as the delivery's first parent, never taken from
// the marker. An empty delivery comes back with the reason; there is no
// partial answer and no fallback range.
func deriveConsumerDelivery(root, rel string) (delivery, base, why string) {
	rel = filepath.ToSlash(rel)
	if rel == "" || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
		return "", "", "the brief is not inside the repository root"
	}
	blob, err := historyGit(root, "show", "--end-of-options", "HEAD:"+rel).Output()
	if err != nil {
		return "", "", "the brief is not committed at HEAD, so no committed Evidence can name its delivery"
	}
	_, evidence := briefSectionsOf(string(blob))
	markers := protectedMarkers(normalizeProtectedEvidence(evidence))
	if len(markers) == 0 {
		return "", "", "the committed Evidence has no Verification-Table marker naming the delivery"
	}
	type triple struct{ delivery, base, original string }
	var chosen *triple
	for _, m := range markers {
		if !m.literal {
			return "", "", "Verification-Table must be literal live text, without code spans"
		}
		fields := strings.Fields(m.text)
		if len(fields) != 4 || !strings.HasPrefix(fields[1], "delivery=") || !strings.HasPrefix(fields[2], "base=") || !strings.HasPrefix(fields[3], "original=") {
			return "", "", "expected Verification-Table: delivery=<full-sha> base=<full-sha> original=<full-sha>"
		}
		t := triple{strings.TrimPrefix(fields[1], "delivery="), strings.TrimPrefix(fields[2], "base="), strings.TrimPrefix(fields[3], "original=")}
		if !consumerCommit.MatchString(t.delivery) || !consumerCommit.MatchString(t.base) || !consumerCommit.MatchString(t.original) {
			return "", "", "the Verification-Table marker needs full 40-hex commit ids"
		}
		if chosen != nil && *chosen != t {
			return "", "", "the committed Evidence names more than one delivery; this instrument does not choose between them"
		}
		chosen = &t
	}
	parents, cwhy := deliveryCoordinates(root, chosen.delivery, chosen.base, chosen.original)
	if cwhy != "" {
		return "", "", cwhy
	}
	// A commit made only in this checkout is not a landed delivery. The named
	// delivery must be reachable from the fetched mainline ref; with no such ref
	// there is nothing to land against, so the answer is could-not-check.
	if err := historyGit(root, "merge-base", "--is-ancestor", chosen.delivery, remoteMainRef).Run(); err != nil {
		return "", "", "the delivery is not reachable from the fetched mainline ref " + remoteMainRef + " (absent, or a local commit that has not landed)"
	}
	// A merge delivery records the divergence baseline as its marker base; the
	// consumer range is always the delivery against its first parent.
	if len(parents) == 0 || !consumerCommit.MatchString(parents[0]) {
		return "", "", "the delivery's first parent is not a full immutable commit id"
	}
	return chosen.delivery, parents[0], ""
}

// runDerivedConsumers is the --consumers --brief path with no --base and no
// --delivery, reached only where the branch diff carries no evidence about the
// named brief (merged main, or no diff at all). It pins the run to the brief's
// recorded delivery and its first parent, or reports COULD-NOT-CHECK.
func runDerivedConsumers(root string, streams []*Stream, briefFilter string) (code int, why string) {
	briefs := selectConsumerBriefs(root, streams, map[string]bool{}, briefFilter)
	if len(briefs) != 1 {
		return 2, "the brief must resolve to exactly one file"
	}
	rel, err := filepath.Rel(root, briefs[0].Path)
	if err != nil {
		return 2, "the brief path is outside the repository root"
	}
	delivery, base, why := deriveConsumerDelivery(root, rel)
	if why != "" {
		return 2, why
	}
	return runDeliveryConsumers(root, base, briefFilter, delivery), ""
}
