// Command deskscanbody DERIVES the issue-loop scan PR's title and body from the branch's
// own diff, and diffs an already-posted title/body against that derivation.
//
// It exists because the scan PR's created/retired counts have now drifted three times
// (#592, #627, #685). Each fix was "regenerate the body on every push" as a discipline,
// and the discipline is what failed: the intake-desk coalesces many scans into ONE PR, so
// the body is written once and the diff keeps growing under it. On #627 the body claimed
// 29/29 against a diff of 48/32 and the reviewer had to catch it twice.
//
// This tool is the derived source. `emit` prints the title and body straight from
// `git diff <merge-base>..HEAD -- docs/streams/issue-loop/`; the scan flow pipes it into
// the PR on EVERY push, so there is no hand-maintained second copy to drift. `check` is
// the belt-and-suspenders half: it FAILS (exit 5) on a title/body whose stated counts
// disagree with the diff, which turns the class into a gate instead of a reviewer catch.
//
// It is a pure LOCAL READ: it runs git in the current worktree and writes nothing, makes
// no network call, and touches no credential.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused · 6 unverifiable.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskscanbody — derive the issue-loop scan PR title/body from the branch diff.

USAGE:
  deskscanbody emit  [--base origin/main] [--dir docs/streams/issue-loop] [--date YYYY-MM-DD]
                     [--format both|title|body]
  deskscanbody check --text-file <f> [--base origin/main] [--dir ...]
  deskscanbody --version

emit  — print the DERIVED title and/or body. --format title prints one line (pipe it to
        ` + "`gh pr edit --title`" + `); --format body prints the body (pipe it to --body-file -);
        --format both prints the title, a blank line, then the body. Run it on EVERY push
        to the coalesced scan branch — that is the whole mechanism.

check — read a title/body from --text-file and REFUSE (exit 5) when the counts it states
        disagree with the diff. Text stating no counts passes: absence is not drift.

The counts come from ` + "`git diff <merge-base(--base, HEAD)>..HEAD -- <dir>`" + `. A created
placeholder is a new .md file under <dir>; a retired one is a file whose diff adds
` + "`status: done`" + ` or that is renamed into <dir>/done/.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable.`

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskscanbody sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill switch gates this tool like every other, even though it performs no
	// outward write: an armed DISABLED means the desk toolchain is stopped, and a
	// tool that kept answering would keep a flow running past the stop.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "emit":
		err = cmdEmit(rest, os.Stdout)
	case "check":
		err = cmdCheck(rest, os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "deskscanbody: unknown subcommand %q\n\n%s\n", sub, usage)
		auditLine("unknown", deskkit.ResultRefused, "unknown subcommand")
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskscanbody:", err.Error())
	}
	auditLine(sub, resultFor(err), detailFor(err))
	return deskkit.ExitCodeOf(err)
}

// resultFor maps the terminal error to exactly one audit result (C-5: one line per
// invocation). Every outcome of this tool is a pure read, so success is ResultDryRun by
// the audit schema's own definition — the write meters must never count a derivation.
func resultFor(err error) string {
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitOK:
		return deskkit.ResultDryRun
	case deskkit.ExitDisabled:
		return deskkit.ResultDisabled
	case deskkit.ExitRefused:
		return deskkit.ResultRefused
	default:
		return deskkit.ResultUnverifiable
	}
}

func detailFor(err error) string {
	if err == nil {
		return "derived"
	}
	return deskkit.StripControl(err.Error())
}

func auditLine(verb, result, detail string) {
	sha, built := deskkit.Version()
	_ = deskkit.Log(deskkit.Entry{
		TS:         time.Now().UTC().Format(time.RFC3339),
		Tool:       "deskscanbody",
		Verb:       verb,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
		Result:     result,
		Detail:     detail,
		SourceSHA:  sha,
		BuiltAt:    built,
		SessionTag: deskkit.SessionTag(),
	})
}

// commonFlags are shared by both verbs so the derivation cannot be configured two
// different ways.
type commonFlags struct {
	base string
	dir  string
}

func bindCommon(fs *flag.FlagSet) *commonFlags {
	c := &commonFlags{}
	// FULLY-QUALIFIED remote-tracking ref, not the bare short name `origin/main`
	// (#885): a stray local `refs/heads/origin/main` decoy would otherwise shadow
	// the real remote tip and take the merge-base against a stale base.
	fs.StringVar(&c.base, "base", "refs/remotes/origin/main", "base ref; the diff is taken from its merge-base with HEAD")
	fs.StringVar(&c.dir, "dir", deskkit.ScanDir, "repo-relative directory the scan is scoped to")
	return c
}

func cmdEmit(args []string, out *os.File) error {
	fs := flag.NewFlagSet("emit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	c := bindCommon(fs)
	format := fs.String("format", "both", "both | title | body")
	date := fs.String("date", "", "scan date for the title (default: today, UTC)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("emit: " + err.Error())
	}

	counts, base, err := derive(c)
	if err != nil {
		return err
	}
	d := strings.TrimSpace(*date)
	if d == "" {
		d = time.Now().UTC().Format("2006-01-02")
	}

	title := deskkit.ScanPRTitle(d, counts)
	body := deskkit.ScanPRBody(d, counts, base)
	switch *format {
	case "title":
		fmt.Fprintln(out, title)
	case "body":
		fmt.Fprint(out, body)
	case "both":
		fmt.Fprintf(out, "%s\n\n%s", title, body)
	default:
		return deskkit.Refused(fmt.Sprintf("emit: unknown --format %q (both | title | body)", *format))
	}
	return nil
}

func cmdCheck(args []string, out *os.File) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	c := bindCommon(fs)
	textFile := fs.String("text-file", "", "file holding the PR title and/or body to check")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("check: " + err.Error())
	}
	if strings.TrimSpace(*textFile) == "" {
		return deskkit.Refused("check: --text-file is required")
	}
	raw, err := os.ReadFile(*textFile)
	if err != nil {
		// Unreadable input is could-not-check, not clean: a gate that cannot read
		// the thing it judges must never pass it.
		return deskkit.Unverifiable("could-not-check: cannot read --text-file", err)
	}

	counts, _, err := derive(c)
	if err != nil {
		return err
	}
	if err := deskkit.ScanBodyDrift(string(raw), counts); err != nil {
		return err
	}
	fmt.Fprintf(out, "checked-clean: stated counts agree with the diff (%d created, %d retired)\n",
		len(counts.Created), len(counts.Retired))
	return nil
}

// derive runs the two git reads and returns the counts plus the resolved merge-base.
// A git failure is ExitUnverifiable (could-not-check), never an empty-diff "clean":
// zero created / zero retired is a real answer only when git actually answered.
func derive(c *commonFlags) (deskkit.ScanCounts, string, error) {
	mb, err := gitOut("merge-base", c.base, "HEAD")
	if err != nil {
		return deskkit.ScanCounts{}, "", deskkit.Unverifiable(
			"could-not-check: cannot resolve the merge-base of "+c.base+" and HEAD", err)
	}
	diff, err := gitOut("diff", "-U0", "-M", "--no-color", mb, "HEAD", "--", c.dir)
	if err != nil {
		return deskkit.ScanCounts{}, "", deskkit.Unverifiable(
			"could-not-check: cannot take the diff of "+mb+"..HEAD -- "+c.dir, err)
	}
	return deskkit.ParseScanDiff(diff, c.dir), mb, nil
}
