package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// options.go — flag parsing, the run's audit line, and the one decision this tool makes
// about itself: whether it is allowed to write.

var weekIDRe = regexp.MustCompile(`^\d{4}-W\d{2}$`)

type opts struct {
	DryRun     bool
	Post       bool
	DigestRepo string
	Repos      []string
	Rulings    string
	Now        time.Time
	Week       string
}

func parseOpts(args []string, stderr io.Writer) (*opts, error) {
	fs := flag.NewFlagSet(toolName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dry := fs.Bool("dry-run", false, "print the digest, write nothing")
	post := fs.Bool("post", false, "create-or-update this week's digest issue")
	digestRepo := fs.String("digest-repo", "", "owner/repo the weekly digest issue lives in")
	repos := fs.String("repos", "", "comma-separated read scope")
	allRepos := fs.Bool("all-repos", false, "read the roster's whole scan scope (explicit opt-in)")
	rulings := fs.String("rulings", defaultRulingsPath, "path to the rulings register")
	nowStr := fs.String("now", "", "freeze the clock (RFC3339)")
	week := fs.String("week", "", "override the ISO week id, e.g. 2026-W33")
	if err := fs.Parse(args); err != nil {
		return nil, deskkit.Refused("refused: " + err.Error())
	}
	if fs.NArg() > 0 {
		return nil, deskkit.Refused("refused: unexpected argument " +
			deskkit.StripControl(fs.Arg(0)) + " — deskdigest takes flags only")
	}
	o := &opts{DryRun: *dry, Post: *post, DigestRepo: strings.TrimSpace(*digestRepo), Rulings: *rulings}

	// --dry-run and --post are mutually exclusive because --dry-run is a PROMISE not to
	// write, and a promise that can be paired with the write flag is not one. deskkit
	// makes the same demand of any tool that emits ResultDryRun: the flag that selects
	// the result must be the flag that suppresses the write.
	if o.DryRun && o.Post {
		return nil, deskkit.Refused("refused: --dry-run and --post are mutually exclusive — " +
			"--dry-run is the promise that this run performs no outward write")
	}

	o.Now = time.Now().UTC()
	if strings.TrimSpace(*nowStr) != "" {
		t, err := time.Parse(time.RFC3339, *nowStr)
		if err != nil {
			return nil, deskkit.Refused("refused: --now must be RFC3339, got " + deskkit.StripControl(*nowStr))
		}
		o.Now = t.UTC()
	}
	o.Week = isoWeek(o.Now)
	if w := strings.TrimSpace(*week); w != "" {
		if !weekIDRe.MatchString(w) {
			return nil, deskkit.Refused("refused: --week must look like 2026-W33, got " + deskkit.StripControl(w))
		}
		o.Week = w
	}

	if o.Post {
		if o.DigestRepo == "" {
			return nil, deskkit.Refused("refused: --post needs --digest-repo <owner/repo> — deskdigest has no " +
				"compiled-in home. A tool that defaults its own write target is a tool that writes somewhere " +
				"nobody chose.")
		}
		// The write boundary is deskkit's, not this tool's: deskdigest introduces NO repo
		// list of its own, exactly as deskfile does not.
		if !deskkit.IsAllowedRepo(o.DigestRepo) {
			return nil, deskkit.Refused("refused: " + deskkit.StripControl(o.DigestRepo) +
				" is not in the desk-tools repo set (deskkit.IsAllowedRepo)")
		}
	}

	// The READ scope is chosen, never defaulted. --post already refuses to default its
	// WRITE target for the same reason, and a probe that picks its own read targets is the
	// same sentence with one word changed: a bare invocation would silently fan a live `gh`
	// label-list plus a paginated issue-and-comment read across the roster's whole scan
	// scope, a scope nobody named at the call site. So the scope is an explicit choice —
	// --repos to name it, --all-repos to opt into the whole roster scan scope — and neither
	// is refused BEFORE any network read happens. `--repos` that resolves EMPTY (e.g. " , ")
	// is a different case: the scope WAS named, it just came out empty, which dispatch reports
	// as could-not-check rather than silence.
	switch {
	case strings.TrimSpace(*repos) != "":
		for _, r := range strings.Split(*repos, ",") {
			if r = strings.TrimSpace(r); r != "" {
				o.Repos = append(o.Repos, r)
			}
		}
	case *allRepos:
		o.Repos = deskkit.ScanRepos()
	default:
		return nil, deskkit.Refused("refused: no read scope. Pass --repos <owner/repo,...> to name the repos to " +
			"read, or --all-repos to read the roster's whole scan scope. deskdigest does not default its own read " +
			"target: a tool that reads somewhere nobody chose is the same failure as --post writing somewhere " +
			"nobody chose.")
	}
	return o, nil
}

// auditCtx accumulates what the run's single audit line will say.
type auditCtx struct {
	verb   string
	repo   string
	target *int
	detail string
	body   string
	// wrote records that an outward write was ATTEMPTED. It is set BEFORE the call, not
	// after: an error return from a create must still be billed as a write, because the
	// remote may have received it.
	wrote bool
}

func (a *auditCtx) log(result string) {
	e := deskkit.Entry{
		Tool:       toolName,
		Verb:       a.verb,
		Result:     result,
		Detail:     a.detail,
		Repo:       a.repo,
		PR:         a.target,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	}
	if a.body != "" {
		e.BodyDigest = deskkit.Sha256Hex([]byte(a.body))
	}
	_ = deskkit.Log(e)
}

// finalize maps the terminal error to exactly one audit result.
//
// A run that never attempted a write and failed a precondition is ResultUnwritten, not
// ResultUnverifiable: deskkit bills those differently and the difference is real here —
// a repo that would not answer must not consume the outward-write budget of a tool that
// never reached the write.
func (a *auditCtx) finalize(err error) {
	switch {
	case err == nil && !a.wrote:
		a.log(deskkit.ResultDryRun)
	case err == nil:
		a.log(deskkit.ResultOK)
	case deskkit.IsDisabled(err):
		a.log(deskkit.ResultDisabled)
	case deskkit.IsRateLimited(err):
		a.log(deskkit.ResultRateLimited)
	case deskkit.IsRefused(err):
		a.log(deskkit.ResultRefused)
	case a.wrote:
		a.log(deskkit.ResultUnverifiable)
	default:
		a.log(deskkit.ResultUnwritten)
	}
}

// dispatch is the whole run: parse, read, classify, render, and — only under --post —
// write exactly one issue.
func dispatch(args []string, stdout, stderr io.Writer) (err error) {
	o, perr := parseOpts(args, stderr)
	if perr != nil {
		(&auditCtx{verb: "parse", detail: perr.Error()}).finalize(perr)
		return perr
	}

	ac := &auditCtx{verb: "digest", repo: o.DigestRepo}
	if o.Post {
		ac.verb = "post"
	}
	defer func() { ac.finalize(err) }()

	coll := collect(o.Repos)
	d := build(coll, readR3SignOff(o.Rulings), o.Now, o.Week)
	body := d.Render()
	ac.body = body

	// PARTIALITY IS REPORTED, NOT SUPPRESSED. A run that could not read every repo still
	// produces a digest, and the digest names the gap in its own body (renderGaps). The
	// exit code carries the same news to the caller. A digest that admits it is partial
	// beats no digest at all: the alternative is a silent week, and a silent week is the
	// one failure this tool exists to make impossible.
	var partial error
	if len(o.Repos) == 0 {
		partial = deskkit.Unverifiable("could-not-check: the read scope resolved EMPTY — a digest over no "+
			"repos is not evidence that there is nothing to decide. Set the roster's scan scope or pass --repos", nil)
	} else if coll.Partial() {
		partial = deskkit.Unverifiable("could-not-check: at least one repo in scope did not answer; the digest "+
			"names the gap under 'Read gaps' and its queue is NOT exhaustive", nil)
	}

	if !o.Post {
		fmt.Fprint(stdout, body)
		ac.detail = fmt.Sprintf("week=%s items=%d rendered=%d (no write)", d.Week, len(coll.Items), len(d.Rows))
		return partial
	}

	num, created, perr2 := postDigest(ac, d, o.DigestRepo, body, stderr)
	if perr2 != nil {
		return perr2
	}
	verb := "updated"
	if created {
		verb = "created"
	}
	ac.detail = fmt.Sprintf("week=%s items=%d %s %s#%d", d.Week, len(coll.Items), verb, o.DigestRepo, num)
	fmt.Fprintf(stdout, "%s %s#%d %s\n", verb, o.DigestRepo, num, d.title())
	return partial
}
