package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// manifest.go — batch closure. The batch is the dangerous verb, so it is the one with
// the most gates: a human-authorized event bound to an exact row set, one row at a
// time, an audit line per row, and a stop that is always resumable.
//
// THE PARSER IS STRICT ON PURPOSE. An unknown key is a refusal, not an ignored line.
// The lesson is recent and expensive: a permissive reader that silently drops what it
// does not understand turns "the human approved these rows" into "the human approved
// the rows the parser happened to recognise". There is no yaml dependency in this
// module and this is not a reason to add one — the accepted grammar is small enough to
// state exactly, and stating it exactly is the feature.

// manifestRow is one closure.
type manifestRow struct {
	Issue  int
	Mode   string
	Target string
	Mined  string
	Line   int // source line, for the refusal message
}

type manifest struct {
	AuthorizedBy   string
	DeclaredDigest string
	Rows           []manifestRow
	Path           string
}

// Digest is the sha256 the authorizing comment must carry. It covers exactly the fields
// that determine WHAT GETS CLOSED and WHY — issue, mode, target, mined — in file order.
// Comments, whitespace and key order are not covered: re-indenting an approved manifest
// must not invalidate a human's approval, but retargeting a row must.
func (m *manifest) Digest() string {
	var b strings.Builder
	for _, r := range m.Rows {
		fmt.Fprintf(&b, "%d\t%s\t%s\t%s\n", r.Issue, r.Mode, r.Target, r.Mined)
	}
	return deskkit.Sha256Hex([]byte(b.String()))
}

var manifestTopKeys = map[string]bool{"authorized-by": true, "manifest-sha256": true, "rows": true}
var manifestRowKeys = map[string]bool{"issue": true, "mode": true, "target": true, "mined": true}

// parseManifest reads the accepted grammar and refuses everything else:
//
//	authorized-by: <blessing-authority comment permalink>
//	manifest-sha256: <hex>            # optional; checked when present
//	rows:
//	  - issue: 41
//	    mode: superseded
//	    target: "#40"
//	    mined: "nothing-unique"       # duplicate rows only
//
// No anchors, no flow style, no nesting beyond this. Anything else is a refusal naming
// the line.
func parseManifest(path string) (*manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Unverifiable("could-not-check: cannot read the manifest at "+path, err)
	}
	m := &manifest{Path: path}
	var cur *manifestRow
	inRows := false
	flush := func() {
		if cur != nil {
			m.Rows = append(m.Rows, *cur)
			cur = nil
		}
	}
	for i, ln := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		lineNo := i + 1
		if t := strings.TrimSpace(ln); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		indented := strings.HasPrefix(ln, " ") || strings.HasPrefix(ln, "\t")
		body := strings.TrimSpace(ln)

		if !indented {
			flush()
			inRows = false
			key, val, ok := strings.Cut(body, ":")
			key = strings.ToLower(strings.TrimSpace(key))
			if !ok || !manifestTopKeys[key] {
				return nil, manifestRefusal(path, lineNo, body, "unknown top-level key — accepted: authorized-by, manifest-sha256, rows")
			}
			val = unquote(strings.TrimSpace(val))
			switch key {
			case "authorized-by":
				m.AuthorizedBy = val
			case "manifest-sha256":
				m.DeclaredDigest = strings.TrimPrefix(strings.ToLower(val), "sha256:")
			case "rows":
				if val != "" {
					return nil, manifestRefusal(path, lineNo, body, "rows: must introduce a block list, not an inline value")
				}
				inRows = true
			}
			continue
		}

		if !inRows {
			return nil, manifestRefusal(path, lineNo, body, "indented content outside the rows: block")
		}
		if after, isNew := strings.CutPrefix(body, "- "); isNew {
			flush()
			cur = &manifestRow{Line: lineNo}
			body = strings.TrimSpace(after)
		}
		if cur == nil {
			return nil, manifestRefusal(path, lineNo, body, "a row field before any `- ` row start")
		}
		key, val, ok := strings.Cut(body, ":")
		key = strings.ToLower(strings.TrimSpace(key))
		if !ok || !manifestRowKeys[key] {
			return nil, manifestRefusal(path, lineNo, body, "unknown row key — accepted: issue, mode, target, mined")
		}
		val = unquote(strings.TrimSpace(val))
		switch key {
		case "issue":
			n, aerr := atoiPositive(val, "row issue")
			if aerr != nil {
				return nil, aerr
			}
			cur.Issue = n
		case "mode":
			cur.Mode = strings.ToLower(val)
		case "target":
			cur.Target = val
		case "mined":
			cur.Mined = val
		}
	}
	flush()

	if strings.TrimSpace(m.AuthorizedBy) == "" {
		return nil, deskkit.Refused("refused: the manifest carries no `authorized-by:` comment URL. " +
			"A batch close with no authorizing human artifact is the thing this tool exists to prevent; " +
			"there is no flag that supplies one.")
	}
	if len(m.Rows) == 0 {
		return nil, deskkit.Refused("refused: the manifest lists no rows")
	}
	for _, r := range m.Rows {
		if r.Issue == 0 {
			return nil, manifestRefusal(path, r.Line, "", "row has no `issue:`")
		}
		if !contains(rowModes(), r.Mode) {
			return nil, manifestRefusal(path, r.Line, "mode: "+r.Mode,
				"unknown mode — a manifest row is one of: "+strings.Join(rowModes(), " | "))
		}
		if r.Mode != modeReviewRequest && strings.TrimSpace(r.Target) == "" {
			return nil, manifestRefusal(path, r.Line, "", "the "+r.Mode+" lane requires a `target:`")
		}
		if r.Mode == modeDuplicate && strings.TrimSpace(r.Mined) == "" {
			return nil, manifestRefusal(path, r.Line, "", "a duplicate row requires `mined:`")
		}
	}
	return m, nil
}

func manifestRefusal(path string, line int, text, why string) error {
	return deskkit.Refused(fmt.Sprintf("refused: %s:%d: %s%s — the manifest grammar is closed, and a "+
		"line this reader does not understand is never silently dropped: a dropped line is a row the "+
		"human approved and the tool did something else with",
		path, line, why, quotedSuffix(text)))
}

func quotedSuffix(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return fmt.Sprintf(" (%q)", deskkit.StripControl(text))
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- the run

// sleepFn is time.Sleep behind a variable so the resume test exercises the real
// wait-and-retry control flow without a real wall-clock wait.
var sleepFn = time.Sleep

// defaultMaxWait bounds the total time a manifest run will spend waiting on rate
// limits before it exits CLEANLY with a resume point. It is a bound on the RUN, never
// a licence to skip: whichever row the wait was for is the row --resume-from names.
const defaultMaxWait = 90 * time.Minute

func cmdManifest(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeManifest, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	file := fs.String("file", "", "path to the manifest")
	resumeFrom := fs.Int("resume-from", 0, "skip rows before this issue number (resume a stopped run)")
	maxWait := fs.Duration("max-wait", defaultMaxWait, "total time to spend waiting on rate limits before exiting with a resume point")
	flags, pos := splitPositionals(args)
	if err := fs.Parse(flags); err != nil {
		return deskkit.Refused("manifest: " + err.Error())
	}
	if len(pos)+len(fs.Args()) > 0 {
		return deskkit.Refused("refused: manifest takes no positional arguments (rows come from --file)")
	}
	if err := requireRepo(c.repo); err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" {
		return deskkit.Refused("refused: manifest requires --file <manifest.yaml>")
	}

	m, err := parseManifest(*file)
	if err != nil {
		return err
	}
	// The RULING gate first (is any lane granted at all?), then the MANIFEST gate (is
	// THIS row set approved?). Both are fetch-and-verify; neither can be satisfied by a
	// flag. Nothing below this point runs if either refuses.
	g, err := gateFor(&c)
	if err != nil {
		return err
	}
	auth, err := authorizeManifest(m)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "authorized\t%s\tby=%s\tdigest=%s\trows=%d\n",
		deskkit.StripControl(auth.HTMLURL), deskkit.StripControl(auth.User.Login), m.Digest(), len(m.Rows))

	return applyRows(m, g, c, *resumeFrom, *maxWait, out)
}

// applyRows is the row loop. Its three properties, in the order they matter:
//
//	NEVER SKIPS   — a rate-limit refusal (exit 4) WAITS and retries the SAME row. It is
//	                not an error and never advances the cursor. A batch closer that
//	                silently skips is worse than one that stops, because the operator
//	                believes the batch is done.
//	ALWAYS RESUMABLE — every stop prints the exact --resume-from to continue with, and
//	                a resumed run re-reads each row's live state, so an already-closed
//	                row is an idempotent no-op rather than a second close.
//	AUDITS PER ROW — one line per row per outcome, before moving on.
func applyRows(m *manifest, g grant, c common, resumeFrom int, maxWait time.Duration, out io.Writer) error {
	started := false
	var waited time.Duration

	for idx, row := range m.Rows {
		if resumeFrom > 0 && !started {
			if row.Issue != resumeFrom {
				fmt.Fprintf(out, "skip-before-resume\t%s#%d\t(row %d)\n", c.repo, row.Issue, idx+1)
				continue
			}
			started = true
		}
		req := closeReq{
			repo: c.repo, number: row.Issue, mode: row.Mode,
			target: row.Target, mined: row.Mined, dryRun: c.dryRun, g: g,
		}
		for {
			err := applyClose(req, out)
			if err == nil {
				break
			}
			if !deskkit.IsRateLimited(err) {
				// HARD error: stop here, leaving everything from this row onward
				// untouched and explicitly resumable.
				fmt.Fprintf(out, "stopped\trow %d of %d\t%s#%d\t%s\n",
					idx+1, len(m.Rows), c.repo, row.Issue, deskkit.StripControl(err.Error()))
				fmt.Fprintf(out, "resume with: %s %s -R %s --file %s --resume-from %d\n",
					toolName, modeManifest, c.repo, m.Path, row.Issue)
				return err
			}
			d := deskkit.RetryAfterOf(err)
			if d <= 0 {
				d = time.Minute
			}
			if waited+d > maxWait {
				// Exit CLEANLY on the budget, still pointing at the unapplied row. The
				// exit code stays 4 — the caller must wait — and the row is not skipped,
				// it is deferred.
				fmt.Fprintf(out, "paused\trow %d of %d\t%s#%d\twaited %s of %s\n",
					idx+1, len(m.Rows), c.repo, row.Issue, waited, maxWait)
				fmt.Fprintf(out, "resume with: %s %s -R %s --file %s --resume-from %d\n",
					toolName, modeManifest, c.repo, m.Path, row.Issue)
				return err
			}
			fmt.Fprintf(out, "rate-limited\t%s#%d\twaiting %s then retrying THIS row (not skipping it)\n",
				c.repo, row.Issue, d)
			sleepFn(d)
			waited += d
		}
	}
	if resumeFrom > 0 && !started {
		return deskkit.Refused(fmt.Sprintf(
			"refused: --resume-from %d names no row in %s — refusing rather than running the whole batch "+
				"from the top", resumeFrom, m.Path))
	}
	fmt.Fprintf(out, "done\t%d rows\n", len(m.Rows))
	return nil
}
