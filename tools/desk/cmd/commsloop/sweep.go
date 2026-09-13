package main

// sweep.go — `commsloop sweep`: the daily lane-violation sweep (#1767's
// standing-guard ruling names a daily audit sweep for full autonomous
// firing), an OUT-OF-BAND reconciler that re-derives, on its own timescale
// and in its own process, three properties every inline guard in this
// package family already enforces IN-BAND at message time:
//
//   - lane legality: is (from.cell/role -> to.cell/role, verb) still allowed
//     by the CURRENT compiled lane ACL. A message legal under an OLDER matrix
//     but illegal now (a live ACL edit narrowed it) is a finding, not
//     silence — this sweep never trusts a cached "it was fine when it
//     landed" verdict.
//   - peer-auth validity: does the message's recorded signed assertion still
//     come from a cell the CURRENT trust store trusts
//     (comms.VerifyIdentityOnly). A revoked cell, a rotated key, or a
//     signature that was simply forged past a defect in the inline gate all
//     surface here even though the inline gate itself cannot see its own
//     blind spot.
//   - firing-record integrity: does every spawned session trace to BOTH a
//     landed/quarantined message it claims to answer AND a legal
//     (action, class, risk) row in the compiled assign table. A session with
//     no accountable routing decision behind it is an "orphan spawn".
//
// THREE-STATE, FAIL CLOSED (C4). Every run reports exactly one of
// checked-clean / checked-failed / could-not-check; an unreadable or
// unparseable input NEVER reads as clean, even when every OTHER input was
// fine — see readJournal's corrupt-line accumulation and Sweep's precedence
// (could-not-check outranks checked-failed outranks checked-clean).
//
// INPUTS — what is real today, and what is not.
//
//  1. held/*.json (commsqueue.ListHeld) is REAL: every quarantined message's
//     FULL envelope, assertion included, is preserved indefinitely. Lane and
//     assertion re-checks against held items need no format change anywhere.
//
//  2. journal.log is commsloop's append-only land record. TODAY (loop.go's
//     Land, VerdictPass branch) every line is the free-text summary
//     `<RFC3339> landed id=... from=cell/role to=cell/role verb=... (...)`,
//     which carries enough for a LANE re-check but no signed assertion and
//     no firing/spawn record — that data does not exist on disk anywhere
//     yet, because Dispatch stays interim (no session is ever actually
//     fired) until the executor dispatch leg's cutover (#1767 ruling 2,
//     brief 08). This sweep additionally understands a richer,
//     forward-compatible JSON-line shape on the SAME file (SweepRecord,
//     below): a line that parses as one is fully reconciled (lane +
//     assertion + spawn-lineage); a line that parses only as the legacy
//     free-text shape is lane-checked and contributes nothing else — never a
//     guess at data that was never recorded. A line that parses as NEITHER
//     is corrupt (could-not-check).
//
//     Upgrading loop.go/commsgw to EMIT SweepRecord lines — once the prose
//     router (05) and the executor dispatch leg (08) land real actions and
//     sessions worth recording — is the natural follow-up; it is out of
//     THIS brief's named scope (`sweep.go`, new, + tests) and is filed as a
//     discovered gap rather than solved here.
//
// SCOPE. `--cell` selects which cell's own sweep this run is (a message is
// in scope if the cell is either its From or To cell; a spawn record is in
// scope if its own Cell field matches). `--since` floors every input by its
// own timestamp; omitted, the whole history is swept.
//
// FINDINGS never mutate anything: this sweep is read-only over held/ and
// journal.log, and routes every finding as a filed issue via the shared
// deskfile-issue-filing convention (commsqueue.RaisedByRole) — never a
// console-only report (silent desk).

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// EnvTrustStore names the JSON {cell: base64-ed25519-pubkey} trust-store file
// the sweep re-verifies recorded assertions against — the SAME shape
// cmd/commsgw/deps.go's LoadTrustStore reads (house-side config, never
// committed publicly; "mechanism public, values house"). Optional: a run
// with no assertions to check needs none, but any assertion present with
// this unset is a could-not-check for THAT check, never a silent skip.
const EnvTrustStore = "ASSAY_COMMS_TRUST_STORE"

// --- structured journal-line shape (forward-compatible; see file doc) -----

// SweepRecordKind is the closed vocabulary of structured journal-line kinds
// this sweep reconciles.
type SweepRecordKind string

const (
	SweepKindLanded      SweepRecordKind = "landed"
	SweepKindQuarantined SweepRecordKind = "quarantined"
	SweepKindSpawn       SweepRecordKind = "spawn"
)

// SweepRecord is one structured journal.log line this sweep can fully
// reconcile — a superset of what loop.go/commsgw journal today (see file
// doc). Cell scopes a "spawn" record to the cell that fired it; landed/
// quarantined records are scoped by From.Cell/To.Cell instead.
type SweepRecord struct {
	Time      time.Time        `json:"time"`
	Kind      SweepRecordKind  `json:"kind"`
	ID        string           `json:"id"`
	Cell      string           `json:"cell,omitempty"`
	From      comms.SenderID   `json:"from,omitempty"`
	To        comms.Lane       `json:"to,omitempty"`
	Verb      string           `json:"verb,omitempty"`
	Class     string           `json:"class,omitempty"`
	Assertion *comms.Assertion `json:"assertion,omitempty"`
	// Action / AssignClass / AssignRisk are the prose-routed decision a
	// "spawn" record claims to trace to — the (action, class, risk) triple
	// assign.go's Assign resolves.
	Action      string `json:"action,omitempty"`
	AssignClass string `json:"assignClass,omitempty"`
	AssignRisk  bool   `json:"assignRisk,omitempty"`
	// MsgID / SessionID are spawn-only: the message this session answers,
	// and the session's own id.
	MsgID     string `json:"msgId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

// legacyLandedRe matches loop.go's Land (VerdictPass) free-text line:
// "<RFC3339> landed id=<id> from=<cell>/<role> to=<cell>/<role> verb=<verb> (...)".
var legacyLandedRe = regexp.MustCompile(`^(\S+) landed id=(\S+) from=([^/\s]+)/(\S+) to=([^/\s]+)/(\S+) verb=(\S+)\b`)

// legacyLine is one parsed free-text journal.log line — lane-checkable, but
// carrying no assertion or spawn data (never recorded in this shape).
type legacyLine struct {
	Time time.Time
	ID   string
	From comms.SenderID
	To   comms.Lane
	Verb string
}

// readJournal reads root's journal.log, splitting each line into a
// structured SweepRecord, a legacy free-text line, or a corrupt-line note.
// A MISSING journal.log is an EMPTY sweep input (nothing landed yet), not an
// error — mirroring commsqueue.ListAccepted/ListHeld's "absent dir/file is
// empty" convention. Any OTHER read failure (permission, I/O) is returned as
// err so the caller reports could-not-check for the whole run, never a
// partial read presented as complete.
func readJournal(path string) (records []SweepRecord, legacy []legacyLine, corrupt []string, err error) {
	f, oerr := os.Open(path)
	if oerr != nil {
		if os.IsNotExist(oerr) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("cannot open %s: %w", path, oerr)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var rec SweepRecord
			if jerr := json.Unmarshal([]byte(line), &rec); jerr != nil || rec.Kind == "" || rec.ID == "" {
				corrupt = append(corrupt, fmt.Sprintf("%s:%d: does not parse as a structured sweep record", filepath.Base(path), lineNo))
				continue
			}
			records = append(records, rec)
			continue
		}
		m := legacyLandedRe.FindStringSubmatch(line)
		if m == nil {
			corrupt = append(corrupt, fmt.Sprintf("%s:%d: unrecognised line shape", filepath.Base(path), lineNo))
			continue
		}
		t, terr := time.Parse(time.RFC3339, m[1])
		if terr != nil {
			corrupt = append(corrupt, fmt.Sprintf("%s:%d: bad timestamp %q: %v", filepath.Base(path), lineNo, m[1], terr))
			continue
		}
		legacy = append(legacy, legacyLine{
			Time: t,
			ID:   m[2],
			From: comms.SenderID{Cell: m[3], Role: m[4]},
			To:   comms.Lane{Cell: m[5], Role: m[6]},
			Verb: m[7],
		})
	}
	if serr := sc.Err(); serr != nil {
		return records, legacy, corrupt, fmt.Errorf("truncated/unreadable read of %s: %w", path, serr)
	}
	return records, legacy, corrupt, nil
}

// --- findings ---------------------------------------------------------

// FindingKind is the closed vocabulary of violation classes this sweep
// detects — distinct so a filed issue and an incident review can always
// tell one class from another.
type FindingKind string

const (
	FindingLaneViolation    FindingKind = "lane-violation"
	FindingInvalidAssertion FindingKind = "invalid-assertion"
	FindingOrphanSpawn      FindingKind = "orphan-spawn"
)

// Finding is one violation this sweep detected.
type Finding struct {
	Kind   FindingKind
	ID     string
	Detail string
}

// --- three-state report -------------------------------------------------

const (
	SweepCheckedClean  = "checked-clean"
	SweepCheckedFailed = "checked-failed"
	SweepCouldNotCheck = "could-not-check"
)

// SweepReport is the outcome of one Sweep run. State precedence is
// deliberate: could-not-check outranks checked-failed outranks
// checked-clean — an incomplete read is never presented as a confident
// verdict of either other kind (C4: could-not-check is never clean, and is
// never silently downgraded to a plain "failed" either).
type SweepReport struct {
	State                string
	Cell                 string
	Checked              int
	Findings             []Finding
	CouldNotCheckReasons []string
}

func (r *SweepReport) resolveState() {
	switch {
	case len(r.CouldNotCheckReasons) > 0:
		r.State = SweepCouldNotCheck
	case len(r.Findings) > 0:
		r.State = SweepCheckedFailed
	default:
		r.State = SweepCheckedClean
	}
}

// --- the sweep itself ----------------------------------------------------

// SweepDeps is everything Sweep consults besides the on-disk queue root —
// injected so tests drive it without real key material (mirrors
// commsqueue.Quarantine's IssueFiler seam).
type SweepDeps struct {
	// ACL is the CURRENT compiled lane ACL every record is re-checked
	// against. nil defaults to comms.Compiled().
	ACL *comms.ACL
	// Trust resolves a cell's current public key for assertion re-checks.
	// nil is a valid, fail-closed state: any assertion actually encountered
	// then reports could-not-check (never a silent skip) rather than a
	// panic or an assumed-valid read.
	//
	// The re-check (comms.VerifyIdentityOnly) deliberately takes no current
	// time or skew: it asks only "does this signature still come from a cell
	// this trust store trusts", never "is this assertion still inside its
	// original receipt window" — that window has necessarily closed by the
	// time a periodic sweep reaches the record.
	Trust comms.TrustStore
}

func inScope(cell, a, b string) bool {
	return cell == "" || a == cell || b == cell
}

// Sweep re-derives lane legality, peer-auth validity and firing-record
// integrity for every held/landed/quarantined/spawned record under root that
// is in scope for cell and no older than since (the zero Time means no
// floor). It is a PURE read: no mailbox is ever mutated, and findings are
// returned for the caller to file — Sweep itself never shells out.
func Sweep(root, cell string, since time.Time, deps SweepDeps) SweepReport {
	acl := deps.ACL
	if acl == nil {
		c := comms.Compiled()
		acl = &c
	}
	report := SweepReport{Cell: cell}
	knownMsgIDs := map[string]bool{}
	var spawns []SweepRecord

	checkOne := func(id string, from comms.SenderID, to comms.Lane, verb string, assertion *comms.Assertion) {
		report.Checked++
		if !acl.Allow(from.Cell, from.Role, verb, to.Cell, to.Role) {
			report.Findings = append(report.Findings, Finding{
				Kind: FindingLaneViolation,
				ID:   id,
				Detail: fmt.Sprintf("(%s/%s -> %s/%s) verb %q is not permitted by the CURRENT compiled lane ACL",
					from.Cell, from.Role, to.Cell, to.Role, verb),
			})
		}
		if assertion != nil && assertion.Cell != "" {
			if deps.Trust == nil {
				report.CouldNotCheckReasons = append(report.CouldNotCheckReasons, fmt.Sprintf(
					"message %s: cannot re-verify its recorded assertion — no trust store configured (%s)", id, EnvTrustStore))
			} else if verr := comms.VerifyIdentityOnly(*assertion, deps.Trust); verr != nil {
				// VerifyIdentityOnly, not Verify: this is a re-check of a
				// HISTORICAL assertion, well outside its original
				// receipt window by the time a daily sweep reaches it.
				// The sweep asks "does this signature still come from a
				// cell the CURRENT trust store trusts", never
				// re-litigating the original window or single-use.
				report.Findings = append(report.Findings, Finding{
					Kind:   FindingInvalidAssertion,
					ID:     id,
					Detail: fmt.Sprintf("recorded assertion for cell %q no longer verifies: %v", assertion.Cell, verr),
				})
			}
		}
	}

	// held/*.json — real, structured, full envelopes (including assertions)
	// preserved indefinitely.
	held, herr := commsqueue.ListHeld(root)
	if herr != nil {
		report.CouldNotCheckReasons = append(report.CouldNotCheckReasons, fmt.Sprintf("cannot read held mailbox: %v", herr))
	} else {
		for _, h := range held {
			if !since.IsZero() && h.HeldAt.Before(since) {
				continue
			}
			if !inScope(cell, h.Envelope.From.Cell, h.Envelope.To.Cell) {
				continue
			}
			knownMsgIDs[h.Envelope.ID] = true
			checkOne(h.Envelope.ID, h.Envelope.From, h.Envelope.To, h.Envelope.Verb, &h.Envelope.Sig)
		}
	}

	// journal.log — legacy free-text (lane-only) + forward SweepRecord lines.
	records, legacyLines, corrupt, jerr := readJournal(filepath.Join(root, "journal.log"))
	if jerr != nil {
		report.CouldNotCheckReasons = append(report.CouldNotCheckReasons, jerr.Error())
	}
	report.CouldNotCheckReasons = append(report.CouldNotCheckReasons, corrupt...)

	for _, ll := range legacyLines {
		if !since.IsZero() && ll.Time.Before(since) {
			continue
		}
		if !inScope(cell, ll.From.Cell, ll.To.Cell) {
			continue
		}
		knownMsgIDs[ll.ID] = true
		checkOne(ll.ID, ll.From, ll.To, ll.Verb, nil)
	}

	for _, rec := range records {
		if !since.IsZero() && !rec.Time.IsZero() && rec.Time.Before(since) {
			continue
		}
		switch rec.Kind {
		case SweepKindLanded, SweepKindQuarantined:
			if !inScope(cell, rec.From.Cell, rec.To.Cell) {
				continue
			}
			knownMsgIDs[rec.ID] = true
			checkOne(rec.ID, rec.From, rec.To, rec.Verb, rec.Assertion)
		case SweepKindSpawn:
			if !inScope(cell, rec.Cell, rec.Cell) {
				continue
			}
			spawns = append(spawns, rec)
		default:
			report.CouldNotCheckReasons = append(report.CouldNotCheckReasons,
				fmt.Sprintf("journal.log: record %s carries unrecognised kind %q", rec.ID, rec.Kind))
		}
	}

	// Firing-record reconciliation: every spawn must trace to a known
	// message id AND a legal (action, class, risk) assign-table row.
	for _, s := range spawns {
		report.Checked++
		var reasons []string
		if s.MsgID == "" || !knownMsgIDs[s.MsgID] {
			reasons = append(reasons, fmt.Sprintf("traces to no landed/quarantined message id %q seen in this sweep's window", s.MsgID))
		}
		class := s.AssignClass
		if class == "" {
			class = "routine"
		}
		if !KnownActions[s.Action] {
			reasons = append(reasons, fmt.Sprintf("action %q is not in the closed action vocabulary", s.Action))
		} else if _, aerr := Assign(s.Action, class, s.AssignRisk); aerr != nil {
			reasons = append(reasons, fmt.Sprintf("no legal assign-table row for (action=%s class=%s risk=%v): %v", s.Action, class, s.AssignRisk, aerr))
		}
		if len(reasons) > 0 {
			id := s.SessionID
			if id == "" {
				id = s.ID
			}
			report.Findings = append(report.Findings, Finding{
				Kind:   FindingOrphanSpawn,
				ID:     id,
				Detail: fmt.Sprintf("spawned session %s (msgId=%q): %s", id, s.MsgID, strings.Join(reasons, "; ")),
			})
		}
	}

	report.resolveState()
	return report
}

// --- CLI wiring: `commsloop sweep --cell <slug> [--since <dur>]` --------

// sweepIssueFiler routes one finding as a filed issue. It is an interface so
// tests never shell out (mirrors commsqueue.IssueFiler).
type sweepIssueFiler interface {
	File(cell string, f Finding) error
}

// deskfileSweepFiler is the concrete filer: it shells out to `deskfile new`,
// the desk write verb every filing goes through (never a hand-rolled `gh
// issue create`) — same convention as commsqueue.DeskfileIssueFiler.
type deskfileSweepFiler struct{ Repo string }

func (d deskfileSweepFiler) File(cell string, f Finding) error {
	title := fmt.Sprintf("lane-violation sweep: %s (cell %s, id %s)", f.Kind, cell, f.ID)
	body := fmt.Sprintf("The daily lane-violation sweep found a %s.\n\n"+
		"- cell: %s\n- id: %s\n- detail: %s\n", f.Kind, cell, f.ID, f.Detail)
	cmd := exec.Command("deskfile", "new", "--raised-by", commsqueue.RaisedByRole, "--repo", d.Repo,
		"--title", title, "--label", "help wanted", "--body", body)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deskfile new failed: %w: %s", err, stderr.String())
	}
	return nil
}

// loadSweepTrustStore reads a JSON {cell: base64-pubkey} file into a
// comms.Ed25519TrustStore. DELIBERATELY duplicated from
// cmd/commsgw/deps.go's LoadTrustStore rather than shared: the two live in
// different `main` packages (separate binaries) that cannot import one
// another, and the same "duplicate the small check across the process
// boundary" call routing.go's file doc already makes for the lane-ACL
// re-check applies here too.
func loadSweepTrustStore(path string) (comms.Ed25519TrustStore, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("commsloop sweep: cannot read trust store %s", path), err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, deskkit.Refused(fmt.Sprintf("commsloop sweep: trust store %s does not parse as JSON: %v", path, err))
	}
	out := make(comms.Ed25519TrustStore, len(m))
	for cell, enc := range m {
		if cell == "" {
			return nil, deskkit.Refused(fmt.Sprintf("commsloop sweep: trust store %s: empty cell key", path))
		}
		keyRaw, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, deskkit.Refused(fmt.Sprintf("commsloop sweep: trust store %s: cell %q key is not valid base64: %v", path, cell, err))
		}
		if len(keyRaw) != ed25519.PublicKeySize {
			return nil, deskkit.Refused(fmt.Sprintf("commsloop sweep: trust store %s: cell %q key is %d bytes, want %d", path, cell, len(keyRaw), ed25519.PublicKeySize))
		}
		out[cell] = ed25519.PublicKey(keyRaw)
	}
	return out, nil
}

func printSweepReport(w io.Writer, r SweepReport) {
	fmt.Fprintf(w, "commsloop sweep: cell=%s state=%s checked=%d findings=%d could-not-check=%d\n",
		r.Cell, r.State, r.Checked, len(r.Findings), len(r.CouldNotCheckReasons))
	for _, f := range r.Findings {
		fmt.Fprintf(w, "  FINDING %s id=%s: %s\n", f.Kind, f.ID, f.Detail)
	}
	for _, c := range r.CouldNotCheckReasons {
		fmt.Fprintf(w, "  COULD-NOT-CHECK: %s\n", c)
	}
}

// cmdSweep is `commsloop sweep`'s entry point: parse flags, resolve the
// queue root and (optional) trust store from the SAME env vars the drain
// loop uses, run Sweep, print the report, file every finding, and return a
// distinctly-coded error — nil (checked-clean), deskkit.Refused (exit 5,
// checked-failed), or deskkit.Unverifiable (exit 6, could-not-check). It
// never mutates a mailbox and never "fixes" anything it finds.
func cmdSweep(args []string, getenv func(string) string, stdout io.Writer) error {
	fs := flag.NewFlagSet("commsloop sweep", flag.ContinueOnError)
	cell := fs.String("cell", "", "the cell slug to sweep (required)")
	since := fs.Duration("since", 0, "only reconcile records/held items newer than this duration ago (0 = no floor)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("commsloop sweep: " + err.Error())
	}
	if strings.TrimSpace(*cell) == "" {
		return deskkit.Refused("commsloop sweep: --cell is required")
	}
	root := strings.TrimSpace(getenv(EnvQueueDir))
	if root == "" {
		return deskkit.Refused(fmt.Sprintf("commsloop sweep: %s is not set — no queue root to sweep", EnvQueueDir))
	}
	repo := strings.TrimSpace(getenv(EnvRepo))
	if repo == "" {
		repo = "medici-finance/assay"
	}

	var trust comms.TrustStore
	if tsPath := strings.TrimSpace(getenv(EnvTrustStore)); tsPath != "" {
		ts, err := loadSweepTrustStore(tsPath)
		if err != nil {
			return err
		}
		trust = ts
	}

	now := time.Now().UTC()
	var sinceTime time.Time
	if *since > 0 {
		sinceTime = now.Add(-*since)
	}

	report := Sweep(root, *cell, sinceTime, SweepDeps{Trust: trust})
	printSweepReport(stdout, report)

	var filer sweepIssueFiler = deskfileSweepFiler{Repo: repo}
	for _, f := range report.Findings {
		if err := filer.File(*cell, f); err != nil {
			// A filing failure is never a dropped finding — it is already
			// in the report/exit code above; this only means the second,
			// human-visible half of "held + filed" (quarantine.go's
			// convention) did not land. Surfaced, never silent.
			fmt.Fprintf(os.Stderr, "commsloop sweep: could not file issue for %s finding %s: %v\n", f.Kind, f.ID, err)
		}
	}

	switch report.State {
	case SweepCheckedClean:
		return nil
	case SweepCheckedFailed:
		return deskkit.Refused(fmt.Sprintf("commsloop sweep: cell %s: %d finding(s) — see filed issues", *cell, len(report.Findings)))
	default:
		return deskkit.Unverifiable(fmt.Sprintf("commsloop sweep: cell %s: could not check — %s",
			*cell, strings.Join(report.CouldNotCheckReasons, "; ")), nil)
	}
}
