package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskrun"

// nowFunc is the audit clock, swapped in tests.
var nowFunc = time.Now

// kvFlags collects repeated `-f key=value` inputs.
type kvFlags map[string]string

func (k kvFlags) String() string { return "" }

func (k kvFlags) Set(v string) error {
	key, val, ok := strings.Cut(v, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return fmt.Errorf("input %q is not key=value", deskkit.StripControl(v))
	}
	if _, dup := k[key]; dup {
		return fmt.Errorf("input %q given twice", deskkit.StripControl(key))
	}
	k[key] = val
	return nil
}

// parseInterspersed parses fs over args, allowing flags after positionals.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

// parseRepo validates <owner/repo> and the write-authorisation scope.
func parseRepo(raw string) (deskkit.ForgeRepo, error) {
	raw = strings.TrimSpace(raw)
	owner, name, ok := strings.Cut(raw, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return deskkit.ForgeRepo{}, deskkit.Refused(fmt.Sprintf("repo %q is not <owner/repo>", deskkit.StripControl(raw)))
	}
	if !deskkit.IsAllowedRepo(raw) {
		return deskkit.ForgeRepo{}, deskkit.Refused(fmt.Sprintf("repo %q is outside the desk-tools repo set", deskkit.StripControl(raw)))
	}
	return deskkit.ForgeRepo{Owner: owner, Name: name}, nil
}

// bindingFor applies the identity rule BEFORE anything is minted: deskkit.ResolveRunCredential
// refuses a human-bound repo (exit 5) and reports an unbound one as could-not-check (exit 6).
// Either way the refusal is audited and nothing else runs.
func bindingFor(verb string, fr deskkit.ForgeRepo) (deskkit.RunCredential, error) {
	cred, err := resolveCredFn(fr)
	if err != nil {
		result := deskkit.ResultUnverifiable
		if deskkit.ExitCodeOf(err) == deskkit.ExitRefused {
			result = deskkit.ResultRefused
		}
		auditLine(fr.Slug(), verb, result, "binding: "+err.Error())
		return cred, err
	}
	if cred.Role != deskkit.ReleaseRunnerRole {
		// Unreachable through ResolveRunCredential (a binding is human OR release-runner) —
		// kept fail-closed so a future binding kind can never fall through to a mint.
		err := deskkit.Refused(fmt.Sprintf("refused: %s's run-credential binding is not the %s role", fr.Slug(), deskkit.ReleaseRunnerRole))
		auditLine(fr.Slug(), verb, deskkit.ResultRefused, err.Error())
		return cred, err
	}
	return cred, nil
}

// resolveForge obtains the backend under the release-runner custody. A custody refusal (no
// token provisioned, an empty token) is surfaced as-is: it is the resolver's own refusal and
// no forge call has happened.
func resolveForge(verb string, fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
	fg, res, err := forgeForFn(fr)
	if err != nil {
		var de *deskkit.DeskError
		if !errors.As(err, &de) {
			err = deskkit.Unverifiable("could-not-check: cannot obtain the "+deskkit.ReleaseRunnerRole+" credential for "+fr.Slug(), err)
		}
		auditLine(fr.Slug(), verb, resultOf(err), "custody: "+err.Error())
		return nil, res, err
	}
	return fg, res, nil
}

// cmdDispatch is `deskrun <owner/repo> <workflow> --ref <r> [-f k=v ...]`.
func cmdDispatch(args []string, out io.Writer) error {
	const verb = "dispatch"
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	ref := fs.String("ref", "", "the branch or tag to run on (required)")
	dryRun := fs.Bool("dry-run", false, "resolve the binding and budget, print what would happen, mint nothing")
	inputs := kvFlags{}
	fs.Var(inputs, "f", "a workflow input / pipeline variable, key=value (repeatable)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return deskkit.Refused(verb + ": " + err.Error())
	}
	if len(pos) != 2 {
		return deskkit.Refused(fmt.Sprintf("%s: want exactly two positionals <owner/repo> <workflow>, got %d", verb, len(pos)))
	}
	if strings.TrimSpace(*ref) == "" {
		return deskkit.Refused(verb + ": --ref is required — a run needs a branch or tag")
	}
	fr, err := parseRepo(pos[0])
	if err != nil {
		return err
	}
	workflow := strings.TrimSpace(pos[1])
	if workflow == "-" {
		workflow = ""
	}
	if err := deskkit.RequireLoopIdentity(toolName); err != nil {
		return err
	}
	if _, err := bindingFor(verb, fr); err != nil {
		return err
	}
	if err := deskkit.AllowWrite(toolName, fr.Slug(), 0); err != nil {
		return err
	}
	if *dryRun {
		fmt.Fprintf(out, "dry-run: would dispatch %s on %s@%s with %d input(s) as the %s credential\n",
			displayWorkflow(workflow), fr.Slug(), *ref, len(inputs), deskkit.ReleaseRunnerRole)
		auditLine(fr.Slug(), verb, deskkit.ResultDryRun, workflow+"@"+*ref)
		return nil
	}
	fg, res, err := resolveForge(verb, fr)
	if err != nil {
		return err
	}
	in := deskkit.RunWorkflowInput{Workflow: workflow, Ref: *ref, Inputs: map[string]string(inputs)}
	if res.Kind == deskkit.ForgeGitHub {
		// The actor narrows GitHub's created-run correlation read; a release-runner with no
		// roster App binding (e.g. a fine-grained token) correlates without it.
		if login, ok := deskkit.RoleAppLogin(deskkit.ReleaseRunnerRole); ok {
			in.Actor = login
		}
	}
	runRef, err := fg.RunWorkflow(fr, in)
	if err != nil {
		auditLine(fr.Slug(), verb, resultOf(err), "run: "+err.Error())
		return err
	}
	auditLine(fr.Slug(), verb, deskkit.ResultOK, fmt.Sprintf("%s@%s run %s", workflow, *ref, runRef.ID))
	fmt.Fprintf(out, "dispatched: %s on %s@%s — run %s %s (inputs: %s)\n", displayWorkflow(workflow), fr.Slug(), *ref,
		runRef.ID, runRef.URL, inputKeys(inputs))
	return nil
}

// cmdApprove is `deskrun approve <owner/repo> <run-id> --gate <name>`.
func cmdApprove(args []string, out io.Writer) error {
	const verb = "approve"
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	gate := fs.String("gate", "", "the deployment gate (environment / manual job) to approve (required)")
	dryRun := fs.Bool("dry-run", false, "resolve the binding and budget, print what would happen, mint nothing")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return deskkit.Refused(verb + ": " + err.Error())
	}
	if len(pos) != 2 {
		return deskkit.Refused(fmt.Sprintf("%s: want exactly two positionals <owner/repo> <run-id>, got %d", verb, len(pos)))
	}
	if strings.TrimSpace(*gate) == "" {
		return deskkit.Refused(verb + ": --gate is required — the gate is named, never inferred from what is pending")
	}
	fr, err := parseRepo(pos[0])
	if err != nil {
		return err
	}
	run := deskkit.RunRef{ID: strings.TrimSpace(pos[1])}
	if _, err := deskkit.ValidateRunID(run); err != nil {
		return err
	}
	if err := deskkit.RequireLoopIdentity(toolName); err != nil {
		return err
	}
	cred, err := bindingFor(verb, fr)
	if err != nil {
		return err
	}
	if err := deskkit.AllowWrite(toolName, fr.Slug(), 0); err != nil {
		return err
	}
	if *dryRun {
		fmt.Fprintf(out, "dry-run: would approve gate %q (shape %s) on %s run %s as the %s credential\n",
			*gate, displayShape(cred.GateShape), fr.Slug(), run.ID, deskkit.ReleaseRunnerRole)
		auditLine(fr.Slug(), verb, deskkit.ResultDryRun, *gate+" on run "+run.ID)
		return nil
	}
	fg, _, err := resolveForge(verb, fr)
	if err != nil {
		return err
	}
	if err := fg.ApproveGate(fr, run, deskkit.ApproveGateInput{Gate: *gate, Shape: cred.GateShape}); err != nil {
		auditLine(fr.Slug(), verb, resultOf(err), "approve: "+err.Error())
		return err
	}
	auditLine(fr.Slug(), verb, deskkit.ResultOK, *gate+" on run "+run.ID)
	fmt.Fprintf(out, "approved: gate %q on %s run %s\n", *gate, fr.Slug(), run.ID)
	return nil
}

// cmdStatus is `deskrun status <owner/repo> <run-id>` — a READ under the same binding (the
// release-runner credential is the one that can see the run it started; a human-bound repo is
// refused here too, because deskrun never reads a forge under any other identity).
func cmdStatus(args []string, out io.Writer) error {
	const verb = "status"
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return deskkit.Refused(verb + ": " + err.Error())
	}
	if len(pos) != 2 {
		return deskkit.Refused(fmt.Sprintf("%s: want exactly two positionals <owner/repo> <run-id>, got %d", verb, len(pos)))
	}
	fr, err := parseRepo(pos[0])
	if err != nil {
		return err
	}
	run := deskkit.RunRef{ID: strings.TrimSpace(pos[1])}
	if _, err := deskkit.ValidateRunID(run); err != nil {
		return err
	}
	if _, err := bindingFor(verb, fr); err != nil {
		return err
	}
	fg, _, err := resolveForge(verb, fr)
	if err != nil {
		return err
	}
	st, err := fg.RunStatus(fr, run)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("run %s on %s: %s", run.ID, fr.Slug(), st.Status)
	if st.Conclusion != "" {
		line += " (" + st.Conclusion + ")"
	}
	if st.URL != "" {
		line += " " + st.URL
	}
	fmt.Fprintln(out, line)
	return nil
}

func displayWorkflow(w string) string {
	if w == "" {
		return "the pipeline"
	}
	return w
}

func displayShape(s deskkit.GateShape) string {
	if s == "" {
		return "(undeclared)"
	}
	return string(s)
}

// inputKeys renders the input NAMES only — values may be sensitive and never reach stdout.
func inputKeys(in kvFlags) string {
	if len(in) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, deskkit.StripControl(k))
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func resultOf(err error) string {
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitRefused:
		return deskkit.ResultRefused
	default:
		return deskkit.ResultUnverifiable
	}
}

// auditLine writes deskrun's one audit line. A run has no PR number, so the line records
// none — the unnumbered bucket AllowWrite(…, 0) gates on (the bucket a gate reads must be the
// bucket its writes land in).
func auditLine(repo, verb, result, detail string) {
	sha, built := deskkit.Version()
	_ = deskkit.Log(deskkit.Entry{
		TS:         nowFunc().UTC().Format(time.RFC3339),
		Tool:       toolName,
		Verb:       verb,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
		Repo:       repo,
		Result:     result,
		Detail:     deskkit.StripControl(detail),
		SourceSHA:  sha,
		BuiltAt:    built,
		SessionTag: deskkit.SessionTag(),
	})
}
