package main

// deliverable.go — CROSS-REPO DISPATCH: which repo an item's deliverable lands in, resolved
// through the alias registry and nothing else.
//
// THE DEFECT CLASS IT CLOSES. A brief is TRACKED in one repo (its file, its board row) while its
// DELIVERABLE may land in a sibling repo. Every lane used to assume the two were the same checkout,
// so a brief whose code lives elsewhere was dispatched into the tracking checkout: the worker got a
// worktree that did not contain the files it had to change, and either stalled or recreated the
// work where nobody asked for it. Warnings did not help — the tool said something looked off and
// then cut the worktree anyway.
//
// THE RULE. The alias registry (docs/streams/graph-repos.yaml, schema graph-repos-v1) is the ONLY
// resolution path. An item names its deliverable repo by ALIAS — never by a guess from a repo name's
// resemblance to an alias — through one of three declarations:
//
//   - the brief's `deliverable_repo: <alias>` frontmatter field (the topology contract's field);
//   - the brief's `homed-in: <owner>/<name>` frontmatter field — accepted only when that repo is
//     REGISTERED under some alias, so it resolves through the same registry;
//   - an alias prefix on the item key, `<alias>:<stream>/<NN>` — the alias names the repo the brief
//     is TRACKED in, and with no deliverable declared the deliverable is that same repo.
//
// When none of the three is present nothing here runs, and the dispatch behaves exactly as before.
// When any is present:
//
//   - no registry at the registry root → could-not-check (exit 6), never a guess;
//   - an alias the registry does not define → REFUSAL (exit 5) naming the missing alias;
//   - an alias the registry reserves but does not publish (`repo: null`) → could-not-check (exit 6):
//     point --claim-root at a checkout whose registry copy names it;
//   - the resolved repo differs from --repo, or from --root's own origin → HARD FAIL (exit 5)
//     naming the alias, the resolved repo and the checkout's actual repo. It runs before the claim,
//     so no claim is taken and no worktree is cut.
//
// WHERE THE REGISTRY IS READ. Under --claim-root when given, else --root. --claim-root is the
// tracking checkout — the one that carries the brief, its board and the full registry copy — and it
// is already authoritative for the claim tool; the registry follows it for the same reason. The
// deliverable repo's own checkout (--root) stays the worktree source.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// registryRel is the alias registry's path under a stream root.
const registryRel = "docs/streams/graph-repos.yaml"

// registrySchema is the only registry schema this verb reads.
const registrySchema = "graph-repos-v1"

// aliasRe bounds a registry alias as it may appear as an item-key prefix. It deliberately admits
// no `/`, so the repo-qualified plan-key spelling `<owner>/<name>:…` is never read as an alias.
var aliasRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// itemAliasRe splits `<alias>:<rest>` off an item key.
var itemAliasRe = regexp.MustCompile(`^([a-z][a-z0-9_-]{0,31}):(.+)$`)

// splitItemAlias separates an `<alias>:` prefix from an item key. A key with no prefix returns
// ("", item) unchanged. The remainder is validated by the ordinary item-key grammar afterwards, so
// a hostile remainder (`x:../escape`, `x:--flag`) is refused exactly as a bare one is.
func splitItemAlias(item string) (alias, rest string) {
	m := itemAliasRe.FindStringSubmatch(item)
	if m == nil {
		return "", item
	}
	return m[1], m[2]
}

type registryEntry struct {
	Cell        string  `yaml:"cell"`
	Repo        *string `yaml:"repo"`
	Unpublished bool    `yaml:"unpublished"`
}

// aliasRegistry is the parsed graph-repos-v1 registry.
type aliasRegistry struct {
	path  string
	self  string
	repos map[string]registryEntry
}

// loadAliasRegistry reads the registry under root. ok=false (with a nil error) means the file is
// absent; a present-but-unreadable or malformed file is an error.
func loadAliasRegistry(root string) (*aliasRegistry, bool, error) {
	path := filepath.Join(root, filepath.FromSlash(registryRel))
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var doc struct {
		Schema string                   `yaml:"schema"`
		Self   string                   `yaml:"self"`
		Repos  map[string]registryEntry `yaml:"repos"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, false, fmt.Errorf("%s: %v", path, err)
	}
	if doc.Schema != registrySchema {
		return nil, false, fmt.Errorf("%s: schema must be %s, got %q", path, registrySchema, doc.Schema)
	}
	return &aliasRegistry{path: path, self: strings.TrimSpace(doc.Self), repos: doc.Repos}, true, nil
}

// repoOf returns the published owner/name for alias. known=false means the alias is not defined;
// known=true with an empty repo means the alias is reserved but unpublished in this copy.
func (r *aliasRegistry) repoOf(alias string) (repo string, known bool) {
	e, ok := r.repos[alias]
	if !ok {
		return "", false
	}
	if e.Repo == nil {
		return "", true
	}
	return strings.TrimSpace(*e.Repo), true
}

// aliasFor is the reverse lookup: the alias whose published repo is repo (case-insensitive).
func (r *aliasRegistry) aliasFor(repo string) (string, bool) {
	keys := make([]string, 0, len(r.repos))
	for k := range r.repos {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if got, _ := r.repoOf(k); got != "" && strings.EqualFold(got, repo) {
			return k, true
		}
	}
	return "", false
}

// deliverable is the resolved cross-repo shape of one dispatch. active=false means the item
// declared no alias at all and the legacy repo resolution applies unchanged.
type deliverable struct {
	active bool
	// alias / source: the deliverable alias and which declaration named it.
	alias  string
	source string
	// repo is the deliverable repo the alias resolves to — the claim, token, worktree and PR repo.
	repo string
	// homeAlias / trackingRepo: the alias the brief is TRACKED under (the item-key prefix, else the
	// registry's own `self`) and its published repo, "" when unknown or unpublished.
	homeAlias    string
	trackingRepo string
	// trackingRoot is the ABSOLUTE registry root — the checkout `deskpr create --root` must name so
	// the PR's `Brief:` trailer resolves against the tracking repo's board.
	trackingRoot string
	registryPath string
}

// crossRepo reports whether the deliverable lands in a repo other than the one tracking the brief.
// An unknown tracking repo with an active resolution is treated as cross-repo whenever the tracking
// root is not the deliverable checkout itself, so the prompt errs toward naming the tracking root.
func (d deliverable) crossRepo(root string) bool {
	if !d.active {
		return false
	}
	if d.trackingRepo != "" {
		return !strings.EqualFold(d.trackingRepo, d.repo)
	}
	return resolvePath(d.trackingRoot) != resolvePath(root)
}

var (
	briefDeliverableRe = regexp.MustCompile(`(?m)^deliverable_repo:\s*(.*)$`)
	briefHomedInRe     = regexp.MustCompile(`(?m)^homed-in:\s*(.*)$`)
)

// briefField returns a trimmed, unquoted frontmatter scalar (an inline `# comment` dropped).
func briefField(front string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(front)
	if m == nil {
		return ""
	}
	v := m[1]
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.Trim(strings.TrimSpace(v), `"'`)
}

// resolveBriefPath locates --brief: absolute as given; relative under --root first (the base the
// decision gate reads), then under --claim-root (a cross-repo brief lives in the tracking checkout).
// "" when it resolves nowhere.
func resolveBriefPath(o dispatchOpts) string {
	b := strings.TrimSpace(o.brief)
	if b == "" {
		return ""
	}
	if filepath.IsAbs(b) {
		if fileExists(b) {
			return b
		}
		return ""
	}
	for _, base := range []string{o.root, o.claimRoot} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		if p := filepath.Join(base, filepath.FromSlash(b)); fileExists(p) {
			return p
		}
	}
	return ""
}

// resolveDeliverable resolves the item's deliverable repo through the alias registry and checks it
// against the checkout the worktree would be cut from. It runs inside validateCallerPreconditions —
// before admission, the token mint, the claim and the worktree — so every refusal here costs
// nothing durable.
func resolveDeliverable(o dispatchOpts) (deliverable, error) {
	var d deliverable

	var declDeliverable, declHomedIn string
	if strings.TrimSpace(o.brief) != "" {
		if p := resolveBriefPath(o); p != "" {
			if raw, err := os.ReadFile(p); err == nil {
				front := briefFrontmatterBlock(string(raw))
				declDeliverable = briefField(front, briefDeliverableRe)
				declHomedIn = briefField(front, briefHomedInRe)
			}
		} else {
			// Could-not-check, reported as itself: the brief may declare a deliverable repo this
			// run cannot see. The dispatch is not refused (a --brief is also a best-effort input to
			// the decision gate), but the gap is printed rather than read as "same repo".
			fmt.Fprintf(os.Stderr, "deskdispatch: NOTICE — --brief %q did not resolve under --root or "+
				"--claim-root, so its deliverable_repo/homed-in declaration could not be read (could-not-check)\n",
				o.brief)
		}
	}
	if o.itemAlias == "" && declDeliverable == "" && declHomedIn == "" {
		return d, nil
	}
	d.active = true

	regRoot := o.root
	if s := strings.TrimSpace(o.claimRoot); s != "" {
		regRoot = s
	}
	if abs, err := filepath.Abs(regRoot); err == nil {
		regRoot = abs
	}
	d.trackingRoot = regRoot

	reg, ok, err := loadAliasRegistry(regRoot)
	if err != nil {
		return d, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the alias registry could not be read (%v) — the deliverable repo is resolved through it "+
				"and nothing else, so no repo is guessed. Nothing was claimed.", stepClaimAcquire, err), err)
	}
	if !ok {
		return d, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: this item declares a repo alias, but there is no alias registry at %s — it is the only "+
				"legal resolution path, so no repo is guessed. Point --claim-root at the tracking checkout that "+
				"carries %s. Nothing was claimed.",
			stepClaimAcquire, filepath.Join(regRoot, filepath.FromSlash(registryRel)), registryRel), nil)
	}
	d.registryPath = reg.path

	// The home (tracking) alias: the item-key prefix, else the registry's own `self`.
	d.homeAlias = o.itemAlias
	if d.homeAlias == "" {
		d.homeAlias = reg.self
	}
	if o.itemAlias != "" {
		if _, known := reg.repoOf(o.itemAlias); !known {
			return d, unregisteredAlias(o.itemAlias, "the item-key prefix", reg)
		}
	}
	if d.homeAlias != "" {
		d.trackingRepo, _ = reg.repoOf(d.homeAlias)
	}

	// The deliverable alias: deliverable_repo, else homed-in (reverse-resolved), else the home alias.
	switch {
	case declDeliverable != "":
		d.alias, d.source = declDeliverable, "the brief's deliverable_repo"
		if declHomedIn != "" {
			if got, _ := reg.repoOf(declDeliverable); got != "" && !strings.EqualFold(got, declHomedIn) {
				return d, deskkit.Refused(fmt.Sprintf(
					"step %s: the brief declares deliverable_repo %q (→ %s via %s) and homed-in %q — two "+
						"different deliverable repos. Fix the brief so both name one repo. Nothing was claimed.",
					stepClaimAcquire, declDeliverable, got, reg.path, declHomedIn))
			}
		}
	case declHomedIn != "":
		alias, found := reg.aliasFor(declHomedIn)
		if !found {
			return d, deskkit.Refused(fmt.Sprintf(
				"step %s: the brief is homed-in %q, but no alias in %s publishes that repo — the registry is "+
					"the only legal resolution path, so an unregistered repo is refused rather than trusted. "+
					"Register it (or declare deliverable_repo: <alias>). Nothing was claimed.",
				stepClaimAcquire, declHomedIn, reg.path))
		}
		d.alias, d.source = alias, "the brief's homed-in"
	default:
		d.alias, d.source = o.itemAlias, "the item-key prefix"
	}
	if !aliasRe.MatchString(d.alias) {
		return d, deskkit.Refused(fmt.Sprintf(
			"step %s: %s names %q, which is not a registry alias (lowercase letters, digits, dash, "+
				"underscore). Nothing was claimed.", stepClaimAcquire, d.source, d.alias))
	}
	repo, known := reg.repoOf(d.alias)
	if !known {
		return d, unregisteredAlias(d.alias, d.source, reg)
	}
	if repo == "" {
		return d, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: alias %q (from %s) is reserved in %s but its repo is not published there, so it cannot "+
				"be resolved from this registry copy (could-not-check). Point --claim-root at the tracking "+
				"checkout whose registry names it. Nothing was claimed.",
			stepClaimAcquire, d.alias, d.source, reg.path), nil)
	}
	d.repo = repo

	// HARD FAIL on a mismatch — the check the whole class turns on. Two witnesses, both pre-claim.
	if r := strings.TrimSpace(o.repo); r != "" && !strings.EqualFold(r, repo) {
		return d, mismatch(d, "--repo", r)
	}
	origin := runCmd(o.root, "git", "remote", "get-url", "origin")
	if origin.err != nil {
		return d, origin.run.FailVerbatim(deskkit.ExitUnverifiable, fmt.Sprintf(
			"step %s: alias %q resolves to %s, but %s's origin could not be read (%s), so whether --root is a "+
				"checkout of that repo cannot be established. Nothing was claimed.",
			stepClaimAcquire, d.alias, repo, o.root, origin.run.Said()))
	}
	actual := repoSlugFromURL(origin.stdout)
	if actual == "" {
		return d, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: alias %q resolves to %s, but --root's origin %q does not parse to an owner/name. "+
				"Nothing was claimed.", stepClaimAcquire, d.alias, repo, origin.stdout), nil)
	}
	if !strings.EqualFold(actual, repo) {
		return d, mismatch(d, "--root "+o.root, actual)
	}
	return d, nil
}

func unregisteredAlias(alias, source string, reg *aliasRegistry) error {
	keys := make([]string, 0, len(reg.repos))
	for k := range reg.repos {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return deskkit.Refused(fmt.Sprintf(
		"step %s: alias %q (from %s) is not registered in %s (registered: %s). The registry is the only "+
			"legal resolution path — an unregistered alias is refused, never resolved by resemblance. "+
			"Nothing was claimed and no worktree was cut.",
		stepClaimAcquire, alias, source, reg.path, strings.Join(keys, ", ")))
}

func mismatch(d deliverable, what, actual string) error {
	return deskkit.Refused(fmt.Sprintf(
		"step %s: HARD FAIL — alias %q (from %s) resolves to %s via %s, but %s is %s. The deliverable "+
			"lands in %s, so dispatching from this checkout would cut the worker's worktree in the wrong "+
			"repo. Nothing was claimed and no worktree was cut. Re-run with --root at a checkout of %s "+
			"(and --claim-root at the tracking checkout that carries the registry).",
		stepClaimAcquire, d.alias, d.source, d.repo, d.registryPath, what, actual, d.repo, d.repo))
}
