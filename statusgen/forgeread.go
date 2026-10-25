package main

// forgeread.go — statusgen's forge reads, on the desk-tools seam.
//
// THE BOUNDARY THIS KEEPS. statusgen is its own Go module and does not import deskkit. That is
// deliberate and it stays: every ONLINE forge read is reached by RUNNING the desk-tools read verb
// (`deskread`) and parsing its JSON, not by linking a package. What leaves is the forge dependency
// — the hand-rolled CLI shell-outs, each with its own argv, its own error handling and its own
// idea of what a failure means.
//
// WHAT IS LINKED. The one thing statusgen links is the ROOT package of the forgeread module: the
// shared issue/unavailable types, the envelope parser (fail-closed on an unknown schema, every
// requested repo exactly once) and the offline and frozen-result readers. That package imports no
// subpackage, no net/http and no os/exec, so linking it adds no way to reach a forge and no
// credential. The authenticated adapters live below it and are linked by desk-tools alone; a
// statusgen import of any of them is a counted failure (forgeread_sdk_test.go, the guard row).
//
// THE DEFAULT IS OFFLINE, AND THAT IS THE WHOLE SAFETY ARGUMENT. `forgeReader` is a three-state
// instrument by construction: every method answers with the data it read AND the repos it could
// NOT read, and the two sets are disjoint. The DEFAULT implementation is `offlineReader`, which
// reports every repo as could-not-check and starts no process. A caller that has not thought
// about the unavailable list gets an empty data map and a full unavailable list — it cannot
// mistake "did not look" for "looked and found nothing", because there is no shape in which the
// unavailable repos also appear as empty results.
//
// This is the failure the whole brief exists to prevent, so it is worth being explicit about the
// direction: a lint that got faster by checking less, and said so nowhere, is strictly worse than
// the slow lint it replaced.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/medici-finance/assay/forgeread"
)

// deskreadSchema is the envelope version this build understands. An envelope declaring anything
// else is REFUSED rather than best-effort parsed — the same fail-closed direction parseBriefFile
// takes on an unrecognised brief schema, and for the same reason: a pinned consumer that
// silently mis-reads a newer contract produces confident wrong answers.
const deskreadSchema = forgeread.EnvelopeSchema

// forgeIssue is one open issue, in the fields statusgen's checks actually consume. It is
// deliberately NOT a copy of any forge's own issue shape: the verb has already reduced both
// backends to one, and re-widening it here would put forge knowledge back in this module.
type forgeIssue struct {
	Number    int
	Title     string
	State     string
	Author    string
	Labels    []string
	CreatedAt time.Time
	URL       string
}

// forgeUnavailable is one repo that could NOT be read, and why. It is a first-class RESULT, not
// an error: a set in which two repos of ten were unreadable has answered eight questions and
// declined two, and a caller must be able to act on exactly that.
type forgeUnavailable struct {
	Repo   string
	Reason string
}

// forgeReader is the seam statusgen's forge-backed checks read through.
//
// The method set is derived from the call sites, never invented: a method exists here only once
// something consumes it, mirroring the freeze rule the desk-tools interface itself follows. This
// slice lands OpenIssues; the remaining kinds (merged-change lists, a change's head/reviews/check
// rollup, comment lists for corroboration) are added one per migrated call site. The per-ISSUE
// reads --scan-issues consumes (IssueTrust, IssueComments, below) are methods on deskreadReader
// only: --scan-issues is not a --forge-gated check, so there is no offline twin for them to
// answer through.
//
// CONTRACT. err is returned ONLY for a fault in the reader itself — a malformed envelope, an
// unreadable verb. A repo the forge would not serve is never an err: it is a forgeUnavailable.
// Every repo passed in appears in exactly one of the two results.
type forgeReader interface {
	OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error)
}

// --- the offline default --------------------------------------------------------------------

// sdkReader adapts a forgeread.Reader (the SDK's offline or frozen-result reader) to statusgen's
// forgeReader seam, converting the shared types to statusgen's own. It is the ONLY direct-library
// path: those readers start no process and make no request, which is why linking them is safe.
type sdkReader struct {
	r forgeread.Reader
}

func (s sdkReader) OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error) {
	res, err := s.r.OpenIssues(context.Background(), repos)
	if err != nil {
		return nil, nil, err
	}
	data, un := fromSDKResult(res)
	return data, un, nil
}

// fromSDKResult converts the shared three-state Result to statusgen's types. An unavailable
// repo gets no data entry, and a read repo keeps its (possibly empty) slice.
func fromSDKResult(res forgeread.Result) (map[string][]forgeIssue, []forgeUnavailable) {
	data := map[string][]forgeIssue{}
	for repo, issues := range res.Issues {
		out := make([]forgeIssue, 0, len(issues))
		for _, is := range issues {
			// No labels is nil on every path (the wire omits an empty list), so the same issue
			// compares equal whether it arrived through the library or through the verb.
			var labels []string
			if len(is.Labels) > 0 {
				labels = is.Labels
			}
			out = append(out, forgeIssue{
				Number: is.Number, Title: is.Title, State: is.State,
				Author: is.AuthorLogin, Labels: labels, URL: is.URL,
				// A stamp that will not parse leaves CreatedAt zero, and the stale computation
				// already skips a record with no credible age rather than bucketing it at the
				// epoch — so a bad stamp degrades to "not aged", never to "infinitely old".
				CreatedAt: is.Created(),
			})
		}
		data[repo] = out
	}
	un := make([]forgeUnavailable, 0, len(res.Unavailable))
	for _, u := range res.Unavailable {
		un = append(un, forgeUnavailable{Repo: u.Repo, Reason: u.Reason})
	}
	return data, un
}

// newOfflineReader is the DEFAULT reader: the SDK's offline reader, which reports every repo as
// could-not-check and starts nothing. The reason names the opt-in, so the absence explains itself.
func newOfflineReader() forgeReader {
	return sdkReader{r: forgeread.OfflineReader{
		Why: "statusgen ran offline — forge reads are opt-in (--forge); no forge read was attempted"}}
}

// newFrozenReader answers from an already-obtained Result (a recorded snapshot): no read, and a
// repo the snapshot does not cover is unavailable, never empty.
func newFrozenReader(res forgeread.Result) forgeReader {
	return sdkReader{r: forgeread.NewFrozenReader(res)}
}

// --- the desk-tools-backed reader -----------------------------------------------------------

// deskreadReader runs the desk-tools read verb ONCE for the whole repo set and parses its
// envelope. One invocation per read kind per repo SET is the point, not an optimisation detail:
// the shape it replaces was one process start, one token resolution and one sequential round trip
// PER REPO.
type deskreadReader struct {
	// bin is the verb to run; overridden in tests with a stub on PATH.
	bin string
	// timeout bounds the whole set read.
	timeout time.Duration
}

func newDeskreadReader() deskreadReader {
	return deskreadReader{bin: "deskread", timeout: 90 * time.Second}
}

func (d deskreadReader) OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error) {
	if len(repos) == 0 {
		return map[string][]forgeIssue{}, nil, nil
	}
	args := []string{"issues"}
	for _, r := range repos {
		args = append(args, "--repo", r)
	}
	// The deskread phase times the child; perfChildArgs hands it this run's context (perfrecord.go).
	argv := perfChildArgs(d.bin, args) // before the phase: the one-time probe is not the child's wait
	leave := perfEnter("deskread")
	out, err := exec.Command(d.bin, argv...).Output()
	leave()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = firstLine(string(ee.Stderr))
		}
		// The verb could not be run, or answered exit 6 (nothing readable). Either way this is
		// could-not-check for the WHOLE set — reported as such per repo, never as an empty answer.
		reason := fmt.Sprintf("%s issues: %v %s", d.bin, err, detail)
		un := make([]forgeUnavailable, 0, len(repos))
		for _, r := range repos {
			un = append(un, forgeUnavailable{Repo: r, Reason: strings.TrimSpace(reason)})
		}
		return map[string][]forgeIssue{}, un, nil
	}

	res, perr := forgeread.ParseIssuesEnvelope(out, repos)
	if perr != nil {
		if errors.Is(perr, forgeread.ErrUnsupportedSchema) {
			// REFUSE rather than guess. A pinned statusgen that best-effort parses a contract it
			// was not built against is exactly how a silent wrong answer ships.
			return nil, nil, fmt.Errorf("%s answered an envelope this build does not understand (%v) — "+
				"upgrade statusgen rather than reading a contract it was not built against", d.bin, perr)
		}
		return nil, nil, fmt.Errorf("parsing the %s envelope: %w", d.bin, perr)
	}
	data, un := fromSDKResult(res)
	return data, un, nil
}

// --- the run's reader -----------------------------------------------------------------------

// forgeReaderForRun is the reader every forge-backed check reads through. It is OFFLINE unless
// --forge was given; nothing else sets it, so there is no path by which a check reaches a forge
// from a run that did not ask for one.
var forgeReaderForRun forgeReader = newOfflineReader()

// --- the per-issue kinds (--scan-issues' trust gate and un-block lane) ------------------------

// deskreadItemEnvelope mirrors the verb's per-issue contract (`deskread trust|comments --issue
// owner/name#N`). Only the consumed fields are declared.
type deskreadItemEnvelope struct {
	Schema int    `json:"schema"`
	Kind   string `json:"kind"`
	Items  []struct {
		Repo   string `json:"repo"`
		Number int    `json:"number"`
		Trust  *struct {
			BodyEditedAt string `json:"bodyEditedAt"`
			Complete     bool   `json:"complete"`
			Events       []struct {
				AuthorLogin string `json:"authorLogin"`
				AuthorID    int64  `json:"authorId"`
				CreatedAt   string `json:"createdAt"`
				EditedAt    string `json:"editedAt"`
			} `json:"events"`
		} `json:"trust"`
		Comments *[]struct {
			AuthorLogin string `json:"authorLogin"`
			AuthorID    int64  `json:"authorId"`
			CreatedAt   string `json:"createdAt"`
			Body        string `json:"body"`
		} `json:"comments"`
		// LabelEvents is the `label-events` kind. Complete is a pointer on purpose: an item that
		// carries no completeness field at all is not a history this build can trust, and
		// decoding it as false-or-true would be a guess either way.
		LabelEvents *struct {
			Complete *bool `json:"complete"`
			Events   []struct {
				Label      string `json:"label"`
				Removed    bool   `json:"removed"`
				CreatedAt  string `json:"createdAt"`
				ActorLogin string `json:"actorLogin"`
				ActorID    int64  `json:"actorId"`
			} `json:"events"`
		} `json:"labelEvents"`
	} `json:"items"`
	Partial []struct {
		Repo   string `json:"repo"`
		Number int    `json:"number"`
		Reason string `json:"reason"`
	} `json:"partial"`
}

// readItem runs `deskread <kind> --issue <repo>#<n>` and returns the envelope's entry for that
// ONE issue. Every way the issue went unread — the verb would not run, it answered no envelope,
// it listed the issue in `partial`, or it mentioned the issue nowhere — is an ERROR, never a
// zero-value answer: the callers (the trust gate, the un-block lane) already degrade an error to
// a could-not-check NOTICE, and a zero value would read as "not blessed" / "nobody answered".
func (d deskreadReader) readItem(kind, repo string, number int) (*deskreadItemEnvelope, int, error) {
	target := fmt.Sprintf("%s#%d", repo, number)
	argv := perfChildArgs(d.bin, []string{kind, "--issue", target}) // probe outside the phase
	leave := perfEnter("deskread")
	out, err := exec.Command(d.bin, argv...).Output()
	leave()
	var env deskreadItemEnvelope
	jerr := json.Unmarshal(out, &env)
	if jerr == nil && env.Schema != deskreadSchema {
		return nil, 0, fmt.Errorf("%s answered envelope schema %d, this build understands %d — "+
			"upgrade statusgen rather than reading a contract it was not built against",
			d.bin, env.Schema, deskreadSchema)
	}
	if jerr == nil {
		// A parsed envelope is authoritative even on a non-zero exit (the verb exits 6 when its
		// one issue was unreadable, with the reason in `partial`).
		for _, p := range env.Partial {
			if p.Repo == repo && p.Number == number {
				return nil, 0, fmt.Errorf("%s %s --issue %s: could-not-check: %s", d.bin, kind, target, p.Reason)
			}
		}
		for i, it := range env.Items {
			if it.Repo == repo && it.Number == number {
				return &env, i, nil
			}
		}
	}
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = firstLine(string(ee.Stderr))
		}
		return nil, 0, fmt.Errorf("%s %s --issue %s: could-not-check: %v %s", d.bin, kind, target, err, strings.TrimSpace(detail))
	}
	if jerr != nil {
		return nil, 0, fmt.Errorf("parsing the %s %s envelope for %s: %w", d.bin, kind, target, jerr)
	}
	return nil, 0, fmt.Errorf("%s %s --issue %s: could-not-check: the envelope returned no result and no reason for this issue",
		d.bin, kind, target)
}

// issueTrustRead is one issue's trust-gate content, as the native forge served it: the body's
// content-edit time, every comment's author identity and times, and whether the thread fit in
// the single bounded page (complete=false fails the blessing closed).
type issueTrustRead struct {
	BodyEdited time.Time
	Events     []blessEvent
	Complete   bool
}

// IssueTrust is the `trust` kind: deskkit.Forge.IssueTrustEvents for one issue.
func (d deskreadReader) IssueTrust(repo string, number int) (issueTrustRead, error) {
	env, i, err := d.readItem("trust", repo, number)
	if err != nil {
		return issueTrustRead{}, err
	}
	tr := env.Items[i].Trust
	if tr == nil {
		return issueTrustRead{}, fmt.Errorf("%s trust --issue %s#%d: could-not-check: the item carries no trust payload", d.bin, repo, number)
	}
	parse := func(what, s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil
		}
		t, perr := time.Parse(time.RFC3339Nano, s)
		if perr != nil {
			return time.Time{}, fmt.Errorf("bad %s %q in the %s trust envelope for %s#%d: %w", what, s, d.bin, repo, number, perr)
		}
		return t, nil
	}
	out := issueTrustRead{Complete: tr.Complete}
	if out.BodyEdited, err = parse("bodyEditedAt", tr.BodyEditedAt); err != nil {
		return issueTrustRead{}, err
	}
	for _, e := range tr.Events {
		created, cerr := parse("createdAt", e.CreatedAt)
		if cerr != nil {
			return issueTrustRead{}, cerr
		}
		edited, eerr := parse("editedAt", e.EditedAt)
		if eerr != nil {
			return issueTrustRead{}, eerr
		}
		out.Events = append(out.Events, blessEvent{login: e.AuthorLogin, id: e.AuthorID, created: created, edited: edited})
	}
	return out, nil
}

// IssueComments is the `comments` kind: deskkit.Forge.ListCommentsTyped on one issue, the whole
// thread oldest first. The author TYPE is not on the wire; a GitHub App author arrives already
// rendered "<slug>[bot]", which isBotComment recognises by that suffix.
func (d deskreadReader) IssueComments(repo string, number int) ([]issueComment, error) {
	env, i, err := d.readItem("comments", repo, number)
	if err != nil {
		return nil, err
	}
	cs := env.Items[i].Comments
	if cs == nil {
		return nil, fmt.Errorf("%s comments --issue %s#%d: could-not-check: the item carries no comment list", d.bin, repo, number)
	}
	out := make([]issueComment, 0, len(*cs))
	for _, c := range *cs {
		out = append(out, issueComment{
			User:      issueCommentUser{Login: c.AuthorLogin, ID: c.AuthorID},
			CreatedAt: c.CreatedAt,
			Body:      c.Body,
		})
	}
	return out, nil
}

// --- the bounded issue list (--decision-latency, --assayscore, --issues) ----------------------

// deskreadListEnvelope mirrors the verb's `issue-list` contract (`deskread issue-list --repo
// owner/name --state open|closed|all [--label L]`). Only the consumed fields are declared, and
// the three a missing value would turn into a guess are pointers: an entry with no issue list
// or no completeness field is not an answer this build can trust.
type deskreadListEnvelope struct {
	Schema int    `json:"schema"`
	Kind   string `json:"kind"`
	Repos  []struct {
		Repo       string             `json:"repo"`
		Issues     *[]forgeread.Issue `json:"issues"`
		Incomplete *bool              `json:"incomplete"`
		PageCap    int                `json:"pageCap"`
	} `json:"repos"`
	Partial []struct {
		Repo   string `json:"repo"`
		Reason string `json:"reason"`
	} `json:"partial"`
}

// IssueList is the `issue-list` kind: deskkit.Forge.ListIssues for ONE repo, at one stated
// state and at most one label. It is a deskreadReader method only, like IssueTrust: its callers
// are report modes, not --forge-gated checks, so there is no offline twin to answer through.
//
// Every way the list went unread is an ERROR, never a short or empty list: the verb would not
// run and answered no envelope, it named the repo in `partial`, it answered no entry for the
// repo, the entry carries no issue list or no completeness field, or the walk hit its page
// bound with the forge still paginating (incomplete). The last is the one the `gh issue list
// --limit 1000` reads this replaces could not see at all: they stopped at 1000 issues and said
// nothing, so a larger queue read as a complete one.
func (d deskreadReader) IssueList(repo, state, label string) ([]forgeread.Issue, error) {
	args := []string{"issue-list", "--repo", repo, "--state", state}
	if label != "" {
		args = append(args, "--label", label)
	}
	what := fmt.Sprintf("%s issue-list --repo %s --state %s", d.bin, repo, state)
	if label != "" {
		what += " --label " + label
	}
	argv := perfChildArgs(d.bin, args) // probe outside the phase
	leave := perfEnter("deskread")
	out, err := exec.Command(d.bin, argv...).Output()
	leave()
	var env deskreadListEnvelope
	jerr := json.Unmarshal(out, &env)
	if jerr == nil && env.Schema != deskreadSchema {
		return nil, fmt.Errorf("%s answered envelope schema %d, this build understands %d — "+
			"upgrade statusgen rather than reading a contract it was not built against",
			d.bin, env.Schema, deskreadSchema)
	}
	if jerr == nil && env.Kind != "issue-list" {
		return nil, fmt.Errorf("%s: could-not-check: the envelope is kind %q, not \"issue-list\"", what, env.Kind)
	}
	if jerr == nil {
		// A parsed envelope is authoritative even on a non-zero exit (the verb exits 6 when its
		// one repo was unreadable, with the reason in `partial`).
		for _, p := range env.Partial {
			if p.Repo == repo {
				return nil, fmt.Errorf("%s: could-not-check: %s", what, p.Reason)
			}
		}
		for _, r := range env.Repos {
			if r.Repo != repo {
				continue
			}
			switch {
			case r.Issues == nil:
				return nil, fmt.Errorf("%s: could-not-check: the entry carries no issue list", what)
			case r.Incomplete == nil:
				return nil, fmt.Errorf("%s: could-not-check: the entry carries no completeness field", what)
			case *r.Incomplete:
				return nil, fmt.Errorf("%s: could-not-check: the list stopped at its %d-page bound with the forge still paginating, so it is not provably complete", what, r.PageCap)
			}
			return *r.Issues, nil
		}
	}
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = firstLine(string(ee.Stderr))
		}
		return nil, fmt.Errorf("%s: could-not-check: %v %s", what, err, strings.TrimSpace(detail))
	}
	if jerr != nil {
		return nil, fmt.Errorf("parsing the %s envelope: %w", what, jerr)
	}
	return nil, fmt.Errorf("%s: could-not-check: the envelope returned no result and no reason for this repo", what)
}

// labelEvent is one label application or removal on an issue, as the forge served it. Label is
// empty for an event whose label the forge no longer names; Login is empty and ID 0 where the
// forge reported no actor; At is the forge's own timestamp text, parsed only where the hold rule
// has to compare it. Nothing here says who the actor is.
type labelEvent struct {
	Label   string
	Removed bool
	At      string
	Login   string
	ID      int64
}

// labelHistoryRead is one issue's whole label history, oldest first, and whether the forge
// served all of it. Complete=false means a page bound was hit: the events are a prefix.
type labelHistoryRead struct {
	Events   []labelEvent
	Complete bool
}

// IssueLabelHistory is the `label-events` kind: deskkit.Forge.IssueLabelHistory for one issue.
// A history this build cannot trust is an ERROR, never a short or empty answer: the verb
// listing the issue in `partial`, answering no item, an item with no completeness field, a
// non-zero exit, and a verb that rejects the kind all land in readItem's error or the checks
// below. An item marked incomplete is NOT an error — it decodes as incomplete and the hold rule
// holds the issue as could-not-check.
func (d deskreadReader) IssueLabelHistory(repo string, number int) (labelHistoryRead, error) {
	env, i, err := d.readItem("label-events", repo, number)
	if err != nil {
		return labelHistoryRead{}, err
	}
	le := env.Items[i].LabelEvents
	if le == nil {
		return labelHistoryRead{}, fmt.Errorf("%s label-events --issue %s#%d: could-not-check: the item carries no label history", d.bin, repo, number)
	}
	if le.Complete == nil {
		return labelHistoryRead{}, fmt.Errorf("%s label-events --issue %s#%d: could-not-check: the label history carries no completeness field", d.bin, repo, number)
	}
	out := labelHistoryRead{Complete: *le.Complete, Events: make([]labelEvent, 0, len(le.Events))}
	for _, e := range le.Events {
		out.Events = append(out.Events, labelEvent{Label: e.Label, Removed: e.Removed, At: e.CreatedAt, Login: e.ActorLogin, ID: e.ActorID})
	}
	return out, nil
}
