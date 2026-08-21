// Command upgrade-assay is the adopter-facing upgrade verb: it
// moves an adopter from the umbrella version they are on to either the latest
// stable umbrella or a named one, previews the change (dry-run by default in the
// skill workflow), runs whatever migrations the step implies, and shows the
// release notes for the span. It is the one entry point the `assay:upgrade-assay`
// skill drives.
//
// IT DRIVES THE VERSION MARKER AND MIGRATION RUNNER; IT DOES NOT REIMPLEMENT THEM.
// The "what version am I on" question is answered by the version marker
// (deskkit.ReadMarker, the code half of `deskversion`); the migrations are
// selected and run by the migration runner
// (deskkit.{LoadMigrations,SelectMigrations,RunMigrations}, the code half of
// `deskmigrate`). This command composes those existing functions into one flow —
// it never opens a second version resolver, which is exactly the divergence the
// pinned-distribution model exists to remove.
//
// TWO VERBS, NEVER MORE:
//
//   - move to LATEST STABLE — `--to` omitted: the highest published bare `vX.Y.Z`
//     umbrella release the adopter has materialised under `<root>/releases`.
//   - move to a NAMED umbrella version — `--to vX.Y.Z`: a bare umbrella tag.
//
// A per-artifact tag (`statusgen/v0.13.0`) is NOT a resolvable target — the verb
// moves the whole umbrella, never a single artifact — and is refused, not
// silently accommodated: the resolver resolves umbrella versions only.
//
// THREE MARKER STATES ARE THE SPINE. The marker's answer decides everything:
//
//   - could-not-determine → REFUSE (exit 6). Never "assume latest": a migration
//     run against a guessed from-version corrupts state rather than failing.
//   - known-inconsistent  → REFUSE (exit 5), naming the records that disagree.
//   - known               → proceed.
//
// THERE IS NO ROLLBACK. The platform has no downgrade verb and prunes cached
// prior versions after ~14 days, so "move to an older named version" is a
// re-point and re-resolve, not a rollback — and an artifact older than roughly
// two weeks may simply be unavailable. Nothing this tool prints promises a
// rollback, atomicity, or tamper-proofing the mechanism does not deliver —
// publish the honest weaker claim.
//
// LOCAL-ONLY. It reads and (on apply) writes files under `--root` — the adopter's
// own repository. It NEVER pushes, triggers a workflow, cuts a tag, edits the
// platform install cache, or runs mutating infra. Re-pointing the marketplace ref
// (`/plugin`) is the adopter's own step; this tool PRINTS the command it would run
// and never executes it.
//
// USAGE:
//
//	upgrade-assay --root <adopter-repo> [--to vX.Y.Z] [--dry-run] [--releases <dir>]
//	upgrade-assay --version
//
// EXIT CODES — each refusal is a first-class outcome with a distinct code, never
// generic error text:
//
//	0  applied, previewed (dry-run), or an idempotent "already at" no-op
//	5  refused: the marker's records are known-inconsistent (they disagree)
//	6  refused: the current umbrella version could not be determined
//	7  refused: the target is not a bare umbrella version (a per-artifact tag,
//	            or a malformed version) — the verb moves the whole umbrella
//	8  refused: the target names no published umbrella release (unsupported /
//	            could-not — never a nearest-match guess)
//	9  refused: the target resolves but its artifacts are unavailable (an older
//	            release pruned from the cache and no longer fetchable)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"gopkg.in/yaml.v3"
)

// upgrade-assay exit codes — the refusal spine, one distinct code per outcome so a
// caller can branch on the process result alone. 0/5/6 reuse the deskkit contract
// (ExitOK / ExitRefused / ExitUnverifiable); 7/8/9 are this verb's own refusal
// kinds, following deskpins' precedent of local validator codes above the shared
// set.
const (
	exitOK               = deskkit.ExitOK           // 0
	exitInconsistent     = deskkit.ExitRefused      // 5  marker known-inconsistent
	exitUndetermined     = deskkit.ExitUnverifiable // 6  marker could-not-determine
	exitWrongVersionKind = 7                        // 7  not a bare umbrella version
	exitUnknownTarget    = 8                        // 8  no such published umbrella
	exitArtifactsGone    = 9                        // 9  target resolves, artifacts unavailable
)

const usage = `upgrade-assay — move an adopter to latest-stable or a named umbrella version.

usage:
  upgrade-assay --root <adopter-repo> [--to vX.Y.Z] [--dry-run] [--releases <dir>]
  upgrade-assay --version

verbs:
  --to omitted     move to LATEST STABLE (highest published umbrella under <releases>)
  --to vX.Y.Z      move to a NAMED bare umbrella version

  A per-artifact tag (e.g. statusgen/v0.13.0) is NOT a target: the verb moves the
  whole umbrella, never one artifact. Bare umbrella versions only.

exit: 0 ok/dry-run/already-at · 5 inconsistent · 6 undetermined ·
      7 not-a-bare-umbrella · 8 no-such-release · 9 artifacts-unavailable

There is no rollback: the platform has no downgrade verb and prunes cached prior
versions after ~14 days. Moving to an older named version re-points and
re-resolves; it is not a rollback, and an artifact older than ~2 weeks may be
unavailable.`

// bareUmbrella is the grammar of a resolvable target: a STRICT bare vMAJOR.MINOR.PATCH
// (no leading zeros), with no `<component>/` prefix. The prefix is what separates a
// per-artifact tag (refused) from an umbrella version (resolvable).
var bareUmbrella = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade-assay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		root     = fs.String("root", ".", "adopter repo root holding .assay-versions")
		to       = fs.String("to", "", "target bare umbrella version vX.Y.Z (omit for latest stable)")
		dryRun   = fs.Bool("dry-run", false, "preview only; write nothing")
		releases = fs.String("releases", "", "materialised composition-manifest dir (default <root>/releases)")
		version  = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return exitWrongVersionKind
	}

	if *version {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "upgrade-assay sourceSHA=%s builtAt=%s releaseTag=%s\n",
			sha, built, deskkit.ReleaseTagOrDev())
		return exitOK
	}

	relDir := *releases
	if relDir == "" {
		relDir = filepath.Join(*root, deskkit.ReleasesDir)
	}

	// ── 1. Target KIND — checked BEFORE the marker, so a per-artifact tag or a
	// malformed version is refused as a wrong-kind input regardless of what state
	// the adopter's records happen to be in. (If the marker ran first, a repo whose
	// records are inconsistent would mask this refusal behind exit 5.)
	target := *to
	if target != "" {
		if strings.Contains(target, "/") {
			fmt.Fprintf(stderr,
				"upgrade-assay: refusing — %q is a per-artifact tag, not an umbrella version.\n"+
					"This verb moves the whole umbrella, never a single artifact. Name a bare\n"+
					"umbrella version instead, e.g. `--to v0.13.0`.\n", target)
			return exitWrongVersionKind
		}
		if !bareUmbrella.MatchString(target) {
			fmt.Fprintf(stderr,
				"upgrade-assay: refusing — %q is not a bare umbrella version (want strict vX.Y.Z,\n"+
					"e.g. `--to v0.13.0`). Only bare umbrella versions resolve.\n", target)
			return exitWrongVersionKind
		}
	}

	// ── 2. Discover the published umbrella set the adopter has materialised — the
	// composition manifests under <releases>. This is the offline stand-in for "what
	// the release home has published": a manifest is present iff the umbrella was
	// materialised for this repo.
	published, err := discoverPublished(relDir)
	if err != nil {
		fmt.Fprintf(stderr, "upgrade-assay: refusing — cannot read release manifests under %s: %v\n", relDir, err)
		return exitUnknownTarget
	}

	// ── 3. Resolve the target: latest stable, or the named one — and prove it is a
	// published umbrella. An unpublished bare version is an explicit unsupported /
	// could-not state, NEVER a nearest-match guess.
	if target == "" {
		latest, ok := latestOf(published)
		if !ok {
			fmt.Fprintf(stderr, "upgrade-assay: refusing — no published umbrella release is available under %s to resolve as latest stable.\n", relDir)
			return exitUnknownTarget
		}
		target = latest
	} else if !published[target] {
		fmt.Fprintf(stderr,
			"upgrade-assay: refusing — %s names no published umbrella release under %s.\n"+
				"This is unsupported / could-not-resolve: the tool refuses rather than guessing\n"+
				"a neighbouring version. Name a published umbrella version.\n", target, relDir)
		return exitUnknownTarget
	}

	// ── 4. Resolve FROM via the version marker — the three-state spine. deskkit.ReadMarker
	// IS the version marker (the code half of `deskversion`); this drives it,
	// never a second resolver.
	marker := deskkit.ReadMarker(*root, relDir)
	switch marker.State {
	case deskkit.MarkerCouldNotDetermine:
		fmt.Fprint(stdout, marker.Report())
		fmt.Fprintln(stderr, "upgrade-assay: refusing — the current umbrella version could not be determined; not moving anything (no version is guessed).")
		return exitUndetermined
	case deskkit.MarkerInconsistent:
		fmt.Fprint(stdout, marker.Report())
		fmt.Fprintln(stderr, "upgrade-assay: refusing — the adopter's records are inconsistent (named above); resolve the disagreement before upgrading.")
		return exitInconsistent
	}
	from := marker.Umbrella

	// ── 5. Idempotent no-op: already at the target.
	if from == target {
		fmt.Fprintf(stdout, "upgrade-assay: already at %s — nothing to do (no migrations re-run, nothing re-pinned).\n", target)
		return exitOK
	}

	// Load the two compositions (both are published; the marker already proved the
	// `from` manifest readable). deskkit.LoadComposition is the shared reader — the
	// authoritative artifact/tag list, not a reimplementation.
	fromComp, err := deskkit.LoadComposition(relDir, from)
	if err != nil {
		fmt.Fprintf(stderr, "upgrade-assay: refusing — cannot read the current composition for %s: %v\n", from, err)
		return exitUndetermined
	}
	toComp, err := deskkit.LoadComposition(relDir, target)
	if err != nil {
		fmt.Fprintf(stderr, "upgrade-assay: refusing — cannot read the target composition for %s: %v\n", target, err)
		return exitArtifactsGone
	}

	// Direction: forward (from < target) runs migrations; a move to an OLDER umbrella
	// is a re-point + re-resolve, honestly NOT a rollback — no forward migration is
	// reversed.
	forward := cmpSemver(mustParse(from), mustParse(target)) < 0

	deltas := artifactDeltas(fromComp, toComp)

	var selected []deskkit.Migration
	if forward {
		migs, err := deskkit.LoadMigrations(filepath.Join(*root, deskkit.MigrationsDir))
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return deskkit.ExitCodeOf(err)
		}
		selected, err = deskkit.SelectMigrations(migs, from, target)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return deskkit.ExitCodeOf(err)
		}
	}

	if *dryRun {
		return dryRunReport(stdout, *root, from, target, forward, deltas, selected)
	}
	return apply(stdout, stderr, *root, relDir, from, target, forward, deltas, selected, toComp)
}

// dryRunReport prints exactly what an apply would do and writes NOTHING. The literal
// tokens "would run" and "release note" appear here so an adopter previews both the
// migrations and the notes for the span before consenting.
func dryRunReport(stdout io.Writer, root, from, target string, forward bool, deltas []delta, selected []deskkit.Migration) int {
	fmt.Fprintf(stdout, "upgrade-assay: DRY-RUN — would move umbrella %s -> %s (writing nothing).\n", from, target)
	if !forward {
		fmt.Fprintf(stdout, "note: %s is BELOW the current %s — this would re-point and re-resolve, which is NOT a rollback; forward migrations are not reversed.\n", target, from)
	}
	printDeltas(stdout, deltas)

	if len(selected) == 0 {
		fmt.Fprintln(stdout, "migrations that would run: none for this span (a clean no-op).")
	} else {
		fmt.Fprintf(stdout, "migrations that would run (%d):\n", len(selected))
		actions, _ := deskkit.RunMigrations(root, selected, true)
		for _, m := range selected {
			fmt.Fprintf(stdout, "  - %s (%s -> %s)\n", m.ID, m.From, m.To)
		}
		for _, a := range actions {
			fmt.Fprintf(stdout, "    [%s] %s\n", a.Migration, a.Desc)
		}
	}

	printReleaseNotes(stdout, from, target, selected)
	printReresolve(stdout, target)
	return exitOK
}

// apply performs the move for real against the adopter repo: re-pin .assay-versions,
// run the migrations, then print the marketplace re-resolve command the adopter runs
// (this tool never executes it).
func apply(stdout, stderr io.Writer, root, relDir, from, target string, forward bool, deltas []delta, selected []deskkit.Migration, toComp deskkit.Composition) int {
	fmt.Fprintf(stdout, "upgrade-assay: moving umbrella %s -> %s.\n", from, target)
	if !forward {
		fmt.Fprintf(stdout, "note: %s is BELOW the current %s — re-pointing and re-resolving, which is NOT a rollback; forward migrations are not reversed.\n", target, from)
	}
	printDeltas(stdout, deltas)

	if err := repin(root, relDir, target, toComp); err != nil {
		fmt.Fprintf(stderr, "upgrade-assay: refusing — could not re-pin .assay-versions: %v\n", err)
		return exitArtifactsGone
	}
	fmt.Fprintf(stdout, "re-pinned %s to umbrella %s and its artifact tags.\n", deskkit.AssayVersionsFile, target)

	if len(selected) == 0 {
		fmt.Fprintln(stdout, "migrations: none for this span (a clean no-op).")
	} else {
		actions, err := deskkit.RunMigrations(root, selected, false)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return deskkit.ExitCodeOf(err)
		}
		fmt.Fprintf(stdout, "migrations applied (%d):\n", len(selected))
		for _, a := range actions {
			fmt.Fprintf(stdout, "  [%s] %s\n", a.Migration, a.Desc)
		}
	}

	printReleaseNotes(stdout, from, target, selected)
	printReresolve(stdout, target)
	return exitOK
}

// printReleaseNotes surfaces the human "what changed" bodies of the migrations in
// the span — the release notes, sourced from the migration files themselves, never
// synthesised. Absent notes are reported as absent.
func printReleaseNotes(stdout io.Writer, from, target string, selected []deskkit.Migration) {
	fmt.Fprintf(stdout, "release notes for %s -> %s:\n", from, target)
	if len(selected) == 0 {
		fmt.Fprintln(stdout, "  (none recorded for this span — reported as absent, not synthesised.)")
		return
	}
	for _, m := range selected {
		fmt.Fprintf(stdout, "  --- %s (%s -> %s) ---\n", m.ID, m.From, m.To)
		notes := strings.TrimSpace(m.Notes)
		if notes == "" {
			fmt.Fprintln(stdout, "  (this migration records no `what changed` note — reported as absent.)")
			continue
		}
		for _, line := range strings.Split(notes, "\n") {
			fmt.Fprintf(stdout, "  %s\n", line)
		}
	}
}

// printReresolve prints the marketplace re-point command the adopter runs to
// re-resolve the plugin at the target ref. This tool NEVER runs it — re-pointing
// the marketplace is the adopter's own `/plugin` step.
func printReresolve(stdout io.Writer, target string) {
	fmt.Fprintln(stdout, "next (you run this — upgrade-assay never re-points the marketplace for you):")
	fmt.Fprintf(stdout, "  /plugin marketplace add medici-finance/assay@%s\n", target)
	fmt.Fprintln(stdout, "  /plugin update assay@assay")
}

// delta is one artifact's version change between the from- and to-compositions.
type delta struct {
	Artifact string
	FromTag  string
	ToTag    string
}

func artifactDeltas(fromComp, toComp deskkit.Composition) []delta {
	var out []delta
	for _, a := range toComp.Artifacts {
		name := a.ArtifactName()
		if name == "" {
			continue
		}
		fromTag, _ := fromComp.TagFor(name)
		out = append(out, delta{Artifact: name, FromTag: fromTag, ToTag: a.Tag})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Artifact < out[j].Artifact })
	return out
}

func printDeltas(stdout io.Writer, deltas []delta) {
	fmt.Fprintln(stdout, "artifact version deltas:")
	for _, d := range deltas {
		fromTag := d.FromTag
		if fromTag == "" {
			fromTag = "(not currently pinned)"
		}
		if d.FromTag == d.ToTag {
			fmt.Fprintf(stdout, "  - %s %s (unchanged)\n", d.Artifact, d.ToTag)
		} else {
			fmt.Fprintf(stdout, "  - %s %s -> %s\n", d.Artifact, fromTag, d.ToTag)
		}
	}
}

// ── target discovery / semver (target selection, NOT a from-resolver) ───────────

// discoverPublished returns the set of bare umbrella versions the adopter has
// materialised under releasesDir — one composition manifest per umbrella. A missing
// dir is an empty set, not an error (an adopter with no manifests simply cannot
// resolve any target).
func discoverPublished(releasesDir string) (map[string]bool, error) {
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	out := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		tag := strings.TrimSuffix(e.Name(), ".yaml")
		if !bareUmbrella.MatchString(tag) {
			continue // only bare vX.Y.Z manifests are umbrella releases
		}
		// Confirm it parses as a composition for its own tag; a manifest that does
		// not is not a resolvable release.
		if _, err := deskkit.LoadComposition(releasesDir, tag); err != nil {
			continue
		}
		out[tag] = true
	}
	return out, nil
}

func latestOf(published map[string]bool) (string, bool) {
	var best string
	var bestV [3]int
	found := false
	for tag := range published {
		v, err := parseSemver(tag)
		if err != nil {
			continue
		}
		if !found || cmpSemver(v, bestV) > 0 {
			best, bestV, found = tag, v, true
		}
	}
	return best, found
}

func parseSemver(tag string) ([3]int, error) {
	var out [3]int
	if !strings.HasPrefix(tag, "v") {
		return out, fmt.Errorf("version %q must start with v", tag)
	}
	parts := strings.Split(tag[1:], ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("version %q is not vX.Y.Z", tag)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, fmt.Errorf("version %q has a non-numeric field %q", tag, p)
		}
		out[i] = n
	}
	return out, nil
}

func mustParse(tag string) [3]int {
	v, _ := parseSemver(tag)
	return v
}

func cmpSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// ── re-pin ──────────────────────────────────────────────────────────────────────

// repin rewrites <root>/.assay-versions so the umbrella line names target and each
// artifact line the target composition names carries the target's tag. The digest
// (field 3) is sourced from the target composition manifest's `sha256:` field when
// present — the manifest is what a real adopter materialises from the release home;
// when a manifest carries no digest for an artifact, the adopter's existing digest
// is carried forward and a warning is printed (offline the tool cannot fetch one,
// and it never fabricates a digest silently). Comments, blank lines and lines the
// composition does not name are preserved verbatim.
func repin(root, relDir, target string, toComp deskkit.Composition) error {
	path := filepath.Join(root, deskkit.AssayVersionsFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	shas, _ := readShas(relDir, target) // best-effort; empty on any read/parse miss

	// Map artifact name -> target tag from the authoritative shared reader.
	tagFor := map[string]string{}
	for _, a := range toComp.Artifacts {
		if n := a.ArtifactName(); n != "" {
			tagFor[n] = a.Tag
		}
	}

	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == deskkit.UmbrellaArtifact {
			lines[i] = deskkit.UmbrellaArtifact + " " + target
			continue
		}
		newTag, ok := tagFor[name]
		if !ok {
			continue // artifact not part of this umbrella — leave untouched
		}
		sha := ""
		if len(fields) >= 3 {
			sha = fields[2]
		}
		if s, ok := shas[name]; ok && s != "" {
			sha = s
		} else {
			fmt.Fprintf(os.Stderr, "upgrade-assay: warning — target composition names no sha256 for %s; carrying the existing digest forward (refresh it from the release home's published sha256).\n", name)
		}
		lines[i] = fmt.Sprintf("%s %s %s", name, newTag, sha)
	}
	out := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(out), 0o644)
}

// compForShas is a supplementary read of the composition manifest for its per-artifact
// `sha256:` digests — the one field deskkit.LoadComposition (which yields the
// authoritative tags) does not surface. It is a detail read, not a second resolver.
type compForShas struct {
	Artifacts []struct {
		Artifact string `yaml:"artifact"`
		Tag      string `yaml:"tag"`
		SHA256   string `yaml:"sha256"`
	} `yaml:"artifacts"`
}

func readShas(releasesDir, umbrellaTag string) (map[string]string, error) {
	name := strings.ReplaceAll(umbrellaTag, "/", "-") + ".yaml"
	raw, err := os.ReadFile(filepath.Join(releasesDir, name))
	if err != nil {
		return nil, err
	}
	var c compForShas
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, a := range c.Artifacts {
		n := a.Artifact
		if n == "" && strings.Contains(a.Tag, "/") {
			n = a.Tag[:strings.IndexByte(a.Tag, '/')]
		}
		if n != "" && a.SHA256 != "" {
			out[n] = a.SHA256
		}
	}
	return out, nil
}
