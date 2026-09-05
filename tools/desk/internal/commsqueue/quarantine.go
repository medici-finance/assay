package commsqueue

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
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
		return fmt.Errorf("%w: %s: held, but its quarantine issue failed to file: %v", ErrQuarantined, env.ID, err)
	}
	return fmt.Errorf("%w: %s: %s", ErrQuarantined, env.ID, reason)
}

// RaisedByRole is the desk role commsgw/commsloop's own filings are
// attributed to, per house convention (`deskfile new --raised-by <role>`).
const RaisedByRole = "worker-desk"

// DeskfileIssueFiler is the concrete IssueFiler: it shells out to the
// `deskfile` CLI, the desk write verb every quarantine-issue filing goes
// through (never a hand-rolled `gh issue create`).
type DeskfileIssueFiler struct {
	// Deskfile is the path/name of the deskfile binary. Empty defaults to
	// "deskfile" resolved on PATH.
	Deskfile string
	// Repo is the owner/repo the issue is filed against.
	Repo string
}

func (f DeskfileIssueFiler) File(env comms.Envelope, reason string) error {
	bin := f.Deskfile
	if bin == "" {
		bin = "deskfile"
	}
	title := fmt.Sprintf("comms quarantine: message %s (%s -> %s, verb %s)", env.ID, env.From.Cell, env.To.Cell, env.Verb)
	body := fmt.Sprintf("A cell-gateway message was quarantined (held, never dropped).\n\n"+
		"- id: %s\n- from: %s/%s\n- to: %s/%s\n- verb: %s\n- reason: %s\n",
		env.ID, env.From.Cell, env.From.Role, env.To.Cell, env.To.Role, env.Verb, reason)
	args := []string{"new", "--raised-by", RaisedByRole, "--repo", f.Repo, "--title", title, "--label", "help wanted", "--body", body}
	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deskfile new failed: %w: %s", err, stderr.String())
	}
	return nil
}

// AppendJournal appends one line to the queue's append-only journal
// (<root>/journal.log) — Land's durable record that a message reached a
// terminal disposition. It never truncates or rewrites prior lines.
func AppendJournal(root, line string) error {
	return appendLine(root, "journal.log", line)
}
