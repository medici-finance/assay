package main

// siblingmerge.go — the SIBLING-MERGE-UNRECONCILED detector. The SEVENTH
// board-honesty phantom class.
//
// THE DEFECT IT CLOSES. A brief tracked on one repo's board can be DELIVERED by
// a PR merged in a SIBLING repo (a multi-repo convention: code briefs land via
// sibling PRs). The only merge boardhonesty.go's class 1
// (already-merged-unflipped) can see is THIS repo's own first-parent history
// (mergedPRsFromGit) — a worker dispatched into the sibling can only write
// there, so the home row never moves and Next-up keeps offering work that has
// already landed. Six rows sat `todo` for up to eight days after merging in a
// sibling repo before a worker found it by nearly redoing the work.
//
// WHAT THIS DETECTOR IS, PRECISELY. Reading a sibling's git history turns that
// discovery into a printed PROMPT on the day it happens — a prompt to read the
// merged change Task by Task, NEVER proof of delivery. Two of six merges this
// class would have caught turned out to be PARTIAL deliveries; the class never
// derives a lifecycle cell and its NOTICE text never claims "implemented" or
// "delivered". It is the SECOND layer, not the only one: the first is the
// worker's own hand-back (a sibling brief). They fail for different reasons in
// different components — the hand-back fails when a worker forgets or dies, this
// detector fails when the sibling history carries no matchable key (the
// withheld-identifier shape below) — so neither alone closes the class.
//
// SIBLING-SET DERIVATION. A brief names its sibling
// three ways, in order of reliability:
//
//	homed-in: <owner>/<repo>        the brief's OWN de-housing field
//	                                (statusgen/12) — matched against a
//	                                registry entry whose Repo equals it.
//	deliverable_repo: <alias>       an EXPLICIT registry alias —
//	                                an unresolvable alias here IS a
//	                                could-not-check (the declaration says a
//	                                sibling exists; only the LOOKUP failed).
//	../<basename>/ in the raw file  a heuristic: the brief's own frontmatter
//	                                text happens to embed a path prefix
//	                                naming a REGISTERED sibling's checkout
//	                                basename. Unlike deliverable_repo, an
//	                                unregistered basename is simply NOT a
//	                                sibling — never a could-not-check — so an
//	                                incidental "../foo/" in prose can never
//	                                invent a finding (Task item 5's
//	                                "unregistered basename" fixture).
//
// A brief that declares NONE of the three has an EMPTY sibling set: the class
// does not apply to that row at all — not a could-not-check (Interface
// contract item 1).
//
// THREE-STATE, PER ROW PER SIBLING (item 4). checked-failed = a match (the
// NOTICE names the sibling, the short sha, the subject, and which key
// matched). checked-clean = the sibling's history was read and named nothing.
// could-not-check = the sibling checkout is absent, not a git directory,
// shallow, or an unresolvable `deliverable_repo:` alias (unregistered or
// withheld) — reported ONCE PER SIBLING PER RUN, never per row, and never
// rounded to "no phantom" for the rows that needed it.
//
// SEVERITY. NOTICE in `--lint`, exactly like the other six classes
// (boardhonesty.go) — never a PROBLEM, never an exit-code change there (item
// 5): a class that fires the day a sibling merges must never be the thing that
// freezes the whole board's STATUS.md write on that same day. Redness comes
// from two OTHER places instead: the named Next-up eligibility exclusion
// (nextup.go's `MergedInSibling`/`MergedElsewhere`, the HomedIn/HomedElsewhere
// shape) held on a checked-failed TODO row, and the dedicated `statusgen
// phantoms --class sibling-merge-unreconciled` verb (phantomscli.go), which
// exits 0/1/2 without touching the regen path.
//
// WHY THIS IS ITS OWN DRIVER, NOT A classifyPhantom ARM. classifyPhantom
// (boardhonesty.go) is a PURE, single-tree function: every one of the other six
// classes reads only the brief body, the row's own cell, and a merge set
// already computed from THIS repo's `git log`. This class needs a LIVE read of
// ANOTHER repo's checkout — a dependency none of the six carry — so folding it
// into classifyPhantom's signature would smuggle "shell out to a sibling
// checkout" into every existing call site and test of that pure function. It
// is wired ALONGSIDE class 1-6 (main.go calls siblingMergeCheck right after
// boardHonestyNotices) and shares this package's phantom-class vocabulary
// (phantomSiblingMergeUnreconciled, phantomRemediation — boardhonesty.go) so
// its NOTICE reads as the same family, not a bolt-on.
//
// homed-in does NOT suppress this class: it is a
// landed-work fact, like class 1 — homedInSupersedes (boardhonesty.go) never
// lists it.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// siblingMergeHistoryLimit bounds how far back a sibling's first-parent history
// is read — the same depth mergedPRLimit uses for this repo's own history
// (mergedstatus.go), for the same reason: deep enough to cover the window a row
// could still be wrongly todo, shallow enough to stay a sub-second offline
// check.
const siblingMergeHistoryLimit = mergedPRLimit

// siblingRootsEnv is the environment variable the desk tools already use for
// "where does each registered repo's checkout live on this machine"
// (tools/desk/internal/deskkit/roots.go's DESK_ROOTS,
// "<owner>/<repo>=<path>,..."). This detector reads the SAME variable — never a
// second name — so a desk session that already exported it for deskboard/
// verifyloop gets sibling-checkout resolution for free; statusgen is a
// separate Go module from tools/desk (its own go.mod) so it cannot import
// deskkit's parser, but it reads the identical format.
const siblingRootsEnv = "DESK_ROOTS"

// siblingTarget is one resolved sibling-repo target: a registry alias, its
// published "<owner>/<repo>", and the checkout directory basename used for
// default root resolution. Only PUBLISHED, registered aliases ever become a
// siblingTarget — an unpublished or unregistered candidate is either a
// could-not-check (an explicit `deliverable_repo:` alias that cannot be
// resolved) or simply not a sibling (a heuristic homed-in/basename candidate
// that matches no registry entry), never a siblingTarget.
type siblingTarget struct {
	Alias    string
	Repo     string // "<owner>/<repo>"
	Basename string
}

// siblingRootFlags is a repeatable `--sibling-root <owner>/<repo>=<path>` CLI
// flag (item 2), the same shape flag.Var already uses for --root/--budget in
// main.go.
type siblingRootFlags []string

func (f *siblingRootFlags) String() string { return strings.Join(*f, ", ") }
func (f *siblingRootFlags) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// siblingRootFlagValues is the top-level --sibling-root flag's accumulator
// (wired in main.go's flag.Var call). It is a PACKAGE-level var, not a local
// one like --root/--budget's, because run() — which calls siblingMergeCheck —
// takes no such parameter: threading one through would touch every one of
// run()'s ~90 existing call sites across this package's tests for a flag none
// of them set. Empty (its zero value) on every run that never passes
// --sibling-root, which is every existing test.
var siblingRootFlagValues siblingRootFlags

// parseSiblingRootSpecs parses a list of "<owner>/<repo>=<path>" entries (from
// either --sibling-root's repeated flag values or a DESK_ROOTS-shaped
// comma-separated environment string already split by the caller) into an
// override map. Malformed entries are DROPPED rather than treated as a hard
// error: this is best-effort local configuration for an advisory NOTICE
// detector, not a security boundary, and a typo'd entry must not crash the
// board build — the sibling it names simply falls back to the default
// sibling-checkout path and, if that is also absent, reports could-not-check
// by itself.
func parseSiblingRootSpecs(specs []string) map[string]string {
	out := map[string]string{}
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		repo, path, ok := strings.Cut(spec, "=")
		repo, path = strings.TrimSpace(repo), strings.TrimSpace(path)
		if !ok || repo == "" || path == "" {
			continue
		}
		out[repo] = path
	}
	return out
}

// parseDeskRootsEnv reads DESK_ROOTS and parses it the same way
// parseSiblingRootSpecs does, splitting on commas first.
func parseDeskRootsEnv() map[string]string {
	raw := strings.TrimSpace(os.Getenv(siblingRootsEnv))
	if raw == "" {
		return nil
	}
	return parseSiblingRootSpecs(strings.Split(raw, ","))
}

// mergeSiblingRootOverrides combines the env-sourced and flag-sourced override
// maps, flag entries winning on a collision (item 2 lists the flag first: "also
// read from the environment", i.e. the flag is the more specific override).
func mergeSiblingRootOverrides(fromEnv, fromFlag map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range fromEnv {
		out[k] = v
	}
	for k, v := range fromFlag {
		out[k] = v
	}
	return out
}

// effectiveSiblingRootOverrides is the ONE call site every consumer (run()'s
// pipeline in main.go, the `phantoms` verb) uses to resolve the override map
// from the process's real environment and CLI flags.
func effectiveSiblingRootOverrides(flagSpecs []string) map[string]string {
	return mergeSiblingRootOverrides(parseDeskRootsEnv(), parseSiblingRootSpecs(flagSpecs))
}

// basenameOf returns the last path segment of an "<owner>/<repo>" value.
func basenameOf(ownerRepo string) string {
	if i := strings.LastIndexByte(ownerRepo, '/'); i >= 0 {
		return ownerRepo[i+1:]
	}
	return ownerRepo
}

// ownRepoFor determines the "<owner>/<repo>" this very tree lives in, so
// siblingTargetsForBrief can exclude the registry entry that names it: a
// board's own repo is never its own sibling — an own-repo merge is class 1's
// job (already-merged-unflipped), which has its own semantics, and reading
// the board's own history back through this detector reports "merged in a
// sibling" naming the home repo on any own-repo commit that happens to
// mention a todo row's id (an audit or authoring commit, say).
//
// The registry's own `self:` key wins when present — it is an explicit,
// unambiguous declaration. Absent that, it falls back to the repo a stream's
// own `repo:` frontmatter declares (rootRepo, multiroot.go) — the same value
// the dispatch-queue JSON and the roadmap deck already attribute a root to.
// "" (neither present) means this tree has no declared identity to compare
// against, so no registry entry is excluded on this basis — the path-based
// backstop in siblingMergeCheck is what catches that case instead.
func ownRepoFor(reg *graphRepos, streams []*Stream) string {
	if reg != nil && reg.Self != "" {
		if entry, ok := reg.Aliases[reg.Self]; ok && !entry.Unpublished && entry.Repo != "" {
			return entry.Repo
		}
	}
	repo, _ := rootRepo(streams)
	return repo
}

// siblingTargetsForBrief resolves the sibling set for one brief. rawBody is
// the brief file's raw, unparsed content — frontmatter and prose
// together — so a `../<basename>/` occurrence anywhere in it (a `sources:`,
// `consumers:`, or a free-text Context-section `files:` line) is visible
// without a second structured parser. unresolved carries one entry per
// DECLARED-but-unresolvable `deliverable_repo:` alias (a could-not-check) — a
// heuristic candidate that fails to resolve is silently dropped instead
// (never could-not-check — an incidental "../foo/" naming no registered
// sibling is simply not a sibling). ownRepo, when non-empty, excludes any
// registry entry whose Repo matches it: the board's own repo is never a
// candidate sibling, from either derivation path.
func siblingTargetsForBrief(b Brief, rawBody string, reg *graphRepos, ownRepo string) (targets []siblingTarget, unresolved []string) {
	seen := map[string]bool{}
	add := func(t siblingTarget) {
		if seen[t.Alias] {
			return
		}
		seen[t.Alias] = true
		targets = append(targets, t)
	}

	if reg != nil {
		// Deterministic order: sort the alias set before walking it, so two
		// runs over the same tree produce byte-identical NOTICE ordering.
		aliases := make([]string, 0, len(reg.Aliases))
		for alias := range reg.Aliases {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)

		for _, alias := range aliases {
			entry := reg.Aliases[alias]
			if entry.Unpublished || entry.Repo == "" {
				continue
			}
			if ownRepo != "" && entry.Repo == ownRepo {
				// The board's own repo is never its own sibling — see
				// ownRepoFor's comment. Skip silently: this is not a
				// could-not-check, it is simply not a candidate.
				continue
			}
			basename := basenameOf(entry.Repo)
			// homed-in: <owner>/<repo> — must match a registry entry's Repo
			// exactly. An owner/repo naming no registered sibling is NOT a
			// sibling for this detector's purposes (item 1's ground rule):
			// only cell-registered repos are eligible git-history reads.
			if b.HomedIn != "" && b.HomedIn == entry.Repo {
				add(siblingTarget{Alias: alias, Repo: entry.Repo, Basename: basename})
			}
			// ../<basename>/ heuristic: scan the WHOLE raw file for the
			// checkout basename of a KNOWN, PUBLISHED registry entry, rather
			// than extracting an arbitrary basename from prose and looking it
			// up. This makes an unregistered "../foo/" in prose structurally
			// unable to invent a finding — there is no registry entry to
			// compare it against.
			if strings.Contains(rawBody, "../"+basename+"/") {
				add(siblingTarget{Alias: alias, Repo: entry.Repo, Basename: basename})
			}
		}
	}

	if b.DeliverableRepo != "" {
		switch {
		case reg == nil:
			unresolved = append(unresolved, fmt.Sprintf(
				"deliverable_repo: %s cannot be resolved — no docs/streams/graph-repos.yaml registry", b.DeliverableRepo))
		default:
			entry, ok := reg.Aliases[b.DeliverableRepo]
			switch {
			case !ok:
				unresolved = append(unresolved, fmt.Sprintf(
					"deliverable_repo: %s is not in docs/streams/graph-repos.yaml", b.DeliverableRepo))
			case entry.Unpublished || entry.Repo == "":
				unresolved = append(unresolved, fmt.Sprintf(
					"deliverable_repo: %s is unpublished — its target repo is not resolvable from this tree", b.DeliverableRepo))
			case ownRepo != "" && entry.Repo == ownRepo:
				// An explicit deliverable_repo: naming the board's own repo is
				// the same non-sibling case as the homed-in/basename walk
				// above — skip silently, never a could-not-check.
			default:
				add(siblingTarget{Alias: b.DeliverableRepo, Repo: entry.Repo, Basename: basenameOf(entry.Repo)})
			}
		}
	}

	return targets, unresolved
}

// siblingRootPath resolves the local checkout path for a target: an override
// (flag/env, keyed by "<owner>/<repo>") wins; otherwise the sibling-checkout
// convention eligibility.go's resolveCrossRepoBriefRef already uses — a
// directory named after the basename, next to root.
func siblingRootPath(root string, target siblingTarget, overrides map[string]string) string {
	if p, ok := overrides[target.Repo]; ok && p != "" {
		return p
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	return filepath.Join(filepath.Dir(absRoot), target.Basename)
}

// pathIsRoot reports whether path resolves to this tree's own root — the
// backstop ownRepoFor's name-based exclusion needs (finding C1): a registry
// entry can name a DIFFERENT alias/repo string than the one this tree
// declares for itself (a stale `self:`, a registry/name mismatch, a symlinked
// checkout) and still resolve, via siblingRootPath's directory-next-to-root
// convention or an explicit --sibling-root override, to the SAME directory
// root already is. Reading that directory's git history back against root's
// own briefs is exactly class 1's job (already-merged-unflipped), not this
// detector's — so this is a path-identity check, never a repo-name one.
//
// Resolved with EvalSymlinks first (a checkout reached through a symlink, or
// CI's own working-copy symlink shape, must compare equal to the real
// directory it points at); either side failing to resolve (most commonly:
// path does not exist yet — the ordinary could-not-check case one caller down
// handles on its own) falls back to a plain Abs+Clean comparison, so a
// not-yet-existing sibling checkout is never misreported as "is root" and
// never panics.
func pathIsRoot(root, path string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	resolvedRoot, rootErr := filepath.EvalSymlinks(absRoot)
	resolvedPath, pathErr := filepath.EvalSymlinks(absPath)
	if rootErr == nil && pathErr == nil {
		return resolvedRoot == resolvedPath
	}
	return filepath.Clean(absRoot) == filepath.Clean(absPath)
}

// checkSiblingCheckout reports whether path is a usable, non-shallow git
// checkout — the could-not-check causes item 4 names: root absent, not a git
// dir, a shallow clone, or git itself failing.
func checkSiblingCheckout(path string) (ok bool, reason string) {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return false, fmt.Sprintf("sibling checkout not found at %s", path)
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return false, fmt.Sprintf("%s is not a git directory (no .git)", path)
	}
	out, err := exec.Command("git", "-C", path, "rev-parse", "--is-shallow-repository").Output()
	if err != nil {
		return false, fmt.Sprintf("git rev-parse --is-shallow-repository failed at %s: %v", path, err)
	}
	if strings.TrimSpace(string(out)) == "true" {
		return false, fmt.Sprintf("%s is a shallow clone — first-parent history is truncated", path)
	}
	return true, ""
}

// siblingCommit is one first-parent commit of a sibling's history, reduced to
// what matching needs.
type siblingCommit struct {
	SHA     string
	Subject string
	Body    string // the full raw commit message (%B) — subject IS its first line
	PR      int    // 0 when the subject carries neither squash nor merge-commit form
}

// readSiblingCommits reads a sibling checkout's first-parent history. The
// format uses %x1e (record separator) ahead of each commit and %x00 between
// its hash and its raw body, so a multi-line %B can never be mistaken for a
// record boundary — mergedPRsFromGit (mergedstatus.go) can get away with
// %s-per-line because it reads only the SUBJECT; this detector needs the
// full body for the body-only id shape (fact 3) and the Issue:/closing-keyword
// trailer (key k2), so it cannot use a newline-delimited format.
func readSiblingCommits(path string) ([]siblingCommit, error) {
	out, err := exec.Command("git", "-C", path, "log", "--first-parent",
		"-n", strconv.Itoa(siblingMergeHistoryLimit), "--format=%x1e%H%x00%B").Output()
	if err != nil {
		return nil, err
	}
	var commits []siblingCommit
	for _, rec := range strings.Split(string(out), "\x1e") {
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		sha := parts[0]
		body := strings.TrimRight(parts[1], "\n")
		subject := body
		if i := strings.IndexByte(body, '\n'); i >= 0 {
			subject = body[:i]
		}
		pr := 0
		if m := mergeSubjectRe.FindStringSubmatch(subject); m != nil {
			pr, _ = strconv.Atoi(m[1])
		} else if m := squashSubjectRe.FindStringSubmatch(subject); m != nil {
			pr, _ = strconv.Atoi(m[1])
		}
		commits = append(commits, siblingCommit{SHA: sha, Subject: subject, Body: body, PR: pr})
	}
	return commits, nil
}

// siblingIDPattern matches a brief id in either <stream>/<NN> or <stream>-<NN>
// form (key k1), word-bounded so it can never match as a substring of a longer
// token.
func siblingIDPattern(id string) *regexp.Regexp {
	hyphen := strings.Replace(id, "/", "-", 1)
	return regexp.MustCompile(`\b(?:` + regexp.QuoteMeta(id) + `|` + regexp.QuoteMeta(hyphen) + `)\b`)
}

// siblingIssuePattern matches an `Issue: #<N>` trailer or a GitHub closing
// keyword for issue #<N> (key k2).
func siblingIssuePattern(n int) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:\bissue:\s*#|\b(?:close[sd]?|fixe[sd]?|resolve[sd]?)\s+#)` + strconv.Itoa(n) + `\b`)
}

// matchedSiblingCommit is one commit that matched a row, via which key.
type matchedSiblingCommit struct {
	Commit siblingCommit
	Via    string // "id" | "tracked-in"
}

// matchSiblingCommits walks commits NEWEST-FIRST (git log's own order) and
// returns every one matching id (k1) or one of trackedNums (k2, the issue
// numbers this row declared `tracked-in:` for THIS sibling's alias) — at most
// one entry per commit, in newest-first order, so the caller's "most recent
// match" is simply the first element.
func matchSiblingCommits(id string, trackedNums []int, commits []siblingCommit) []matchedSiblingCommit {
	idPat := siblingIDPattern(id)
	issuePats := make([]*regexp.Regexp, 0, len(trackedNums))
	for _, n := range trackedNums {
		issuePats = append(issuePats, siblingIssuePattern(n))
	}
	var out []matchedSiblingCommit
	for _, c := range commits {
		switch {
		case idPat.MatchString(c.Body):
			out = append(out, matchedSiblingCommit{Commit: c, Via: "id"})
		default:
			for _, p := range issuePats {
				if p.MatchString(c.Body) {
					out = append(out, matchedSiblingCommit{Commit: c, Via: "tracked-in"})
					break
				}
			}
		}
	}
	return out
}

// trackedIssueNumsFor returns the issue numbers a brief's `tracked-in:` list
// declares for one specific sibling alias — "<alias>#<N>" entries whose alias
// matches.
func trackedIssueNumsFor(trackedIn []string, alias string) []int {
	var out []int
	for _, ref := range trackedIn {
		a, nStr, ok := strings.Cut(ref, "#")
		if !ok || a != alias {
			continue
		}
		if n, err := strconv.Atoi(nStr); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// deliveryAck reports whether a brief's `delivery:` claims acknowledge PR
// number pr in the named sibling alias, and how (item 7). pr==0 (a matched
// commit whose subject carries no parseable PR number) can never be
// acknowledged — there is nothing for a `delivery:` entry's `in:` ref to name.
func deliveryAck(delivery []DeliveryClaim, alias string, pr int) (full, partial bool) {
	if pr == 0 {
		return false, false
	}
	want := alias + "#" + strconv.Itoa(pr)
	for _, d := range delivery {
		if d.In != want {
			continue
		}
		switch d.Covers {
		case "full":
			full = true
		case "partial":
			partial = true
		}
	}
	return full, partial
}

// siblingMergeCheck is the DRIVER: for every todo/in-progress brief across
// streams, it derives the sibling set (item 1), resolves and reads each
// distinct sibling checkout ONCE (never once per row), matches k1/k2, applies
// the `delivery:` acknowledgement (item 7), and returns:
//
//   - notices: the --lint/`phantoms` NOTICE lines (checked-failed findings,
//     one could-not-check line per sibling per run).
//   - checkedFailed: count of ROWS carrying at least one live (unreleased)
//     checked-failed finding — the `phantoms` verb's exit-1 condition.
//   - couldNotCheck: count of DISTINCT could-not-check causes (siblings/
//     aliases) this run hit — the `phantoms` verb's exit-2 condition.
//
// It also MUTATES Brief.MergedInSibling on every checked-failed TODO row that
// is not released by an acknowledged partial delivery claim (item 5(i), item
// 7) — the Next-up eligibility exclusion nextup.go's eligibleBase reads, in
// the same shape HomedIn already uses. An in-progress row is NEVER mutated
// (item 6: surfaced, never excluded) even when it carries the same finding.
func siblingMergeCheck(streams []*Stream, root string, overrides map[string]string) (notices []string, checkedFailed, couldNotCheck int) {
	reg, _, _ := loadGraphRepos(root)
	ownRepo := ownRepoFor(reg, streams)

	type rowRef struct {
		stream *Stream
		idx    int // index into stream.Briefs
		id     string
		body   string
	}
	// neededBySibling groups every row that needs a given (already-resolved)
	// sibling target, so its checkout is read ONCE regardless of how many
	// rows reference it.
	neededBySibling := map[string][]rowRef{}
	targetByAlias := map[string]siblingTarget{}
	unresolvedReasons := map[string]bool{} // deduped could-not-check lines from deliverable_repo resolution

	pathByNumFor := func(s *Stream) map[string]string {
		m := map[string]string{}
		for _, path := range briefFilePaths(s) {
			if _, num, ok := expectedBriefID(path); ok {
				m[num] = path
			}
		}
		return m
	}

	for _, s := range streams {
		pathByNum := pathByNumFor(s)
		for i, b := range s.Briefs {
			if b.Status != "todo" && b.Status != "in-progress" {
				continue
			}
			body := ""
			if path, ok := pathByNum[b.Num]; ok {
				if raw, err := os.ReadFile(path); err == nil {
					body = string(raw)
				}
				// An unreadable brief file here is silently skipped: boardhonesty.go's
				// own could-not-check NOTICE for the same read already covers it, and
				// duplicating it would double-report one root cause under two class
				// names.
			}
			targets, unresolved := siblingTargetsForBrief(b, body, reg, ownRepo)
			for _, u := range unresolved {
				unresolvedReasons[u] = true
			}
			if len(targets) == 0 {
				continue
			}
			id := s.Name + "/" + b.Num
			for _, t := range targets {
				targetByAlias[t.Alias] = t
				neededBySibling[t.Alias] = append(neededBySibling[t.Alias], rowRef{stream: s, idx: i, id: id, body: body})
			}
		}
	}

	for u := range unresolvedReasons {
		notices = append(notices, fmt.Sprintf(
			"could-not-check: sibling-merge-unreconciled — %s. No conclusion is drawn about whether work for this repo's briefs has already merged there.", u))
		couldNotCheck++
	}

	aliases := make([]string, 0, len(neededBySibling))
	for alias := range neededBySibling {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	for _, alias := range aliases {
		target := targetByAlias[alias]
		rows := neededBySibling[alias]
		path := siblingRootPath(root, target, overrides)
		if pathIsRoot(root, path) {
			// Backstop for ownRepoFor's name-based exclusion: whatever the
			// repo-name comparison concluded, if the resolved checkout path IS
			// this tree's own root (a symlink, a same-directory override, or a
			// registry/name mismatch this run could not otherwise catch), it is
			// not an external sibling to read. Skip silently — never a
			// could-not-check, never a checked read of the tree against itself.
			continue
		}
		if ok, reason := checkSiblingCheckout(path); !ok {
			notices = append(notices, fmt.Sprintf(
				"could-not-check: sibling-merge-unreconciled could not read %s's history at %s (%s) — no conclusion is drawn about whether work for this repo's briefs has already merged there.",
				target.Repo, path, reason))
			couldNotCheck++
			continue
		}
		commits, err := readSiblingCommits(path)
		if err != nil {
			notices = append(notices, fmt.Sprintf(
				"could-not-check: sibling-merge-unreconciled could not read %s's history at %s (%v) — no conclusion is drawn about whether work for this repo's briefs has already merged there.",
				target.Repo, path, err))
			couldNotCheck++
			continue
		}

		for _, row := range rows {
			b := row.stream.Briefs[row.idx]
			trackedNums := trackedIssueNumsFor(b.TrackedIn, alias)
			matched := matchSiblingCommits(row.id, trackedNums, commits)
			if len(matched) == 0 {
				continue // checked-clean for this row+sibling — silent
			}

			var unacked, fullAcked *matchedSiblingCommit
			var released bool
			for i := range matched {
				m := &matched[i]
				full, partial := deliveryAck(b.Delivery, alias, m.Commit.PR)
				switch {
				case full:
					if fullAcked == nil {
						fullAcked = m
					}
				case partial:
					released = true
				default:
					unacked = m
				}
				if unacked != nil {
					break // most-recent unacked match wins immediately
				}
			}

			switch {
			case unacked != nil:
				checkedFailed++
				notices = append(notices, fmt.Sprintf(
					"NON-DISPATCHABLE (%s): %s — a change in %s (%s, %q) names this brief (matched via %s) — "+
						"check what it covers, Task by Task; this NEVER derives a lifecycle cell. Fix: %s. (board-honesty)",
					phantomSiblingMergeUnreconciled, row.id, target.Repo, shortSHA(unacked.Commit.SHA),
					unacked.Commit.Subject, unacked.Via, phantomRemediation[phantomSiblingMergeUnreconciled]))
				if b.Status == "todo" {
					row.stream.Briefs[row.idx].MergedInSibling = target.Repo
				}
			case fullAcked != nil:
				checkedFailed++
				notices = append(notices, fmt.Sprintf(
					"NON-DISPATCHABLE (%s): %s — claimed delivered in %s (%s, PR #%d) via a `delivery: covers: full` "+
						"claim, but the row's cell has not landed here yet — check the claim and reconcile the row. (board-honesty)",
					phantomSiblingMergeUnreconciled, row.id, target.Repo, shortSHA(fullAcked.Commit.SHA), fullAcked.Commit.PR))
				if b.Status == "todo" {
					row.stream.Briefs[row.idx].MergedInSibling = target.Repo
				}
			case released:
				// Every match is covered by an acknowledged `covers: partial`
				// claim: the rest is real work, so the row is eligible again —
				// no NOTICE, no mutation.
			}
		}
	}

	sort.Strings(notices)
	return notices, checkedFailed, couldNotCheck
}
