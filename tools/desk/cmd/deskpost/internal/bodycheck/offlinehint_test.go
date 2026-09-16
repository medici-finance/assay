package bodycheck

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestSchemaRefusalNamesTheOfflineCheck.
//
// These two are the verb's largest refusal classes by a wide margin — measured on one
// operating desk host over 32 days, 1,065 of 1,611 refusals (66.1%) — and both are decidable
// with no network at all. The verb has shipped an offline rehearsal (`--dry-run`: run every
// check, stop before the write, exit 0, charged to neither write meter) the whole time, and
// neither refusal mentioned it. The rehearsal was documented in `--help`, which is precisely
// the text an operator who is mid-write does not have open; the refusal is the text they ARE
// looking at, so the refusal carries the pointer.
func TestSchemaRefusalNamesTheOfflineCheck(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want string // a distinctive fragment of the diagnosis, which must survive intact
	}{
		{
			name: "no H2 heading",
			body: "Verdict: approve\n",
			want: "no '## ' heading",
		},
		{
			name: "no verdict line",
			body: "## Summary\n\nLooks fine to me.\n",
			want: "no verdict line",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := Review([]byte(c.body))
			if err == nil {
				t.Fatalf("a body with %s was accepted", c.name)
			}
			got := err.Error()
			// The diagnosis itself must survive: a hint that replaced the explanation would
			// trade one usability problem for a worse one.
			if !strings.Contains(got, c.want) {
				t.Errorf("the original diagnosis is gone — want it to contain %q:\n%s", c.want, got)
			}
			// And the pointer must be there, naming both the verb and the flag.
			if !strings.Contains(got, "--dry-run") {
				t.Errorf("the refusal does not name the offline check that would have caught it:\n%s", got)
			}
			if !strings.Contains(got, "deskpost") {
				t.Errorf("the refusal names a flag but not the verb to pass it to:\n%s", got)
			}
			// Adding a hint is not a change of verdict.
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Errorf("exit = %d, want %d", deskkit.ExitCodeOf(err), deskkit.ExitRefused)
			}
		})
	}

	// THE NEGATIVE CONTROL. A well-formed body must still pass: the hint is appended to
	// refusals, never to an acceptance, and a check that started refusing valid bodies to
	// deliver advice would be a far worse defect than the one this fixes.
	if err := Review([]byte("## Summary\n\nVerdict: approve\n")); err != nil {
		t.Errorf("a well-formed review body was refused: %v", err)
	}
}
