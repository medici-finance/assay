package main

// workerkind.go — the KIND of a worker run, and what each kind is handed (#2439).
//
// A worker kit is dispatched for two different jobs. An IMPLEMENTING run starts from a brief
// or an issue with no change open and ends by opening one. A SHEPHERDING run is dispatched
// with --pr onto a change that is already open and works that change: its red checks, its
// review findings, its conflicts. Both were handed the same assignment half — "open the
// draft PR", `deskpr create` — and the same kit, whichever job they had.
//
// THE KIND IS DERIVED, NEVER STATED. It is workerResume(o): a worker kit with --pr is
// shepherding, every other worker dispatch (a fresh brief, an issue-only item, a --rework
// follow-up on a new branch) is implementing. There is no flag, so a dispatcher cannot
// hand a run the wrong half.
//
// WHAT DIFFERS BY KIND, and what does not:
//
//   - the ASSIGNMENT's action half (writeWorkerAssignment / writeShepherdAssignment) — the
//     one place the two jobs are told to do different things;
//   - kit text inside a marked stretch — text that binds one kind only;
//   - NOTHING ELSE. Unmarked kit text reaches both kinds. A clause whose scope is not
//     plainly one kind's is left unmarked: a resumed change may be half-implemented, and an
//     implementing run may see a review land before it hands back, so most of the kit binds
//     both. Marking a stretch is a claim that the other kind is never bound by it.
//
// THE MARKERS are whole lines, the shape the review kit's per-lane markers use:
//
//	<!-- kind:implementing:begin -->
//	…
//	<!-- kind:implementing:end -->
//
// and the same for `shepherding`. Stretches do not nest. Clause headings stay outside a
// stretch, so clause numbers — which other documents cite — are the same for both kinds.
//
// THE COMMON-CLAUSE COPY. The objective kit FILE carries a copy of the common clauses
// between `<!-- common-clauses:begin -->` and `<!-- common-clauses:end -->`, held
// byte-identical to references/common-clauses.md. Every prompt already quotes the common
// clauses once, ahead of the class kit, so a dispatch does not quote that copy a second
// time: the stretch stays in the file for a reader of the file on its own.
//
// kitText still returns the file as written — what the parity tests, the leak scans and the
// greps over the file read. workerKitText is what a dispatch quotes.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// workerKind is the kind of run a worker kit is dispatched for.
type workerKind string

const (
	kindImplementing workerKind = "implementing"
	kindShepherding  workerKind = "shepherding"
)

// workerKinds is the CLOSED set of kinds a worker kit is cut for.
func workerKinds() []workerKind { return []workerKind{kindImplementing, kindShepherding} }

func isWorkerKind(name string) bool {
	for _, k := range workerKinds() {
		if string(k) == name {
			return true
		}
	}
	return false
}

// workerKindOf derives the kind from the dispatch. Only meaningful on a worker kit.
func workerKindOf(o dispatchOpts) workerKind {
	if workerResume(o) {
		return kindShepherding
	}
	return kindImplementing
}

// workerKit reports whether name is one of the implementer kits — the kits a kind applies to.
func workerKit(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "worker", "worker-objective":
		return true
	}
	return false
}

// kindMarkerPrefix is what makes a line a kind marker. A line carrying it that is not a
// well-formed whole-line marker is a kit defect, never text to emit.
const kindMarkerPrefix = "<!-- kind:"

var kindMarkerRE = regexp.MustCompile(`^<!-- kind:([a-z-]+):(begin|end) -->$`)

const (
	commonCopyBegin = "<!-- common-clauses:begin -->"
	commonCopyEnd   = "<!-- common-clauses:end -->"
)

// workerKitForKind cuts a worker kit for one kind of run: every unmarked line, plus the
// stretches marked for that kind, minus the file's copy of the common clauses. Marker lines
// are never emitted.
//
// A malformed, unknown, nested, stray or unclosed marker is UNVERIFIABLE. A guess in either
// direction is wrong: emitting an unclosed stretch's tail to the other kind hands it another
// job's procedure, and dropping it withholds a clause that binds it.
func workerKitForKind(kit string, kind workerKind) (string, error) {
	if !isWorkerKind(string(kind)) {
		return "", deskkit.Unverifiable("worker kit has no kind "+string(kind), nil)
	}
	var out []string
	open := ""      // the kind stretch that is open, "" = none
	inCopy := false // inside the common-clause copy
	copies := 0     // how many common-clause copies the file carries
	for i, line := range strings.Split(kit, "\n") {
		switch strings.TrimSpace(line) {
		case commonCopyBegin:
			if inCopy || open != "" || copies > 0 {
				return "", kindMarkerDefect(i, "opens a common-clause copy inside another stretch, or a second one")
			}
			inCopy = true
			copies++
			continue
		case commonCopyEnd:
			if !inCopy {
				return "", kindMarkerDefect(i, "closes a common-clause copy that was never opened")
			}
			inCopy = false
			continue
		}
		if strings.Contains(line, kindMarkerPrefix) {
			m := kindMarkerRE.FindStringSubmatch(line)
			if m == nil {
				return "", kindMarkerDefect(i, "is not a whole-line `<!-- kind:<name>:begin|end -->` marker")
			}
			name, edge := m[1], m[2]
			switch {
			case !isWorkerKind(name):
				return "", kindMarkerDefect(i, "names kind "+name+", which a worker kit is not cut for")
			case inCopy:
				return "", kindMarkerDefect(i, "sits inside the common-clause copy")
			case edge == "begin" && open != "":
				return "", kindMarkerDefect(i, "opens "+name+" inside the open "+open+" stretch")
			case edge == "end" && open != name:
				return "", kindMarkerDefect(i, "closes "+name+" with no matching begin")
			}
			if edge == "begin" {
				open = name
			} else {
				open = ""
			}
			continue
		}
		if inCopy || (open != "" && open != string(kind)) {
			continue
		}
		// A stretch cut out between two blank lines leaves them adjacent; the kits never
		// carry two in a row, so folding them only repairs the cut's own seam.
		if strings.TrimSpace(line) == "" && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		out = append(out, line)
	}
	if open != "" {
		return "", deskkit.Unverifiable("worker kit kind marker for "+open+" is never closed — "+
			"do not dispatch on a kit whose kind boundaries cannot be read", nil)
	}
	if inCopy {
		return "", deskkit.Unverifiable("worker kit common-clause copy is never closed — "+
			"do not dispatch on a kit whose boundaries cannot be read", nil)
	}
	text := strings.TrimSpace(strings.Join(out, "\n"))
	if text == "" {
		return "", deskkit.Unverifiable("worker kit is EMPTY after the cut for "+string(kind)+" — an agent "+
			"dispatched with an empty clause set looks like a successful dispatch and is not one", nil)
	}
	return text + "\n", nil
}

func kindMarkerDefect(index int, what string) error {
	return deskkit.Unverifiable("worker kit line "+strconv.Itoa(index+1)+" "+what+" — do not dispatch on "+
		"a kit whose kind boundaries cannot be read", nil)
}

// workerKitText is the class-kit text THIS dispatch quotes. A worker kit is cut for the kind
// of run; every other kit is quoted as written.
func workerKitText(o dispatchOpts) (string, error) {
	kit, err := kitText(o.kit)
	if err != nil || !workerKit(o.kit) {
		return kit, err
	}
	return workerKitForKind(kit, workerKindOf(o))
}

// writeResumeIsolation is the shepherding run's "if you must recreate it" recipe. The
// implementing recipe cuts a detached tree at the mainline tip, which for a resumed change is
// the wrong tree: the work is on the change's source branch.
func writeResumeIsolation(b *strings.Builder, plan dispatchPlan, base, home string) {
	b.WriteString("Work in an owned worktree OF THAT REPO. It already exists, on the open change's source branch; " +
		"if you must recreate it:\n\n")
	fmt.Fprintf(b, "```\ngit -C %s fetch origin\ngit -C %s worktree add %s %s\n```\n\n", base, base, home, plan.branch)
	if plan.resume != nil && plan.resume.head != "" {
		fmt.Fprintf(b, "A recreated worktree must be at, or ahead of, the head recorded at dispatch — "+
			"`git merge-base --is-ancestor %s HEAD` must succeed inside it; if it does not, stop and report.\n\n",
			plan.resume.head)
	}
}

// writeShepherdAssignment emits the action half for a run dispatched onto an OPEN change. It
// opens nothing: the change exists, the dispatcher has already registered the work entry
// against it, and the worktree is cut at the change's own head. What it keeps from the
// implementing half, in the same words, is everything that is not about opening a change —
// the loop identity, the stop at `implemented`, the issue-comment verb and the claim release.
func writeShepherdAssignment(b *strings.Builder, o dispatchOpts, plan dispatchPlan, repo string) {
	b.WriteString("## Work the open change — do not open another\n\n")
	fmt.Fprintf(b, "This dispatch RESUMES `%s#%d`. The change is already open and your worktree is on its source "+
		"branch `%s`", repo, o.pr, plan.branch)
	if plan.resume != nil && plan.resume.head != "" {
		fmt.Fprintf(b, ", cut at `%s` — the head the forge reported at dispatch", plan.resume.head)
	}
	b.WriteString(". Do NOT run `deskpr create`: one item is one branch and one draft PR, and this item has its " +
		"PR. Push follow-up commits with `deskpr update` from INSIDE your worktree, correct the PR's own body or " +
		"title with `deskpr edit`, and reply on it with `deskreply`. " +
		"Stop at `implemented`: never set verified/done and never flip a PR ready.\n\n")
	b.WriteString("Every clause below binds the change you now carry, whoever opened it: what the kit requires of " +
		"a change before hand-back — its board row, fail-first evidence for a test it adds, a changelog fragment " +
		"where the repo enforces one — is required of this one. Where a clause is timed \"before `deskpr create`\" " +
		"or \"before you open the PR\", read it for this run as \"before the push that carries it\".\n\n")
	if plan.dl.crossRepo(o.root) {
		fmt.Fprintf(b, "CROSS-REPO delivery: this brief is tracked in %s, not in `%s`. Run `deskpr update --root %s` "+
			"(still from INSIDE your worktree) so the `Brief:` trailer resolves against the tracking repo's board. "+
			"That checkout is READ-ONLY for you — never write, commit or branch there — and this PR cannot flip the "+
			"tracking repo's board row: say in the PR body that the home row is handed back separately, and that "+
			"the brief's status is the minimum over its constituent PRs.\n\n",
			trackingLabel(plan.dl), repo, plan.dl.trackingRoot)
	}

	b.WriteString("Before any desk write verb (`deskpr update`, `deskpr edit`, `deskfile`, `deskreply`) — " +
		"set your OWN loop identity in this shell (do NOT inherit the dispatching desk's):\n\n")
	b.WriteString("```\nexport DESK_LOOP=worker-desk\n```\n\n")

	if n, ok := issueNumFromItem(o.item); ok {
		fmt.Fprintf(b, "To post on the ISSUE you were dispatched from (a `BLOCKED-ON-HUMAN` report, a "+
			"could-not-check note), the sanctioned verb is `deskfile attach -R %s --to %d --body-file F` "+
			"(with DESK_LOOP set, above). `deskreply` is for your OWN open PR only, and a hand-rolled `gh` "+
			"write on the issue bypasses deskfile's dedupe, budget and self-containment gates.\n\n", repo, n)
	}

	b.WriteString("Release the dispatch claim once your branch is pushed — branch-as-claim takes over:\n\n")
	writeReleaseClaim(b, o, plan, repo)
}
