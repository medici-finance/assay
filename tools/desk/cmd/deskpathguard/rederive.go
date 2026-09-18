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

var verifySepRowRe = regexp.MustCompile(`^[\s:|-]+$`)

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
func extractVerifySection(md string) string {
	lines := strings.Split(md, "\n")
	start := -1
	for i, l := range lines {
		if strings.EqualFold(strings.TrimSpace(l), "## Verify") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// parseVerifyTable parses the data rows of a `## Verify` section, keyed on the `#` column.
// A table with no `#` column falls back to ordinal numbering, matching statusgen's own
// parseVerifyItems fallback.
func parseVerifyTable(section string) []verifyRow {
	var rows []verifyRow
	cmdIdx, expIdx, numIdx, classIdx := -1, -1, -1, -1
	haveHeader := false
	ordinal := 0
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if verifySepRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRowCells(line)
		if !haveHeader {
			for j, c := range cells {
				switch strings.ToLower(strings.TrimSpace(c)) {
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
			haveHeader = true
			continue
		}
		ordinal++
		id := strconv.Itoa(ordinal)
		if numIdx >= 0 && numIdx < len(cells) {
			if v := strings.TrimSpace(cells[numIdx]); v != "" {
				id = v
			}
		}
		row := verifyRow{Num: id}
		if classIdx >= 0 && classIdx < len(cells) {
			row.Class = strings.TrimSpace(cells[classIdx])
		}
		if cmdIdx >= 0 && cmdIdx < len(cells) {
			row.Command = strings.TrimSpace(cells[cmdIdx])
		}
		if expIdx >= 0 && expIdx < len(cells) {
			row.Expect = strings.TrimSpace(cells[expIdx])
		}
		rows = append(rows, row)
	}
	return rows
}

// splitRowCells splits a markdown table row on `|`, dropping the leading/trailing empty
// cells a `| a | b |`-shaped line produces. It does not need splitRowEscaped's `\|`
// awareness (statusgen/verifyrows.go): the two columns this parse reads (Command, Expect)
// are read as opaque text either way, never re-split downstream.
func splitRowCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

// RederiveTable is the pure diff (Verify row 7): every row present at HEAD whose `#` also
// appears at BASE carries the BASE row's command/expect (an edit to an EXISTING row's
// command at HEAD is bypassed, never trusted); a row whose `#` has no match at BASE is
// author-added.
func RederiveTable(baseMD, headMD string) []rederivedRow {
	base := parseVerifyTable(extractVerifySection(baseMD))
	baseByNum := make(map[string]verifyRow, len(base))
	for _, r := range base {
		baseByNum[r.Num] = r
	}
	head := parseVerifyTable(extractVerifySection(headMD))
	out := make([]rederivedRow, 0, len(head))
	for _, r := range head {
		if b, ok := baseByNum[r.Num]; ok {
			out = append(out, rederivedRow{Num: r.Num, Base: b})
		} else {
			out = append(out, rederivedRow{Num: r.Num, AuthorAdded: true})
		}
	}
	return out
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

	mb, err := runGitFn(req.root, "merge-base", "HEAD", req.mainRef)
	if err != nil || strings.TrimSpace(mb) == "" {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot resolve the merge-base of HEAD and %s in %s — the re-derivation has no "+
				"base table to read", req.mainRef, req.root), err)
	}
	base := strings.TrimSpace(mb)

	baseMD, baseErr := runGitFn(req.root, "show", base+":"+req.brief)
	if baseErr != nil {
		baseMD = "" // the brief did not exist at base: every row at HEAD is author-added
	}

	headBytes, herr := os.ReadFile(filepath.Join(req.root, filepath.FromSlash(req.brief)))
	if herr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: cannot read %s at HEAD", req.brief), herr)
	}

	rows := RederiveTable(baseMD, string(headBytes))
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
