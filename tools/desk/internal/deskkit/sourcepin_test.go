package deskkit

// sourcepin_test.go — the reader for a SOURCE-CHANNEL ("channel D") pin line, and
// the three-state release-line lookup a caller needs in order to fall through to
// it on ABSENCE only.
//
// A source line pins the commit a consumer builds from rather than the sha256 of
// a published release asset. Two column layouts are both in the field, and a
// reader that knows only one of them reports a valid pin as unreadable — which is
// how a whole channel of adopters read as unpinned.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

// pinFile writes a `.assay-versions` holding body and returns its directory.
func pinFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AssayVersionsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestSourcePin_BothColumnLayouts: the commit is read from whichever column
// carries it, and field 2 is read as a TAG when it is one.
func TestSourcePin_BothColumnLayouts(t *testing.T) {
	cases := []struct {
		name       string
		line       string
		wantCommit string
		wantTag    string
		wantRef    string
	}{
		{
			name:       "commit in field 3, tag in field 2",
			line:       "statusgen-source statusgen/v0.9.1 " + testCommit,
			wantCommit: testCommit,
			wantTag:    "statusgen/v0.9.1",
			wantRef:    "statusgen/v0.9.1", // a tag is comparable against a running binary's version
		},
		{
			name:       "commit in field 2, channel marker in field 3",
			line:       "statusgen-source " + testCommit + " channel-D",
			wantCommit: testCommit,
			wantRef:    testCommit,
		},
		{
			name:    "plain tag with a channel marker and no commit",
			line:    "statusgen-source v1.0.9 channel-D",
			wantTag: "v1.0.9",
			wantRef: "v1.0.9",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ref, found, err := SourcePin(pinFile(t, c.line+"\n"), "statusgen")
			if err != nil || !found {
				t.Fatalf("SourcePin: found=%v err=%v", found, err)
			}
			if ref.Artifact != "statusgen"+SourcePinSuffix {
				t.Errorf("artifact = %q", ref.Artifact)
			}
			if ref.Commit != c.wantCommit {
				t.Errorf("commit = %q, want %q", ref.Commit, c.wantCommit)
			}
			if ref.Tag != c.wantTag {
				t.Errorf("tag = %q, want %q", ref.Tag, c.wantTag)
			}
			if ref.Ref() != c.wantRef || !ref.Usable() {
				t.Errorf("Ref() = %q (usable=%v), want %q", ref.Ref(), ref.Usable(), c.wantRef)
			}
		})
	}
}

// TestSourcePin_ThreeStates: absent is not an error, an unusable line is found
// but not usable, and a malformed line fails closed like every other pin read.
func TestSourcePin_ThreeStates(t *testing.T) {
	t.Run("absent is not an error", func(t *testing.T) {
		ref, found, err := SourcePin(pinFile(t, "statusgen v0.9.1 "+strings.Repeat("a", 64)+"\n"), "statusgen")
		if err != nil || found {
			t.Fatalf("found=%v err=%v, want the absent third state", found, err)
		}
		if ref.Usable() {
			t.Errorf("an absent pin reported usable")
		}
	})

	t.Run("neither column identifies anything: found, but not usable", func(t *testing.T) {
		ref, found, err := SourcePin(pinFile(t, "statusgen-source not-a-tag channel-D\n"), "statusgen")
		if err != nil || !found {
			t.Fatalf("found=%v err=%v — the line IS present and must be reported as such", found, err)
		}
		if ref.Usable() || ref.Ref() != "" {
			t.Errorf("Ref() = %q, want an unusable pin", ref.Ref())
		}
		if ref.Field2 != "not-a-tag" || ref.Field3 != "channel-D" {
			t.Errorf("raw columns not preserved: %q / %q — a refusal must be able to quote what it read",
				ref.Field2, ref.Field3)
		}
	})

	t.Run("a short line fails closed", func(t *testing.T) {
		if _, _, err := SourcePin(pinFile(t, "statusgen-source "+testCommit+"\n"), "statusgen"); !IsUnverifiable(err) {
			t.Fatalf("err = %v, want Unverifiable", err)
		}
	})

	t.Run("an unreadable file fails closed", func(t *testing.T) {
		if _, _, err := SourcePin(t.TempDir(), "statusgen"); !IsUnverifiable(err) {
			t.Fatalf("err = %v, want Unverifiable", err)
		}
	})

	t.Run("the trailing-space prefix match still disambiguates", func(t *testing.T) {
		_, found, err := SourcePin(pinFile(t, "statusgen-source-notes v0.9.1 "+testCommit+"\n"), "statusgen")
		if err != nil || found {
			t.Fatalf("found=%v err=%v — `statusgen-source ` must not match `statusgen-source-notes`", found, err)
		}
	})
}

// TestPlatformPinLookup_ThreeStates: the release-line reader tells ABSENT from
// MALFORMED. Collapsing them is what would let a caller with a further pin shape
// to try skip a broken release line instead of failing closed on it.
func TestPlatformPinLookup_ThreeStates(t *testing.T) {
	t.Run("absent: no error, found=false", func(t *testing.T) {
		_, _, found, err := PlatformPinLookup(pinFile(t, "desk-tools v0.9.1 "+strings.Repeat("a", 64)+"\n"), "statusgen")
		if err != nil || found {
			t.Fatalf("found=%v err=%v, want absent", found, err)
		}
	})

	t.Run("malformed: fail-closed, never absent", func(t *testing.T) {
		_, _, found, err := PlatformPinLookup(pinFile(t, "statusgen only-a-tag\n"), "statusgen")
		if !IsUnverifiable(err) || found {
			t.Fatalf("found=%v err=%v, want a fail-closed refusal", found, err)
		}
	})

	t.Run("present: the bare line", func(t *testing.T) {
		sha := strings.Repeat("b", 64)
		tag, got, found, err := PlatformPinLookup(pinFile(t, "statusgen v0.9.1 "+sha+"\n"), "statusgen")
		if err != nil || !found || tag != "v0.9.1" || got != sha {
			t.Fatalf("tag=%q sha=%q found=%v err=%v", tag, got, found, err)
		}
	})

	t.Run("present: this host's platform line", func(t *testing.T) {
		sha := strings.Repeat("c", 64)
		host := HostPlatformAssets("statusgen")[0]
		tag, got, found, err := PlatformPinLookup(pinFile(t, host+" v0.9.1 "+sha+"\n"), "statusgen")
		if err != nil || !found || tag != "v0.9.1" || got != sha {
			t.Fatalf("tag=%q sha=%q found=%v err=%v", tag, got, found, err)
		}
	})
}

// TestPlatformPin_StillNamesWhatItLookedFor: the wrapper's absent-case message is
// unchanged by the three-state split under it.
func TestPlatformPin_StillNamesWhatItLookedFor(t *testing.T) {
	_, _, err := PlatformPin(pinFile(t, "desk-tools v0.9.1 "+strings.Repeat("a", 64)+"\n"), "statusgen")
	if !IsUnverifiable(err) {
		t.Fatalf("err = %v, want Unverifiable", err)
	}
	for _, want := range []string{"no statusgen pin", "bare `statusgen ` line", HostPlatformAssets("statusgen")[0]} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message does not name %q: %v", want, err)
		}
	}
}
