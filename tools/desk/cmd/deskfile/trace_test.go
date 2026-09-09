package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// trace_test.go — the dedupe-search outage says WHAT failed.
//
// deskfile fails CLOSED when its dedupe search cannot be answered: minting a possibly
// duplicate issue is the expensive direction, so an unanswered search is exit 6 rather than a
// guess at absence. That is right and is not touched here.
//
// What was missing is the diagnosis. The refusal named the POLICY ("dedupe search failed —
// refuse rather than mint a possible duplicate") but the operator could not tell a rate
// limit from a revoked token from a repo the App cannot see — three failures with three
// completely different fixes, all reaching them as one sentence. gh had already said which
// it was, in an `HTTP <status>` line on its stderr; it just had nowhere to go.
//
// The assertions below are per-status, because "it carries gh's text" is not the property
// that matters: the property that matters is that a reader can tell the three apart.

// ghFailure installs a stubbed `gh` that writes text on stderr and exits non-zero.
func ghFailure(t *testing.T, stderr string, code int) *[][]string {
	t.Helper()
	calls := &[][]string{}
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return exec.Command("/bin/sh", "-c",
			"cat <<'STUBEOF' 1>&2\n"+stderr+"\nSTUBEOF\nexit "+itoa(code))
	}
	t.Cleanup(func() { execCommand = old })
	return calls
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestDedupeSearchOutageNamesTheAPIStatus is the fail-first pin, one row per status an
// operator has to be able to tell apart.
func TestDedupeSearchOutageNamesTheAPIStatus(t *testing.T) {
	cases := []struct {
		name, ghStderr, want string
	}{
		{
			"forbidden",
			"gh: Resource not accessible by integration (HTTP 403)",
			"HTTP 403",
		},
		{
			"rate limited",
			"gh: API rate limit exceeded (HTTP 429)",
			"HTTP 429",
		},
		{
			"bad credentials",
			"gh: Bad credentials (HTTP 401)",
			"HTTP 401",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deskkit.ResetTrace()
			ghFailure(t, c.ghStderr, 1)
			_, err := dedupeSearch("medici-finance/assay", "a title with several scorable words")
			if err == nil {
				t.Fatal("a failed gh search must return an error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the search failure does not name %s — an operator cannot tell this "+
					"from the other two failure classes:\n%s", c.want, err.Error())
			}

			// And the exit-6 refusal the caller builds over it must carry the status too:
			// that is the sentence the operator actually sees.
			refusal := deskkit.Unverifiable(
				"dedupe search failed — refuse rather than mint a possible duplicate "+
					"(override with --force-new --reason)", err)
			if !strings.Contains(refusal.Error(), c.want) {
				t.Errorf("the exit-6 refusal swallowed the API status:\n%s", refusal.Error())
			}
			if deskkit.ExitCodeOf(refusal) != deskkit.ExitUnverifiable {
				t.Errorf("the fail-closed verdict changed: got %d", deskkit.ExitCodeOf(refusal))
			}
		})
	}
}

// TestSearchFailureCarriesCommandAndStatusForTheTrace pins the structured half: the argv and
// gh's exit status must be on the error, or DESK_TRACE has nothing to print for the commonest
// deskfile failure there is.
func TestSearchFailureCarriesCommandAndStatusForTheTrace(t *testing.T) {
	deskkit.ResetTrace()
	ghFailure(t, "gh: Bad credentials (HTTP 401)", 1)
	_, err := dedupeSearch("medici-finance/assay", "a title with several scorable words")
	if err == nil {
		t.Fatal("expected the stubbed failure")
	}
	var de *deskkit.DeskError
	if !errors.As(err, &de) {
		t.Fatalf("deskfile's gh runner no longer produces a *DeskError: %T", err)
	}
	if !strings.Contains(de.Cmd, "search issues") {
		t.Errorf("the command line as executed was not carried: %q", de.Cmd)
	}
	if de.ExitStatus == 0 {
		t.Errorf("gh's exit status was not carried: %d", de.ExitStatus)
	}

	deskkit.SetTrace(true)
	defer deskkit.ResetTrace()
	var b strings.Builder
	deskkit.ReportError(&b, err)
	for _, want := range []string{"desk-trace: failing command:", "search issues", "HTTP 401"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("DESK_TRACE output missing %q; got:\n%s", want, b.String())
		}
	}
}

// TestDeskfileTraceOffIsByteIdentical — the no-regression floor.
func TestDeskfileTraceOffIsByteIdentical(t *testing.T) {
	deskkit.ResetTrace()
	t.Setenv("DESK_TRACE", "")
	ghFailure(t, "gh: Bad credentials (HTTP 401)", 1)
	_, err := dedupeSearch("medici-finance/assay", "a title with several scorable words")
	if err == nil {
		t.Fatal("expected the stubbed failure")
	}
	var b strings.Builder
	deskkit.ReportError(&b, err)
	if b.String() != err.Error()+"\n" {
		t.Errorf("trace-off output is not byte-identical to err.Error():\n got %q\nwant %q",
			b.String(), err.Error()+"\n")
	}
}

// TestGhStderrIsStrippedOfControlSequences is a SECURITY-adjacent no-regression check: gh's
// stderr quotes issue titles authored by arbitrary users on public repos, so it is
// attacker-influenced text on its way to an operator's terminal. The pre-existing
// StripControl at this choke point must survive the move to the shared runner.
func TestGhStderrIsStrippedOfControlSequences(t *testing.T) {
	deskkit.ResetTrace()
	ghFailure(t, "gh: could not resolve \x1b[31mtitle\x07 (HTTP 422)", 1)
	_, err := dedupeSearch("medici-finance/assay", "a title with several scorable words")
	if err == nil {
		t.Fatal("expected the stubbed failure")
	}
	if strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Errorf("terminal-active bytes reached the message: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "HTTP 422") {
		t.Errorf("stripping ate the diagnosis: %q", err.Error())
	}
}
