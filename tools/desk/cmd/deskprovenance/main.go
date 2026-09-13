// Command deskprovenance gathers the contributor provenance signals
// (internal/deskkit/provenance.go) for one pull request and either prints the resulting
// card (--dry-run) or upserts it as a comment on that pull request.
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
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// provenanceCardMarker is the exact-match line every card this tool posts carries, so a
// re-run finds and replaces its OWN previous card rather than appending a second one. It is
// never rendered by deskkit.Card itself (that function has no forge/CLI concerns at all);
// this command wraps the card with it before posting.
const provenanceCardMarker = "<!-- assay:provenance-card -->"

const usageText = `deskprovenance — gather the contributor provenance signals for a pull request and
print or post the resulting card. Facts only: no score, no rating, no verdict.

USAGE:
  deskprovenance --dry-run --fixture <path.json>
  deskprovenance --dry-run --repo <owner/name> --pr <N>
  deskprovenance --repo <owner/name> --pr <N> [--role <desk-role>]
  deskprovenance --version

  --dry-run          gather and print the card; post nothing.
  --fixture <path>   load a JSON deskkit.ProvenanceInput instead of reading the forge.
                      Requires --dry-run: fixture data is never posted to a real PR.
  --repo <owner/name> the target repository (required unless --fixture is given).
  --pr <N>           the pull request number (required unless --fixture is given).
  --role <name>      the desk role identity to post as on a live run (default "worker").

Exit: 0 clean · 1 flagged · 6 could-not-check · 2 usage error.`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("deskprovenance", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // usageText below is our own, printed on error ourselves
	dryRun := fs.Bool("dry-run", false, "gather and print the card; post nothing")
	fixture := fs.String("fixture", "", "path to a JSON deskkit.ProvenanceInput fixture")
	repo := fs.String("repo", "", "owner/name of the target repository")
	pr := fs.Int("pr", 0, "pull request number")
	role := fs.String("role", "worker", "desk role identity to post as (live run only)")
	versionFlag := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, usageText)
		return 2
	}

	if *versionFlag {
		sha, built := deskkit.Version()
		fmt.Printf("deskprovenance sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}

	if *fixture == "" && *repo == "" && *pr == 0 {
		fmt.Fprintln(os.Stderr, usageText)
		return 2
	}

	if *fixture != "" && !*dryRun {
		fmt.Fprintln(os.Stderr, "deskprovenance: --fixture requires --dry-run — fixture data is never posted to a real pull request")
		return 2
	}

	var (
		in  deskkit.ProvenanceInput
		err error
	)
	switch {
	case *fixture != "":
		in, err = loadFixture(*fixture)
	case *repo != "" && *pr > 0:
		in, err = gatherLive(*repo, *pr, *role)
	default:
		fmt.Fprintln(os.Stderr, "deskprovenance: --repo and --pr are both required for a live run (or use --fixture)")
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "deskprovenance: %s\n", err)
		return 2
	}

	results := deskkit.Gather(in)
	overall := deskkit.Overall(results)
	card := deskkit.Card(results)

	if *dryRun {
		fmt.Print(card)
		return overall.ExitCode()
	}

	if err := upsertLiveCard(*repo, *pr, *role, provenanceCardMarker+"\n"+card); err != nil {
		fmt.Fprintf(os.Stderr, "deskprovenance: %s\n", err)
		return 2
	}
	fmt.Print(card)
	return overall.ExitCode()
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
