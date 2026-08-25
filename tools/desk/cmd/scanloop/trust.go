package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// trust.go — the trust gate, applied BEFORE anything is queued.
//
// This drain reads repos where ARBITRARY external accounts can author issues and comments.
// Unvetted third-party text must never enter the queue or steer a dispatch, so the gate runs at the
// queueing boundary rather than at the dispatch boundary: an item that was never queued cannot be
// dispatched by a later bug, whereas an item queued "for visibility" and filtered downstream is one
// missing branch away from being acted on.
//
// The rule, read from the roster (the blessing authority and the trusted set, resolved through
// deskkit's existing config reader — never from a flag, never from a file a pull request can
// author): an item is ADMITTED when its author is trusted, OR when the blessing authority has
// commented on it and nothing untrusted has moved since. Everything else stays
// QUARANTINED-VISIBLE — listed, counted, never given a lane — so the authority can see what awaits.
// Every direction fails closed: an unset roster admits nobody, and an author we could not read is
// COULD-NOT-CHECK, never a soft admit.

// AdmissionState is the three-state verdict of the gate.
type AdmissionState string

const (
	// AdmissionAdmitted — trusted author, or a standing blessing.
	AdmissionAdmitted AdmissionState = "ADMITTED"
	// AdmissionQuarantined — read cleanly, untrusted, unblessed. Visible and counted; no lane.
	AdmissionQuarantined AdmissionState = "QUARANTINED"
	// AdmissionCouldNotCheck — the gate could not be evaluated. NOT a quarantine and NOT an
	// admission: a run carrying one of these is blind and exits unverifiable rather than reporting
	// a partial queue as the queue.
	AdmissionCouldNotCheck AdmissionState = "COULD-NOT-CHECK"
)

// Admission is one inbound item's gate verdict.
type Admission struct {
	Item   Inbound
	State  AdmissionState
	Author string // rendered login, empty when unread
	Why    string
}

// Admitted reports whether the item may be queued. Only AdmissionAdmitted may.
func (a Admission) Admitted() bool { return a.State == AdmissionAdmitted }

// TrustProbe answers the gate's two questions for one item. Splitting it out as a seam is what lets
// the gate's SEMANTICS be tested against fixtures without a network, and keeps the one binary that
// could dispatch from also being the one that invents its own GitHub reads.
//
// complete=false means the item's comment thread overflowed a single page: the blessing cannot be
// established from a partial thread, so it is treated as unblessed (quarantine) rather than guessed.
type TrustProbe func(repo string, number int) (author string, bodyEdited time.Time, events []deskkit.ContentEvent, complete bool, err error)

// ApplyTrustGate is the queueing boundary. It returns one Admission per inbound item, in input
// order, and never drops a row: a quarantined item that vanished from the output would be
// indistinguishable from an item that never arrived.
func ApplyTrustGate(items []Inbound, probe TrustProbe) []Admission {
	out := make([]Admission, 0, len(items))
	for _, it := range items {
		out = append(out, admitOne(it, probe))
	}
	return out
}

func admitOne(it Inbound, probe TrustProbe) Admission {
	if probe == nil {
		return Admission{
			Item:  it,
			State: AdmissionCouldNotCheck,
			Why: "no trust probe is wired — the author and the blessing are both unread. " +
				"An unread gate admits nothing; it also does not quarantine, because quarantine is a " +
				"verdict about an item we looked at.",
		}
	}
	author, bodyEdited, events, complete, err := probe(it.Repo, it.Number)
	if err != nil {
		return Admission{
			Item:   it,
			State:  AdmissionCouldNotCheck,
			Author: author,
			Why:    "trust read failed: " + err.Error(),
		}
	}
	if deskkit.TrustedAuthor(author) {
		return Admission{Item: it, State: AdmissionAdmitted, Author: author, Why: "trusted author"}
	}
	if !complete {
		return Admission{
			Item: it, State: AdmissionQuarantined, Author: author,
			Why: "untrusted author and the comment thread overflowed a single page — a blessing " +
				"cannot be read off a partial thread, so it fails closed to quarantine",
		}
	}
	if deskkit.Blessed(bodyEdited, events) {
		return Admission{
			Item: it, State: AdmissionAdmitted, Author: author,
			Why: "blessed by " + blessAuthority() + " and nothing untrusted has moved since",
		}
	}
	return Admission{
		Item: it, State: AdmissionQuarantined, Author: author,
		Why: "untrusted author with no standing blessing — visible and counted, never routed; " +
			"one comment from " + blessAuthority() + " admits it",
	}
}

func blessAuthority() string {
	if l := deskkit.BlessAuthorityLogin(); l != "" {
		return l
	}
	return "the configured blessing authority"
}

// AdmissionCounts summarises a gate pass for the one-line report.
func AdmissionCounts(as []Admission) (admitted, quarantined, couldNotCheck int) {
	for _, a := range as {
		switch a.State {
		case AdmissionAdmitted:
			admitted++
		case AdmissionQuarantined:
			quarantined++
		default:
			couldNotCheck++
		}
	}
	return
}

// ghTrustProbe is the production probe: GET-only reads, one cheap author read plus — for untrusted
// authors ONLY — one bounded thread read. A trusted author costs no second call, which is what
// keeps the API growth bounded on a busy scope.
func ghTrustProbe(run func(args ...string) ([]byte, error)) TrustProbe {
	if run == nil {
		run = func(args ...string) ([]byte, error) {
			cmd := exec.Command("gh", args...)
			return cmd.Output()
		}
	}
	return func(repo string, number int) (string, time.Time, []deskkit.ContentEvent, bool, error) {
		out, err := run("issue", "view", strconv.Itoa(number), "-R", repo, "--json", "author")
		if err != nil {
			return "", time.Time{}, nil, false, fmt.Errorf("cannot read the author of %s#%d: %w", repo, number, err)
		}
		var v struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
		}
		if err := json.Unmarshal(out, &v); err != nil {
			return "", time.Time{}, nil, false, fmt.Errorf("cannot parse the author of %s#%d: %w", repo, number, err)
		}
		author := v.Author.Login
		if deskkit.TrustedAuthor(author) {
			// Trusted authors never need the blessing read. complete=true is honest here: the
			// blessing question is not asked, not answered partially.
			return author, time.Time{}, nil, true, nil
		}
		owner, name, ok := strings.Cut(repo, "/")
		if !ok {
			return author, time.Time{}, nil, false, fmt.Errorf("bad repo slug %q", repo)
		}
		raw, err := run("api", "graphql",
			"-f", "query="+deskkit.IssueTrustQuery,
			"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(number))
		if err != nil {
			return author, time.Time{}, nil, false, fmt.Errorf("cannot read the thread of %s#%d: %w", repo, number, err)
		}
		bodyEdited, events, complete, perr := deskkit.ParseIssueTrustPayload(raw)
		if perr != nil {
			return author, time.Time{}, nil, false, fmt.Errorf("cannot parse the thread of %s#%d: %w", repo, number, perr)
		}
		return author, bodyEdited, events, complete, nil
	}
}
