package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// maxRestatementWords is the receipt cap. A receipt states what the desk UNDERSTOOD in a
// handful of words; anything longer is drifting toward a transcript (which defeats the
// point — a re-quote cannot be corrected as a misreading) and is refused.
const maxRestatementWords = 12

const usage = `deskack — print and record a desk's receipt for a human-typed message.

USAGE:
  deskack [--repo <repo>] [--session <s>] "<restatement>"
  deskack --version

Prints the one line:  ack <role>@<repo-short>: <restatement>
  role       — this session's desk loop, from $DESK_LOOP (required)
  repo-short — the roster alias for --repo (deskkit.RepoShortLabel); omitted if no --repo
  restatement— the desk's OWN one-line reading of the message, at most 12 words

and appends a {ts, role, repo, restatement} record to this session's roster beacon
(<state>/roster/<session>.json). Run it as the FIRST line of the desk's turn after ANY
human-typed message, then act — it is the one line the silent-output contract permits.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable.`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskack sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(stdout, usage)
		return deskkit.ExitOK
	}

	// Kill switch first, like every desk tool: a halted desk does not acknowledge.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	fs := flag.NewFlagSet("deskack", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // craft our own messages
	repo := fs.String("repo", "", "repo the message concerns (short name or owner/repo); its roster alias renders as <repo-short>")
	session := fs.String("session", "", "session name (env: $DESK_SESSION or $CLAUDE_SESSION_ID)")
	if perr := fs.Parse(args); perr != nil {
		fmt.Fprintln(stderr, "refused: bad flags: "+perr.Error())
		return deskkit.ExitRefused
	}

	restatement := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if restatement == "" {
		fmt.Fprintln(stderr, "refused: a restatement is required — the desk's own one-line reading of the "+
			"message (at most 12 words). Quote nothing; say what you UNDERSTOOD, so a misread can be corrected.")
		return deskkit.ExitRefused
	}
	if n := len(strings.Fields(restatement)); n > maxRestatementWords {
		fmt.Fprintf(stderr, "refused: restatement is %d words; the receipt cap is %d. A receipt is a "+
			"confirmation of what you UNDERSTOOD, not a transcript — shorten it.\n", n, maxRestatementWords)
		return deskkit.ExitRefused
	}

	// Role from $DESK_LOOP. It is the desk loop this session presents (also what a human
	// arms a STOP.<loop> flag on), so a receipt names the same identity everything else in
	// the session does. Unset is a refusal, not a blank role.
	role := strings.TrimSpace(os.Getenv("DESK_LOOP"))
	if role == "" {
		fmt.Fprintln(stderr, "refused: $DESK_LOOP is unset, so this session has no desk role to acknowledge "+
			"under. Run `export DESK_LOOP=<loop>` in this shell (e.g. worker-desk), then re-run deskack.")
		return deskkit.ExitRefused
	}

	// repo-short from the roster alias, when a repo was named.
	repoShort := ""
	if strings.TrimSpace(*repo) != "" {
		repoShort = deskkit.RepoShortLabel(strings.TrimSpace(*repo))
	}

	sess, serr := resolveSession(*session)
	if serr != nil {
		fmt.Fprintln(stderr, serr.Error())
		return deskkit.ExitCodeOf(serr)
	}

	// Append the receipt to the roster beacon BEFORE printing the line, so a printed
	// receipt is always one that was recorded (the metric reads the beacon, not the
	// console). A write failure is unverifiable — the desk must not believe it left a
	// durable receipt it did not.
	if _, aerr := deskkit.AppendAck(sess, deskkit.AckRecord{Role: role, Repo: repoShort, Restatement: restatement}); aerr != nil {
		fmt.Fprintln(stderr, "deskack: "+aerr.Error())
		return deskkit.ExitCodeOf(aerr)
	}

	fmt.Fprintln(stdout, ackLine(role, repoShort, restatement))
	return deskkit.ExitOK
}

// ackLine renders the fixed receipt line. The `@<repo-short>` segment is present only when
// a repo was named — a desk acknowledging a message that concerns no single repo prints
// `ack <role>: <restatement>` rather than a placeholder.
func ackLine(role, repoShort, restatement string) string {
	if repoShort == "" {
		return "ack " + role + ": " + restatement
	}
	return "ack " + role + "@" + repoShort + ": " + restatement
}

// resolveSession mirrors deskroster's own order — $DESK_SESSION → $CLAUDE_SESSION_ID →
// --session — so a receipt lands on the SAME beacon file deskroster keeps this session's
// open work in. An unresolvable session is unverifiable (exit 6), never a guessed key.
func resolveSession(sessionFlag string) (string, error) {
	if s := strings.TrimSpace(os.Getenv("DESK_SESSION")); s != "" {
		return s, nil
	}
	if s := strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID")); s != "" {
		return s, nil
	}
	if strings.TrimSpace(sessionFlag) != "" {
		return strings.TrimSpace(sessionFlag), nil
	}
	return "", deskkit.Unverifiable(
		"cannot resolve session identity for the receipt beacon: set $DESK_SESSION, "+
			"$CLAUDE_SESSION_ID, or pass --session", nil)
}
