package main

// darsync.go — DAR ConfigMap / daml.yaml version-sync check (issue #465).
//
// The medici-deploy Job deploys whatever DAR the DAR ConfigMaps
// (MEDICI_LOAN_DAR_CONFIGMAP_PREFIX{1,2,3}.yaml) hold — it does NOT build from
// source. A daml.yaml version bump
// without regenerating those ConfigMaps ships nothing: the Job re-uploads the
// old DAR, Canton matches the hash on-chain, and the Job exits 0 (silent
// no-op). This check makes that drift a hard --lint PROBLEM at PR time:
// daml.yaml, both ConfigMap pairs (dev + prod), and each env's deploy version
// pin must all agree.
//
// That pin lives in one of two manifests, because the deploy surface is
// migrating off the Job (oit #1284): the medici-deploy Job pins it as the
// EXPECTED_DAR_VERSION env, and the deploy-reconciler that replaces it pins
// dar.version in deploy-manifest.yaml. Either satisfies an env — see
// darVersionPinProblems for why the check tolerates both rather than pinning
// one path per env.
//
// The ConfigMap DAR version is read from the artifact itself, not from any
// declared marker: zip local-file headers store entry names uncompressed, and
// every dalf in the DAR lives under "<package>-v<N>-<version>-<hash>/" (the
// package being MEDICI_LOAN_DAR_PACKAGE), so the first pattern match in the
// reassembled bytes is the package's own version. The parts are concatenated
// before matching because the byte split can land mid-filename.
//
// Full DAML CI (dpm build/test/upgrade-check on PRs touching daml/) is #472's
// scope — this check is the cheap, offline, merge-time guard for the
// artifact-drift half of the failure.
//
// The two product identities — the ConfigMap filename prefix and the DAML
// package name — are NOT compiled in: they are repo-level product config
// (MEDICI_LOAN_DAR_CONFIGMAP_PREFIX / MEDICI_LOAN_DAR_PACKAGE, see
// rosterconfig.go). When either is unset — the OSS tool, or any non-product
// repo — this whole check is a clean no-op, so statusgen ships free of the
// house deployment names.

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	damlYamlVersionRe = regexp.MustCompile(`(?m)^version:\s*([0-9]+\.[0-9]+\.[0-9]+)\s*$`)
	darB64Re          = regexp.MustCompile(`(?m)^\s+dar\.b64:\s*([A-Za-z0-9+/=]+)\s*$`)
	expectedDarVerRe  = regexp.MustCompile(`(?s)name:\s*EXPECTED_DAR_VERSION.*?value:\s*"([0-9]+\.[0-9]+\.[0-9]+)"`)

	// deployManifestDarVerRe extracts dar.version from a deploy-reconciler
	// manifest. It is anchored on the top-level `dar:` key and consumes only that
	// key's indented block (a line starting in column 0 ends it), so a `version:`
	// belonging to any other block — governanceContracts, images, … — can never
	// be misread as the DAR pin. Quotes are optional: `version: 0.1.43` is
	// equally valid YAML and must not read as an unwired gate.
	deployManifestDarVerRe = regexp.MustCompile(`(?m)^dar:[^\n]*\n(?:(?:[ \t][^\n]*)?\n)*?[ \t]+version:\s*"?([0-9]+\.[0-9]+\.[0-9]+)"?`)

	// jobNameRe extracts the medici-deploy Job's metadata.name (e.g. medici-deploy-v63).
	// Anchored at 2-space indent (^  name:) to avoid matching the container name
	// which is deeper indented. The name carries the immutable spec.template
	// version: a bump is required whenever Job spec/env/init/scripts change (#465).
	jobNameRe = regexp.MustCompile(`(?m)^  name:\s*(medici-deploy-v\d+)`)
)

// darEntryVersionRegexp builds the regex that reads the DAR version out of the
// reassembled ConfigMap bytes, from the configured DAML package name. The package
// name is repo-level product config (MEDICI_LOAN_DAR_PACKAGE), so it is
// QuoteMeta'd — an identity carrying a regex metacharacter must be matched
// literally, never able to alter the pattern.
func darEntryVersionRegexp(pkg string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(pkg) + `-v[0-9]+-([0-9]+\.[0-9]+\.[0-9]+)`)
}

// darConfigMapPart is the repo-relative path of env's Nth DAR ConfigMap part,
// k8s/<env>/app/<prefix><n>.yaml. The prefix is repo-level product config
// (MEDICI_LOAN_DAR_CONFIGMAP_PREFIX).
func darConfigMapPart(prefix, env string, n int) string {
	return filepath.Join("k8s", env, "app", fmt.Sprintf("%s%d.yaml", prefix, n))
}

// darConfigMapGlob renders the human-facing "<prefix>{1,2,3}.yaml" phrase used in
// the problem messages that name the whole ConfigMap set.
func darConfigMapGlob(prefix string) string {
	return prefix + "{1,2,3}.yaml"
}

// darVersionPin is one manifest that may carry an env's expected-DAR version.
// The deploy surface is mid-migration (oit #1284): the medici-deploy Job pins
// EXPECTED_DAR_VERSION, while the deploy-reconciler that replaces it pins
// dar.version in deploy-manifest.yaml and lets the Job manifest be deleted.
type darVersionPin struct {
	rel     string         // repo-relative manifest path
	re      *regexp.Regexp // extracts the pinned semver
	field   string         // pinned field's name, for problem messages
	unwired string         // problem clause for a manifest present but unpinned
}

// darVersionPins lists every manifest that may pin env's expected-DAR version.
func darVersionPins(env string) []darVersionPin {
	return []darVersionPin{
		{
			rel:     filepath.Join("k8s", env, "app", "medici-deploy.yaml"),
			re:      expectedDarVerRe,
			field:   "EXPECTED_DAR_VERSION",
			unwired: "no EXPECTED_DAR_VERSION env with a quoted semver value — the deploy script's stale-DAR gate is unwired",
		},
		{
			rel:     filepath.Join("k8s", env, "app", "deploy-manifest.yaml"),
			re:      deployManifestDarVerRe,
			field:   "dar.version",
			unwired: "no dar.version with a semver value under the top-level dar: key — the deploy-reconciler's stale-DAR gate is unwired",
		},
	}
}

// darVersionPinProblems checks env's expected-DAR version pin against want.
//
// Either manifest satisfies the env (assay-toolkit #151). Hard-requiring one
// would force the statusgen pin bump and the k8s cutover to land atomically:
// before the cutover only medici-deploy.yaml exists, after it only
// deploy-manifest.yaml does, and whichever half merged first would red-gate the
// other. Accepting either lets them land in any order.
//
// One agreeing pin clears the env. During the cutover window both manifests
// exist and only the one the live deploy path reads is authoritative, so
// demanding that every present manifest agree would red-gate the transition on
// a file that deploys nothing. Problems are reported only when NO present
// manifest pins want — and then for every manifest found, so the message names
// each place to fix.
func darVersionPinProblems(root, env, want string) []string {
	var problems []string
	found := false
	for _, pin := range darVersionPins(env) {
		raw, err := os.ReadFile(filepath.Join(root, pin.rel))
		if err != nil {
			continue
		}
		found = true
		m := pin.re.FindSubmatch(raw)
		if m == nil {
			problems = append(problems, fmt.Sprintf("%s: %s (issue #465)", pin.rel, pin.unwired))
			continue
		}
		if got := string(m[1]); got != want {
			problems = append(problems, fmt.Sprintf("%s: %s is %s but daml.yaml is %s (issue #465)", pin.rel, pin.field, got, want))
			continue
		}
		return nil // this env's deploy path pins the version daml.yaml declares
	}
	if !found {
		return []string{fmt.Sprintf(
			"k8s/%s/app: no deploy version pin found — expected EXPECTED_DAR_VERSION in medici-deploy.yaml or dar.version in deploy-manifest.yaml (dar-sync check, issue #465)",
			env)}
	}
	return problems
}

// darVersionPinEveryProblems is darVersionPinProblems for a release-pinned env:
// EVERY manifest present must pin want, not just one of them.
//
// The tolerant "one agreeing pin clears the env" rule exists for the deploy
// surface cutover (oit #1284), where the two manifests legitimately disagree
// while only one is live. A release pin has no such window: the value is fixed
// by a release, both manifests are edited by the same release cut, and a
// disagreement between them means one of them is about to deploy a version
// nobody declared. Tolerating that here would let a release-pinned env carry a
// silently divergent second pin.
//
// Note this closes assay-toolkit #155 for release-pinned envs ONLY. #155 is the
// observation that the tolerant rule above returns on the FIRST agreeing
// manifest, so a second, present, divergent pin is never read at all — and that
// remains true for every unpinned env (oit dev today). Nothing in this function
// makes that acceptable elsewhere; it is simply out of this check's scope.
func darVersionPinEveryProblems(root, env, want string) []string {
	var problems []string
	found := false
	for _, pin := range darVersionPins(env) {
		raw, err := os.ReadFile(filepath.Join(root, pin.rel))
		if err != nil {
			continue
		}
		found = true
		m := pin.re.FindSubmatch(raw)
		if m == nil {
			problems = append(problems, fmt.Sprintf("%s: %s (issue #465)", pin.rel, pin.unwired))
			continue
		}
		if got := string(m[1]); got != want {
			problems = append(problems, fmt.Sprintf(
				"%s: %s is %s but this env is pinned to the %s release — every deploy pin in a release-pinned env must name the released version (oit issue #1333)",
				pin.rel, pin.field, got, want))
		}
	}
	if !found {
		return []string{fmt.Sprintf(
			"k8s/%s/app: no deploy version pin found — expected EXPECTED_DAR_VERSION in medici-deploy.yaml or dar.version in deploy-manifest.yaml (dar-sync check, issue #465)",
			env)}
	}
	return problems
}

// darInDomain reports whether a changed repo-relative path is in the DAR
// check's domain — the deploy surface these checks actually guard: k8s/**,
// daml/** (source), or daml.yaml. Paths use forward slashes (git output).
func darInDomain(p string) bool {
	p = filepath.ToSlash(p)
	return p == "daml.yaml" || strings.HasPrefix(p, "k8s/") || strings.HasPrefix(p, "daml/")
}

// darSyncProblems returns hard problems when the DAR ConfigMaps or an env's
// deploy version pin disagree with daml.yaml — the problems-only view of
// darSyncCheck, for callers that do not consume its informational notices.
//
// Only darsync_test.go calls it now. That is deliberate and not dead code: it
// is what lets the pre-existing #465/#587 test suite keep exercising this
// checker through its original signature, unedited, so "every prior darsync
// test still passes" stays a claim about behaviour rather than about a rewrite.
// Removing it means editing those call sites, which should be a decision, not a
// cleanup.
func darSyncProblems(root string, changed []string) []string {
	problems, _ := darSyncCheck(root, changed)
	return problems
}

// darSyncCheck returns hard problems when the DAR ConfigMaps or an env's
// deploy version pin disagree with the version that env is supposed to be at,
// plus informational notices about envs that are deliberately not at main's.
// A tree without daml.yaml (test fixtures) is out of scope and returns nil.
//
// "The version that env is supposed to be at" is daml.yaml's — UNLESS the env
// declares a release pin (darrelease.go, oit issue #1333), in which case it is
// the declared release version and the env is validated against that instead.
// prod moves on a release cadence, so a lag behind main is its intended state;
// validating it against main would make the intended state permanently
// indistinguishable from drift. The check is replaced, never removed: a pinned
// env is still checked on every run, against an artifact-verified pin.
//
// Path-scoping (methodology-metrics/31): this is a finance-app *deploy* check —
// #465 drift is a real hazard for a PR that touches the deploy surface, but it
// must not red-gate an unrelated doc PR that never touched k8s/ or daml (the
// PR #788 false-red). When a changed-set is supplied (CI passes --changed) and
// NONE of it is in the DAR domain, the drift is pre-existing main state this PR
// did not introduce — skip. An empty changed-set (main regen, or no signal)
// runs the check unconditionally, preserving whole-tree authority.
func darSyncCheck(root string, changed []string) (problems, notices []string) {
	// Repo-level product config (MEDICI_LOAN_DAR_*). Unset — the OSS tool, or any
	// non-product repo — makes the WHOLE DAR-drift check (issue #465 / oit #1333)
	// a clean no-op: not an error, not a false problem. The names of the deploy
	// artifacts this check reads are the product's, so with no product identity
	// configured there is nothing here to check. Only when BOTH are set does the
	// check run, with the real values.
	cfg := scanEffectiveConfig()
	if !cfg.DarConfigured() {
		return nil, nil
	}
	cmPrefix := cfg.DarConfigMapPrefix
	entryVersionRe := darEntryVersionRegexp(cfg.DarPackage)

	if len(changed) > 0 {
		inDomain := false
		for _, p := range changed {
			if darInDomain(p) {
				inDomain = true
				break
			}
		}
		if !inDomain {
			return nil, nil
		}
	}
	damlYaml, err := os.ReadFile(filepath.Join(root, "daml.yaml"))
	if err != nil {
		return nil, nil
	}
	m := damlYamlVersionRe.FindSubmatch(damlYaml)
	if m == nil {
		return []string{"daml.yaml: no version: field found (dar-sync check, issue #465)"}, nil
	}
	mainWant := string(m[1])

	for _, env := range []string{"dev", "prod"} {
		// Release pin (oit issue #1333). A malformed declaration yields no pin
		// AND problems: the env then falls back to the daml.yaml comparison,
		// which is the stricter of the two, so a broken declaration can never
		// relax anything.
		pin, pinProblems := darReleasePinFor(root, env)
		problems = append(problems, pinProblems...)
		pinned := pin != nil
		want := mainWant
		ahead := false
		if pinned {
			want = pin.Version
			// A pinned env AHEAD of main cannot be a release lag — either the
			// declaration is wrong or main was reverted. Fail closed: the
			// PROBLEM below is raised AND `ahead` disqualifies the pin from
			// verified status, so an ahead pin relaxes nothing (it neither
			// suppresses the #587 check nor earns the reassuring notice). A
			// declaration that is red on its face must not be simultaneously
			// trusted enough to turn another check off.
			if semverLess(mainWant, pin.Version) {
				ahead = true
				problems = append(problems, fmt.Sprintf(
					"%s: dar.release.version %s is AHEAD of daml.yaml %s — a release pin may lag main, never lead it (oit issue #1333)",
					darReleaseManifestRel(env), pin.Version, mainWant))
			}
		}
		var dar []byte
		ok := true
		for n := 1; n <= 3; n++ {
			rel := darConfigMapPart(cmPrefix, env, n)
			raw, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: missing DAR ConfigMap (dar-sync check, issue #465)", rel))
				ok = false
				continue
			}
			bm := darB64Re.FindSubmatch(raw)
			if bm == nil {
				problems = append(problems, fmt.Sprintf("%s: no dar.b64 binaryData entry (dar-sync check, issue #465)", rel))
				ok = false
				continue
			}
			dec, err := base64.StdEncoding.DecodeString(string(bm[1]))
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: dar.b64 is not valid base64 (dar-sync check, issue #465): %v", rel, err))
				ok = false
				continue
			}
			dar = append(dar, dec...)
		}
		// Derived version: read out of the DAR BYTES, never from a label. For a
		// pinned env this is the other half of the declared-vs-derived
		// cross-check — the declaration is only honoured if the artifact it
		// names is the artifact actually committed.
		derived, derivedOK := "", false
		if ok {
			vm := entryVersionRe.FindSubmatch(dar)
			if vm == nil {
				problems = append(problems, fmt.Sprintf("k8s/%s/app/%s: reassembled DAR has no %s-v<N>-<version> entry (dar-sync check, issue #465)", env, darConfigMapGlob(cmPrefix), cfg.DarPackage))
			} else if derived, derivedOK = string(vm[1]), true; derived != want {
				if pinned {
					problems = append(problems, fmt.Sprintf(
						"k8s/%s/app/%s: DAR ConfigMaps hold %s but %s declares dar.release.version %s — the release pin does not match the DAR bytes it claims to pin (oit issue #1333)",
						env, darConfigMapGlob(cmPrefix), derived, darReleaseManifestRel(env), pin.Version))
				} else {
					problems = append(problems, fmt.Sprintf("k8s/%s/app/%s: DAR ConfigMaps hold %s but daml.yaml is %s — regenerate via scripts/gen-dar-configmaps.sh and bump the medici-deploy Job name (issue #465)", env, darConfigMapGlob(cmPrefix), derived, want))
				}
			}
		}
		// A pin whose artifact cannot be read has not been cross-checked, and an
		// un-cross-checked pin is exactly the editable label this feature must
		// not become. Say so explicitly rather than leaning on the ConfigMap
		// problems above still being hard failures.
		if pinned && !derivedOK {
			problems = append(problems, fmt.Sprintf(
				"%s: dar.release.version %s cannot be cross-checked — this env's DAR bytes could not be read, so the release pin is unverified (oit issue #1333)",
				darReleaseManifestRel(env), pin.Version))
		}
		// Everything below honours the pin only if it survived the cross-check
		// AND is a legal pin in the first place (`!ahead`). Both terms are
		// load-bearing: the cross-check is what stops an edited label from
		// silencing drift, and !ahead is what stops a pin the checker has
		// already called wrong from relaxing anything on its way out.
		pinVerified := pinned && !ahead && derivedOK && derived == pin.Version

		// Content-drift check (issue #587): the label check above can be made to
		// pass trivially by "resolving" a label mismatch with a daml.yaml
		// DOWNGRADE to match a stale ConfigMap, rather than rebuilding the DAR —
		// every label then agrees while the ConfigMap DAR still holds none of the
		// PR's DAML (the #432 near-miss). Labels can't see that; a git diff can:
		// if daml/** changed since the merge-base with origin/main but this env's
		// DAR ConfigMaps did not, the DAR is stale regardless of what the version
		// strings say.
		//
		// Suppressed for a VERIFIED release-pinned env (oit issue #1333): under a
		// release cadence, daml/** moving while that env's DAR ConfigMaps stay put
		// IS the expected state — the env is pinned to the last release, not to
		// main. The suppression is deliberately gated on pinVerified, not on
		// "a declaration exists": a pin that failed its cross-check, or that could
		// not be cross-checked at all, leaves this check running, so a bad
		// declaration can never be the thing that turns a drift signal off.
		if ok && !pinVerified && damlSourceChanged(root) && !darConfigMapChanged(root, env, cmPrefix) {
			problems = append(problems, fmt.Sprintf(
				"k8s/%s/app/%s: daml/** changed since the merge-base with origin/main but these DAR ConfigMaps did not — the DAR is stale vs source even though version labels agree; rebuild via scripts/gen-dar-configmaps.sh (issue #587)",
				env, darConfigMapGlob(cmPrefix)))
		}

		// Keyed on `pinned`, not `pinVerified`: once an env declares a pin, want
		// IS the declared version, so the unpinned check's "…but daml.yaml is X"
		// wording would name the wrong authority. A declared-but-unverified pin
		// gets the stricter check and the accurate message.
		if pinned {
			problems = append(problems, darVersionPinEveryProblems(root, env, want)...)
		} else {
			problems = append(problems, darVersionPinProblems(root, env, want)...)
		}

		// The pinned state is never invisible: an env validated against its own
		// release rather than against main says so on every run, whether or not
		// it currently lags. Informational — it must never move the exit code,
		// because a deliberate lag is not a defect.
		if pinVerified {
			lag := "in step with main"
			if pin.Version != mainWant {
				lag = "main at " + mainWant
			}
			notices = append(notices, fmt.Sprintf(
				"dar-sync: %s pinned to %s (release %s, cut %s), %s — %s is validated against its own release pin, not daml.yaml (oit issue #1333)",
				env, pin.Version, pin.Tag, pin.Date, lag, darReleaseManifestRel(env)))
		}
	}
	p, n := jobNameBumpProblems(root)
	problems = append(problems, p...)
	notices = append(notices, n...)
	return problems, notices
}

// darMergeBase resolves the "landed" cut for the content-drift check below: the
// merge-base of HEAD and origin/main, falling back to HEAD when there is no
// git checkout or no resolvable origin/main (a fixture tempdir, main's own CI,
// or an offline host). Same fallback convention as deletedRegisterFiles/
// grandfatheredIDs — a fallback to HEAD makes every git-diff comparison below
// trivially empty (HEAD vs HEAD, modulo uncommitted worktree edits), so an
// unresolvable base degrades to "no signal" rather than a false PROBLEM.
func darMergeBase(root string) string {
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return ""
	}
	base := "HEAD"
	if mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output(); err == nil && strings.TrimSpace(string(mb)) != "" {
		base = strings.TrimSpace(string(mb))
	}
	return base
}

// gitPathChanged reports whether relPath differs between base and the current
// state (working tree + any uncommitted edits), via `git diff --name-only`.
// Returns false on any git error (not a checkout, path never existed, etc.) —
// this is a best-effort signal, never a hard failure source.
func gitPathChanged(root, base, relPath string) bool {
	if base == "" {
		return false
	}
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", base, "--", relPath).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// damlSourceChanged reports whether daml/** differs from the landed
// merge-base with origin/main (issue #587: the DAR-parity label check cannot
// see a stale ConfigMap DAR when the version labels happen to still agree —
// this is the independent git-diff signal that catches it).
func damlSourceChanged(root string) bool {
	return gitPathChanged(root, darMergeBase(root), "daml")
}

// darConfigMapChanged reports whether any part of env's DAR ConfigMap set
// differs from the landed merge-base with origin/main. prefix is the configured
// ConfigMap filename prefix (MEDICI_LOAN_DAR_CONFIGMAP_PREFIX).
func darConfigMapChanged(root, env, prefix string) bool {
	base := darMergeBase(root)
	for n := 1; n <= 3; n++ {
		if gitPathChanged(root, base, darConfigMapPart(prefix, env, n)) {
			return true
		}
	}
	return false
}

// jobNameBumpProblems checks that PRs changing any medici-deploy file also bump
// the medici-deploy Job name (medici-deploy-v<N>), per the immutable
// spec.template rule (#465). A modified Job manifest whose content changed
// but whose metadata.name stayed the same will fail at deploy time because
// spec.template is immutable in Kubernetes.
//
// For each env, if ANY medici-deploy* file changed between the merge-base and
// HEAD, the check extracts the Job name from medici-deploy.yaml at both refs.
// A content change without a name bump is a hard PROBLEM.
//
// A file that exists only at HEAD (new, renamed) is implicitly a bump and is
// not flagged. A file that does not exist at HEAD (env uses deploy-manifest
// instead) is skipped.
//
// The "any medici-deploy* file changed" gate is deliberately broad: ANY touch
// of a file whose path contains "medici-deploy" under k8s/<env>/app (a
// comment, a label, an annotation, a whitespace reflow, a scripts ConfigMap
// edit) will demand a Job-name bump. This is an acknowledged
// over-approximation — the check trades false positives on comment-only edits
// for implementation simplicity and for the guarantee that no genuine spec
// change escapes. Accept it or scope the diff narrower; the message below
// says "manifest(s) changed", which is what the check actually observed.
func jobNameBumpProblems(root string) (problems, notices []string) {
	base := darMergeBase(root)
	if base == "" {
		return nil, nil // no git checkout — out of scope
	}
	if base == "HEAD" {
		// origin/main unresolvable — the check cannot compare against the
		// landed baseline. Degrade to a NOTICE rather than silent skip, so
		// the run is not mute when medici-deploy manifests are present but
		// the lint workflow forgot to fetch origin/main first (the same
		// hazard registerBaseFallbackNotices guards against).
		for _, env := range []string{"dev", "prod"} {
			deployYAML := filepath.Join("k8s", env, "app", "medici-deploy.yaml")
			if _, err := os.Stat(filepath.Join(root, deployYAML)); err == nil {
				notices = append(notices, fmt.Sprintf(
					"dar-sync: job-name-bump check skipped for %s: origin/main unresolvable (base fell back to HEAD) — committed name-freeze violations will not be caught; the lint workflow must fetch origin/main first",
					deployYAML))
			}
		}
		return nil, notices
	}

	for _, env := range []string{"dev", "prod"} {
		// Did ANY medici-deploy file change in this env?
		out, err := exec.Command("git", "-C", root, "diff", "--name-only", base, "HEAD", "--",
			filepath.Join("k8s", env, "app")).Output()
		if err != nil {
			continue
		}
		anyDeployChanged := false
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && strings.Contains(line, "medici-deploy") {
				anyDeployChanged = true
				break
			}
		}
		if !anyDeployChanged {
			continue
		}

		// A medici-deploy file changed — check whether the Job name was bumped.
		deployYAML := filepath.Join("k8s", env, "app", "medici-deploy.yaml")
		currentRaw, err := os.ReadFile(filepath.Join(root, deployYAML))
		if err != nil {
			continue // Job manifest absent (env uses deploy-manifest instead)
		}
		currentMatch := jobNameRe.FindSubmatch(currentRaw)
		if currentMatch == nil {
			continue // cannot extract name — not our format, skip
		}
		currentName := string(currentMatch[1])

		// Get the merge-base version of the same file.
		baseRaw, err := exec.Command("git", "-C", root, "show", base+":"+deployYAML).Output()
		if err != nil {
			continue // file didn't exist at merge-base → new file, implicitly bumped
		}
		baseMatch := jobNameRe.FindSubmatch(baseRaw)
		if baseMatch == nil {
			continue // no name at merge-base — name was added, OK
		}
		baseName := string(baseMatch[1])

		if currentName == baseName {
			problems = append(problems, fmt.Sprintf(
				"%s: medici-deploy manifest(s) changed but Job name was not bumped (stays %s) — bump metadata.name (medici-deploy-v<N>) per the immutable spec.template rule (issue #465)",
				deployYAML, currentName))
		}
	}
	return problems, nil
}
