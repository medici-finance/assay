package main

// rederive.go — verify-desk's re-derivation step (brief verify-integrity/01, task item 3).
//
// A labelled brief's Verify table is re-read as it stood at the merge-base of the
// delivering PR with origin/main, not as merged: a row that exists ONLY in the post-change
// table is reported author-added, never pass, because a row the AUTHOR added is a row the
// author also wrote the assertion for — the self-report this stream exists to stop trusting.
//
// The two versions are read the way statusgen's own merge-base grandfathering
// (closedAtBase, statusgen/unrun.go) already reads a brief at the base: `git merge-base HEAD
// origin/main` then `git show <base>:<path>`. This file deliberately does NOT import
// statusgen (a separate `package main`; cross-command duplication of a small parse is this
// tree's own documented pattern — see cmd/desklabel/rosterfixture_test.go's header) — it
// re-implements the small `## Verify` row-parse it needs, keyed on the `#` column, same as
// statusgen's parseVerifyItems.
import (
	"github.com/medici-finance/assay/statusgen/briefdoc"

	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var verifySepCellRe = regexp.MustCompile(`^:?-+:?$`)

// verifyRow is one data row of a `## Verify` table.
type verifyRow struct {
	Num     string
	Class   string
	Command string
	Expect  string
}

// rederivedRow is one row of the re-derivation.
type rederivedRow struct {
	Num         string
	AuthorAdded bool
	Base        verifyRow // zero value when AuthorAdded
}

// extractVerifySection returns the text of md's `## Verify` section (from just below the
// heading through the next level-2 heading or EOF), "" if the heading is absent.
// Prefix grammar mirrors statusgen/verifyissues.go extractSectionByPrefix.
// Keep the historical case-insensitive exact spelling supported as well.
func extractVerifySection(md string) string { return briefdoc.Section(md, "Verify", true) }

// isVerifyHeading is shared with the diff detector: descriptive Verify headings
// must protect the same rows that re-derivation reads.
func isVerifyHeading(line string) bool { return briefdoc.IsVerify(line) }

// parseVerifyTable reads complete, structurally valid tables. Unlike statusgen's
// best-effort lint readers, re-derivation cannot silently omit unreadable rows.
// Auxiliary tables (neither Command nor Expect in the header) are skipped as
// whole blocks only if no later cell names Command or Expect: the executor can
// adopt such a later row as its header, so ambiguity must fail closed. Multiple
// Verify tables merge only with unique IDs across them.
// An absent # column uses per-table ordinal IDs; explicit IDs must be nonempty.
func parseVerifyTable(section string) ([]verifyRow, error) {
	var rows []verifyRow
	seen := map[string]bool{}
	var header []string
	cmdIdx, expIdx, numIdx, classIdx := -1, -1, -1, -1
	needSeparator := false
	skipTable := false
	tableRows := 0
	for n, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		fail := func(why string) ([]verifyRow, error) {
			return nil, fmt.Errorf("Verify section line %d: %s", n+1, why)
		}
		if !strings.HasPrefix(line, "|") {
			if header != nil && (needSeparator || tableRows == 0) {
				return fail("table has no data rows or separator")
			}
			header = nil
			skipTable = false
			continue
		}
		cells := splitRowCells(line)
		if skipTable {
			for _, cell := range cells {
				if strings.EqualFold(cell, "Command") || strings.EqualFold(cell, "Expect") {
					return fail("auxiliary table contains a possible Verify header")
				}
			}
			continue
		}
		if header == nil {
			cmdIdx, expIdx, numIdx, classIdx = -1, -1, -1, -1
			columns := map[string]bool{}
			duplicate := false
			for j, c := range cells {
				name := strings.ToLower(c)
				if columns[name] {
					duplicate = true
				}
				columns[name] = true
				switch name {
				case "#":
					numIdx = j
				case "class":
					classIdx = j
				case "command":
					cmdIdx = j
				case "expect":
					expIdx = j
				}
			}
			if cmdIdx < 0 && expIdx < 0 {
				skipTable = true
				continue
			}
			if duplicate {
				return fail("duplicate header column")
			}
			if cmdIdx < 0 || expIdx < 0 {
				return fail("table requires Command and Expect columns")
			}
			header, needSeparator, tableRows = cells, true, 0
			continue
		}
		if len(cells) != len(header) {
			return fail("row width differs from header (escape cell pipes as \\|)")
		}
		if needSeparator {
			for _, c := range cells {
				if !verifySepCellRe.MatchString(c) {
					return fail("missing or malformed table separator")
				}
			}
			needSeparator = false
			continue
		}
		if cells[cmdIdx] == "" || cells[expIdx] == "" {
			return fail("Command and Expect must be nonempty")
		}
		id := strconv.Itoa(tableRows + 1)
		if numIdx >= 0 {
			id = cells[numIdx]
			if id == "" {
				return fail("empty row ID")
			}
		}
		if seen[id] {
			return fail("duplicate row ID " + id)
		}
		seen[id] = true
		row := verifyRow{Num: id, Command: cells[cmdIdx], Expect: cells[expIdx]}
		if classIdx >= 0 {
			row.Class = cells[classIdx]
		}
		rows = append(rows, row)
		tableRows++
	}
	if len(rows) == 0 || (header != nil && (needSeparator || tableRows == 0)) {
		return nil, fmt.Errorf("Verify table missing or empty")
	}
	return rows, nil
}

// splitRowCells mirrors statusgen's splitRowEscaped, preserving raw cell text
// (including backslashes and code spans). Re-derivation reports, never executes.
// The existing shared executor splitter is being introduced separately in #3160;
// until it lands there is no importable table splitter in this module.
func splitRowCells(line string) []string {
	s := strings.TrimSpace(line)
	var cells []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '|' {
			i++
			continue
		}
		if s[i] == '|' {
			cells = append(cells, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	cells = append(cells, strings.TrimSpace(s[start:]))
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// RederiveTable is the pure diff (Verify row 7): every row present at HEAD whose `#` also
// appears at BASE carries the BASE row's command/expect (an edit to an EXISTING row's
// command at HEAD is bypassed, never trusted); a row whose `#` has no match at BASE is
// author-added.
func RederiveTable(baseMD, headMD string) ([]rederivedRow, error) {
	var base []verifyRow
	var err error
	// Empty baseMD is reserved for a path proven absent in the base tree.
	if baseMD != "" {
		base, err = parseVerifyTable(extractVerifySection(baseMD))
		if err != nil {
			return nil, fmt.Errorf("base: %w", err)
		}
	}
	baseByNum := make(map[string]verifyRow, len(base))
	for _, r := range base {
		baseByNum[r.Num] = r
	}
	head, err := parseVerifyTable(extractVerifySection(headMD))
	if err != nil {
		return nil, fmt.Errorf("candidate: %w", err)
	}
	out := make([]rederivedRow, 0, len(head))
	for _, r := range head {
		if b, ok := baseByNum[r.Num]; ok {
			out = append(out, rederivedRow{Num: r.Num, Base: b})
		} else {
			out = append(out, rederivedRow{Num: r.Num, AuthorAdded: true})
		}
	}
	return out, nil
}

// --- CLI wiring -------------------------------------------------------------------------

const defaultMainRef = "origin/main"

type rederiveRequest struct {
	root    string
	brief   string
	mainRef string
}

func parseRederiveArgs(args []string) (rederiveRequest, error) {
	fs := flag.NewFlagSet("rederive", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	root := fs.String("root", "", "the checkout root (a medici-finance/assay working tree)")
	brief := fs.String("brief", "", "the brief file's path, relative to --root")
	mainRef := fs.String("main", defaultMainRef, "the ref to merge-base HEAD against")
	if err := fs.Parse(args); err != nil {
		return rederiveRequest{}, deskkit.Refused("rederive: " + err.Error())
	}
	req := rederiveRequest{root: strings.TrimSpace(*root), brief: strings.TrimSpace(*brief), mainRef: strings.TrimSpace(*mainRef)}
	if req.root == "" {
		return rederiveRequest{}, deskkit.Refused("rederive: --root is required")
	}
	if req.brief == "" {
		return rederiveRequest{}, deskkit.Refused("rederive: --brief is required")
	}
	if req.mainRef == "" {
		req.mainRef = defaultMainRef
	}
	return req, nil
}

// runGitFn is the git-exec seam; a test substitutes a fake so the suite needs no real repo.
var runGitFn = runGit

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("%v: %s", err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

func cmdRederive(args []string, out io.Writer) error {
	req, err := parseRederiveArgs(args)
	if err != nil {
		return err
	}

	prefix, err := runGitFn(req.root, "rev-parse", "--show-prefix")
	if err != nil || strings.TrimSpace(prefix) != "" {
		return deskkit.Unverifiable("could-not-check: --root must be the Git checkout root", err)
	}
	mb, err := runGitFn(req.root, "merge-base", "HEAD", req.mainRef)
	if err != nil || strings.TrimSpace(mb) == "" {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot resolve the merge-base of HEAD and %s in %s — the re-derivation has no "+
				"base table to read", req.mainRef, req.root), err)
	}
	base := strings.TrimSpace(mb)

	baseMD, baseErr := runGitFn(req.root, "show", base+":"+req.brief)
	if baseErr != nil {
		// A failed show is not proof that a brief is new: inspect the tree.
		entry, treeErr := runGitFn(req.root, "--literal-pathspecs", "ls-tree", "--full-tree", "--name-only", base, "--", req.brief)
		if treeErr != nil || strings.TrimSpace(entry) != "" {
			return deskkit.Unverifiable("could-not-check: cannot read brief at base", baseErr)
		}
		baseMD = "" // proven absent: every candidate row is author-added
	}

	headBytes, herr := os.ReadFile(filepath.Join(req.root, filepath.FromSlash(req.brief)))
	if herr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: cannot read %s at HEAD", req.brief), herr)
	}

	if baseErr == nil && baseMD == "" {
		return deskkit.Unverifiable("could-not-check: base Verify table missing or empty", nil)
	}
	rows, err := RederiveTable(baseMD, string(headBytes))
	if err != nil {
		return deskkit.Unverifiable("could-not-check: cannot rederive Verify table", err)
	}
	authorAdded := 0
	for _, r := range rows {
		if r.AuthorAdded {
			authorAdded++
			fmt.Fprintf(out, "row %s: author-added\n", r.Num)
			continue
		}
		fmt.Fprintf(out, "row %s: from-base class=%q command=%q expect=%q\n", r.Num, r.Base.Class, r.Base.Command, r.Base.Expect)
	}
	fmt.Fprintf(out, "deskpathguard rederive: %d row(s), %d author-added, base=%s\n", len(rows), authorAdded, base)
	return nil
}
