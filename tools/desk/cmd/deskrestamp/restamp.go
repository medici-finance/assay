package main

// restamp.go — the re-stamp verb itself.
//
// THE ORDER IS THE CONTROL:
//
//  1. parse + repo gate                          (local; no forge)
//  2. resolve the session role                   (local; no forge) — and REFUSE it unless
//     it is one of deskkit.DispatcherRoles(): the re-applied pair must stand under an
//     identity the floor accepts, and a non-dispatcher role would only mint a second
//     unreadable stamp.
//  3. resolve the forge under that role's custody, minting the role's own token
//  4. read the PR's current labels + label timeline (UNVERIFIABLE on failure — a blind
//     re-stamp is the no-op-and-still-refused state this verb exists to end)
//  5. the CONTENT gate: the present stamp must read as one complete (model, tier) pair.
//     This verb preserves content; it never invents it.
//  6. compute the removal set with deskkit.ReStampRemovals under the SAME authority
//     predicate the floor reads (deskkit.IsStampAuthorityLogin) — reader and writer
//     project the standing-applier resolution from one place, so they cannot disagree.
//  7. no-op when nothing needs removing and the pair is present; stop on --dry-run
//  8. write budget; REMOVE (one write), then APPLY the pair (a second write) — two
//     events, because a label named in both halves of one reconciliation is skipped and
//     the standing applier would not change.
//  9. post the record comment naming both actors (#336's recorded-act requirement).

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskrestamp"

// nowFunc is the clock seam; tests pin it so the audit timestamp is deterministic.
var nowFunc = time.Now

// roleFn resolves the App role this session acts under. Production binds it to
// sessionRole (DESK_LOOP → role via deskkit.SessionTokenRole, no default); a test
// substitutes a fixed role.
var roleFn = sessionRole

// sessionRole reads the session's App role. There is NO default and NO override flag: the
// re-stamp must be written under an identity the floor accepts, and a claimed role would
// be exactly the hole the dispatcher check exists to close.
func sessionRole() (string, error) {
	role, _, err := deskkit.SessionTokenRole(toolName)
	if err != nil {
		return "", deskkit.Refused(
			"refused: deskrestamp could not resolve which App role this session acts under (" + err.Error() +
				") — a re-stamp must be written under a bound dispatcher identity read from the SESSION, " +
				"and no flag asserts one. Run inside a booted desk window (DESK_LOOP set) whose role is one " +
				"of the dispatching roles.")
	}
	return role, nil
}

// request is the parsed invocation.
type request struct {
	repo   string
	number int
	dryRun bool
}

// parseArgs accepts flags before, between or after the two positionals
// (`<owner/repo> <pr>`), so `--dry-run` can trail the way an operator types it.
func parseArgs(args []string) (request, error) {
	fs := flag.NewFlagSet("deskrestamp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "read the PR and its label timeline, print the plan, write nothing")
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return request{}, deskkit.Refused("deskrestamp: " + err.Error())
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
	if len(pos) != 2 {
		return request{}, deskkit.Refused(fmt.Sprintf(
			"deskrestamp: want exactly two positionals <owner/repo> <pr>, got %d", len(pos)))
	}
	req := request{repo: strings.TrimSpace(pos[0]), dryRun: *dryRun}
	owner, name, ok := strings.Cut(req.repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return request{}, deskkit.Refused(fmt.Sprintf("deskrestamp: repo %q is not <owner/repo>", deskkit.StripControl(req.repo)))
	}
	numText := strings.TrimLeft(strings.TrimSpace(pos[1]), "#!")
	n, err := strconv.Atoi(numText)
	if err != nil || n <= 0 {
		return request{}, deskkit.Refused(fmt.Sprintf("deskrestamp: number %q is not a positive integer", deskkit.StripControl(pos[1])))
	}
	req.number = n
	return req, nil
}

// requireRepo enforces the allowed-repo set. This tool introduces NO repo list of its own:
// the roster is the single declared source, and an unconfigured roster refuses.
func requireRepo(repo string) error {
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused(fmt.Sprintf("repo %q is outside the desk-tools repo set", deskkit.StripControl(repo)))
	}
	return nil
}

// requireDispatcherRole refuses any session role the floor would not accept as a stamp
// authority. The re-applied pair's standing application belongs to THIS role's App; a role
// outside deskkit.DispatcherRoles() writes a stamp the floor refuses, which is strictly
// worse than refusing here — the PR would carry a fresh unreadable stamp instead of the
// old one.
func requireDispatcherRole(role string) error {
	for _, r := range deskkit.DispatcherRoles() {
		if r == role {
			return nil
		}
	}
	return deskkit.Refused(fmt.Sprintf(
		"refused: this session's App role is %q, which is not one of the dispatching roles (%s) — "+
			"a re-stamp must be written under an identity the model-capability floor accepts, and "+
			"the role is read from the session, never from a flag.",
		deskkit.StripControl(role), strings.Join(deskkit.DispatcherRoles(), " | ")))
}

// labelsPresentFold reports whether every wanted label is already on the change, by the
// same case-insensitive comparison the stamp reader uses for label names.
func labelsPresentFold(present, want []string) bool {
	for _, w := range want {
		found := false
		for _, p := range present {
			if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(w)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func cmdReStamp(args []string, out io.Writer) error {
	req, err := parseArgs(args)
	if err != nil {
		return err
	}
	if err := requireRepo(req.repo); err != nil {
		return err
	}

	role, err := roleFn()
	if err != nil {
		return err
	}
	if err := requireDispatcherRole(role); err != nil {
		auditLine(req.repo, req.number, deskkit.ResultRefused, "role: "+err.Error())
		return err
	}
	mintedRole = role

	fg, fr, ferr := forgeForFn(req.repo)
	if ferr != nil {
		return ferr
	}

	// Read the present labels and the label timeline. Both are UNVERIFIABLE on failure:
	// proceeding blind is the apply-over-a-present-label no-op that leaves the PR carrying
	// the stamp the floor refuses — the state this verb exists to end.
	change, perr := fg.GetPullRequest(fr, req.number)
	if perr != nil {
		err := deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read the labels currently on %s#%d (%s) — whether this PR already "+
				"carries a stamp, and which one, is unknown, and a re-stamp preserves content rather than "+
				"inventing it, so there is nothing safe to write.",
			req.repo, req.number, firstLine(perr.Error())), perr)
		auditLine(req.repo, req.number, deskkit.ResultUnverifiable, "read-labels: "+err.Error())
		return err
	}
	events, eerr := fg.ListLabelEvents(fr, req.number)
	if eerr != nil {
		err := deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read the label history of %s#%d (%s) — WHO applied the stamp this "+
				"PR carries cannot be established, so a foreign application could not be replaced.",
			req.repo, req.number, firstLine(eerr.Error())), eerr)
		auditLine(req.repo, req.number, deskkit.ResultUnverifiable, "read-events: "+err.Error())
		return err
	}
	tl := deskkit.StampTimeline{Present: change.Labels, Events: events}

	// THE CONTENT GATE. This verb preserves the stamp the PR already carries; it never
	// invents one. An unreadable or incomplete stamp is the dispatch ceremony's territory
	// (its stamp step validates an explicit --model/--tier and replaces corrupt content) —
	// a verb that rewrote content here would be a laundering path, not a repair.
	stamp, state := deskkit.ModelStampOf(change.Labels)
	if state != deskkit.ModelStamped {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: the dispatched-* labels on %s#%d do not read as ONE complete (model, tier) pair "+
				"(state: %s) — deskrestamp preserves a stamp's content and never invents it. A conflicting, "+
				"malformed or half stamp is repaired by re-running the dispatch ceremony, which validates "+
				"an explicit --model/--tier and re-stamps to THAT.",
			req.repo, req.number, state))
		auditLine(req.repo, req.number, deskkit.ResultRefused, "content: "+state.String())
		return err
	}
	want, werr := deskkit.ModelStampLabels(stamp.Model, stamp.Tier)
	if werr != nil {
		// ModelStampOf returned Stamped, so the pair built from it must validate; an error
		// here means the two readers of the same labels disagree — a defect, not input.
		return deskkit.Unverifiable(fmt.Sprintf(
			"the stamp on %s#%d read as (%s, %s) but will not rebuild as labels (%v) — the two reads "+
				"of the same content disagree", req.repo, req.number, stamp.Model, stamp.Tier, werr), werr)
	}

	// The removal set, under the SAME authority predicate the floor reads. Reader and
	// writer project the standing-applier resolution from one place.
	remove := deskkit.ReStampRemovals(tl, want, deskkit.IsStampAuthorityLogin)
	original := deskkit.NonDispatcherStampAppliers(tl, deskkit.IsStampAuthorityLogin)

	if len(remove) == 0 && labelsPresentFold(change.Labels, want) {
		fmt.Fprintf(out, "noop: %s#%d already carries %s standing under an identity the floor accepts — nothing to repair\n",
			req.repo, req.number, strings.Join(want, " + "))
		auditLine(req.repo, req.number, deskkit.ResultNoop, strings.Join(want, ","))
		return nil
	}

	// THE PROVENANCE GATE (#336, SEC-1/C4). Preserving content is not the same as vouching
	// for whoever wrote it: this verb must never re-attest a pair under the dispatcher when
	// its standing foreign applier is an App/bot, an untrusted login, or a present label the
	// timeline cannot attribute to anyone at all. Only a TRUSTED HUMAN login of the parsed
	// roster (deskkit.IsTrustedHumanLogin — the same set the stamp-authority allowance
	// itself must already draw from) is a foreign applier this verb repairs; anyone else is
	// laundering, not repair, and is refused before any write.
	unvouched := unvouchedForeignAppliers(original, wantLabelsAmong(want, deskkit.UnattributedStampLabels(tl)))
	if len(unvouched) > 0 {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d's standing %s stamp was applied by %s, which this roster does not "+
				"vouch for as a trusted human login — deskrestamp preserves a stamp's content, but "+
				"re-attesting content an unvouched identity applied under the dispatcher is exactly "+
				"the laundering the model-capability floor exists to refuse. This verb repairs only a "+
				"trusted-human-applied stamp (#336); an App/bot, an untrusted login, or a present label "+
				"the timeline cannot attribute is not repaired here — re-run the dispatch ceremony "+
				"instead, which validates an explicit --model/--tier.",
			req.repo, req.number, strings.Join(want, " + "), joinOrNone(unvouched)))
		auditLine(req.repo, req.number, deskkit.ResultRefused, "foreign-applier: "+joinOrNone(unvouched))
		return err
	}

	if req.dryRun {
		fmt.Fprintf(out, "dry-run: would remove %s and re-apply %s on %s#%d as the %s App (previous applier(s): %s)\n",
			joinOrNone(remove), strings.Join(want, " + "), req.repo, req.number, role, joinOrNone(original))
		auditLine(req.repo, req.number, deskkit.ResultDryRun, strings.Join(want, ","))
		return nil
	}

	if err := deskkit.AllowWrite(toolName, req.repo, req.number); err != nil {
		return err
	}

	// The removal is its OWN write, ahead of the application: a label named in both halves
	// of one reconciliation is skipped by the backends, so a single write would leave the
	// forge's label set unchanged — no new application event, and the standing applier
	// would not change. Two writes are two events.
	if len(remove) > 0 {
		if _, aerr := fg.ApplyLabels(fr, req.number, deskkit.LabelChange{
			Target: deskkit.TargetChange,
			Remove: remove,
		}); aerr != nil {
			err := deskkit.Unverifiable(fmt.Sprintf(
				"could not remove the stamp label(s) %s from %s#%d (%s) — re-applying the pair on top "+
					"would be a no-op, leaving the PR carrying the stamp the floor refuses.",
				strings.Join(remove, " + "), req.repo, req.number, firstLine(aerr.Error())), aerr)
			auditLine(req.repo, req.number, deskkit.ResultUnverifiable, "remove: "+err.Error())
			return err
		}
	}
	add := make([]deskkit.LabelSpec, 0, len(want))
	for _, l := range want {
		add = append(add, deskkit.LabelSpec{Name: l, Color: stampLabelColorHex, Description: stampLabelDescription})
	}
	if _, aerr := fg.ApplyLabels(fr, req.number, deskkit.LabelChange{
		Target: deskkit.TargetChange,
		Add:    add,
	}); aerr != nil {
		err := deskkit.Unverifiable(fmt.Sprintf(
			"could not apply %s to %s#%d (%s) — an INCOMPLETE or absent stamp reads as indeterminate, "+
				"and one half of a stamp is worse than no stamp at all.",
			strings.Join(want, " + "), req.repo, req.number, firstLine(aerr.Error())), aerr)
		auditLine(req.repo, req.number, deskkit.ResultUnverifiable, "apply: "+err.Error())
		return err
	}

	// THE RECORD. The ruling this verb serves requires the re-stamp actor and the original
	// actor both recorded on the PR; an `unlabeled` event does not say why the label came
	// off, so the record is a comment, posted once per actual re-stamp.
	comment := fmt.Sprintf(
		"Re-stamped under the %s App: removed %s (previously applied by %s) and re-applied the same "+
			"(model, tier) pair — %s — under this dispatcher identity, so the model-capability floor "+
			"reads the stamp as this App's attestation (#336). This comment is the record of both actors.",
		role, joinOrNone(remove), joinOrNone(original), strings.Join(want, " + "))
	if _, cerr := fg.PostCommentTyped(fr, req.number, deskkit.TargetChange, comment); cerr != nil {
		// The re-stamp ITSELF succeeded; the record did not land. That is not silently
		// droppable — the comment is the audit trail the ruling requires — so it is
		// reported as unverifiable WITH the re-stamp's own outcome stated, never as a
		// clean run and never as a reason to retry the label writes.
		err := deskkit.Unverifiable(fmt.Sprintf(
			"the re-stamp on %s#%d LANDED (%s now stands under the %s App) but the record comment "+
				"could not be posted (%s) — post it by hand: %s",
			req.repo, req.number, strings.Join(want, " + "), role, firstLine(cerr.Error()), comment), cerr)
		auditLine(req.repo, req.number, deskkit.ResultUnverifiable, "comment: "+firstLine(cerr.Error()))
		return err
	}

	fmt.Fprintf(out, "re-stamped: removed %s and applied %s on %s#%d as the %s App (previous applier(s): %s); the record comment is posted\n",
		joinOrNone(remove), strings.Join(want, " + "), req.repo, req.number, role, joinOrNone(original))
	auditLine(req.repo, req.number, deskkit.ResultOK, strings.Join(want, ","))
	return nil
}

// unvouchedForeignAppliers filters foreign (the deskkit.NonDispatcherStampAppliers list)
// plus a count of unattributed WANT labels down to the ones this verb refuses to launder:
// anyone who is not a trusted HUMAN login of the parsed roster. A bot/App rendering and an
// untrusted login both fail deskkit.IsTrustedHumanLogin (fail-closed on a non-matching
// input), and an unattributed present label is rendered as a fixed placeholder before the
// same check, which — naming nobody — also fails it.
func unvouchedForeignAppliers(foreign []string, unattributedPlaceholders []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(who string) {
		if deskkit.IsTrustedHumanLogin(who) {
			return
		}
		if seen[who] {
			return
		}
		seen[who] = true
		out = append(out, who)
	}
	for _, who := range foreign {
		add(who)
	}
	for range unattributedPlaceholders {
		add("(an actor the timeline does not name)")
	}
	sort.Strings(out)
	return out
}

// wantLabelsAmong intersects want with a label set (here, deskkit.UnattributedStampLabels'
// output), case/whitespace-insensitively — a present WANT label the timeline cannot
// attribute to anyone is exactly the "no applier information at all" gap SEC-2 named.
func wantLabelsAmong(want, labels []string) []string {
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[strings.ToLower(strings.TrimSpace(w))] = true
	}
	var out []string
	for _, l := range labels {
		if wantSet[strings.ToLower(strings.TrimSpace(l))] {
			out = append(out, l)
		}
	}
	return out
}

// joinOrNone renders a possibly-empty set for an operator message — an empty removal set
// or an empty previous-applier set must never print as a blank.
func joinOrNone(xs []string) string {
	if len(xs) == 0 {
		return "(none)"
	}
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = deskkit.StripControl(x)
	}
	return strings.Join(out, " + ")
}

func auditLine(repo string, number int, result, detail string) {
	sha, built := deskkit.Version()
	n := number
	_ = deskkit.Log(deskkit.Entry{
		TS:         nowFunc().UTC().Format(time.RFC3339),
		Tool:       toolName,
		Verb:       "restamp",
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
		Repo:       repo,
		PR:         &n,
		Result:     result,
		Detail:     deskkit.StripControl(detail),
		SourceSHA:  sha,
		BuiltAt:    built,
		SessionTag: deskkit.SessionTag(),
	})
}
