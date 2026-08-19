package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskdisposition"

// nowFunc is the clock seam; tests pin it so the recorded date is deterministic.
var nowFunc = time.Now

// prView is the shape of `gh pr view --json labels,comments`.
type prView struct {
	Labels   []struct{ Name string } `json:"labels"`
	Comments []struct{ Body string } `json:"comments"`
}

// prListItem is the shape of `gh pr list --json number,title,labels`.
type prListItem struct {
	Number int                     `json:"number"`
	Title  string                  `json:"title"`
	Labels []struct{ Name string } `json:"labels"`
}

func labelNames(ls []struct{ Name string }) []string {
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.Name)
	}
	return out
}

// requireRepo enforces the allowed-repo set. This tool introduces NO repo list of its
// own: the roster is the single declared source, and an unconfigured roster refuses.
func requireRepo(repo string) error {
	if strings.TrimSpace(repo) == "" {
		return deskkit.Refused("-R <owner/repo> is required")
	}
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused(fmt.Sprintf("repo %q is outside the desk-tools repo set", deskkit.StripControl(repo)))
	}
	return nil
}

// ---------------------------------------------------------------- set

func cmdSet(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repo := fs.String("R", "", "owner/repo")
	pr := fs.Int("pr", 0, "pull request number")
	verdict := fs.String("verdict", "", "SUPERSEDED | RESOLVED-ELSEWHERE | NEEDS-REBASE")
	evidence := fs.String("evidence", "", "URL or owner/repo#N of the superseding PR / settling issue")
	by := fs.String("by", "", "who recorded this (default: session tag)")
	date := fs.String("date", "", "record date YYYY-MM-DD (default: today, UTC)")
	dryRun := fs.Bool("dry-run", false, "validate and read, then stop before the write")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("set: " + err.Error())
	}
	if err := requireRepo(*repo); err != nil {
		return err
	}
	if *pr <= 0 {
		return deskkit.Refused("set: --pr <N> is required")
	}
	v, err := deskkit.ParseDispositionVerdict(*verdict)
	if err != nil {
		return err
	}
	who := strings.TrimSpace(*by)
	if who == "" {
		who = deskkit.SessionTag()
	}
	when := strings.TrimSpace(*date)
	if when == "" {
		when = nowFunc().UTC().Format("2006-01-02")
	}
	rec := deskkit.Disposition{Verdict: v, Evidence: strings.TrimSpace(*evidence), RecordedBy: who, RecordedAt: when}
	if err := rec.Validate(); err != nil {
		return err
	}

	// Read the current state first. Idempotency is anchored on the REMOTE, not on the
	// local audit log: this verb is meant to be re-run by every pass over the same PR,
	// and a fresh session with an empty audit log must still no-op.
	cur, readErr := viewPR(*repo, *pr)
	if readErr != nil {
		return deskkit.Unverifiable(
			fmt.Sprintf("could-not-check: cannot read %s#%d before writing — refusing to write a "+
				"record whose current state is unknown", *repo, *pr), readErr)
	}
	existing := deskkit.ReadDisposition(labelNames(cur.Labels), commentBodies(cur), nil)
	if existing.State == deskkit.DispositionCheckedFailed &&
		existing.Record.Verdict == rec.Verdict &&
		existing.Record.Evidence == rec.Evidence {
		fmt.Fprintf(out, "noop: %s#%d already carries %s (%s)\n", *repo, *pr, rec.Verdict, rec.Evidence)
		auditLine(*repo, *pr, "set", deskkit.ResultNoop, string(rec.Verdict))
		return nil
	}

	if *dryRun {
		fmt.Fprintf(out, "dry-run: would record %s on %s#%d\n%s\n", rec.Verdict, *repo, *pr, rec.Marker())
		auditLine(*repo, *pr, "set", deskkit.ResultDryRun, string(rec.Verdict))
		return nil
	}

	// Two outward writes, budgeted as one act: the label and the comment are halves of
	// one record and there is no useful state in which only one was intended.
	if err := deskkit.AllowWrite(toolName, *repo, *pr); err != nil {
		return err
	}

	// LABEL FIRST, comment second. The sweep reads labels; a marker with no label is
	// invisible to it. Ordering the writes this way means a partial failure leaves the
	// PR indexed-but-unevidenced (which `read` reports and a re-run repairs) rather
	// than evidenced-but-unindexed (which the sweep would silently re-dispatch).
	if err := ensureLabel(*repo, v); err != nil {
		return err
	}
	if _, err := gh("pr", "edit", fmt.Sprint(*pr), "-R", *repo, "--add-label", v.Label()); err != nil {
		auditLine(*repo, *pr, "set", deskkit.ResultUnverifiable, "add-label: "+err.Error())
		return deskkit.Unverifiable("set: cannot add "+v.Label(), err)
	}

	body := rec.Marker()
	f, err := os.CreateTemp("", "desk-disposition-*.md")
	if err != nil {
		return deskkit.Unverifiable("set: cannot stage the record body", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return deskkit.Unverifiable("set: cannot stage the record body", err)
	}
	f.Close()
	if _, err := gh("pr", "comment", fmt.Sprint(*pr), "-R", *repo, "--body-file", f.Name()); err != nil {
		auditLine(*repo, *pr, "set", deskkit.ResultUnverifiable, "comment: "+err.Error())
		return deskkit.Unverifiable("set: label applied but the record comment failed — re-run to repair", err)
	}

	auditLine(*repo, *pr, "set", deskkit.ResultOK, string(rec.Verdict))
	fmt.Fprintf(out, "recorded: %s on %s#%d (evidence %s)\n", rec.Verdict, *repo, *pr, rec.Evidence)
	fmt.Fprintf(out, "queued for deskclose — this tool does not close PRs\n")
	return nil
}

// ensureLabel creates the disposition label when the repo does not have it yet. A repo
// that has never carried a disposition would otherwise fail every first `set`.
// Failure here is NOT fatal on its own: the add-label call that follows is the real
// gate, and it reports the actionable error.
func ensureLabel(repo string, v deskkit.DispositionVerdict) error {
	if _, err := gh("label", "list", "-R", repo, "--search", v.Label(), "--limit", "5"); err != nil {
		return deskkit.Unverifiable("set: cannot read the repo's labels", err)
	}
	// `label create` is idempotent-enough for this purpose: an existing label makes it
	// exit non-zero, which is why the error is deliberately swallowed. The add-label
	// call is what decides success.
	_, _ = gh("label", "create", v.Label(), "-R", repo,
		"--description", "worker disposition record: "+string(v)+" (see desk-disposition marker comment)",
		"--color", "BFD4F2")
	return nil
}

// ---------------------------------------------------------------- read

func cmdRead(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repo := fs.String("R", "", "owner/repo")
	pr := fs.Int("pr", 0, "pull request number")
	asJSON := fs.Bool("json", false, "emit the record as JSON")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("read: " + err.Error())
	}
	if err := requireRepo(*repo); err != nil {
		return err
	}
	if *pr <= 0 {
		return deskkit.Refused("read: --pr <N> is required")
	}

	view, readErr := viewPR(*repo, *pr)
	var r deskkit.DispositionRead
	if readErr != nil {
		r = deskkit.ReadDisposition(nil, nil, readErr)
	} else {
		r = deskkit.ReadDisposition(labelNames(view.Labels), commentBodies(view), nil)
	}

	if *asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Repo string `json:"repo"`
			PR   int    `json:"pr"`
			deskkit.DispositionRead
			DispatchEligible bool `json:"dispatchEligible"`
		}{*repo, *pr, r, r.DispatchEligible()}); err != nil {
			return deskkit.Unverifiable("read: cannot encode", err)
		}
	} else {
		fmt.Fprintf(out, "%s#%d\t%s\t%s\tdispatch-eligible=%t\n", *repo, *pr, r.State, r.Record.Verdict, r.DispatchEligible())
		if r.Record.Evidence != "" {
			fmt.Fprintf(out, "evidence: %s (recorded %s by %s)\n", r.Record.Evidence, r.Record.RecordedAt, r.Record.RecordedBy)
		}
		if r.Reason != "" {
			fmt.Fprintf(out, "note: %s\n", r.Reason)
		}
	}
	if r.State == deskkit.DispositionCouldNotCheck {
		return deskkit.Unverifiable("read: could-not-check", readErr)
	}
	return nil
}

// ---------------------------------------------------------------- sweep

func cmdSweep(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("sweep", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repo := fs.String("R", "", "owner/repo")
	limit := fs.Int("limit", 100, "max open PRs to list")
	eligibleOnly := fs.Bool("eligible-only", false, "print only PRs the orphan sweep may dispatch")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("sweep: " + err.Error())
	}
	if err := requireRepo(*repo); err != nil {
		return err
	}

	raw, err := gh("pr", "list", "-R", *repo, "--state", "open",
		"--limit", fmt.Sprint(*limit), "--json", "number,title,labels")
	if err != nil {
		// A sweep that could not look reports could-not-check for the WHOLE repo and
		// exits 6. It must never be read as "this repo has no orphans" — the empty
		// board is the #777 failure this three-state exists to prevent.
		fmt.Fprintf(out, "could-not-check\t%s\tPR list read failed — this repo's queue is UNKNOWN, not empty\n", *repo)
		return deskkit.Unverifiable("sweep: could-not-check for "+*repo, err)
	}
	var items []prListItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		fmt.Fprintf(out, "could-not-check\t%s\tPR list unparseable — this repo's queue is UNKNOWN, not empty\n", *repo)
		return deskkit.Unverifiable("sweep: could-not-check for "+*repo, err)
	}

	// At the cap the page is possibly truncated. Say so: a sweep that silently
	// truncates reports a short queue as the whole queue (the #80/#79 trap).
	if len(items) >= *limit {
		fmt.Fprintf(os.Stderr, "%s: sweep returned %d results at --limit %d — treat as POSSIBLY TRUNCATED "+
			"and widen the limit rather than claiming this is the whole queue\n", *repo, len(items), *limit)
	}

	for _, it := range items {
		r := deskkit.ReadDispositionIndex(labelNames(it.Labels), nil)
		if *eligibleOnly && !r.DispatchEligible() {
			continue
		}
		fmt.Fprintf(out, "%d\t%s\t%s\t%t\t%s\n", it.Number, r.State, r.Record.Verdict,
			r.DispatchEligible(), deskkit.StripControl(it.Title))
	}
	return nil
}

// ---------------------------------------------------------------- helpers

func commentBodies(v *prView) []string {
	out := make([]string, 0, len(v.Comments))
	for _, c := range v.Comments {
		out = append(out, c.Body)
	}
	return out
}

func viewPR(repo string, pr int) (*prView, error) {
	raw, err := gh("pr", "view", fmt.Sprint(pr), "-R", repo, "--json", "labels,comments")
	if err != nil {
		return nil, err
	}
	var v prView
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func auditLine(repo string, pr int, verb, result, detail string) {
	sha, built := deskkit.Version()
	p := pr
	_ = deskkit.Log(deskkit.Entry{
		TS:         nowFunc().UTC().Format(time.RFC3339),
		Tool:       toolName,
		Verb:       verb,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
		Repo:       repo,
		PR:         &p,
		Result:     result,
		Detail:     deskkit.StripControl(detail),
		SourceSHA:  sha,
		BuiltAt:    built,
		SessionTag: deskkit.SessionTag(),
	})
}
