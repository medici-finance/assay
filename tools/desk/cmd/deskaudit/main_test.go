package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// seedHome points the desk state dir at a temp HOME and returns the audit.jsonl path.
func seedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	return filepath.Join(dir, "audit.jsonl")
}

// TestRecoverVerbQuarantinesAndCarries runs the actual `deskaudit recover` path (Guard +
// RecoverCorruptAudit) against a corrupt log and asserts it exits OK, carries the good
// entries, and quarantines the bad line.
func TestRecoverVerbQuarantinesAndCarries(t *testing.T) {
	audit := seedHome(t)
	good := `{"ts":"2026-01-01T00:00:00Z","tool":"deskpost","verb":"comment","result":"ok","pr":1,"headSHA":null}`
	bad := `{"ts":"2026-01-01T00:00:01Z","tool":"deskpost","verb":"comm`
	if err := os.WriteFile(audit, []byte(good+"\n"+bad+"\n"), 0o600); err != nil {
		t.Fatalf("seed audit: %v", err)
	}

	var out, errb bytes.Buffer
	if code := run([]string{"recover"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("recover exit=%d (want 0); stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "carried 1") || !strings.Contains(out.String(), "quarantined 1") {
		t.Errorf("recover summary unexpected: %q", out.String())
	}

	// The live log now parses cleanly with only the good line.
	b, err := os.ReadFile(audit)
	if err != nil {
		t.Fatalf("read recovered audit: %v", err)
	}
	if strings.Contains(string(b), `"verb":"comm"`) || strings.Count(string(b), "\n") != 1 {
		t.Errorf("recovered log still holds the bad line or wrong count: %q", string(b))
	}
	// A corrupt-* sidecar exists.
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(audit), "audit.jsonl.corrupt-*"))
	if len(matches) == 0 {
		t.Errorf("no quarantine sidecar written")
	}
}

// TestRecoverVerbCleanLogIsNoOp — a clean log reports "already clean" and exits OK.
func TestRecoverVerbCleanLogIsNoOp(t *testing.T) {
	audit := seedHome(t)
	good := `{"ts":"2026-01-01T00:00:00Z","tool":"deskpost","verb":"comment","result":"ok","pr":1,"headSHA":null}`
	if err := os.WriteFile(audit, []byte(good+"\n"), 0o600); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"recover"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("recover exit=%d (want 0); stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "already clean") {
		t.Errorf("clean-log summary unexpected: %q", out.String())
	}
}

// TestUnknownVerbRefused — an unknown verb is a refusal (exit 5), not a silent success.
func TestUnknownVerbRefused(t *testing.T) {
	seedHome(t)
	var out, errb bytes.Buffer
	if code := run([]string{"frobnicate"}, &out, &errb); code != deskkit.ExitRefused {
		t.Fatalf("unknown verb exit=%d (want %d)", code, deskkit.ExitRefused)
	}
}

// TestTailAcrossSegmentsAndThreeStates — `deskaudit tail` is the ledger's read verb. It has
// to cross the daily segment boundary (the rotation is invisible to every other reader, and
// must be invisible here too), and it has to report its three states distinctly: an absent
// ledger is empty history, an unreadable one is a refusal, and a malformed line is PRINTED
// with a note rather than dropped — a reader that silently skips the one line that broke
// every other tool is the opposite of what the verb is for.
func TestTailAcrossSegmentsAndThreeStates(t *testing.T) {
	t.Run("absent ledger is empty history, not a fault", func(t *testing.T) {
		audit := seedHome(t)
		if err := os.Remove(audit); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove: %v", err)
		}
		var out, errb bytes.Buffer
		if code := run([]string{"tail"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("tail on an absent ledger exit=%d (want 0); stderr=%s", code, errb.String())
		}
		if !strings.Contains(out.String(), "no audit history") {
			t.Fatalf("an absent ledger must say so by name, not print nothing: %q", out.String())
		}
	})

	t.Run("crosses the segment boundary, newest N, oldest first", func(t *testing.T) {
		audit := seedHome(t)
		dir := filepath.Dir(audit)
		seg := filepath.Join(dir, "audit.jsonl.2026-09-13")
		var segLines, liveLines []string
		for i := 1; i <= 3; i++ {
			segLines = append(segLines, `{"ts":"2026-09-13T00:00:0`+itoa(i)+`Z","tool":"deskpost","verb":"comment","result":"ok","pr":`+itoa(i)+`,"headSHA":null,"detail":"seg`+itoa(i)+`"}`)
		}
		for i := 1; i <= 3; i++ {
			liveLines = append(liveLines, `{"ts":"2026-09-14T00:00:0`+itoa(i)+`Z","tool":"deskpost","verb":"comment","result":"ok","pr":`+itoa(i)+`,"headSHA":null,"detail":"live`+itoa(i)+`"}`)
		}
		if err := os.WriteFile(seg, []byte(strings.Join(segLines, "\n")+"\n"), 0o600); err != nil {
			t.Fatalf("write segment: %v", err)
		}
		if err := os.WriteFile(audit, []byte(strings.Join(liveLines, "\n")+"\n"), 0o600); err != nil {
			t.Fatalf("write live: %v", err)
		}

		var out, errb bytes.Buffer
		if code := run([]string{"tail", "5"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("tail 5 exit=%d; stderr=%s", code, errb.String())
		}
		got := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
		if len(got) != 5 {
			t.Fatalf("tail 5 printed %d lines, want 5: %q", len(got), out.String())
		}
		want := []string{"seg2", "seg3", "live1", "live2", "live3"}
		for i, w := range want {
			if !strings.Contains(got[i], `"detail":"`+w+`"`) {
				t.Fatalf("line %d = %q, want the %s row — tail must read across segments, oldest-first among those printed", i, got[i], w)
			}
		}
		// Raw lines: byte-for-byte what is on disk, no reserialisation.
		if got[4] != liveLines[2] {
			t.Fatalf("tail reserialised a row:\n got %q\nwant %q", got[4], liveLines[2])
		}
	})

	t.Run("a malformed line is printed with a note, never dropped", func(t *testing.T) {
		audit := seedHome(t)
		good := `{"ts":"2026-09-14T00:00:01Z","tool":"deskpost","verb":"comment","result":"ok","pr":1,"headSHA":null}`
		if err := os.WriteFile(audit, []byte(good+"\n{half a line\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		var out, errb bytes.Buffer
		if code := run([]string{"tail", "2"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("tail exit=%d; stderr=%s", code, errb.String())
		}
		if !strings.Contains(out.String(), "{half a line") {
			t.Fatalf("the malformed line was dropped: %q", out.String())
		}
		if !strings.Contains(errb.String(), "deskaudit recover") {
			t.Fatalf("a malformed line must name the recovery verb on stderr: %q", errb.String())
		}
	})

	t.Run("bad N is refused rather than defaulted", func(t *testing.T) {
		seedHome(t)
		for _, arg := range []string{"0", "-1", "ten", "10001"} {
			var out, errb bytes.Buffer
			if code := run([]string{"tail", arg}, &out, &errb); code != deskkit.ExitRefused {
				t.Fatalf("tail %q exit=%d, want %d (refused)", arg, code, deskkit.ExitRefused)
			}
		}
	})
}

func itoa(i int) string { return string(rune('0' + i)) }
