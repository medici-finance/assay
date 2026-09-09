package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Stream WIP cap + `parked` lane (attention-budget/04).
//
// TWO lint rules and one new stream status live here:
//
//   - `stream-cap` caps the number of `status: active` streams in a root. The cap
//     is the operator-set roster value ASSAY_STREAM_CAP (rosterconfig.go),
//     read exactly like every other roster key (env over roster.env, per class).
//     ABSENT ⇒ the rule is INERT: it emits ONE NOTICE (`stream-cap: unset`) and
//     never a PROBLEM. A default number is deliberately NOT invented — a default
//     is a policy nobody ruled.
//
//   - `stream-source` codifies the intake rule "a stream is scaffolded only from
//     an APPROVED spec": a diff that adds (or flips to) a `status: active` stream
//     README must name, in its `spec:` frontmatter, a repo-relative scoping doc
//     whose header parses as `**Status:** approved` (spec/lifecycle-v1.md §8.1).
//     A `parked` stream may cite a `draft`.
//
// DIFFERENTIAL, and why it is keyed on the --changed set rather than emitting a
// bare PROBLEM. A full `statusgen --lint` (the daily board regen, an adopter with
// a standing over-cap tree) must NOT redden: an over-cap count there is a NOTICE
// naming the count and the cap, so a tree that is already over budget does not
// gate every unrelated PR. The PR-side gate — a lint invoked with the PR's
// --changed set — is where a change that ADDS active-stream pressure past the cap
// becomes a PROBLEM. This is the same scoping discipline registerIntegrityScoped
// (registers.go) draws: a defect the diff introduces is a hard PROBLEM; a
// pre-existing one on a path the diff never touched is surfaced as a NOTICE and
// left to main's own regen, which owns it.
//
// The head active-count already encodes the "park to make room" case: parking a
// stream in the same change drops it out of the active count, so if the count is
// still over the cap after the whole diff, the parking did not make room and the
// PROBLEM stands; if it brought the count back to the cap, no PROBLEM fires.

// streamStatusParked is the README `status:` value for a shelved stream: it keeps
// its briefs but is excluded from Next-up and every dispatch view, renders under
// its own `## Parked` board heading, and re-activates by a README flip (itself
// subject to the cap).
const streamStatusParked = "parked"

// streamCapConfigFn resolves the configured active-stream cap and whether it was
// set. Indirected through a package var so tests exercise the rule without the
// per-class roster machinery; production reads the shared roster config, so an
// operator sets ASSAY_STREAM_CAP exactly like every other roster key.
var streamCapConfigFn = func() (cap int, set bool) {
	c := scanEffectiveConfig()
	return c.StreamCap, c.StreamCapSet
}

// streamCapEcho renders the ASSAY_STREAM_CAP effective-config value: the number
// when set, "(unset)" otherwise.
func streamCapEcho(cap int, set bool) string {
	if !set {
		return "(unset)"
	}
	return fmt.Sprintf("%d", cap)
}

// streamBaseIsActive reports whether the stream README at repo-relative relpath
// was ALREADY `status: active` on the diff base (origin/main). A README absent on
// base, or non-active on base, is an ADD or a flip-to-active for cap/source
// purposes. Indirected for tests; production reads git. It FAILS toward "not
// already active" (an add): when git cannot answer, a changed active README is
// treated as diff-introduced, the fail-closed direction for a budget gate (a
// human can always park a stream to clear it).
var streamBaseIsActive = func(root, relpath string) bool {
	return productionStreamBaseIsActive(root, relpath)
}

// productionStreamBaseIsActive reads the stream README at relpath as it stood on
// origin/main and reports whether its `status:` was `active` there. Any error
// (no git, a shallow clone missing the ref, the file absent on base) returns
// false — treat as diff-introduced.
func productionStreamBaseIsActive(root, relpath string) bool {
	mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", remoteMainRef).Output()
	if err != nil {
		return false
	}
	base := strings.TrimSpace(string(mb))
	if base == "" {
		return false
	}
	out, err := exec.Command("git", "-C", root, "show", base+":"+relpath).Output()
	if err != nil {
		return false
	}
	fmRaw, _, err := splitFrontmatter(string(out))
	if err != nil {
		return false
	}
	return frontmatterStatus(fmRaw) == "active"
}

// frontmatterStatus extracts the bare `status:` token from a frontmatter block,
// without a full YAML parse — enough to compare against "active"/"parked".
func frontmatterStatus(fmRaw string) string {
	for _, line := range strings.Split(fmRaw, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "status:"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"'`)
		}
	}
	return ""
}

// streamREADMERel is the repo-relative, slash-form path of a stream's README.
func streamREADMERel(s *Stream) string {
	return "docs/streams/" + s.Name + "/README.md"
}

// activeStreamCount counts the `status: active` streams in the set. `parked`,
// `paused`, `done` do not count — the cap is on ACTIVE streams only.
func activeStreamCount(streams []*Stream) int {
	n := 0
	for _, s := range streams {
		if s.Status == "active" {
			n++
		}
	}
	return n
}

// streamCapLint is the `stream-cap` --lint entry point. `streams` is the root's
// stream set (post-scope is irrelevant — the cap is a per-root property);
// `changed` is the PR's --changed set (nil/empty for a full lint).
func streamCapLint(streams []*Stream, root string, changed []string) (problems, notices []string) {
	cap, set := streamCapConfigFn()
	active := activeStreamCount(streams)

	if !set {
		return nil, []string{fmt.Sprintf(
			"[stream-cap] stream-cap: unset — %d active stream(s) in this root; set ASSAY_STREAM_CAP "+
				"to enforce a per-root cap on active streams (absent, the cap is inert — a default is a policy nobody ruled)",
			active)}
	}

	// Full lint: never a PROBLEM. Name the count and the cap so the standing state
	// is on record (and the daily regen surfaces a standing over-cap).
	if len(changed) == 0 {
		if active > cap {
			return nil, []string{fmt.Sprintf(
				"[stream-cap] %d active streams exceeds the cap of %d — a standing over-cap "+
					"(a full lint never gates); consolidate or park a stream to bring the fleet back under budget",
				active, cap)}
		}
		return nil, []string{fmt.Sprintf("[stream-cap] %d/%d active streams (within cap)", active, cap)}
	}

	// PR-diff gate. A PROBLEM only when the diff ADDS active-stream pressure (a
	// stream README the diff added or flipped to active) AND the whole diff leaves
	// the root over the cap. The count already nets out any park in the same diff.
	if active > cap && len(streamCapAdds(streams, root, changed)) > 0 {
		adds := streamCapAdds(streams, root, changed)
		return []string{fmt.Sprintf(
			"[stream-cap] this change adds an active stream (%s) leaving %d active streams, over the cap of %d — "+
				"no net new streams past the cap: land the new stream `status: parked`, or park/archive an existing "+
				"active stream in the same change (ASSAY_STREAM_CAP)",
			strings.Join(adds, ", "), active, cap)}, nil
	}
	if active > cap {
		// Over cap, but this diff did not add active pressure — a standing over-cap
		// this PR did not introduce. Surfaced, never gated (main's regen owns it).
		return nil, []string{fmt.Sprintf(
			"[stream-cap] %d active streams exceeds the cap of %d — a standing over-cap this change did not introduce", active, cap)}
	}
	return nil, []string{fmt.Sprintf("[stream-cap] %d/%d active streams (within cap)", active, cap)}
}

// streamCapAdds returns the names of streams the diff added-as-active or
// flipped-to-active: a currently-active stream whose README is in the changed set
// and was NOT already active on the base.
func streamCapAdds(streams []*Stream, root string, changed []string) []string {
	set := map[string]bool{}
	for _, c := range changed {
		if n := normalizeChangedPath(c); n != "" {
			set[n] = true
		}
	}
	var adds []string
	for _, s := range streams {
		if s.Status != "active" {
			continue
		}
		rel := streamREADMERel(s)
		if set[rel] && !streamBaseIsActive(root, rel) {
			adds = append(adds, s.Name)
		}
	}
	sort.Strings(adds)
	return adds
}

// streamSourceLint is the `stream-source` --lint entry point. It is differential:
// it binds only a stream README the diff ADDS or flips to active (a full lint
// grandfathers the standing corpus, which predates the `spec:` requirement). An
// added/flipped active stream that does not cite an `approved` spec is a PROBLEM.
func streamSourceLint(streams []*Stream, root string, changed []string) (problems, notices []string) {
	if len(changed) == 0 {
		return nil, nil
	}
	set := map[string]bool{}
	for _, c := range changed {
		if n := normalizeChangedPath(c); n != "" {
			set[n] = true
		}
	}
	for _, s := range streams {
		if s.Status != "active" {
			continue
		}
		rel := streamREADMERel(s)
		if !set[rel] || streamBaseIsActive(root, rel) {
			continue // not in this diff, or already active on base (grandfathered)
		}
		if p := streamSourceProblem(root, s); p != "" {
			problems = append(problems, p)
		}
	}
	return problems, nil
}

// streamSourceProblem is the pure per-stream predicate: an ACTIVE stream must cite
// in `spec:` a repo-relative scoping doc whose §8.1 header state is `approved`.
// A non-active (e.g. `parked`) stream is exempt — it may cite a `draft`, or
// nothing. Returns "" when the stream satisfies the rule.
func streamSourceProblem(root string, s *Stream) string {
	if s.Status != "active" {
		return ""
	}
	spec := strings.TrimSpace(s.Spec)
	if spec == "" {
		return fmt.Sprintf(
			"[stream-source] %s: an active stream names no `spec:` — a stream is scaffolded ONLY from a "+
				"scoping doc whose header is `**Status:** approved`, never from an idea, an issue thread, or a "+
				"`draft`. Add a `spec:` pointing at the approved doc, or land the stream `status: parked`",
			s.Name)
	}
	h, err := parseSpecLifecycleHeader(root, filepath.Join(root, filepath.FromSlash(spec)))
	if err != nil {
		return fmt.Sprintf(
			"[stream-source] %s: `spec: %s` cannot be read (%v) — an active stream must cite a readable scoping "+
				"doc whose header is `**Status:** approved`", s.Name, spec, err)
	}
	if h.State != lifecycleApproved {
		got := h.StateRaw
		if got == "" {
			got = "unclassified"
		}
		return fmt.Sprintf(
			"[stream-source] %s: `spec: %s` is `%s`, not `approved` — an active stream is scaffolded only from an "+
				"APPROVED spec (a `parked` stream may cite a `draft`)", s.Name, spec, got)
	}
	return ""
}
