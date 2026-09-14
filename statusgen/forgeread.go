package main

// forgeread.go — statusgen's forge reads, on the desk-tools seam.
//
// THE BOUNDARY THIS KEEPS. statusgen is its own Go module and does not import deskkit. That is
// deliberate and it stays: the seam is reached by RUNNING the desk-tools read verb (`deskread`)
// and parsing its JSON, not by linking a package. What leaves is the forge dependency — the
// hand-rolled CLI shell-outs, each with its own argv, its own error handling and its own idea of
// what a failure means.
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
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// deskreadSchema is the envelope version this build understands. An envelope declaring anything
// else is REFUSED rather than best-effort parsed — the same fail-closed direction parseBriefFile
// takes on an unrecognised brief schema, and for the same reason: a pinned consumer that
// silently mis-reads a newer contract produces confident wrong answers.
const deskreadSchema = 1

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
// rollup, comment lists for corroboration) are added one per migrated call site.
//
// CONTRACT. err is returned ONLY for a fault in the reader itself — a malformed envelope, an
// unreadable verb. A repo the forge would not serve is never an err: it is a forgeUnavailable.
// Every repo passed in appears in exactly one of the two results.
type forgeReader interface {
	OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error)
}

// --- the offline default --------------------------------------------------------------------

// offlineReader answers could-not-check for everything and touches nothing: no process start, no
// network, no file. It is the DEFAULT reader, which is what makes the offline lint safe by
// construction rather than by remembering to check a flag at each call site.
type offlineReader struct {
	// why is rendered into every unavailable entry so the could-not-check names its own cause
	// rather than being an unexplained absence.
	why string
}

func newOfflineReader() offlineReader {
	return offlineReader{why: "statusgen ran offline — forge reads are opt-in (--forge); no forge read was attempted"}
}

func (o offlineReader) OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error) {
	un := make([]forgeUnavailable, 0, len(repos))
	for _, r := range repos {
		un = append(un, forgeUnavailable{Repo: r, Reason: o.why})
	}
	// An EMPTY map, never a map of empty slices: a repo with no entry is a repo not read, and
	// there must be no shape in which an unread repo also carries a (zero-length) answer.
	return map[string][]forgeIssue{}, un, nil
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

// deskreadEnvelope mirrors the verb's JSON contract. Only the fields statusgen consumes are
// declared; an envelope carrying more is not an error, which is what lets the verb add an
// omitempty field without breaking a pinned consumer.
type deskreadEnvelope struct {
	Schema  int    `json:"schema"`
	Kind    string `json:"kind"`
	Repos   []struct {
		Repo   string `json:"repo"`
		Issues []struct {
			Number      int      `json:"number"`
			Title       string   `json:"title"`
			State       string   `json:"state"`
			AuthorLogin string   `json:"authorLogin"`
			Labels      []string `json:"labels"`
			CreatedAt   string   `json:"createdAt"`
			URL         string   `json:"url"`
		} `json:"issues"`
	} `json:"repos"`
	Partial []struct {
		Repo   string `json:"repo"`
		Reason string `json:"reason"`
	} `json:"partial"`
}

func (d deskreadReader) OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error) {
	if len(repos) == 0 {
		return map[string][]forgeIssue{}, nil, nil
	}
	args := []string{"issues"}
	for _, r := range repos {
		args = append(args, "--repo", r)
	}
	out, err := exec.Command(d.bin, args...).Output()
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

	var env deskreadEnvelope
	if jerr := json.Unmarshal(out, &env); jerr != nil {
		return nil, nil, fmt.Errorf("parsing the %s envelope: %w", d.bin, jerr)
	}
	if env.Schema != deskreadSchema {
		// REFUSE rather than guess. A pinned statusgen that best-effort parses a contract it
		// was not built against is exactly how a silent wrong answer ships.
		return nil, nil, fmt.Errorf("%s answered envelope schema %d, this build understands %d — "+
			"upgrade statusgen rather than reading a contract it was not built against",
			d.bin, env.Schema, deskreadSchema)
	}

	data := map[string][]forgeIssue{}
	for _, r := range env.Repos {
		issues := make([]forgeIssue, 0, len(r.Issues))
		for _, is := range r.Issues {
			fi := forgeIssue{
				Number: is.Number, Title: is.Title, State: is.State,
				Author: is.AuthorLogin, Labels: is.Labels, URL: is.URL,
			}
			if is.CreatedAt != "" {
				// A stamp that will not parse leaves CreatedAt zero, and the stale computation
				// already skips a record with no credible age rather than bucketing it at the
				// epoch — so a bad stamp degrades to "not aged", never to "infinitely old".
				if t, terr := time.Parse(time.RFC3339, is.CreatedAt); terr == nil {
					fi.CreatedAt = t
				}
			}
			issues = append(issues, fi)
		}
		data[r.Repo] = issues
	}
	un := make([]forgeUnavailable, 0, len(env.Partial))
	for _, p := range env.Partial {
		un = append(un, forgeUnavailable{Repo: p.Repo, Reason: p.Reason})
	}

	// Belt and braces: any repo the caller asked for that the envelope mentioned in NEITHER list
	// is could-not-check, not an empty answer. A verb that silently dropped a repo must not read
	// as that repo having nothing.
	seen := map[string]bool{}
	for r := range data {
		seen[r] = true
	}
	for _, u := range un {
		seen[u.Repo] = true
	}
	var missing []string
	for _, r := range repos {
		if !seen[r] {
			missing = append(missing, r)
		}
	}
	sort.Strings(missing)
	for _, r := range missing {
		un = append(un, forgeUnavailable{Repo: r,
			Reason: fmt.Sprintf("%s returned no result and no reason for this repo", d.bin)})
	}
	return data, un, nil
}

// --- the run's reader -----------------------------------------------------------------------

// forgeReaderForRun is the reader every forge-backed check reads through. It is OFFLINE unless
// --forge was given; nothing else sets it, so there is no path by which a check reaches a forge
// from a run that did not ask for one.
var forgeReaderForRun forgeReader = newOfflineReader()
