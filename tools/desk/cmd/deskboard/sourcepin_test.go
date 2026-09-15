package main

// sourcepin_test.go — #1122: a channel-D consumer IS pinned.
//
// An adopter on a platform or forge the release publishes no binary for pins the
// tools by SOURCE COMMIT — `statusgen-source <40-hex-commit> channel-D` — instead
// of by release asset. The dispatch/awaiting pin resolver only ever asked for a
// bare `statusgen ` line and this host's `statusgen-<os>-<arch>` line, so such a
// pin file read as NO PIN: the verb exited 6 and Next-up came back
// could-not-check on a file that is valid and that `statusgen --lint` passes.
//
// These tests pin the three distinguishable outcomes the resolver must keep
// apart, because the defect was exactly a collapse of the first into the third:
//
//	a usable source pin      -> the board runs, and reports what it is pinned to
//	an UNUSABLE source pin   -> refuses with a NAMED reason that says channel D
//	no pin line of any shape -> refuses, naming every shape it looked for
//
// and the property a fix must not trade away: a release line still wins, and a
// present-but-broken release line is still terminal rather than being skipped in
// favour of a source line.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// channelDCommit is a syntactically real 40-hex commit id; the shape is what the
// reader keys on, so any full-length hex string exercises it.
const channelDCommit = "0123456789abcdef0123456789abcdef01234567"

// pinnedRoot builds a statusgen root whose `.assay-versions` holds exactly body,
// and configures it as the only root. The repo key is one the fixed allowed set
// already contains, so nothing here depends on widening the desk's write set.
func pinnedRoot(t *testing.T, body string) string {
	t.Helper()
	root := makeRoot(t, "channel-d", false)
	if err := os.WriteFile(filepath.Join(root, deskkit.AssayVersionsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskkit.RootsEnv, "medici-finance/assay="+root)
	return root
}

// dispatchOn runs the `dispatch` verb over the configured roots and returns the
// exit code with both streams. `dispatch` is the verb the defect was reported on;
// it resolves the pin through the shared preamble `awaiting` and the dispatch
// stage of `throughput` also use, so one call covers all three.
func dispatchOn(t *testing.T) (int, dispatchReport, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run([]string{"dispatch"}, &out, &errb)
	var rep dispatchReport
	if code == deskkit.ExitOK {
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("dispatch output is not JSON: %v\n%s", err, out.String())
		}
	}
	return code, rep, errb.String()
}

// TestDispatch_ChannelDSourcePinIsAPin is the defect, stated as a test: a pin
// file carrying ONLY source lines must run the board, not exit 6.
func TestDispatch_ChannelDSourcePinIsAPin(t *testing.T) {
	t.Run("commit-in-field-2 with the channel-D marker", func(t *testing.T) {
		installFakeStatusgen(t)
		pinnedRoot(t, "# built from source: no release asset is published for this platform\n"+
			"statusgen-source "+channelDCommit+" channel-D\n"+
			"desk-tools-source "+channelDCommit+" channel-D\n")

		code, rep, stderr := dispatchOn(t)
		if code != deskkit.ExitOK {
			t.Fatalf("dispatch exited %d, want 0 — a source-channel pin file IS pinned; stderr=%s", code, stderr)
		}
		if rep.StatusgenPinned != channelDCommit {
			t.Errorf("statusgenPinned = %q, want the pinned commit %q — the report must say what it is pinned to",
				rep.StatusgenPinned, channelDCommit)
		}
		if rep.StatusgenPinRepo != "medici-finance/assay" {
			t.Errorf("statusgenPinRepo = %q, want the root that carries the pin", rep.StatusgenPinRepo)
		}
	})

	t.Run("tag-in-field-2 reports the tag, so the skew comparison still works", func(t *testing.T) {
		installFakeStatusgen(t) // the shim reports statusgen/v0.1.0
		pinnedRoot(t, "statusgen-source statusgen/v0.1.0 "+channelDCommit+"\n")

		code, rep, stderr := dispatchOn(t)
		if code != deskkit.ExitOK {
			t.Fatalf("dispatch exited %d, want 0; stderr=%s", code, stderr)
		}
		if rep.StatusgenPinned != "statusgen/v0.1.0" {
			t.Errorf("statusgenPinned = %q, want the tag the line carries — a tag is comparable "+
				"against the running binary's version, a commit is not", rep.StatusgenPinned)
		}
		if rep.StatusgenSkew {
			t.Errorf("a source pin whose tag equals the running version reported skew")
		}
	})
}

// TestDispatch_UnusableSourcePinIsNamed: a source line that identifies nothing is
// a refusal — but never the "no pin" refusal, which sends an adopter looking for
// a line they already have.
func TestDispatch_UnusableSourcePinIsNamed(t *testing.T) {
	installFakeStatusgen(t)
	pinnedRoot(t, "statusgen-source not-a-tag channel-D\n")

	code, _, stderr := dispatchOn(t)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d — an unreadable pin must fail closed", code, deskkit.ExitUnverifiable)
	}
	for _, want := range []string{"channel D", "statusgen-source", "build from source", "install a release"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("refusal does not name %q: %s", want, stderr)
		}
	}
	if strings.Contains(stderr, "no statusgen pin") {
		t.Errorf("an unreadable SOURCE pin was reported as no pin at all: %s", stderr)
	}
}

// TestDispatch_NoPinLineOfAnyShapeNamesThemAll: the genuine no-pin case keeps its
// refusal, and now names the source line among the shapes it looked for — an
// error that omits a shape it checks is the next version of this defect.
func TestDispatch_NoPinLineOfAnyShapeNamesThemAll(t *testing.T) {
	installFakeStatusgen(t)
	pinnedRoot(t, "# a pin file that pins something else entirely\nqualgen v1.2.3 "+strings.Repeat("a", 64)+"\n")

	code, _, stderr := dispatchOn(t)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d", code, deskkit.ExitUnverifiable)
	}
	for _, want := range []string{"no statusgen pin", "statusgen-source"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("refusal does not name %q: %s", want, stderr)
		}
	}
}

// TestDispatch_ReleaseLinePinStillResolves is the regression guard on the shape
// every existing consumer uses: a release line still answers, still WINS over a
// source line in the same file, and a broken one is still terminal.
func TestDispatch_ReleaseLinePinStillResolves(t *testing.T) {
	t.Run("bare release line", func(t *testing.T) {
		installFakeStatusgen(t)
		pinnedRoot(t, "statusgen statusgen/v0.1.0 "+strings.Repeat("d", 64)+"\n")

		code, rep, stderr := dispatchOn(t)
		if code != deskkit.ExitOK {
			t.Fatalf("dispatch exited %d, want 0; stderr=%s", code, stderr)
		}
		if rep.StatusgenPinned != "statusgen/v0.1.0" {
			t.Errorf("statusgenPinned = %q, want statusgen/v0.1.0", rep.StatusgenPinned)
		}
	})

	t.Run("this host's per-platform release line", func(t *testing.T) {
		installFakeStatusgen(t)
		hostLine := deskkit.HostPlatformAssets("statusgen")[0]
		pinnedRoot(t, hostLine+" statusgen/v0.1.0 "+strings.Repeat("d", 64)+"\n")

		code, rep, stderr := dispatchOn(t)
		if code != deskkit.ExitOK {
			t.Fatalf("dispatch exited %d, want 0; stderr=%s", code, stderr)
		}
		if rep.StatusgenPinned != "statusgen/v0.1.0" {
			t.Errorf("statusgenPinned = %q, want statusgen/v0.1.0", rep.StatusgenPinned)
		}
	})

	t.Run("a release line WINS over a source line in the same file", func(t *testing.T) {
		installFakeStatusgen(t)
		pinnedRoot(t, "statusgen statusgen/v0.1.0 "+strings.Repeat("d", 64)+"\n"+
			"statusgen-source "+channelDCommit+" channel-D\n")

		code, rep, stderr := dispatchOn(t)
		if code != deskkit.ExitOK {
			t.Fatalf("dispatch exited %d, want 0; stderr=%s", code, stderr)
		}
		if rep.StatusgenPinned != "statusgen/v0.1.0" {
			t.Errorf("statusgenPinned = %q, want the release tag — the release line is authoritative "+
				"when both shapes are present", rep.StatusgenPinned)
		}
	})

	t.Run("a MALFORMED release line is terminal, never skipped for the source line", func(t *testing.T) {
		installFakeStatusgen(t)
		pinnedRoot(t, "statusgen only-a-tag\n"+
			"statusgen-source "+channelDCommit+" channel-D\n")

		code, _, stderr := dispatchOn(t)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("exit %d, want %d — a broken release line must not be silently replaced by a "+
				"source line; stderr=%s", code, deskkit.ExitUnverifiable, stderr)
		}
		if !strings.Contains(stderr, "malformed") {
			t.Errorf("refusal does not say the release line is malformed: %s", stderr)
		}
	})
}
