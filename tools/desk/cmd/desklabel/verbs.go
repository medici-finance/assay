package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "desklabel"

// nowFunc is the clock seam; tests pin it so the audit timestamp is deterministic.
var nowFunc = time.Now

// roleFn resolves the App role this session acts under. Production binds it to
// sessionRole (DESK_LOOP → role via deskkit.SessionTokenRole, no default); a test
// substitutes a fixed role. It is a PURE environment read — it touches no forge and mints
// nothing — so the ownership check that follows it runs before any forge call.
var roleFn = sessionRole

// sessionRole reads the session's App role. There is NO worker default and NO override
// flag: the vocabulary is keyed on this value, and a guessed or claimed role would be
// exactly the hole the table exists to close.
func sessionRole() (string, error) {
	role, _, err := deskkit.SessionTokenRole(toolName)
	if err != nil {
		return "", deskkit.Refused(
			"refused: desklabel could not resolve which App role this session acts under (" + err.Error() +
				") — the label vocabulary is keyed on the SESSION role and no flag asserts one. Run inside a " +
				"booted desk window (DESK_LOOP set) whose role the roster binds to an App.")
	}
	return role, nil
}

// labelOp is the verb: add or rm.
type labelOp string

const (
	opAdd labelOp = "add"
	opRm  labelOp = "rm"
)

func cmdAdd(args []string, out io.Writer) error { return runLabel(opAdd, args, out) }
func cmdRm(args []string, out io.Writer) error  { return runLabel(opRm, args, out) }

// request is the parsed invocation: positionals plus the two flags.
type request struct {
	repo   string
	number int
	label  string
	kind   deskkit.TargetKind // "" when --kind was not given
	dryRun bool
}

// parseArgs accepts flags before, between or after the three positionals
// (`<owner/repo> <number> <label>`), so `--dry-run` can trail the label the way an
// operator types it.
func parseArgs(o labelOp, args []string) (request, error) {
	fs := flag.NewFlagSet(string(o), flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	kind := fs.String("kind", "", "issue | mr (pr is an alias) — the target's kind; needed only where a bare number is ambiguous")
	dryRun := fs.Bool("dry-run", false, "validate the ownership check and read the target, then stop before the write")
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return request{}, deskkit.Refused(string(o) + ": " + err.Error())
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
	if len(pos) != 3 {
		return request{}, deskkit.Refused(fmt.Sprintf(
			"%s: want exactly three positionals <owner/repo> <number> <label>, got %d", o, len(pos)))
	}
	req := request{repo: strings.TrimSpace(pos[0]), label: strings.TrimSpace(pos[2]), dryRun: *dryRun}
	owner, name, ok := strings.Cut(req.repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return request{}, deskkit.Refused(fmt.Sprintf("%s: repo %q is not <owner/repo>", o, deskkit.StripControl(req.repo)))
	}
	numText := strings.TrimLeft(strings.TrimSpace(pos[1]), "#!")
	n, err := strconv.Atoi(numText)
	if err != nil || n <= 0 {
		return request{}, deskkit.Refused(fmt.Sprintf("%s: number %q is not a positive integer", o, deskkit.StripControl(pos[1])))
	}
	req.number = n
	if req.label == "" {
		return request{}, deskkit.Refused(string(o) + ": the label is empty")
	}
	if strings.TrimSpace(*kind) != "" {
		k, kerr := deskkit.ParseTargetKind(*kind)
		if kerr != nil {
			return request{}, kerr
		}
		req.kind = k
	}
	return req, nil
}

// requireRepo enforces the allowed-repo set. This tool introduces NO repo list of its own:
// the roster is the single declared source, and an unconfigured roster refuses.
func requireRepo(repo string) error {
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused(fmt.Sprintf("repo %q is outside the desk-tools repo set", deskkit.StripControl(repo)))
	}
	return nil
}

// runLabel is the one path both verbs take. The ORDER is the control:
//
//  1. parse + repo gate                       (local; no forge)
//  2. resolve the session role                 (local; no forge)
//  3. OWNERSHIP CHECK — authorize(label, role) (local; refuses BEFORE any forge call)
//  4. resolve the forge under the role's custody
//  5. read the target's kind + current labels  (GetIssue, or GetIssueTyped with --kind)
//  6. no-op if the desired state already holds; stop on --dry-run
//  7. write budget, then ApplyLabels with the resolved target
func runLabel(o labelOp, args []string, out io.Writer) error {
	req, err := parseArgs(o, args)
	if err != nil {
		return err
	}
	if err := requireRepo(req.repo); err != nil {
		return err
	}

	role, err := roleFn()
	if err != nil {
		return err
	}
	mintedRole = role

	canonical, aerr := authorize(req.label, role)
	if aerr != nil {
		auditLine(req.repo, req.number, string(o), deskkit.ResultRefused, req.label+": "+aerr.Error())
		return aerr
	}

	fg, fr, ferr := forgeForFn(req.repo)
	if ferr != nil {
		return ferr
	}

	// The kind is learned from the seam's own read. Without --kind, GetIssue (op 2)
	// resolves it and refuses a GitLab both-resolve; with --kind, GetIssueTyped (op 38)
	// reads exactly the stated kind — on a one-sequence forge the kind is VALIDATED there,
	// never used to route.
	var iss *deskkit.Issue
	var rerr error
	if req.kind != "" {
		iss, rerr = fg.GetIssueTyped(fr, req.number, req.kind)
	} else {
		iss, rerr = fg.GetIssue(fr, req.number)
	}
	if rerr != nil {
		var de *deskkit.DeskError
		if !errors.As(rerr, &de) {
			rerr = deskkit.Unverifiable(fmt.Sprintf(
				"could-not-check: cannot read %s#%d before writing — refusing to label a target whose kind "+
					"and current labels are unknown", req.repo, req.number), rerr)
		}
		auditLine(req.repo, req.number, string(o), deskkit.ResultUnverifiable, "read: "+rerr.Error())
		return rerr
	}
	target := deskkit.TargetIssue
	if iss.IsPullRequest {
		target = deskkit.TargetChange
	}
	if req.kind != "" {
		target = req.kind
	}

	// Idempotency, reported: the desired state already holding is a no-op that makes NO
	// write and charges no budget. ApplyLabels is idempotent underneath as well; this is the
	// same property one read earlier, so the audit line says "noop" rather than "ok".
	present := hasLabelFold(iss.Labels, canonical)
	if (o == opAdd && present) || (o == opRm && !present) {
		fmt.Fprintf(out, "noop: %s#%d (%s) already %s %q\n", req.repo, req.number, target, alreadyWord(o), canonical)
		auditLine(req.repo, req.number, string(o), deskkit.ResultNoop, canonical)
		return nil
	}

	if req.dryRun {
		fmt.Fprintf(out, "dry-run: would %s label %q on %s#%d (%s) as the %s role\n",
			verbWord(o), canonical, req.repo, req.number, target, role)
		auditLine(req.repo, req.number, string(o), deskkit.ResultDryRun, canonical)
		return nil
	}

	if err := deskkit.AllowWrite(toolName, req.repo, req.number); err != nil {
		return err
	}
	change := deskkit.LabelChange{Target: target}
	if o == opAdd {
		change.Add = []deskkit.LabelSpec{{Name: canonical}}
	} else {
		change.Remove = []string{canonical}
	}
	outcome, werr := fg.ApplyLabels(fr, req.number, change)
	if werr != nil {
		auditLine(req.repo, req.number, string(o), deskkit.ResultUnverifiable, "write: "+werr.Error())
		if deskkit.ExitCodeOf(werr) == deskkit.ExitRefused {
			return werr
		}
		return deskkit.Unverifiable(fmt.Sprintf("%s: cannot %s %q on %s#%d", o, verbWord(o), canonical, req.repo, req.number), werr)
	}
	auditLine(req.repo, req.number, string(o), deskkit.ResultOK, canonical)
	switch o {
	case opAdd:
		fmt.Fprintf(out, "added: %s on %s#%d (%s) as the %s role\n", strings.Join(outcome.Added, ","), req.repo, req.number, target, role)
	default:
		fmt.Fprintf(out, "removed: %s from %s#%d (%s) as the %s role\n", strings.Join(outcome.Removed, ","), req.repo, req.number, target, role)
	}
	return nil
}

// hasLabelFold reports whether want is among labels, case-insensitively — forge labels are
// case-insensitive, so `Needs-Decision` on the forge IS `needs-decision` for a no-op read.
func hasLabelFold(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}

func verbWord(o labelOp) string {
	if o == opAdd {
		return "add"
	}
	return "remove"
}

func alreadyWord(o labelOp) string {
	if o == opAdd {
		return "carries"
	}
	return "lacks"
}

func auditLine(repo string, number int, verb, result, detail string) {
	sha, built := deskkit.Version()
	n := number
	_ = deskkit.Log(deskkit.Entry{
		TS:         nowFunc().UTC().Format(time.RFC3339),
		Tool:       toolName,
		Verb:       verb,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
		Repo:       repo,
		PR:         &n,
		Result:     result,
		Detail:     deskkit.StripControl(detail),
		SourceSHA:  sha,
		BuiltAt:    built,
		SessionTag: deskkit.SessionTag(),
	})
}
