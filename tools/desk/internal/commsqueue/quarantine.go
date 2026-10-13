package commsqueue

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// quarantine.go — held mailbox + a filed issue, never a silent drop
// ("Quarantine = held mailbox + a filed issue via deskfile (silent-desk
// rule)"). SHARED between cmd/commsgw (which quarantines on an
// inbox-emission failure) and cmd/commsloop (which quarantines a message
// awaiting the not-yet-landed prose router, or an ACL bypass caught at the
// routing boundary) — one filing convention, not two.

// ErrQuarantined is always part of Quarantine's returned error chain when the
// held-mailbox write itself succeeded (regardless of whether issue filing
// also succeeded) — a caller checks errors.Is(err, ErrQuarantined) to log or
// count a quarantine as the distinct, non-happy-path event it is. The message
// itself is SAFE either way (held, never dropped); a caller that discards
// this return value entirely loses nothing durable, only the chance to
// surface the event promptly.
var ErrQuarantined = errors.New("commsqueue: message quarantined (held, never dropped)")

// IssueFiler raises a filed issue naming a quarantined message and why. It is
// an interface so tests drive the failure path without invoking the real
// `deskfile` CLI, and so DeskfileIssueFiler is the ONE place that shells out.
type IssueFiler interface {
	File(env comms.Envelope, reason string) error
}

// Quarantine durably holds env (WriteHeld) and files an issue naming it. It
// NEVER drops env: the held write happens first and unconditionally; a filing
// failure (or a nil filer) is reflected in the returned error but never undoes
// the held write.
func Quarantine(root string, env comms.Envelope, reason string, now time.Time, filer IssueFiler) error {
	if err := WriteHeld(root, env, reason, now); err != nil {
		return fmt.Errorf("commsqueue: quarantine of %s FAILED at the held-mailbox write (message may be lost): %w", env.ID, err)
	}
	if filer == nil {
		return fmt.Errorf("%w: %s: %s", ErrQuarantined, env.ID, reason)
	}
	if err := filer.File(env, reason); err != nil {
		// Callers treat ErrQuarantined as informational and discard it (the
		// message is safe), so a failed filing would otherwise leave a held
		// message nobody is told about. Say so here, once, on one line.
		fmt.Fprintf(os.Stderr, "commsqueue: message %s is held, but its quarantine issue failed to file: %s\n",
			oneLine(env.ID), oneLine(err.Error()))
		return fmt.Errorf("%w: %s: held, but its quarantine issue failed to file: %v", ErrQuarantined, env.ID, err)
	}
	return fmt.Errorf("%w: %s: %s", ErrQuarantined, env.ID, reason)
}

// RaisedByRole is the desk loop commsgw/commsloop's own filings are
// attributed to. It is a LOOP name (it also addresses envelopes); the
// `deskfile new --raised-by` flag takes the roster ROLE that loop runs as,
// which FileDeskIssue resolves through deskkit.TokenRoleForLoop.
const RaisedByRole = "worker-desk"

// deskfileBin is the sibling binary DeskfileIssueFiler.File shells out to,
// resolved on PATH. It is a package-level CONSTANT — never a struct field or
// other caller-supplied value — so the forge-surface ban's shell-exec scanner
// can resolve argv[0] at compile time and confirm it is not a forge CLI,
// rather than having to carry this call site as an unresolved (could-not-check)
// exec site. Same shell-to-desk-verb pattern as cmd/desksupervise/actions.go's
// doFileBlockedTimeout.
const deskfileBin = "deskfile"

// DeskfileIssueFiler is the concrete IssueFiler: it shells out to the
// `deskfile` CLI, the desk write verb every quarantine-issue filing goes
// through (never a hand-rolled `gh issue create`).
type DeskfileIssueFiler struct {
	// Repo is the owner/repo the issue is filed against.
	Repo string
}

func (f DeskfileIssueFiler) File(env comms.Envelope, reason string) error {
	title := fmt.Sprintf("comms quarantine: message %s (%s -> %s, verb %s)", env.ID, env.From.Cell, env.To.Cell, env.Verb)
	return FileDeskIssue(f.Repo, title, "A cell-gateway message was quarantined (held, never dropped).", []string{
		"id: " + env.ID,
		"from: " + env.From.Cell + "/" + env.From.Role,
		"to: " + env.To.Cell + "/" + env.To.Role,
		"verb: " + env.Verb,
		"reason: " + reason,
	})
}

// filingLabel is the escalation label every comms filing carries: a held
// message or a sweep finding needs a human's hands.
const filingLabel = "help wanted"

// FileDeskIssue is the ONE shell-to-deskfile site for the comms tools' filings
// (quarantine here, commsloop's lane-violation sweep): `deskfile new` with the
// argv that verb accepts — -R, the body in a --body-file, the escalation
// label, and --raised-by as the roster role RaisedByRole runs as. facts are
// the record the filing is about; they go in the `### Evidence` fenced block a
// "help wanted" filing must carry. It sets no environment: the credentials and
// loop marker deskfile needs are the calling process's own.
func FileDeskIssue(repo, title, summary string, facts []string) error {
	role, ok := deskkit.TokenRoleForLoop(RaisedByRole)
	if !ok {
		return fmt.Errorf("deskfile new not run: loop %q has no roster role to raise as", RaisedByRole)
	}
	var body strings.Builder
	body.WriteString(summary)
	body.WriteString("\n\n### Evidence\n\n```text\n")
	for _, f := range facts {
		// One line per fact inside the fence: a stray fence marker or a
		// control sequence must not close the block early or reach a terminal.
		body.WriteString("- " + strings.ReplaceAll(oneLine(f), "```", "` ` `") + "\n")
	}
	body.WriteString("```\n")

	bf, err := os.CreateTemp("", "comms-filing-*.md")
	if err != nil {
		return fmt.Errorf("deskfile new not run: cannot create a body file: %w", err)
	}
	defer os.Remove(bf.Name())
	if _, err := bf.WriteString(body.String()); err != nil {
		bf.Close()
		return fmt.Errorf("deskfile new not run: cannot write the body file: %w", err)
	}
	if err := bf.Close(); err != nil {
		return fmt.Errorf("deskfile new not run: cannot close the body file: %w", err)
	}

	cmd := exec.Command(deskfileBin, "new", "-R", repo, "--title", title,
		"--body-file", bf.Name(), "--label", filingLabel, "--raised-by", role)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deskfile new failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// oneLine flattens s onto a single control-free line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(deskkit.StripControl(s)), " ")
}

// AppendJournal appends one line to the queue's append-only journal
// (<root>/journal.log) — Land's durable record that a message reached a
// terminal disposition. It never truncates or rewrites prior lines.
func AppendJournal(root, line string) error {
	return appendLine(root, "journal.log", line)
}
