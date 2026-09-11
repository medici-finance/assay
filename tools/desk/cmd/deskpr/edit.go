package main

// edit.go — `deskpr edit`, the verb that corrects an OPEN pull request's own body text
// (and, optionally, its title) without touching a commit.
//
// WHY IT EXISTS. The rework cycle whose finding is "the PR body says X, it should say Y"
// — a provenance correction, a scope clarification, a stale Refs line — had no sanctioned
// path. `create` opens a PR and `update` pushes commits to one; neither can change the
// description GitHub already holds. A worker told to write through the desk verbs, and
// separately told never to route around an instruction with a different tool, was left
// with two bad exits: leave the body wrong, or fall back to a raw `gh pr edit` that runs
// none of the gates and leaves no audit row. This verb closes that with the SAME gates
// create runs over the text it publishes, so the escape hatch stays audited instead of
// becoming an unrecorded CLI call.
//
// WHAT IT IS NOT. It edits the two TEXT surfaces of an existing PR and nothing else. It
// cannot open a PR, cannot push, cannot flip ready, cannot close or merge, and cannot
// touch labels, reviewers or base. The git argv it never builds is the point: this file
// runs no git command at all past preflight's reads.
//
// THE TRAILER IS NOT EDITABLE. The body's one link trailer (`Brief: <stream>/<NN>` or
// `Issue: #<N>`) is the derived board's DATA EDGE from the PR to its work item. A verb
// that can rewrite the body could otherwise silently re-point a merged-tomorrow PR at a
// different brief, or drop the edge entirely, after every gate that checked it has run.
// So: the replacement body must carry exactly one trailer (the same grammar `create`
// enforces, resolved under --root), and when the PR's CURRENT body already carries one,
// the replacement's must be identical to it. The one asymmetry is deliberate — a PR whose
// current body carries NO trailer may gain one, because that is exactly the migration
// `update` tells the worker to perform by hand ("the worker edits the body, then re-runs
// update"), and refusing it here would leave that instruction with no verb to name.
//
// IT ANNOUNCES ITSELF. A body/title edit moves no head SHA, so a review monitor keyed on
// the head — which is how the review desk notices a PR needs another look — sees nothing
// at all happen. The verb therefore posts one short comment on the PR naming which
// surfaces changed. That comment IS the event; without it the correction is invisible to
// the loop that has to act on it.

import (
	"flag"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdEdit implements `deskpr edit`. Flow: validate flags → read + secret-scan the
// replacement body → trailer grammar on the replacement (local, before any network) →
// preflight → title scan → mint the App token → find the branch's OPEN PR (a merged or
// closed one is not found, and that is the refusal) → read its current body/title →
// trailer-immutability → public-repo self-containment on both published surfaces →
// idempotency noop → rate limit → public-repo gate → `gh pr edit` → the re-review comment.
func cmdEdit(args []string) (err error) {
	ac := &auditCtx{verb: "edit"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // suppress flag's own output; we craft messages
	bodyFile := fs.String("body-file", "", "path to a file holding the replacement PR body (required)")
	title := fs.String("title", "", "replacement PR title (optional; the title is left alone when empty)")
	root := fs.String("root", ".", "repo root the Brief: trailer resolves against (docs/streams under it)")
	scanOverride := fs.String(deskkit.ScanOverrideFlag, "", "override a secret-scan refusal, stating why; writes an audit row (tool, surface digest, reason, identity)")
	explain := fs.Bool("explain", false, "on a secret-scan refusal, also print a scan-explain line naming the rule id and line number (never the offending span)")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	defer func() { explainScanRefusal(*explain, err) }()
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: edit takes no positional arguments — it edits the OPEN PR of the branch this worktree is on")
	}
	// Validate the override BEFORE anything else runs, so a malformed one refuses in
	// milliseconds rather than after a token mint and a remote read (same ordering as create).
	if *scanOverride != "" {
		if verr := deskkit.ValidateScanOverride(*scanOverride); verr != nil {
			return verr
		}
	}
	// --body-file only, deliberately: `create`'s --body-min exists so a one-line body can be
	// opened without a temp file, and a CORRECTION that fits on one line is not the case this
	// verb was asked for. A single input also means there is exactly one way a replacement
	// body reaches the gates below.
	if *bodyFile == "" {
		return deskkit.Refused("refused: --body-file is required — `deskpr edit` replaces the PR body with the contents of that file")
	}
	body, berr := readBody(*bodyFile, "")
	if berr != nil {
		return berr
	}
	if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
		Tool: "deskpr", Verb: "edit", Reason: *scanOverride,
		Surface: "PR body", Content: body,
	}, deskkit.ScanSurface("PR body", body)); serr != nil {
		return serr
	}
	if *title != "" {
		if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
			Tool: "deskpr", Verb: "edit", Reason: *scanOverride,
			Surface: "PR title", Content: []byte(*title),
		}, deskkit.ScanSurface("PR title", []byte(*title))); serr != nil {
			return serr
		}
	}
	// The branch NAME and the branch DIFF are not re-scanned here, and the reason is a
	// statement about what this verb publishes rather than an exemption. create and update
	// scan those two surfaces because a PUSH publishes them; edit pushes nothing — the
	// commits and the branch are already on the forge, unchanged by this call, and the only
	// bytes it puts on the forge are the body and the title, both scanned above. Refusing a
	// body CORRECTION on the grounds of code the branch already carries would aim the
	// #775 stranding shape squarely at the one verb whose job is to fix text.

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}

	// example-stream/02: the replacement body faces the full trailer grammar create runs —
	// exactly one link, and a `Brief:` value that resolves to a brief file under --root —
	// BEFORE any network call. A body that would land on the PR without a resolvable link
	// is refused while the refusal is still free.
	if _, terr := requireTrailer(body, *root, dir); terr != nil {
		return terr
	}

	// edit has no --base: it never opens a PR, so there is no base to open against. An
	// empty base keeps preflight's ahead-count pinned to origin/HEAD, exactly as update.
	facts, perr := preflight(dir, "")
	if perr != nil {
		return perr
	}
	ac.repo, ac.head = facts.repo, facts.head

	if merr := mintWorkerToken(facts.repo); merr != nil {
		return deskkit.Unverifiable("cannot mint the App token", merr)
	}
	fg, fr, ferr := forgeForFn(facts.repo)
	if ferr != nil {
		return ferr
	}

	// The PR is found the way update finds it: the OPEN change whose source branch is this
	// branch. OpenChangeForBranch returns none for a merged or closed PR — one refusal covers
	// "no PR", "already merged" and "closed", and none of the three can be edited.
	pr, lerr := fg.OpenChangeForBranch(fr, facts.branch)
	if lerr != nil {
		return deskkit.Unverifiable("cannot look up the open PR for the branch", lerr)
	}
	if pr == nil {
		return deskkit.Refused("refused: no OPEN PR for " + facts.branch +
			" — `deskpr edit` corrects an existing open PR's text; a merged or closed PR is not editable through this verb, " +
			"and a branch with no PR yet wants `deskpr create`")
	}
	ac.pr = &pr.Number

	// Read the current body/title authoritatively (GetPullRequest, the single-change read the
	// brief pairs with OpenChangeForBranch) — the trailer-immutability check and the
	// idempotency noop both key on the CURRENT text.
	cur, terr := fg.GetPullRequest(fr, pr.Number)
	if terr != nil {
		return deskkit.Unverifiable("cannot read the PR's current body/title", terr)
	}

	// Trailer immutability. See the file header: the link is the board's data edge, and a
	// body-rewrite verb that could re-point or drop it would make the edge assertable
	// exactly once, at create time, and revocable silently forever after.
	oldLink, hadLink := trailerLink([]byte(cur.Body))
	newLink, _ := trailerLink(body)
	if hadLink && oldLink != newLink {
		return deskkit.Refused(fmt.Sprintf(
			"refused: the replacement body changes the PR's link trailer (%s → %s) — a work item link is not "+
				"editable after the fact. Keep `%s` in the body and re-run; a PR that genuinely delivers different "+
				"work is a different PR.", oldLink, newLink, oldLink))
	}

	// #203 public-repo self-containment, over the two surfaces this verb PUBLISHES. It runs
	// here rather than beside the secret scan because it needs the target repo, which only
	// preflight establishes, and the PR number, which only the listing above does. The
	// number is a real hint here in a way it never is for create: the PR exists on this
	// repo, so its own number bounds the bare-`#N` heuristic. The verdict routes through
	// HandleScanRefusal like every other scan on this path, so a refusal advertises the SAME
	// audited override rather than introducing a second bypass.
	scOpts := deskkit.SelfContainOpts{Repo: facts.repo, NumberHint: pr.Number}
	for _, sc := range []struct {
		surface string
		content []byte
	}{
		{"PR body", body},
		{"PR title", []byte(*title)},
	} {
		if len(sc.content) == 0 {
			continue // --title omitted: nothing is published on that surface
		}
		if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
			Tool: "deskpr", Verb: "edit", Repo: facts.repo, Reason: *scanOverride,
			Surface: sc.surface, Content: sc.content,
		}, deskkit.SelfContainCheck(sc.surface, sc.content, scOpts)); serr != nil {
			return serr
		}
	}

	// Idempotency. update keys its noop on the head SHA; an edit moves no head, so the
	// question "has this already been done" can only be answered by the CONTENT that is
	// already there. Identical body and (when asked for) identical title → noop, exit 0,
	// and in particular no second re-review comment on a PR nothing changed on.
	titleUnchanged := *title == "" || *title == cur.Title
	if cur.Body == string(body) && titleUnchanged {
		ac.successResult = deskkit.ResultNoop
		ac.detail = "body/title already match " + pr.URL
		fmt.Printf("noop: %s already carries this body/title\n", pr.URL)
		return nil
	}

	if werr := deskkit.AllowWrite("deskpr", facts.repo, pr.Number); werr != nil {
		return werr
	}

	// Public-repo gate: refuse an outward write unless the repo is authorized
	// (private, or a listed :public allowed-repos entry — see deskkit.PublicRepoGate).
	owner, name := splitOwnerRepo(facts.repo)
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}
	if gerr := publicRepoGateFn(fetcher, owner, name); gerr != nil {
		return gerr
	}

	// EditChange replaces the body and, when --title is given, the title. Only the surfaces
	// this verb edits are sent — the body always, the title only when asked for — so an
	// omitted --title leaves the current title (and the `Draft:` prefix it may carry) untouched.
	changed := []string{"body"}
	editIn := deskkit.EditChangeInput{Body: string(body)}
	if *title != "" {
		editIn.Title = *title
		changed = append(changed, "title")
	}
	if eErr := fg.EditChange(fr, pr.Number, editIn); eErr != nil {
		return deskkit.Unverifiable("edit change failed", eErr)
	}
	ac.detail = "edited " + strings.Join(changed, "+") + " of " + pr.URL

	// The announcement. A body/title edit moves no head SHA, so a head-keyed review monitor
	// records no event for it and the correction is invisible to the loop that must act on
	// it. This comment is that event.
	if _, cErr := fg.PostComment(fr, pr.Number, reviewNotice(changed)); cErr != nil {
		// The edit LANDED. Say so plainly in the same breath as the failure, because a
		// caller that reads this as "the edit failed" would re-run — harmless (the
		// idempotency noop above catches it) but a wasted lap — while a caller that reads
		// exit 0 would never learn the review desk was not told. Exit 6 with both facts is
		// the only reading that leaves nothing silent.
		ac.detail += " — re-review notice FAILED"
		return deskkit.Unverifiable(
			"the body/title edit LANDED at "+pr.URL+", but the re-review notice comment could NOT be posted. "+
				"An edit moves no head SHA, so nothing else will tell the review desk this PR changed — post the "+
				"notice by hand, or re-run this verb once the forge is reachable", cErr)
	}
	fmt.Println(pr.URL)
	return nil
}

// trailerLink renders a body's single link trailer in one comparable form —
// `Brief: <value>` or `Issue: #<n>` — and reports whether the body carries one at all.
//
// It is deliberately TOTAL where requireTrailer refuses: a body with no trailer, a
// multiplicity error, or the machine-written scan-carrier marker all come back
// (`""`, false), meaning "this body asserts no link". The caller pairs it with
// requireTrailer, which has already refused every one of those shapes on the REPLACEMENT
// body; here the false result can only describe the PR's CURRENT body, where it means the
// migration case (a pre-trailer PR may gain a link) rather than a violation.
func trailerLink(body []byte) (string, bool) {
	if strings.HasPrefix(strings.TrimLeft(string(body), " \t\r\n"), deskkit.ScanBodyMarker) {
		return "", false
	}
	trs, err := deskkit.ParseTrailers(body)
	if err != nil || len(trs) == 0 {
		return "", false
	}
	if trs[0].Kind == deskkit.TrailerIssue {
		return "Issue: #" + trs[0].Value, true
	}
	return "Brief: " + trs[0].Value, true
}

// reviewNotice builds the comment the verb posts after a successful edit.
//
// The text is a fixed template plus the literal words "body" and "title": no
// caller-supplied string reaches it — not the new title, not the old one, not the body.
// That is why it is not itself put through the body scan. A generated notice carrying no
// caller content has no surface to smuggle anything through, and running a NON-overridable
// scan over text the caller cannot change could only ever strand the verb on its own
// output.
func reviewNotice(changed []string) string {
	return fmt.Sprintf(
		"PR %s edited by %s — re-review requested.\n\n"+
			"A body/title edit moves no head SHA, so a review monitor keyed on the head records no event for it. "+
			"This comment is that event: the PR's description changed, the commits did not.\n\n"+
			"Posted by `deskpr edit`, which ran the same trailer, secret-scan, self-containment and public-repo "+
			"gates as `deskpr create` and wrote an audit row.\n",
		strings.Join(changed, " and "), editActor())
}

// editActor names the identity the comment is posted under, from the loop identity this
// session presents. Since the ambient `--as-app=false` path is retired, the edit always posts
// as the minted session-role App; on an unmapped loop it falls back to the neutral "the desk"
// rather than naming a role this session may not be acting as. A comment that misnames its own
// author is worse than one that is vague about it.
func editActor() string {
	if role, _, err := deskkit.SessionTokenRole("deskpr"); err == nil {
		return "the " + role + " App"
	}
	return "the desk"
}
