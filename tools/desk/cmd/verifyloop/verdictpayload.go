package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// verdictpayload.go — the verdict-v1 payload the batcher composes and signs. One flush of
// the ~5-minute batch window produces exactly one payload, which becomes the body of exactly
// one verifier-App-authored `verify-verdict` issue (payload.md). The signature is over the
// CANONICAL bytes (deskkit.CanonicalizeJSON), so Go struct field order never affects verify.

// verdictPayload mirrors the verdict-v1 JSON (payload.md). Field tags are the wire contract.
type verdictPayload struct {
	Schema  string         `json:"schema"`
	Repo    string         `json:"repo"`
	TS      string         `json:"ts"`
	Head    string         `json:"head"`
	Entries []verdictEntry `json:"entries"`
}

// verdictEntry is one verified row. Session is OPTIONAL: an absent block means
// ProvenanceUnknown and is never synthesised, so it is omitempty.
type verdictEntry struct {
	Brief    string        `json:"brief"`
	Row      int           `json:"row"`
	Class    string        `json:"class"`
	Result   string        `json:"result"`
	Evidence string        `json:"evidence"`
	Session  *sessionBlock `json:"session,omitempty"`
}

// sessionBlock is the ENGINE-populated provenance for one row (payload.md; brief 04's
// session-provenance capture). It is stamped by the deterministic conductor at Result-time —
// an other-actor by construction, which receives the runner's RAW transcript before any
// model-written text touches the record — never by a runner session about itself. It records
// the sha256 of that raw transcript: attestation of what the engine RECEIVED. A
// session-written digest is worthless by design, so only this code populates the block.
type sessionBlock struct {
	ID               string `json:"id"`
	TranscriptSHA256 string `json:"transcript_sha256"`
	Runner           string `json:"runner"`
}

// rowResult is one executed check/check:ci Verify row — what the deterministic runner
// OBSERVED. Output is the raw transcript the engine received; it feeds both the Evidence key
// line and the provenance digest.
type rowResult struct {
	BriefPath string
	Row       int
	Class     string
	Command   string
	Exit      int
	Output    string
}

// result maps the observed exit code to the payload result. Exit 0 is PASS; any non-zero is
// FAIL — the exit code IS the verdict (row-classes.md).
func (r rowResult) result() string {
	if r.Exit == 0 {
		return loopengine.VerdictPass
	}
	return loopengine.VerdictFail
}

// sessionMeta is the batch-wide provenance the engine stamps onto every entry: the runner
// session id and the runner identity (an App bot login).
type sessionMeta struct {
	ID     string
	Runner string
}

// composePayload builds the verdict-v1 payload for one batch. FAIL rows are INCLUDED (R-6
// cl.8: the transcription lane writes nothing for a FAIL, but the signed verdict itself
// still carries it — the loop's only failure behaviour is filing, never diagnosing).
func composePayload(repo, head string, ts time.Time, meta sessionMeta, rows []rowResult) verdictPayload {
	date := ts.UTC().Format("2006-01-02")
	entries := make([]verdictEntry, 0, len(rows))
	for _, r := range rows {
		sum := sha256.Sum256([]byte(r.Output))
		entries = append(entries, verdictEntry{
			Brief:    r.BriefPath,
			Row:      r.Row,
			Class:    r.Class,
			Result:   r.result(),
			Evidence: evidenceRow(r, date, meta.Runner),
			Session: &sessionBlock{
				ID:               meta.ID,
				TranscriptSHA256: hex.EncodeToString(sum[:]),
				Runner:           meta.Runner,
			},
		})
	}
	return verdictPayload{
		Schema:  deskkit.VerdictSchemaVersion,
		Repo:    repo,
		TS:      ts.UTC().Format(time.RFC3339),
		Head:    head,
		Entries: entries,
	}
}

// evidenceRow renders the exact Evidence markdown row for one result — the verbatim table
// row the transcription lane appends (payload.md: `| # | class | command | exit N | date |
// runner |`). A pipe in observed material would break the table, so it is escaped.
func evidenceRow(r rowResult, date, runner string) string {
	if runner == "" {
		runner = "unknown-runner"
	}
	cmd := strings.ReplaceAll(r.Command, "|", `\|`)
	return fmt.Sprintf("| %d | %s | `%s` | exit %d | %s | %s |",
		r.Row, r.Class, cmd, r.Exit, date, runner)
}

// signPayload marshals the payload, canonicalises it, signs the SHA-256 of the canonical
// bytes (RS256) with the verifier private key at pemPath, and returns the assembled
// issue-body block (fenced canonical payload + HTML-comment signature trailer). It reuses
// the deskkit verdict primitives verbatim so signing and verifying share one canonical form.
func signPayload(p verdictPayload, pemPath string) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal verdict payload: %w", err)
	}
	canonical, err := deskkit.CanonicalizeJSON(raw)
	if err != nil {
		return "", err
	}
	keyPEM, err := os.ReadFile(pemPath)
	if err != nil {
		return "", fmt.Errorf("cannot read verifier key at %s: %w", pemPath, err)
	}
	key, err := deskkit.ParseRSAPrivateKeyPEM(keyPEM)
	if err != nil {
		return "", fmt.Errorf("%w (%s)", err, pemPath)
	}
	sig, err := deskkit.SignVerdictCanonical(canonical, key)
	if err != nil {
		return "", err
	}
	return deskkit.AssembleVerdictBody(canonical, sig), nil
}

// batch accumulates executed rows until the ~5-minute window elapses OR the queue drains,
// whichever comes first; a flush produces exactly one payload → one issue. The cadence is
// the throttle (no daily cap — see deskkit.VerdictIssueTool).
type batch struct {
	windowStart time.Time
	rows        []rowResult
}

// add appends a result, starting the window clock on the first row of a fresh batch.
func (b *batch) add(r rowResult, now time.Time) {
	if len(b.rows) == 0 {
		b.windowStart = now
	}
	b.rows = append(b.rows, r)
}

// dueToFlush reports whether the accumulated batch should flush now: the queue is drained
// (nothing left to add) OR the window has elapsed. An empty batch is never flushed — a flush
// must produce a non-empty payload.
func (b *batch) dueToFlush(now time.Time, window time.Duration, queueDrained bool) bool {
	if len(b.rows) == 0 {
		return false
	}
	return queueDrained || !now.Before(b.windowStart.Add(window))
}

// reset clears the batch after a flush.
func (b *batch) reset() { b.rows = nil; b.windowStart = time.Time{} }

// defaultBatchWindow is the ~5-minute flush window (payload.md / brief 04).
const defaultBatchWindow = 5 * time.Minute
