package main

// check.go — `deskpathguard check`'s CLI wiring: read the PR, run Evaluate (protected.go),
// apply the label when it fails. The forge-facing shape (session-role mint, AllowWrite
// budget, ApplyLabels) follows the desklabel precedent verbatim.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskpathguard"

// roleFn resolves the App role this session acts under (DESK_LOOP → role). Production
// binds it to sessionRole; a test substitutes a fixed role. There is no --as flag and no
// worker default: a claimed role is not a fact, a minted one is.
var roleFn = sessionRole

func sessionRole() (string, error) {
	role, _, err := deskkit.SessionTokenRole(toolName)
	if err != nil {
		return "", deskkit.Refused(
			"refused: deskpathguard could not resolve which App role this session acts under (" + err.Error() +
				") — run inside a booted desk window (DESK_LOOP set) whose role the roster binds to an App.")
	}
	return role, nil
}

type checkRequest struct {
	repo   string
	number int
	dryRun bool
}

func parseCheckArgs(args []string) (checkRequest, error) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "resolve the verdict and print it, but never write the label")
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return checkRequest{}, deskkit.Refused("check: " + err.Error())
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
	if len(pos) != 2 {
		return checkRequest{}, deskkit.Refused(fmt.Sprintf(
			"check: want exactly two positionals <owner/repo> <number>, got %d", len(pos)))
	}
	req := checkRequest{repo: strings.TrimSpace(pos[0]), dryRun: *dryRun}
	owner, name, ok := strings.Cut(req.repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return checkRequest{}, deskkit.Refused(fmt.Sprintf("check: repo %q is not <owner/repo>", deskkit.StripControl(req.repo)))
	}
	numText := strings.TrimLeft(strings.TrimSpace(pos[1]), "#!")
	n, err := strconv.Atoi(numText)
	if err != nil || n <= 0 {
		return checkRequest{}, deskkit.Refused(fmt.Sprintf("check: number %q is not a positive integer", deskkit.StripControl(pos[1])))
	}
	req.number = n
	return req, nil
}

// requireRepo enforces the allowed-repo set. This tool introduces no repo list of its own:
// the roster is the single declared source.
func requireRepo(repo string) error {
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused(fmt.Sprintf("repo %q is outside the desk-tools repo set", deskkit.StripControl(repo)))
	}
	return nil
}

func cmdCheck(args []string, out io.Writer) error {
	req, err := parseCheckArgs(args)
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

	fg, fr, ferr := forgeForFn(req.repo)
	if ferr != nil {
		return ferr
	}

	iss, ierr := fg.GetIssue(fr, req.number)
	if ierr != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d before evaluating its protected-path touch", req.repo, req.number), ierr)
	}

	changed, cerr := fg.ListChangedFiles(fr, req.number)
	diffReadable := cerr == nil
	var entries []FileEntry
	hasBrief := false
	if diffReadable {
		for _, f := range changed {
			entries = append(entries, FileEntry{Path: f.Filename})
			if isBriefPath(f.Filename) {
				hasBrief = true
			}
		}
		if hasBrief {
			diffText, derr := fg.ChangeDiff(fr, req.number)
			if derr != nil {
				diffReadable = false
			} else {
				for i := range entries {
					if isBriefPath(entries[i].Path) {
						entries[i].VerifySectionTouched = verifySectionTouched(diffText, entries[i].Path)
					}
				}
			}
		}
	}

	cfg := deskkit.EffectiveConfig()
	desk, _ := cfg.RoleBotIdentity("desk")
	verifier, _ := cfg.RoleBotIdentity("verifier")

	res := Evaluate(EvalInput{
		AuthorLogin:    iss.Author.Login,
		DeskLogins:     desk.AcceptedLogins(),
		VerifierLogins: verifier.AcceptedLogins(),
		ExistingLabels: iss.Labels,
		Files:          entries,
		DiffReadable:   diffReadable,
	})

	fmt.Fprintf(out, "deskpathguard: %s#%d state=%s reason=%q\n", req.repo, req.number, res.State, res.Reason)

	if res.State == stateCouldNotCheck {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d's diff could not be read — %s", req.repo, req.number, res.Reason), nil)
	}

	if !res.Label {
		fmt.Fprintln(out, "label: none")
		return nil
	}

	fmt.Fprintf(out, "label: %s\n", wroteToTheTestLabel)
	fmt.Fprintln(out, gateForcedLine)

	if req.dryRun {
		fmt.Fprintf(out, "dry-run: would apply %q to %s#%d and force gate: human\n", wroteToTheTestLabel, req.repo, req.number)
		return nil
	}

	if err := deskkit.AllowWrite(toolName, req.repo, req.number); err != nil {
		return err
	}
	change := deskkit.LabelChange{Target: deskkit.TargetChange, Add: []deskkit.LabelSpec{{Name: wroteToTheTestLabel}}}
	if _, werr := fg.ApplyLabels(fr, req.number, change); werr != nil {
		if deskkit.ExitCodeOf(werr) == deskkit.ExitRefused {
			return werr
		}
		return deskkit.Unverifiable(fmt.Sprintf("cannot apply %q to %s#%d", wroteToTheTestLabel, req.repo, req.number), werr)
	}
	fmt.Fprintf(out, "applied: %s on %s#%d\n", wroteToTheTestLabel, req.repo, req.number)
	return nil
}
