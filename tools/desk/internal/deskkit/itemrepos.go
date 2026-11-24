package deskkit

// itemrepos.go — the ONE answer to "which repository?" for a desk item (#6274).
//
// A brief can be TRACKED in one repository (its stream, board row, decision issue, Evidence
// and outcome record) and DELIVER into another (declared by `deliverable_repo:`, `homed-in:`
// or `[alias]` file tags). Each desk verb used to pick one of the two on its own, from
// `--repo`, `--root` or the checkout's origin, so a cross-repository item failed at whichever
// verb picked the other one: a decision issue read in the deliverable repository, an Evidence
// commit looked up in a store that never held it, a deliverable home that could not be found
// because it is not a stream root.
//
// This file holds the shared answer:
//
//   - ItemRepos names both repositories once, and each verb asks it for the ROLE it needs
//     (DecisionRepo, EvidenceRepo, OutcomeRepo, VerifierClaimRepo, HomeRepo). No verb
//     derives a role from `--repo` or `--root` alone.
//   - ResolveItemRepos builds it from the tracking checkout's immutable commit, through the
//     same declaration policy the verifier association already uses (one parser, not two).
//   - CheckoutForRepo answers where a repository's LOCAL checkout is, without requiring that
//     checkout to be a stream root, and proves the answer by the checkout's own origin.
//
// Every function here fails closed: an unreadable declaration, an ambiguous deliverable, an
// unconfigured or mis-identified checkout is a refusal (exit 5) or could-not-check (exit 6),
// never a silent fall-back to the other repository.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CheckoutsEnv names the environment variable that maps repositories which are NOT stream
// roots to their local checkouts, in DESK_ROOTS's grammar:
//
//	DESK_CHECKOUTS="example-org/agents=/src/agents,example-org/platform=/src/platform"
//
// DESK_ROOTS lists only repositories whose `docs/streams/` the board reads, and the tools that
// list the board refuse a root without one. A deliverable repository usually has no streams
// of its own, so it cannot sit there; it sits here instead. Like DESK_ROOTS, this variable
// can never widen the repository set: every entry must already be an allowed repository.
const CheckoutsEnv = "DESK_CHECKOUTS"

// ItemRepos is the tracking/deliverable pair for one item. Tracking holds the brief, its board
// row, its decision issue, its Evidence and its outcome record; Deliverable is where the code
// lands and where homes are cut. They are the same repository for ordinary work.
type ItemRepos struct {
	Tracking    string
	Deliverable string
}

// NewItemRepos validates and pairs the two repositories. An empty deliverable means the item
// delivers into the repository that tracks it.
func NewItemRepos(tracking, deliverable string) (ItemRepos, error) {
	tracking, deliverable = strings.TrimSpace(tracking), strings.TrimSpace(deliverable)
	if !ValidRepoSlug(tracking) {
		return ItemRepos{}, Refused("refused: the item's tracking repository is not an owner/name slug")
	}
	if deliverable == "" {
		deliverable = tracking
	}
	if !ValidRepoSlug(deliverable) {
		return ItemRepos{}, Refused("refused: the item's deliverable repository is not an owner/name slug")
	}
	return ItemRepos{Tracking: tracking, Deliverable: deliverable}, nil
}

// CrossRepo reports whether the item delivers into a repository other than the one tracking it.
func (r ItemRepos) CrossRepo() bool {
	return r.Deliverable != "" && !strings.EqualFold(r.Tracking, r.Deliverable)
}

// DecisionRepo is where the item's `gate: human` decision issue lives and is read: the
// tracking repository. A bare `decision-issue: #N` resolves here (#5525, #6494).
func (r ItemRepos) DecisionRepo() string { return r.Tracking }

// EvidenceRepo is where the brief's Evidence lands: the tracking repository.
func (r ItemRepos) EvidenceRepo() string { return r.Tracking }

// OutcomeRepo is where a verify outcome record lands and where the Evidence commit it binds
// to is read: the tracking repository.
func (r ItemRepos) OutcomeRepo() string { return r.Tracking }

// VerifierClaimRepo is where a verifier dispatch takes its claim: the tracking repository,
// whose verify queue names the item (#5999).
func (r ItemRepos) VerifierClaimRepo() string { return r.Tracking }

// HomeRepo is where code lands and homes are cut: the deliverable repository. A worker or
// reviewer dispatch also claims here.
func (r ItemRepos) HomeRepo() string { return r.Deliverable }

// ResolveItemRepos reads the brief at the tracking checkout's immutable revision and pairs the
// tracking repository with the ONE deliverable it declares. The declaration policy is the
// verifier association's (VerifierDeliverables): alias values come only from that commit's
// graph registry, `homed-in:` must name a registered repository, and conflicting or ambiguous
// declarations refuse.
func ResolveItemRepos(trackingRoot, tracking, revision, brief string) (ItemRepos, error) {
	repos, err := VerifierDeliverables(trackingRoot, revision, brief)
	if err != nil {
		return ItemRepos{}, err
	}
	switch len(repos) {
	case 0:
		return NewItemRepos(tracking, "")
	case 1:
		return NewItemRepos(tracking, repos[0])
	}
	return ItemRepos{}, Unverifiable("the brief declares more than one deliverable repository, so its home is ambiguous", nil)
}

// Checkout is a resolved local checkout and the configuration that named it.
type Checkout struct {
	Repo   string // owner/name, as requested
	Path   string // as configured (DESK_ROOTS / DESK_CHECKOUTS), or the absolute sibling path
	Source string // RootsEnv, "compiled roots", CheckoutsEnv, or "sibling checkout"
}

// CheckoutForRepo resolves the local checkout of repo, which need NOT be a stream root.
// The order is explicit configuration first, then the sibling-checkout convention:
//
//  1. a stream root (DESK_ROOTS, else the compiled roots) for repo;
//  2. a DESK_CHECKOUTS entry for repo;
//  3. when near is set, a directory named after repo beside near (the convention the compiled
//     roots already use: a repository's siblings sit next to it).
//
// Slugs match case-insensitively. repo must be in the allowed set. The first candidate that
// exists is PROVED by its own origin: a configured path whose origin names another repository
// is refused, never skipped for the next candidate (a mis-mapped checkout is a configuration
// error to fix, not a reason to guess). Nothing found is could-not-check, naming both variables.
func CheckoutForRepo(repo, near string) (Checkout, error) {
	repo = strings.TrimSpace(repo)
	if !ValidRepoSlug(repo) {
		return Checkout{}, Refused("refused: " + repo + " is not an owner/name slug")
	}
	if !allowedRepoFold(repo) {
		return Checkout{}, Refused("refused: " + repo + " is outside the configured repository set")
	}
	roots, err := ConfiguredRoots()
	if err != nil {
		return Checkout{}, err
	}
	rootsSource := RootsEnv
	if strings.TrimSpace(os.Getenv(RootsEnv)) == "" {
		rootsSource = "compiled roots"
	}
	for _, r := range roots {
		if strings.EqualFold(r.Repo, repo) {
			return provedCheckout(Checkout{Repo: repo, Path: r.Path, Source: rootsSource})
		}
	}
	extra, err := configuredCheckouts()
	if err != nil {
		return Checkout{}, err
	}
	for _, r := range extra {
		if strings.EqualFold(r.Repo, repo) {
			return provedCheckout(Checkout{Repo: repo, Path: r.Path, Source: CheckoutsEnv})
		}
	}
	if strings.TrimSpace(near) != "" {
		if abs, aerr := filepath.Abs(near); aerr == nil {
			_, name := splitRepo(repo)
			sib := filepath.Join(filepath.Dir(abs), name)
			if info, serr := os.Stat(sib); serr == nil && info.IsDir() && sib != abs {
				return provedCheckout(Checkout{Repo: repo, Path: sib, Source: "sibling checkout"})
			}
		}
	}
	return Checkout{}, Unverifiable("no local checkout is configured for "+repo+
		" (set "+CheckoutsEnv+" or "+RootsEnv+" to <owner>/<repo>=<path>)", nil)
}

// provedCheckout admits c only when it is a readable directory whose origin is c.Repo.
func provedCheckout(c Checkout) (Checkout, error) {
	info, err := os.Stat(c.Path)
	if err != nil || !info.IsDir() {
		return Checkout{}, Unverifiable("the "+c.Source+" checkout for "+c.Repo+" is not a readable directory", err)
	}
	origin, err := verifierOrigin(c.Path)
	if err != nil {
		return Checkout{}, Unverifiable("cannot read the origin of the "+c.Source+" checkout for "+c.Repo, err)
	}
	if !strings.EqualFold(origin, c.Repo) {
		return Checkout{}, Refused("refused: the " + c.Source + " checkout for " + c.Repo +
			" has the wrong origin (" + origin + ")")
	}
	return c, nil
}

// configuredCheckouts parses DESK_CHECKOUTS with ConfiguredRoots's rules: a malformed entry,
// a repository outside the allowed set, or a repeated repository refuses the whole value.
func configuredCheckouts() ([]RootConfig, error) {
	raw := strings.TrimSpace(os.Getenv(CheckoutsEnv))
	if raw == "" {
		return nil, nil
	}
	var out []RootConfig
	seen := map[string]bool{}
	for _, spec := range strings.Split(raw, ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		repo, path, ok := strings.Cut(spec, "=")
		repo, path = strings.TrimSpace(repo), strings.TrimSpace(path)
		if !ok || repo == "" || path == "" {
			return nil, Refused("refused: " + CheckoutsEnv + " entry " + spec + " is not <owner>/<repo>=<path>")
		}
		if !allowedRepoFold(repo) {
			return nil, Refused("refused: " + CheckoutsEnv + " names repo " + repo +
				" which is outside the fixed set — the repo set cannot be widened by env")
		}
		key := strings.ToLower(repo)
		if seen[key] {
			return nil, Refused("refused: " + CheckoutsEnv + " names repo " + repo + " twice")
		}
		seen[key] = true
		out = append(out, RootConfig{Repo: repo, Path: path})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })
	return out, nil
}

// allowedRepoFold is IsAllowedRepo with forge slug case folding: a declaration spelled
// Example-Org/One names the allowed example-org/one. It never admits a repository the
// allowed set (or its owner patterns) would not.
func allowedRepoFold(repo string) bool {
	if IsAllowedRepo(repo) {
		return true
	}
	for allowed := range allowedRepos() {
		if strings.EqualFold(allowed, repo) {
			return true
		}
	}
	return false
}
