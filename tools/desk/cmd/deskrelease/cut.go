package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// defaultReleaseRepo is the SHIPPED release home: this project's own public
// repository. It is correct as a compiled default precisely because it is public —
// there is nothing here to disclose, and an adopter who has configured nothing gets
// the same target the compiled-in slug always gave.
//
// NOTE: the bare slug is a REPO IDENTIFIER, not a Go module path — it is data naming
// a GitHub repo, so it deliberately does NOT carry the `github.com/` prefix. It is
// repo-inventory data, tracked separately from the module-path rename. Do NOT
// "finish" a sweep by adding `github.com/` here: that would rewrite a repo identifier
// into a module path and retarget the repo gate at a path that resolves to nothing.
const defaultReleaseRepo = "medici-finance/assay"

// repoSlug is the repo this tool may act on: defaultReleaseRepo unless the adopter
// has configured ASSAY_RELEASE_REPO, which applyConfiguredReleaseRepo reads once per
// run from the roster.
//
// WHAT DID AND DID NOT CHANGE when this stopped being compiled in. The tool is still
// not RETARGETABLE by its caller: there is no flag, no argument, and no environment
// read on this path — the roster is loaded ClassWrite, which consults the config-home
// file ONLY (deskkit/rosterconfig.go's split-by-action-class rule), so a steered
// session cannot point the release tool at another repository by exporting a
// variable. What changed is that a FORK can name its own release home without
// editing and rebuilding the binary.
//
// The C-4 refusal is untouched and stays the real boundary: whatever this resolves
// to is passed through deskkit.IsAllowedRepo below, so a configured value outside the
// write-authorisation set is refused exactly as an out-of-set repo always was. The
// variable chooses WHICH in-scope repo is the release home; it cannot admit one.
//
// It remains a package var (rather than being resolved at each use) so the repo gate
// below stays exercisable by a white-box test.
var repoSlug = defaultReleaseRepo

// applyConfiguredReleaseRepo points repoSlug at the adopter's configured release home
// when one is set. It is called once at the top of run(), AFTER the roster class is
// declared and BEFORE anything reads repoSlug.
//
// An UNSET variable leaves the compiled default in place untouched — the
// byte-identical-when-unset property — and, because it writes nothing in that case, a
// test that deliberately moves repoSlug to prove the scope gate fires is not quietly
// overwritten by a configuration that has nothing to say.
func applyConfiguredReleaseRepo() {
	if r := deskkit.ConfiguredReleaseRepo(); r != "" {
		repoSlug = r
	}
}

// mainRef is the branch a release tag may point at. releaseguard's `on-main` check is
// unwaivable, so a tag cut anywhere else would be refused in CI anyway; naming it here
// means the wrong commit is never even tagged.
const mainRef = "heads/main"

// tagPattern is the ONLY accepted tag shape: the two release namespaces this repo's
// workflows trigger on, followed by a strict three-part numeric version.
//
// In Go's regexp (RE2, no `m` flag) `^` is beginning-of-TEXT and `$` is end-of-TEXT
// (\z semantics, not Perl's \Z), so this genuinely anchors both ends — it cannot be
// satisfied by a prefix, and a trailing newline does not slip past it.
var tagPattern = regexp.MustCompile(`^(desk-tools|statusgen)/v[0-9]+\.[0-9]+\.[0-9]+$`)

// refspecMetachars are the characters through which a git refspec expresses a
// DESTINATION or a revision peel — the exact vocabulary of the escapes that defeated the
// glob allow-rule. tagPattern already excludes every one of them; this screen runs first
// so a hostile argument is refused by NAME ("refspec metacharacter") rather than by the
// generic shape message, and so the two layers fail independently.
//
//	':'  destination refspec — `<tag>:refs/heads/main`, `<tag>:main`, `<tag>^{}:main`
//	'+'  leading-plus force  — `+HEAD:refs/heads/main` (forces with no --force/-f)
//	'^'  revision peel       — `<tag>^{}` turns a tag into its commit
//	'~'  revision walk       — `HEAD~3`
//	'@'  reflog/upstream     — `@{u}`, `main@{1}`
//	'*'  glob                — a refspec wildcard
//	'?'  glob
//	'['  glob character class
//	'\\' escape
var refspecMetachars = ":+^~@*?[\\"

// cutArgs is the fully-parsed `cut` invocation. Its fields are the COMPLETE set of
// things a caller can influence: one tag, and whether to stop before the write. Nothing
// here selects a repo, a branch, a commit, or a force mode.
type cutArgs struct {
	tag    string
	dryRun bool
}

// parseCutArgs is the end anchor a glob does not have. It accepts EXACTLY
// `<tag>` or `<tag> --dry-run` (in either order) and refuses everything else —
// unknown flags, repeated flags, extra operands, and empty arguments — rather than
// ignoring them.
//
// argv is the ONLY input. The tag is never read from stdin, never from an environment
// variable, and never from a config file: there is no code path in this package that
// reads any of those, so a caller who can set the environment still cannot name a tag.
func parseCutArgs(rest []string) (cutArgs, error) {
	var a cutArgs
	seenTag := false

	for _, arg := range rest {
		switch {
		case arg == "--dry-run":
			if a.dryRun {
				return a, deskkit.Refused("refused: --dry-run given twice")
			}
			a.dryRun = true
		case arg == "":
			return a, deskkit.Refused("refused: empty argument")
		case strings.HasPrefix(arg, "-"):
			// Covers a leading-dash TAG too (`-desk-tools/v1.0.0`), bundled short opts
			// (`-uf`), and every long flag this tool does not implement. There is no
			// pass-through: an unknown flag is never forwarded anywhere.
			return a, deskkit.Refused(fmt.Sprintf(
				"refused: unrecognised flag %q — deskrelease cut takes only <tag> and --dry-run, "+
					"and forwards no flag to any other program", arg))
		case seenTag:
			// The load-bearing refusal. `deskrelease cut <tag> <anything-else>` is the
			// shape a trailing glob wildcard would have absorbed silently; here it stops.
			return a, deskkit.Refused(fmt.Sprintf(
				"refused: extra operand %q — deskrelease cut takes exactly ONE tag; "+
					"an extra argument is never ignored", arg))
		default:
			a.tag = arg
			seenTag = true
		}
	}

	if !seenTag {
		return a, deskkit.Refused("refused: no <tag> given — the tag comes from argv only (never stdin or the environment)")
	}
	return a, nil
}

// validateTag screens the tag in two independent layers, each with its own refusal
// reason so neither can be deleted without a test noticing:
//
//  1. refspec metacharacters, named as such;
//  2. the anchored namespace+semver pattern.
func validateTag(tag string) error {
	for _, r := range tag {
		if r == ' ' || r == '\t' {
			return deskkit.Refused(fmt.Sprintf(
				"refused: tag %q contains whitespace — a second refspec is not a tag", tag))
		}
		if r < 0x20 || r == 0x7f {
			return deskkit.Refused(fmt.Sprintf(
				"refused: tag %q contains a control character", tag))
		}
		if strings.ContainsRune(refspecMetachars, r) {
			return deskkit.Refused(fmt.Sprintf(
				"refused: tag %q contains the refspec metacharacter %q — "+
					"destinations, peels and force markers are not tags", tag, string(r)))
		}
	}
	if !tagPattern.MatchString(tag) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: tag %q does not match %s — the only shapes this tool cuts are "+
				"desk-tools/vX.Y.Z and statusgen/vX.Y.Z", tag, tagPattern.String()))
	}
	return nil
}

// planCut is the whole act. Order matters and is fail-closed at every step:
//
//	parse argv → validate tag → repo gate → mint desk-App token →
//	if the tag exists: noop when it is ALREADY at the current origin/main HEAD,
//	refuse otherwise → re-read origin/main HEAD → create the ref →
//	verify the created ref points where we asked
//
// Every read is against the REMOTE. The tool touches no local checkout, runs no git
// command, and has no cwd dependence — so it cannot be aimed at a stale sibling
// clone and cannot be defeated by a `git -C` variance.
func planCut(rest []string) writeResult {
	verb := "cut"

	a, err := parseCutArgs(rest)
	if err != nil {
		return fromErr(verb, "", err)
	}
	verb = "cut:" + a.tag

	if err := validateTag(a.tag); err != nil {
		return fromErr(verb, "", err)
	}

	// The repo set is CONFIGURED, and no flag or caller-supplied input widens it —
	// including ASSAY_RELEASE_REPO, which selects a target and is then screened here
	// like any other. C-4.
	if !deskkit.IsAllowedRepo(repoSlug) {
		return fromErr(verb, "", deskkit.Refused(
			"refused: "+repoSlug+" is outside the fixed desk repo set"))
	}

	c, err := newGHClient(repoSlug)
	if err != nil {
		return fromErr(verb, "", err)
	}

	// Public-repo trust gate. deskrelease cut creates a tag ref via
	// POST /repos/{owner}/{repo}/git/refs — an outward write. Release tags have no
	// associated issue/PR number (issueNumber=0), so the gate fails closed (exit 6) for
	// any public repo. The gate runs on whatever repoSlug RESOLVED to — the shipped
	// default or a configured one — so making the slug configurable cannot route a
	// release around it.
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: c.token, BaseURL: apiBaseURL}
	if gerr := deskkit.PublicRepoGate(fetcher, c.owner, c.repo, 0); gerr != nil {
		return fromErr(verb, "", gerr)
	}

	// Handle an existing tag BEFORE creating anything, so the message names the real
	// problem. This is defence in depth, not the only guard: POST /git/refs is itself
	// create-only and answers 422 for an existing ref, so even a racing caller cannot MOVE
	// a tag through this tool. Idempotency is therefore anchored on the REMOTE, not on the
	// local audit file — a lost audit log cannot make a re-cut look already-done.
	//
	// The existing tag is discriminated by WHERE IT POINTS, and only two answers exist:
	//
	//   - at the commit this invocation would tag anyway (current main HEAD) → NOOP. The
	//     requested end state is already the actual state; nothing is written and nothing
	//     is moved. This is the re-run path after an ambiguous POST — if `c.http.Do` errors
	//     AFTER GitHub created the ref, the first run exits 6 having done the thing, and a
	//     hard refusal on the retry would burn a version number for a transient network
	//     error. A burnt number cannot be recovered: a deleted tag does not free it.
	//   - at ANY other commit → the hard refusal it has always been. That is the case where
	//     honouring the request would mean MOVING a published tag, which never happens.
	//
	// The noop therefore does not weaken "never move a published tag" — it is precisely the
	// case where no move is required. And a head we cannot positively resolve falls through
	// to the refusal (fail closed): "same commit" must be PROVEN, never assumed.
	existing, found, err := c.getRef("tags/" + a.tag)
	if err != nil {
		return fromErr(verb, "", err)
	}
	if found {
		headNow, headFound, herr := c.getRef(mainRef)
		if herr != nil {
			return fromErr(verb, existing, herr)
		}
		if !headFound {
			return fromErr(verb, existing, deskkit.Refused(fmt.Sprintf(
				"refused: tag %s already exists at %s and refs/%s cannot be resolved, so it "+
					"cannot be PROVEN to already point where this cut would put it — a published "+
					"tag is never moved or re-pointed. Fix forward with the next patch version.",
				a.tag, short(existing), mainRef)))
		}
		if existing == headNow {
			return noop(verb, existing, fmt.Sprintf(
				"noop: tag %s already exists and already points at the current %s HEAD %s — "+
					"nothing was written and no new release was triggered (the release for this "+
					"tag ran when the ref was created)",
				a.tag, mainRef, short(existing)))
		}
		return fromErr(verb, existing, deskkit.Refused(fmt.Sprintf(
			"refused: tag %s already exists at %s, which is not the current %s HEAD %s — a "+
				"published tag is never moved or re-pointed (consumers pin tag+sha256; a moved "+
				"tag is the substitution the pin exists to prevent). Fix forward with the next "+
				"patch version.",
			a.tag, short(existing), mainRef, short(headNow))))
	}

	// The release point is re-read from the remote at act time. There is no --sha flag,
	// so a caller can neither choose nor pin the commit that gets released.
	head, found, err := c.getRef(mainRef)
	if err != nil {
		return fromErr(verb, "", err)
	}
	if !found {
		return fromErr(verb, "", deskkit.Unverifiable(
			"cannot resolve refs/"+mainRef+" on "+repoSlug+" — refusing to cut a tag at an unknown commit", nil))
	}

	// The dry run stops HERE — above createTagRef, the only line in this package that
	// writes anything to the remote. It is audited as ResultDryRun rather than
	// ResultNoop so that rehearsing a release is genuinely free: neither the hourly
	// outward-write budget nor the non-progress circuit breaker can see this line
	// (#214). Before that, five `--dry-run`s in a row opened a 15-minute breaker
	// against the real cut — a release could be locked out by the act of checking
	// whether it was safe.
	if a.dryRun {
		return dryNoop(verb, head, fmt.Sprintf(
			"dry-run: %s is free and would be created at %s HEAD %s — nothing was written",
			a.tag, mainRef, short(head)))
	}

	created, err := c.createTagRef(a.tag, head)
	if err != nil {
		return fromErr(verb, head, err)
	}
	// Positive verification of our own act: the ref GitHub reports having created
	// must be the commit we asked for. Anything else is unverifiable, not success.
	if created != head {
		return fromErr(verb, head, deskkit.Unverifiable(fmt.Sprintf(
			"created %s but GitHub reports it at %s, not the %s HEAD %s we requested",
			a.tag, short(created), mainRef, short(head)), nil))
	}

	return done(verb, head, fmt.Sprintf(
		"cut %s at %s HEAD %s on %s — release-desk/release-statusgen now run releaseguard, then build",
		a.tag, mainRef, short(head), repoSlug))
}
