package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

const goldenRel = "testdata/assay-versions-live.golden"

// writeRoot writes body to root/.assay-versions and returns root.
func writeRoot(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, AssayVersionsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// goldenRoot copies the checked-in live golden into a temp root.
func goldenRoot(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(goldenRel)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, AssayVersionsFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestArtifactPin_TrailingSpaceSemantics is the load-bearing property: a lookup
// for a bare artifact name must NOT match its per-platform lines, and vice
// versa. This is what keeps the bare CI pin and the per-platform install pins
// from colliding.
func TestArtifactPin_TrailingSpaceSemantics(t *testing.T) {
	root := goldenRoot(t)

	// Bare statusgen resolves to the CI (linux-amd64) pin.
	tag, sha, err := ArtifactPin(root, "statusgen")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "statusgen/v0.8.2" {
		t.Errorf("statusgen tag = %q, want statusgen/v0.8.2", tag)
	}
	if len(sha) != 64 {
		t.Errorf("statusgen sha = %q, want a 64-hex digest", sha)
	}

	// desk-tools has NO bare line in the live file: the lookup must fail-closed,
	// NOT silently match desk-tools-linux-amd64.
	if _, _, err := ArtifactPin(root, "desk-tools"); err == nil {
		t.Error("ArtifactPin(desk-tools) matched something — the bare lookup must not match desk-tools-<platform>")
	}

	// The per-platform name resolves on its own, and only to its own line.
	dtTag, _, err := ArtifactPin(root, "desk-tools-linux-amd64")
	if err != nil {
		t.Fatalf("desk-tools-linux-amd64: %v", err)
	}
	if dtTag != "desk-tools/v0.2.6" {
		t.Errorf("desk-tools-linux-amd64 tag = %q, want desk-tools/v0.2.6", dtTag)
	}

	// The source pin's field 3 is a 40-hex commit sha, not a 64-hex digest.
	_, srcField, err := ArtifactPin(root, "desk-tools-source")
	if err != nil {
		t.Fatalf("desk-tools-source: %v", err)
	}
	if len(srcField) != 40 {
		t.Errorf("desk-tools-source field3 = %q, want a 40-hex commit sha", srcField)
	}
}

// TestStatusgenPinIsThinWrapper proves the wrapper still behaves exactly like the
// generalised reader for the statusgen line.
func TestStatusgenPinIsThinWrapper(t *testing.T) {
	root := goldenRoot(t)
	wTag, wSha, wErr := StatusgenPin(root)
	aTag, aSha, aErr := ArtifactPin(root, "statusgen")
	if wTag != aTag || wSha != aSha || (wErr == nil) != (aErr == nil) {
		t.Errorf("StatusgenPin (%q,%q,%v) != ArtifactPin (%q,%q,%v)", wTag, wSha, wErr, aTag, aSha, aErr)
	}
}

// TestUmbrellaPin_ThreeStates: present, absent (the valid third state), and
// unreadable.
func TestUmbrellaPin_ThreeStates(t *testing.T) {
	t.Run("absent is not an error", func(t *testing.T) {
		root := goldenRoot(t) // the live file carries NO umbrella line
		tag, present, err := UmbrellaPin(root)
		if err != nil {
			t.Fatalf("no-umbrella must not be an error: %v", err)
		}
		if present || tag != "" {
			t.Errorf("got (%q, present=%v), want (\"\", false)", tag, present)
		}
	})
	t.Run("present", func(t *testing.T) {
		root := writeRoot(t, "assay assay/v0.9.0\nstatusgen statusgen/v0.8.2 "+lowhex64+"\n")
		tag, present, err := UmbrellaPin(root)
		if err != nil || !present || tag != "assay/v0.9.0" {
			t.Errorf("got (%q, present=%v, err=%v), want (assay/v0.9.0, true, nil)", tag, present, err)
		}
	})
	t.Run("unreadable file is fail-closed", func(t *testing.T) {
		if _, _, err := UmbrellaPin(t.TempDir()); err == nil {
			t.Error("a missing pin file returned no error")
		}
	})
}

const lowhex64 = "0000000000000000000000000000000000000000000000000000000000000000"
const lowhex40 = "0000000000000000000000000000000000000000"

// TestCheckPins_LiveGoldenIsClean is the backward-compat guarantee: the real
// consumer file validates checked-clean — comments, per-platform lines, trailing
// comments, no-bare-desk-tools, and the identical-tag/hash statusgen pair all.
func TestCheckPins_LiveGoldenIsClean(t *testing.T) {
	res := CheckPins(goldenRoot(t))
	if res.State != PinCheckedClean {
		t.Fatalf("golden is %s, want checked-clean: %v", res.State, res.Reasons)
	}
	if res.ArtifactLines != 12 {
		t.Errorf("golden has %d artifact lines, want 12", res.ArtifactLines)
	}
}

func TestCheckPins_NotADuplicate(t *testing.T) {
	// statusgen and statusgen-linux-amd64 share an identical tag AND sha256 but
	// are DISTINCT names — not a duplicate.
	body := "statusgen statusgen/v0.8.2 " + lowhex64 + "\n" +
		"statusgen-linux-amd64 statusgen/v0.8.2 " + lowhex64 + "\n"
	if res := CheckPins(writeRoot(t, body)); res.State != PinCheckedClean {
		t.Fatalf("identical tag+hash across distinct names flagged: %s %v", res.State, res.Reasons)
	}
	// A genuine repeat of the SAME name is a duplicate.
	dup := body + "statusgen statusgen/v0.8.2 " + lowhex64 + "\n"
	if res := CheckPins(writeRoot(t, dup)); res.State != PinCheckedFailed {
		t.Fatalf("a repeated statusgen name is %s, want checked-failed", res.State)
	}
}

func TestCheckPins_States(t *testing.T) {
	cases := []struct {
		name string
		body string // "" means "no file"
		want PinCheckState
	}{
		{"clean", "statusgen statusgen/v0.8.2 " + lowhex64 + "\n", PinCheckedClean},
		{"clean with trailing comment", "statusgen statusgen/v0.8.2 " + lowhex64 + "  # ci\n", PinCheckedClean},
		{"missing sha (2 fields)", "statusgen statusgen/v0.8.2\n", PinMalformed},
		{"bad tag (bare semver)", "statusgen v0.8.2 " + lowhex64 + "\n", PinCheckedFailed},
		{"leading-zero tag", "statusgen statusgen/v0.08.2 " + lowhex64 + "\n", PinCheckedFailed},
		{"short digest", "statusgen statusgen/v0.8.2 deadbeef\n", PinCheckedFailed},
		{"source pin ok (40-hex)", "desk-tools-source desk-tools/v0.2.6 " + lowhex40 + "\n", PinCheckedClean},
		{"source pin with 64-hex is wrong shape", "desk-tools-source desk-tools/v0.2.6 " + lowhex64 + "\n", PinCheckedFailed},
		{"umbrella ok", "assay assay/v0.9.0\n", PinCheckedClean},
		{"umbrella with digest is malformed", "assay assay/v0.9.0 " + lowhex64 + "\n", PinMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var root string
			if c.body == "" {
				root = t.TempDir()
			} else {
				root = writeRoot(t, c.body)
			}
			if res := CheckPins(root); res.State != c.want {
				t.Errorf("state = %s, want %s (reasons: %v)", res.State, c.want, res.Reasons)
			}
		})
	}
}

// TestCheckPins_AbsentIsCouldNotCheck: a missing file is could-not-check,
// distinct from malformed — the two facts the desk must keep apart.
func TestCheckPins_AbsentIsCouldNotCheck(t *testing.T) {
	if res := CheckPins(t.TempDir()); res.State != PinCouldNotCheck {
		t.Fatalf("missing file is %s, want could-not-check", res.State)
	}
	// And it is a DIFFERENT state from a malformed line.
	mal := CheckPins(writeRoot(t, "statusgen only-a-tag\n"))
	if mal.State != PinMalformed {
		t.Fatalf("2-field line is %s, want malformed", mal.State)
	}
	if PinCouldNotCheck == PinMalformed {
		t.Fatal("could-not-check and malformed collapsed to one code")
	}
}

// TestPinTagGrammar keeps this reader's tag pattern honest against the shapes it
// must accept and reject (distribution/02's namespace grammar).
func TestPinTagGrammar(t *testing.T) {
	ok := []string{"statusgen/v0.8.2", "desk-tools/v0.2.6", "daily-harvest/v0.1.0", "assay/v0.9.0", "assay-plugin/v1.0.0"}
	bad := []string{"v0.9.0", "statusgen/0.8.2", "statusgen/v0.08.2", "statusgen/vX.Y.Z", "/v0.1.0", "statusgen/v1.2"}
	for _, s := range ok {
		if !artifactTagPattern.MatchString(s) {
			t.Errorf("tag %q should match the namespace grammar", s)
		}
	}
	for _, s := range bad {
		if artifactTagPattern.MatchString(s) {
			t.Errorf("tag %q should NOT match the namespace grammar", s)
		}
	}
}
