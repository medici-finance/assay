package askassay

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// THE SILENT-CAP REGISTER
// -----------------------
// Every list read in this suite goes through a paging cap, and none of the
// capped calls reports that it capped. The measured failure this register
// exists for: a list read of this exact shape returned exactly 500 rows
// against a true total of 958, exited 0, and told its caller nothing. The
// caller published the 500.
//
// A cap is not a defect. An UNDECLARED cap is. So the caps that exist in this
// module are enumerated here, in source, and a test holds the enumeration true
// against the tree (TestSilentCapRegisterMatchesTheTree). When a cap is added,
// removed or resized without a matching edit here, that test reddens and names
// the site.
//
// This register is what lets the pane's Limit field say a number rather than
// a shrug, and it is why [Question.SaturatesAt] exists: a count that reaches
// its cap is refused, not reported.
//
// THE BOUND, STATED
// -----------------
// The check is TEXTUAL. It counts a literal in non-test Go source of this
// module. A cap expressed through a variable this scan cannot see, or reached
// through a helper that hardcodes it elsewhere, is not counted — so this
// register is a floor on the caps that exist, not a proof that there are no
// others. It is deliberately NOT pinned to line numbers: a line-pinned check
// reddens on an unrelated insertion above the site, which trains readers to
// re-pin without reading. The substance (which file, which literal, how many
// occurrences) is what is held.

// ListCap is one declared paging cap in this module.
type ListCap struct {
	// File is the module-relative path holding the cap.
	File string
	// Needle is the literal that must appear in that file.
	Needle string
	// Occurrences is how many times Needle appears in non-comment source.
	Occurrences int
	// Cap is the effective row cap, as a number, or 0 where the call site
	// takes it from a caller-supplied flag.
	Cap int
	// Effect is what a reader of the capped call cannot tell.
	Effect string
}

// declaredListCaps is the register. Order is stable for diff review.
var declaredListCaps = []ListCap{
	{
		File:        "cmd/issueboard/board.go",
		Needle:      `"--limit", "1000"`,
		Occurrences: 1,
		Cap:         1000,
		Effect:      "the open-issue read for a repo truncates at 1000 rows and reports no truncation. A repo at or over 1000 open issues yields a board that is silently short, and the caller sees a clean exit. This is the live instance, in this tree, of the class that produced the 500-against-958 measurement",
	},
	{
		File:        "cmd/deskboard/board.go",
		Needle:      "const prListLimit = 100",
		Occurrences: 1,
		Cap:         100,
		Effect:      "the PR inventory truncates at 100 open PRs per repo. At the cap, 'how many PRs are open' is unanswerable from this verb, which is why pr-inventory-count declares its limit rather than reporting the length as a total",
	},
	{
		File:        "cmd/deskboard/scope.go",
		Needle:      "const searchLimit = 200",
		Occurrences: 1,
		Cap:         200,
		Effect:      "the cross-repo scope search truncates at 200 results. A scope answer at the cap is a lower bound on what is in scope, never the scope",
	},
	{
		File:        "cmd/deskfile/deskfile.go",
		Needle:      `labelListLimit = "500"`,
		Occurrences: 1,
		Cap:         500,
		Effect:      "the label read truncates at 500 labels. A label absent from a truncated read is indistinguishable from a label that does not exist, which turns a missing label into a false negative rather than an error",
	},
	{
		File:        "cmd/deskfile/matcher.go",
		Needle:      `searchLimit = "20"`,
		Occurrences: 1,
		Cap:         20,
		Effect:      "the duplicate-issue search truncates at 20 hits. A duplicate past the 20th does not exist as far as the matcher is concerned, so 'no duplicate found' from this path is a bounded statement, not a clean one",
	},
	{
		File:        "cmd/deskroster/roster.go",
		Needle:      `"--limit", "50"`,
		Occurrences: 1,
		Cap:         50,
		Effect:      "the roster's open-PR read truncates at 50 per repo. A repo over the cap yields a roster that omits PRs without saying so",
	},
	{
		File:        "cmd/deskdisposition/verbs.go",
		Needle:      `"--limit"`,
		Occurrences: 2,
		Cap:         0,
		Effect:      "two capped reads: a label existence probe fixed at 5, and an issue read whose cap comes from a caller flag. The caller-supplied one is the worse shape — the cap is chosen at the call site and the answer never says which value was used, so the same command produces different totals with no visible difference",
	},
	{
		File:        "cmd/deskdigest/collect.go",
		Needle:      `"--limit", "300"`,
		Occurrences: 1,
		Cap:         300,
		Effect:      "the per-repo label-inventory probe truncates at 300 labels. deskdigest uses this probe to tell 'the human-only label has never been created' apart from 'no items carry it'; a label sitting past the 300th in a truncated read reads as absent, so beyond the cap a real label is misreported as never-created — the exact false negative this probe exists to prevent",
	},
	{
		File:        "cmd/deskdigest/post.go",
		Needle:      `"--limit", "50"`,
		Occurrences: 1,
		Cap:         50,
		Effect:      "the search for this week's existing digest issue (state all) truncates at 50 hits. If more than 50 same-titled digest issues exist, the prior-week lookup can miss the open one past the cap and post a second digest for the week — the duplicate findWeekly's exact-title refusal is built to prevent",
	},
}

// DeclaredListCaps returns the register.
func DeclaredListCaps() []ListCap { return append([]ListCap(nil), declaredListCaps...) }

// CapDrift is one disagreement between the register and the tree.
type CapDrift struct {
	File   string
	Needle string
	Want   int
	Got    int
	Why    string
}

func (d CapDrift) String() string {
	return fmt.Sprintf("CAP DRIFT — %s: %q declared %d occurrence(s), tree holds %d (%s)", d.File, d.Needle, d.Want, d.Got, d.Why)
}

// AuditListCaps holds the register true against a module root. It reports one
// drift per disagreeing row and never returns an error for a clean tree.
func AuditListCaps(moduleRoot string, caps []ListCap) ([]CapDrift, error) {
	var drifts []CapDrift
	for _, c := range caps {
		b, err := os.ReadFile(filepath.Join(moduleRoot, filepath.FromSlash(c.File)))
		if err != nil {
			drifts = append(drifts, CapDrift{File: c.File, Needle: c.Needle, Want: c.Occurrences, Got: -1,
				Why: "the declared file could not be read: " + err.Error()})
			continue
		}
		got := countOutsideLineComments(string(b), c.Needle)
		if got != c.Occurrences {
			why := "the cap moved, was removed, or was duplicated without a matching edit to the register"
			if got == 0 {
				why = "the declared cap is GONE from this file — either it was fixed, in which case delete the row, or it moved, in which case re-point it"
			}
			drifts = append(drifts, CapDrift{File: c.File, Needle: c.Needle, Want: c.Occurrences, Got: got, Why: why})
		}
	}
	return drifts, nil
}

// countOutsideLineComments counts occurrences of needle in src, ignoring text
// after a `//` on each line so that a cap mentioned in a comment is not
// counted as a call site.
func countOutsideLineComments(src, needle string) int {
	var n int
	for _, line := range strings.Split(src, "\n") {
		code := line
		if i := strings.Index(code, "//"); i >= 0 {
			code = code[:i]
		}
		n += strings.Count(code, needle)
	}
	return n
}

// UndeclaredCapScan reports `--limit` call sites in non-test Go source under
// moduleRoot that no register row covers. This is the half that catches a NEW
// silent cap: the register alone only holds the known ones true.
func UndeclaredCapScan(moduleRoot string, caps []ListCap) ([]string, error) {
	declared := map[string]bool{}
	for _, c := range caps {
		declared[filepath.ToSlash(c.File)] = true
	}
	var undeclared []string
	err := filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// askassay is skipped because it DECLARES the caps rather than
			// calling them: its own source quotes the literals this scan looks
			// for. It runs no list call at all — TestExactlyOneSubprocessCallSite
			// and TestPackageHoldsNoWritePath are what hold that true, not this
			// exemption.
			if d.Name() == "testdata" || d.Name() == ".git" || d.Name() == "askassay" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(moduleRoot, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if declared[rel] {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if countOutsideLineComments(string(b), `"--limit"`) > 0 {
			undeclared = append(undeclared, rel)
		}
		return nil
	})
	return undeclared, err
}
