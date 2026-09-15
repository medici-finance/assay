package deskkit

import (
	"errors"
	"flag"
	"testing"
)

// TestHelpOnlyHasNoFalsePositive is the whole safety argument for tier one, as a test.
//
// Tier one returns BEFORE the kill-switch gate, so a shape it wrongly matched would turn a
// real invocation into a help screen — silently, and with the operator believing the work was
// done. The restriction that makes that impossible is that it matches exactly one shape: a
// leading subcommand token followed by EXACTLY one token that is exactly -h / -help / --help.
// A lone trailing token cannot be a preceding flag's VALUE, because there is no preceding
// flag left for it to belong to.
//
// The negative cases below are the ones that would break if that restriction were ever
// relaxed into "contains --help anywhere", which is the obvious and wrong implementation.
func TestHelpOnlyHasNoFalsePositive(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want bool
	}{
		{"subcommand plus long help", []string{"create", "--help"}, true},
		{"subcommand plus short help", []string{"create", "-h"}, true},
		{"subcommand plus single-dash help", []string{"update", "-help"}, true},

		{"nil", nil, false},
		{"empty", []string{}, false},
		{"subcommand alone", []string{"create"}, false},
		{"top-level form belongs to the verb's own dispatch", []string{"--help"}, false},
		{"top-level form with a subcommand after it", []string{"--help", "create"}, false},
		// The four that matter: in each of these --help is a VALUE, a positional, or one of
		// several tokens, and treating any of them as a help request would swallow a real run.
		{"help as a flag value", []string{"create", "--title", "--help"}, false},
		{"help as a body-file path", []string{"create", "--body-file", "--help"}, false},
		{"help after the terminator is a positional", []string{"create", "--", "--help"}, false},
		{"help with a trailing argument", []string{"create", "--help", "extra"}, false},
		{"help with another flag", []string{"create", "--help", "--json"}, false},

		{"an ordinary two-token invocation", []string{"attach", "--to"}, false},
		{"a near-miss spelling", []string{"create", "---help"}, false},
		{"a near-miss word", []string{"create", "help"}, false},
	} {
		if got := HelpOnly(c.args); got != c.want {
			t.Errorf("HelpOnly(%q) = %v, want %v — %s", c.args, got, c.want, c.name)
		}
	}
}

// IsHelpRequest must recognise BOTH the flag package's own sentinel (so a verb can hand
// through whatever fs.Parse returned, untranslated) and this package's, and must recognise
// neither a nil error nor an ordinary refusal — a refusal that stopped writing its audit row
// would be a real loss of the record.
func TestIsHelpRequest(t *testing.T) {
	if !IsHelpRequest(flag.ErrHelp) {
		t.Error("flag.ErrHelp is not recognised — a verb handing through fs.Parse's error would still audit a help screen")
	}
	if !IsHelpRequest(ErrHelpRequested) {
		t.Error("the package's own sentinel is not recognised")
	}
	if !IsHelpRequest(&wrapped{flag.ErrHelp}) {
		t.Error("a WRAPPED flag.ErrHelp is not recognised — errors.Is must see through the wrapper")
	}
	if IsHelpRequest(nil) {
		t.Error("nil is a help request")
	}
	if IsHelpRequest(Refused("refused: --title is required")) {
		t.Error("an ordinary refusal reads as a help request — it would stop writing its audit row")
	}
	if IsHelpRequest(Unverifiable("could not read", nil)) {
		t.Error("an unverifiable reads as a help request")
	}
	if IsHelpRequest(errors.New("plain")) {
		t.Error("a plain error reads as a help request")
	}
	// A help request must not be mapped onto a failing exit code: the operator asked a
	// question and got an answer.
	if got := ExitCodeOf(ErrHelpRequested); got != ExitOK {
		t.Errorf("ExitCodeOf(ErrHelpRequested) = %d, want %d", got, ExitOK)
	}
}

type wrapped struct{ err error }

func (w *wrapped) Error() string { return "wrapped: " + w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }
