package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var consumerCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)

// noMarkerReason is deriveConsumerDelivery's answer when the committed Evidence
// names no delivery at all. It is the only answer that falls through to the
// history search; a malformed or unauthenticated marker is still a refusal.
const noMarkerReason = "the committed Evidence has no Verification-Table marker naming the delivery"

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
	prefix, err := provenanceRootPrefix(root)
	if err != nil || prefix != "" {
		return fail("--root must name the repository root")
	}
	repo := coverageRepo{root: root}
	shallow, ok := repo.IsShallow()
	if !ok || shallow {
		return fail("complete local Git history is required (no fetch is attempted)")
	}
	out, err, supported := commitReadOutput(gitCoreForCommits(root, delivery), delivery)
	actual := strings.TrimSpace(string(out))
	if !supported {
		actual, err = git("rev-parse", "--verify", delivery+"^{commit}")
	}
	if err != nil || actual != delivery {
		return fail("delivery is not a commit in local history")
	}
	read := gitCoreForCommits(root, delivery)
	parent := ""
	if read.err != nil {
		err = read.err
	} else if read.native != "" {
		parent, err = git("rev-parse", "--verify", delivery+"^1")
	} else {
		parents, parentErr := read.repo.ParentHashes(delivery)
		err = parentErr
		if len(parents) != 0 {
			parent = parents[0]
		}
	}
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
	// Only unchanged, fully contained non-board links may resolve. The bounded
	// archive reader writes regular files and never extracts archive links.
	entries, ok := repo.TreeModes(delivery)
	if !ok || len(entries) == 0 {
		return fail("cannot enumerate delivered tree entries")
	}
	tree, err := newConsumerSnapshot(entries, changed)
	if err != nil {
		return fail(err)
	}
	archive := historyGit(root, "archive", "--format=tar", delivery+"^{tree}")
	if err := tree.materialize(archive, snapshot); err != nil {
		return fail(err)
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
	verdicts := corroborateBriefWithSnapshot(snapshot, streams, changed, nil, bf, tree)
	// Both explicit and Evidence-derived pins reach this one historical path.
	// A bounded snapshot authenticates the inputs, not the caller's assertion
	// that this commit implements the producer. Keep that separate from routing.
	fmt.Fprintln(os.Stderr, "PRODUCER-DELIVERY: COULD-NOT-CHECK — producer identity and implementation completeness are not evaluated; follow-up routes prove target existence and reference-back only. Independently establish the actual producer delivery. Routing counts on stdout and the exit status concern declared consumer routes only.")
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
		return "", "", noMarkerReason
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
// delivery and its first parent, or reports COULD-NOT-CHECK. The delivery comes
// from the committed Verification-Table marker when there is one, and otherwise
// from the checked-out history (deriveHistoryDelivery).
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
	if why == noMarkerReason {
		var source, hwhy string
		delivery, base, source, hwhy = deriveHistoryDelivery(root, briefs[0].Brief)
		if hwhy != "" {
			return 2, why + ", and " + hwhy
		}
		fmt.Fprintf(os.Stderr, "statusgen: --consumers: no Verification-Table marker; delivery derived from history: %s (%s), pinned to its first parent %s\n", delivery, source, base)
		why = ""
	}
	if why != "" {
		return 2, why
	}
	return runDeliveryConsumers(root, base, briefFilter, delivery), ""
}

// deriveHistoryDelivery names a merged brief's delivery from local history when
// its Evidence records none: the one first-parent commit of HEAD that names
// `Brief: <stream>/<NN>`, either in its own message (a squash or direct commit)
// or in a commit it merged (a merge commit, whose own message is title-only on
// GitHub). Only fence-aware trailers count. Zero or several such commits is
// COULD-NOT-CHECK, never a pick. The answer must also have landed on the fetched
// mainline ref, and its pin is its first parent. No forge is asked, nothing is
// fetched, and a shallow history is refused because it cannot prove uniqueness.
func deriveHistoryDelivery(root, brief string) (delivery, base, source, why string) {
	want := canonicalBriefKey(brief)
	stream, _, ok := briefStreamNum(want)
	if !ok {
		return "", "", "", "the brief id " + want + " is not a <stream>/<NN> key the history can be searched for"
	}
	if shallow, ok := (coverageRepo{root: root}).IsShallow(); !ok || shallow {
		return "", "", "", "the history cannot be searched: complete local Git history is required (no fetch is attempted)"
	}
	out, err := historyGit(root, "rev-list", "--first-parent", "HEAD").Output()
	chain := strings.Fields(string(out))
	if err != nil || len(chain) == 0 {
		return "", "", "", "the first-parent history of HEAD cannot be read"
	}
	onChain := make(map[string]int, len(chain))
	for i, c := range chain {
		onChain[c] = i
	}
	// The grep is only a prefilter; parseLinkTrailers decides what a commit names.
	logOut, err := historyGit(root, "log", "-z", "--format=%H%n%B", "--regexp-ignore-case", "--fixed-strings", "--all-match", "--grep=brief:", "--grep="+stream, "HEAD", "--").Output()
	if err != nil {
		return "", "", "", "the commit messages reachable from HEAD cannot be read"
	}
	named := map[string]bool{}
	for _, rec := range splitNUL(logOut) {
		hash, body, _ := strings.Cut(rec, "\n")
		_, briefs, _ := parseLinkTrailers(body)
		for _, b := range briefs {
			if b == want {
				named[strings.TrimSpace(hash)] = true
			}
		}
	}
	// Each naming commit belongs to the oldest first-parent commit that contains
	// it: itself when it is on the chain, else the merge that brought it in.
	// Containment is monotone along the chain, so a binary search finds it.
	introducers := map[int]bool{}
	for c := range named {
		if i, ok := onChain[c]; ok {
			introducers[i] = true
			continue
		}
		lo, hi := 0, len(chain)-1 // chain[lo] contains c; find the largest such index
		for lo < hi {
			mid := (lo + hi + 1) / 2
			in, err := isAncestor(root, c, chain[mid])
			if err != nil {
				return "", "", "", "ancestry in the history of HEAD cannot be read"
			}
			if in {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		introducers[lo] = true
	}
	var found []int
	for i := range introducers {
		found = append(found, i)
	}
	sort.Ints(found)
	switch {
	case len(found) == 0:
		return "", "", "", fmt.Sprintf("no first-parent commit of HEAD names `Brief: %s` in its own message or in a commit it merged (searched %d first-parent commits)", want, len(chain))
	case len(found) > 1:
		var shorts []string
		for _, i := range found {
			shorts = append(shorts, chain[i][:12])
		}
		return "", "", "", fmt.Sprintf("the delivery is ambiguous: %d first-parent commits of HEAD name `Brief: %s` (%s); this instrument does not choose between them", len(found), want, strings.Join(shorts, ", "))
	}
	delivery = chain[found[0]]
	parentsOut, err := historyGit(root, "rev-list", "--parents", "-n", "1", delivery).Output()
	parents := strings.Fields(string(parentsOut))
	if err != nil || len(parents) < 2 || len(parents) > 3 || !consumerCommit.MatchString(parents[1]) {
		return "", "", "", "the history-derived delivery " + delivery + " is not a single- or two-parent commit"
	}
	if err := historyGit(root, "merge-base", "--is-ancestor", delivery, remoteMainRef).Run(); err != nil {
		return "", "", "", "the history-derived delivery " + delivery + " is not reachable from the fetched mainline ref " + remoteMainRef + " (absent, or a local commit that has not landed)"
	}
	source = "its own message names Brief: " + want
	if !named[delivery] {
		source = "a commit it merged names Brief: " + want
	}
	return delivery, parents[1], source, ""
}

// isAncestor reports whether a is an ancestor of b. Exit status 1 is "no";
// any other failure is an error, never an answer.
func isAncestor(root, a, b string) (bool, error) {
	err := historyGit(root, "merge-base", "--is-ancestor", a, b).Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	}
	return false, err
}
