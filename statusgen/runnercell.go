package main

import (
	"errors"
	"regexp"
	"strings"
)

// The one reading of a run's Runner cells (#6767).
//
// A verifier run can record its rows in more than one table: the witness table
// statusgen verifyrun writes, whose Runner cell ends in the identity source
// ("(forge-identity)"), and a hand-written Expected/Observed table beside it,
// whose Runner cell names the model instead ("(claude-opus-5-5 verifier)").
// Both name one login at one sha for one principal. verifyflip used to demand
// byte-equal qualifiers and refused such a run, while `brief --check-verified`
// read no Runner cell at all and admitted it. verifyflip now reads the run
// through runnerCellsAgree, and check-verified refuses exactly the runs
// verifyflip's own run selector refuses on that reading
// (latestPassRunnerRefusal). A run verifyflip refuses for another reason, such
// as a Runner cell in another form, is not check-verified's question.
//
// The reading: every row names one login and one sha (an exact prefix of at
// least treeSHALen counts as the same sha, flipSHAAgree). When every row's
// text after the sha is the same, nothing else is read, as before. When it
// differs, each tail must be a list of parenthesised qualifiers (flipQualRe),
// and each qualifier is one of three kinds:
//
//   - a description of who ran it (runnerDescQualRe: "claude-opus-5-5
//     verifier", "model verifier", "model verifier claude-opus-5-5"), which
//     names no identity and may differ from row to row;
//   - an identity source (runnerSourceForge, runnerSourceCIEnv,
//     runnerSourceGitConfig, the constants verifyrun writes), which a row may
//     omit but never contradict: every row that carries one carries the same;
//   - anything else (a principal such as "on-behalf-of human:<name>", a tree
//     state, an unknown note), which must be the same list, in the same order,
//     on every row.
//
// A tree suffix written directly after the sha (+dirty, +unknown) is not a
// qualifier: it is read apart, ahead of the list, and must be the same text on
// every row (#5530), so rows on one dirty tree that differ only in a
// description or a source read as one runner, while +dirty against a bare sha
// or against +unknown still refuses. A tail that is not a clean qualifier list
// after its tree suffix must equal every other tail exactly. The stamp then
// carries the shared tree suffix and only what every row says: a qualifier of
// the first row that every row carries verbatim. A source only some rows carry is
// left out, so a hand-written row never lends its run an identity source it
// did not record, and the stamp never claims more than the weakest row.

// runnerDescQualRe is the closed grammar of a descriptive Runner qualifier.
var runnerDescQualRe = regexp.MustCompile(`^(?:model verifier(?: [a-z0-9][a-z0-9.-]*)?|[a-z0-9][a-z0-9.-]* verifier)$`)

// runnerCellRead is one Runner cell as runnerCellsAgree reads it.
type runnerCellRead struct {
	login, sha, tail string
	tree             string   // the tree suffix after the sha (+dirty, +unknown), or ""
	quals            []string // the parenthesised qualifiers; nil when tail is not a clean list
	clean            bool     // tail after tree is a clean qualifier list (or empty)
}

func readRunnerCell(cell string) (runnerCellRead, bool) {
	m := flipRunnerRe.FindStringSubmatch(cell)
	if m == nil {
		return runnerCellRead{}, false
	}
	r := runnerCellRead{login: m[1], sha: m[2], tail: m[3], clean: true}
	r.tree = runnerTreeRe.FindString(m[3])
	for t := m[3][len(r.tree):]; t != ""; {
		q := flipQualRe.FindStringSubmatch(t)
		if q == nil {
			if strings.TrimSpace(t) != "" {
				r.clean, r.quals = false, nil
			}
			break
		}
		r.quals = append(r.quals, q[1])
		t = t[len(q[0]):]
	}
	return r, true
}

// runnerTreeRe is the tree suffix verifyrun writes directly after the sha
// (sha + "+dirty", the container receipt's +unknown): the text before the
// first space or parenthesis.
var runnerTreeRe = regexp.MustCompile(`^[^()\s]*`)

func runnerSourceQual(q string) bool {
	return q == runnerSourceForge || q == runnerSourceCIEnv || q == runnerSourceGitConfig
}

// runnerIdentityQuals is the qualifiers of r that must agree on every row:
// neither descriptive nor an identity source.
func runnerIdentityQuals(r runnerCellRead) []string {
	var out []string
	for _, q := range r.quals {
		if !runnerDescQualRe.MatchString(q) && !runnerSourceQual(q) {
			out = append(out, q)
		}
	}
	return out
}

// runnerCellsAgree reads the Runner cells of one run (cells, in row order) as
// one runner. It returns the run's login, its sha (the longest form any row
// records) and the stamp tail; the error is a *flipRefusal naming the
// disagreement. A cell that does not read `<login> @ <sha>` refuses.
func runnerCellsAgree(cells []string) (login, sha, tail string, err error) {
	if len(cells) == 0 {
		return "", "", "", refuseFlip("runner mismatch: no Date/Runner rows are recorded for the latest PASS")
	}
	reads := make([]runnerCellRead, len(cells))
	for i, c := range cells {
		r, ok := readRunnerCell(c)
		if !ok {
			return "", "", "", refuseFlip("sha mismatch: runner %q does not read <login> @ <sha>", c)
		}
		reads[i] = r
		if len(r.sha) > len(sha) {
			sha = r.sha
		}
	}
	r0 := reads[0]
	same := true
	for i, r := range reads {
		if r.login != r0.login {
			return "", "", "", refuseRunner("runner mismatch: rows of the latest PASS name %q and %q", r0.login, r.login)
		}
		if !flipSHAAgree(strings.ToLower(r.sha), strings.ToLower(sha)) {
			return "", "", "", refuseRunner("sha mismatch: rows of the latest PASS name %s and %s", sha, r.sha)
		}
		// The sha's form must read the same too: only its length may differ,
		// and only as an exact prefix (a case change is another text).
		if !flipSHAAgree(r.sha, sha) {
			return "", "", "", refuseRunner("runner mismatch: rows of the latest PASS read %q and %q", cells[0], cells[i])
		}
		if r.tail != r0.tail {
			same = false
			if !r.clean || !r0.clean || r.tree != r0.tree {
				return "", "", "", refuseRunner("runner mismatch: rows of the latest PASS read %q and %q", cells[0], cells[i])
			}
		}
	}
	if same {
		return r0.login, sha, r0.tail, nil
	}
	id0, src := strings.Join(runnerIdentityQuals(r0), "\x00"), ""
	every := map[string]int{}
	for i, r := range reads {
		if strings.Join(runnerIdentityQuals(r), "\x00") != id0 {
			return "", "", "", refuseRunner("runner mismatch: rows of the latest PASS read %q and %q", cells[0], cells[i])
		}
		seen := map[string]bool{}
		for _, q := range r.quals {
			if runnerSourceQual(q) {
				if src != "" && q != src {
					return "", "", "", refuseRunner("runner mismatch: rows of the latest PASS record identity sources %q and %q", src, q)
				}
				src = q
			}
			if !seen[q] {
				seen[q] = true
				every[q]++
			}
		}
	}
	tail = r0.tree
	for _, q := range r0.quals {
		if every[q] == len(reads) {
			tail += " (" + q + ")"
		}
	}
	return r0.login, sha, tail, nil
}

// latestPassRunnerRefusal is `brief --check-verified`'s read of the latest
// PASS run's Runner cells. It selects no run of its own: it asks verifyflip's
// selector (selectLatestPassSeen, the one flipLatestPass runs) and refuses
// only when that selector credits no run AND one of the reads it made refused
// on runner agreement (runnerCellsAgree, a refusal marked runner). So when
// verifyflip credits a run, check-verified judges that same run and admits it;
// when verifyflip refuses on runner agreement, check-verified refuses with the
// same reason. A run verifyflip refuses for any other reason (no strict latest
// PASS, a Runner cell in another form, a date) is not this check's question:
// the selected closure's other checks own those, and a verified brief closed
// on an older layout keeps its answer.
func latestPassRunnerRefusal(evidence string) error {
	var agree error
	_, err := selectLatestPassSeen(evidence, nil, func(e error) {
		var r *flipRefusal
		if agree == nil && errors.As(e, &r) && r.runner {
			agree = e
		}
	})
	if err == nil {
		return nil
	}
	return agree
}
