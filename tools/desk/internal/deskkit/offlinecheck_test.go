package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The hint has to NAME the tool and the flag, because a refusal that says "rehearse this"
// without saying with what is worse than saying nothing: it reads as advice and cannot be
// acted on.
func TestOfflineCheckHintNamesToolAndFlag(t *testing.T) {
	got := OfflineCheckHint("deskpost", "--dry-run")
	for _, want := range []string{"deskpost", "--dry-run"} {
		if !strings.Contains(got, want) {
			t.Errorf("hint does not name %q: %q", want, got)
		}
	}
	// It is appended to a message that already ends in a full stop, so it must begin with a
	// separator rather than running into the previous sentence.
	if !strings.HasPrefix(got, " ") {
		t.Errorf("hint does not begin with a separating space: %q", got)
	}
}

// An unnamed tool or flag yields NO hint. A half-built sentence in a refusal is the kind of
// defect nobody reports, and a pointer to a flag that was never named is not a pointer.
func TestOfflineCheckHintEmptyWhenUnnamed(t *testing.T) {
	for _, c := range [][2]string{{"", "--dry-run"}, {"deskpost", ""}, {"", ""}, {"  ", "--dry-run"}} {
		if got := OfflineCheckHint(c[0], c[1]); got != "" {
			t.Errorf("OfflineCheckHint(%q, %q) = %q, want empty", c[0], c[1], got)
		}
	}
}

// SchemaRefusal must PRESERVE the caller's message verbatim and add the hint — a constructor
// that rewrote or truncated the diagnosis would trade one usability problem for a worse one.
// And it must stay exit 5: adding a hint is not a change of verdict.
func TestSchemaRefusalKeepsTheMessageAndTheExitCode(t *testing.T) {
	msg := "refused: review body has no verdict line — it must carry a line 'Verdict: approve'"
	err := SchemaRefusal("deskpost", "--dry-run", msg)
	if !strings.Contains(err.Error(), msg) {
		t.Errorf("the original diagnosis was not preserved:\n got %q\nwant it to contain %q", err.Error(), msg)
	}
	if !strings.Contains(err.Error(), "--dry-run") {
		t.Errorf("the refusal does not name the offline check: %q", err.Error())
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("SchemaRefusal exit = %d, want %d — a hint is not a change of verdict", ExitCodeOf(err), ExitRefused)
	}
}

// ONE COPY. The template literal lives in exactly one file, so a second copy cannot drift
// from the first — which is the failure mode that produced the situation this helper exists
// to fix: the rehearsal flag was documented in one place and absent from the place operators
// were actually reading.
func TestHintTemplateOccursExactlyOnceInTheTree(t *testing.T) {
	root := ".." // tools/desk/internal
	// The distinctive fragment of the template — enough to be unmistakable, short enough
	// that a reflow of the source line cannot hide a second copy from this search.
	needle := "runs every "
	full := "Rehearse this offline first:"
	hits := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		n := strings.Count(string(b), full)
		if n > 0 && !strings.Contains(string(b), needle) {
			t.Errorf("%s carries the hint's opening but not its body — a partial second copy", path)
		}
		hits += n
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if hits != 1 {
		t.Errorf("the hint template literal occurs %d times under %s, want exactly 1 — a second copy will drift", hits, root)
	}
}
