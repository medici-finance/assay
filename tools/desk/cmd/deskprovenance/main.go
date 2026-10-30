// Command deskprovenance gathers the contributor provenance signals
// (internal/deskkit/provenance.go) for one pull request and either prints the resulting
// card (--dry-run, or a live run without --post) or upserts it as a comment on that pull
// request (a live run with --post — see "Posting is double-gated" below).
//
// The card is FACTS ONLY: no score, no rating, no verdict — see docs/contributor-provenance.md,
// the published description of what this probe measures and deliberately does not. This tool
// NEVER fetches, checks out, builds or executes
// the pull request's head; every signal it gathers comes from metadata, either injected via
// --fixture (a JSON [deskkit.ProvenanceInput]) or read from the forge's own PR/changed-files
// endpoints.
//
// --fixture always implies --dry-run: fixture data is never posted to a real pull request.
//
// Posting is double-gated (clause 10, no-default-probe on the write side). Naming a live
// target (--repo/--pr) never by itself writes anything: a live run without --post gathers and
// prints the card exactly like --dry-run, so the flags that merely say WHERE to look never
// double as the flag that says WRITE. Passing --post is necessary but not sufficient — see
// liveCardRefusalReason: the write path additionally refuses unconditionally until the
// design record DR-provenance-card's ruling is recorded (it is still
// "PROPOSED — no ruling is recorded" as of this writing). Both gates are independent single
// points that either one alone would already close; keeping both is deliberate depth, not
// redundancy — one is a flag the caller can get right in isolation, the other holds even if a
// caller supplies every flag correctly.
//
// KNOWN SCOPE BOUNDARY (see the PR this shipped on). The live (non-fixture) gather path
// currently reads only what today's Forge interface already exposes: the pull request's own
// body and its changed file paths. The remaining five signals (account age, fork-to-PR
// elapsed, cross-repository burst, prior merged/closed ratio, commit signature) need forge
// reads this interface does not yet offer, and are reported as could-not-check on a live run
// rather than guessed — never rounded up to clean, never invented. Extending the Forge
// interface to carry them is a follow-up, not part of this brief's declared file list.
//
// Exit: 0 clean · 1 flagged · 6 could-not-check · 2 usage error.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// provenanceCardMarker is the exact-match line every card this tool posts carries, so a
// re-run finds and replaces its OWN previous card rather than appending a second one. It is
// never rendered by deskkit.Card itself (that function has no forge/CLI concerns at all);
// this command wraps the card with it before posting.
const provenanceCardMarker = "<!-- assay:provenance-card -->"

// liveCardRefusalReason is why every live-write attempt is refused today, regardless of flags.
// The design record DR-provenance-card's `decided-by:` field is still the
// "human:<name>" placeholder and the record itself says "PROPOSED — no ruling is recorded" —
// so nothing in the repo yet authorizes posting this card to a real, public pull request.
// A doc-only gate on a live-capable write path is not a control (the finding this refusal
// closes); refusing unconditionally in code is. Delete this constant and its one call site in
// provenance(), re-enabling the upsertLiveCard call it currently guards, in the SAME change that
// records the ruling on the DR — never before, and never by loosening this message instead.
const liveCardRefusalReason = "refusing to post: the design record DR-provenance-card " +
	"is still PROPOSED — no human ruling is recorded yet on whether this card may be posted " +
	"to a real pull request. Use --dry-run (or --fixture) to gather and print the card without " +
	"posting; this refusal holds even with --post until the DR is ratified."

// helpLong is the root help's prose and the usage a run with no target prints. The flags and
// their descriptions are generated from the command tree (desktools-v2/46).
const helpLong = `deskprovenance — gather the contributor provenance signals for a pull request and
print or post the resulting card. Facts only: no score, no rating, no verdict.

  deskprovenance --dry-run --fixture <path.json>
  deskprovenance --dry-run --repo <owner/name> --pr <N>
  deskprovenance --repo <owner/name> --pr <N> [--role <desk-role>]
  deskprovenance --repo <owner/name> --pr <N> --post [--role <desk-role>]
  deskprovenance --version

Naming a target with --repo/--pr alone never writes: without --post a live run only gathers
and prints the card, same as --dry-run. A live run with --post is currently refused
unconditionally — see the package doc comment. --fixture requires --dry-run: fixture data is
never posted to a real pull request.

Exit: 0 clean · 1 flagged · 6 could-not-check · 2 usage error.`

func main() {
	os.Exit(deskkit.PerfRun("deskprovenance", nil, os.Args[1:], func(a []string) int {
		return run(a)
	}))
}

// run is one invocation on the process streams.
func run(args []string) int {
	return execute(args, os.Stdout, os.Stderr)
}

// provOpts is everything a run's command line says.
type provOpts struct {
	dryRun   bool
	fixture  string
	repo     string
	pr       int
	post     bool
	role     string
	override string
}

// provenanceFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var provenanceFn = provenance

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// gatherFn reads a live pull request's signals; the contract tests replace it so the posting
// gates are exercised on the real handler without reaching the forge.
var gatherFn = gatherLive

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == deskkit.ExitOK {
		return nil
	}
	return exitErr(code)
}

// versionLine is what --version and -version print.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskprovenance sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// bindings are every setting deskprovenance declares: flags and defaults only, no environment
// variable and no configuration file. The scan-override flag is the one every outbound verb
// declares (deskkit.ScanOverrideFlag), applied by provenance exactly as
// deskkit.RegisterOutboundOverride applied it.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run", Default: false, Usage: "gather and print the card; post nothing"},
		{Key: "fixture", Kind: cli.String, Flag: "fixture",
			Usage: "load a JSON deskkit.ProvenanceInput from `path` instead of reading the forge (requires --dry-run)"},
		{Key: "repo", Kind: cli.String, Flag: "repo", Usage: "the target repository `owner/name` (required unless --fixture is given)"},
		{Key: "pr", Kind: cli.Int, Flag: "pr", Default: 0, Usage: "the pull request number `N` (required unless --fixture is given)"},
		{Key: "post", Kind: cli.Bool, Flag: "post", Default: false,
			Usage: "required, IN ADDITION to omitting --dry-run, to post/edit a live comment on a real pull request; currently refused unconditionally"},
		{Key: "role", Kind: cli.String, Flag: "role", Default: "worker", Usage: "the desk role identity to post as on a live run"},
		{Key: deskkit.ScanOverrideFlag, Kind: cli.String, Flag: deskkit.ScanOverrideFlag, Usage: deskkit.OutboundOverrideHelp},
	}
}

// execute runs one invocation against a fresh command tree.
func execute(args []string, stdout, stderr io.Writer) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("deskprovenance", "gather the contributor provenance signals for a pull request and print the card")
		root.Long = helpLong
		set := cli.Declare(root, bindings()...)
		// deskprovenance takes no positional argument; a stray word is refused before anything
		// runs, never silently ignored.
		root.Args = func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q — deskprovenance takes flags only", args[0])
			}
			return nil
		}
		root.RunE = func(*cobra.Command, []string) error {
			v, err := set.Resolve(cli.Inputs{})
			if err != nil {
				fmt.Fprintf(stderr, "deskprovenance: %v\n", err)
				return exitErr(2)
			}
			if onResolve != nil {
				onResolve(v)
			}
			return codeErr(provenanceFn(provOpts{
				dryRun:   v.Bool("dry-run"),
				fixture:  v.String("fixture"),
				repo:     v.String("repo"),
				pr:       v.Int("pr"),
				post:     v.Bool("post"),
				role:     v.String("role"),
				override: v.String(deskkit.ScanOverrideFlag),
			}, stdout, stderr))
		}
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A parse error keeps the legacy usage-error code (2).
		UsageExit: 2,
		// The flags were Go flag-package flags; the single-dash spellings stay accepted.
		GoFlagCompat: true,
	})
}

// provenance is one run from parsed options: the outbound context, the target checks, the
// gather, then the two posting gates.
func provenance(o provOpts, stdout, stderr io.Writer) int {
	// The scan-override reason, applied as deskkit.RegisterOutboundOverride applies it: a
	// non-blank reason must validate, and the outbound context is set either way.
	if strings.TrimSpace(o.override) != "" {
		if err := deskkit.ValidateScanOverride(o.override); err != nil {
			fmt.Fprintln(stderr, "deskprovenance:", err)
			return deskkit.ExitCodeOf(err)
		}
	}
	deskkit.SetOutboundContext(deskkit.OutboundContext{Tool: "deskprovenance", Verb: "post", OverrideReason: o.override})

	if o.fixture == "" && o.repo == "" && o.pr == 0 {
		fmt.Fprintln(stderr, helpLong)
		return 2
	}

	if o.fixture != "" && !o.dryRun {
		fmt.Fprintln(stderr, "deskprovenance: --fixture requires --dry-run — fixture data is never posted to a real pull request")
		return 2
	}

	var (
		in  deskkit.ProvenanceInput
		err error
	)
	switch {
	case o.fixture != "":
		in, err = loadFixture(o.fixture)
	case o.repo != "" && o.pr > 0:
		in, err = gatherFn(o.repo, o.pr, o.role)
	default:
		fmt.Fprintln(stderr, "deskprovenance: --repo and --pr are both required for a live run (or use --fixture)")
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "deskprovenance: %s\n", err)
		return 2
	}

	results := deskkit.Gather(in)
	overall := deskkit.Overall(results)
	card := deskkit.Card(results)

	// Gate 1: naming a live target (--repo/--pr) never by itself writes. --dry-run prints and
	// stops, as always; a live run that omits --post ALSO only prints — --post is a separate,
	// explicit opt-in to write, never inferred from the flags that merely say where to look.
	if o.dryRun || !o.post {
		if !o.dryRun {
			fmt.Fprintln(stderr, "deskprovenance: --post was not given — gathering and printing the card without posting (pass --post to write it; see --help)")
		}
		fmt.Fprint(stdout, card)
		return overall.ExitCode()
	}

	// Gate 2: independent of gate 1, refuse the write outright until the human ruling this
	// tool's own docs describe as pending is actually recorded. See liveCardRefusalReason —
	// this is the line to remove, in the same change that records the ruling, to re-enable
	// the upsertLiveCard call it currently guards.
	fmt.Fprintln(stderr, "deskprovenance: "+liveCardRefusalReason)
	return 2
}

// loadFixture reads a JSON-encoded deskkit.ProvenanceInput from path. A field the fixture
// omits decodes to nil, which is exactly "could not be read" — no separate sentinel is
// needed for that state.
func loadFixture(path string) (deskkit.ProvenanceInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return deskkit.ProvenanceInput{}, fmt.Errorf("reading fixture %s: %w", path, err)
	}
	var in deskkit.ProvenanceInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return deskkit.ProvenanceInput{}, fmt.Errorf("parsing fixture %s: %w", path, err)
	}
	return in, nil
}

// gatherLive reads what today's Forge interface exposes about repo/pr: the PR's own body and
// its changed file paths. Every other signal's input is left nil — genuinely could-not-check,
// see the package doc comment's "KNOWN SCOPE BOUNDARY" — rather than approximated.
func gatherLive(repoSlug string, prNumber int, role string) (deskkit.ProvenanceInput, error) {
	fr, err := parseRepoSlug(repoSlug)
	if err != nil {
		return deskkit.ProvenanceInput{}, err
	}
	forge, err := deskkit.ForgeFor(fr, role)
	if err != nil {
		return deskkit.ProvenanceInput{}, fmt.Errorf("resolving forge for %s: %w", repoSlug, err)
	}
	pull, err := forge.GetPullRequest(fr, prNumber)
	if err != nil {
		return deskkit.ProvenanceInput{}, fmt.Errorf("reading %s#%d: %w", repoSlug, prNumber, err)
	}
	changed, err := forge.ListChangedFiles(fr, prNumber)
	if err != nil {
		return deskkit.ProvenanceInput{}, fmt.Errorf("reading changed files for %s#%d: %w", repoSlug, prNumber, err)
	}
	var paths []string
	for _, f := range changed {
		paths = append(paths, f.Filename)
	}
	body := pull.Body
	return deskkit.ProvenanceInput{
		ThisBody:     &body,
		ChangedPaths: &paths,
		// AccountCreatedAt, FirstActivityAt, ForkCreatedAt, PROpenedAt, CrossRepoPRCount24h,
		// PriorMergedCount, PriorClosedCount, RecentBodies, CommitsSigned: left nil. See the
		// package doc comment.
	}, nil
}

// upsertLiveCard finds the newest comment on repoSlug#prNumber carrying provenanceCardMarker
// as one of its own lines and replaces its body, or posts body as a new comment when no such
// comment exists — so one pull request carries at most one card.
//
// Not called from provenance() today: gate 2 (liveCardRefusalReason) refuses every live write before
// reaching this function. Kept, rather than deleted, because the refusal is meant to lift —
// removing this function along with the refusal would just mean rewriting it later.
func upsertLiveCard(repoSlug string, prNumber int, role, body string) error {
	fr, err := parseRepoSlug(repoSlug)
	if err != nil {
		return err
	}
	forge, err := deskkit.ForgeFor(fr, role)
	if err != nil {
		return fmt.Errorf("resolving forge for %s: %w", repoSlug, err)
	}
	comments, err := forge.ListComments(fr, prNumber)
	if err != nil {
		return fmt.Errorf("listing comments on %s#%d: %w", repoSlug, prNumber, err)
	}
	for i := len(comments) - 1; i >= 0; i-- {
		if hasMarkerLine(comments[i].Body, provenanceCardMarker) {
			return forge.EditComment(fr, comments[i].ID, body)
		}
	}
	_, err = forge.PostComment(fr, prNumber, body)
	return err
}

// hasMarkerLine reports whether marker appears as one exact, whole line of body (trimmed of
// surrounding whitespace) — never merely as a substring, so a comment that only mentions the
// marker in prose is not mistaken for a card this tool posted.
func hasMarkerLine(body, marker string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == marker {
			return true
		}
	}
	return false
}

func parseRepoSlug(slug string) (deskkit.ForgeRepo, error) {
	parts := strings.SplitN(slug, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return deskkit.ForgeRepo{}, fmt.Errorf("%q is not a valid owner/name repository slug", slug)
	}
	return deskkit.ForgeRepo{Owner: parts[0], Name: parts[1]}, nil
}
