package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// fake statusgen — a shell shim selected via STATUSGEN_BIN. Every test drives
// the REAL exec path (deskboard shells out to a binary), so the fail-closed
// behaviour under test is the behaviour that ships.
// ---------------------------------------------------------------------------

const statusgenShim = `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "${FAKE_SG_VERSION:-statusgen/v0.1.0}"
  exit 0
fi
# --gate-scores --root <abs>
root=""
while [ $# -gt 0 ]; do
  [ "$1" = "--root" ] && { shift; root="$1"; }
  shift
done
if [ -n "$FAKE_SG_FAIL_MATCH" ] && case "$root" in *"$FAKE_SG_FAIL_MATCH"*) true;; *) false;; esac; then
  echo "statusgen: simulated failure for $root" >&2
  exit 1
fi
if [ -n "$FAKE_SG_GARBAGE_MATCH" ] && case "$root" in *"$FAKE_SG_GARBAGE_MATCH"*) true;; *) false;; esac; then
  echo "this is not json"
  exit 0
fi
name=$(basename "$root")
if [ -n "$FAKE_SG_REPO" ]; then
  # multi-root statusgen stamps each row with the repo the ROOT declares
  # (stream repo: frontmatter), which may disagree with the configured key.
  printf '[{"brief":"%s-stream/01","score":42,"blockedCount":1,"stream":"%s-stream","status":"implemented","repo":"%s"}]\n' "$name" "$name" "$FAKE_SG_REPO"
  exit 0
fi
printf '[{"brief":"%s-stream/01","score":42,"blockedCount":1,"stream":"%s-stream","status":"implemented"}]\n' "$name" "$name"
`

// installFakeStatusgen writes the shim and points STATUSGEN_BIN at it.
func installFakeStatusgen(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "statusgen")
	if err := os.WriteFile(bin, []byte(statusgenShim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(statusgenBinEnv, bin)
	return bin
}

// makeRoot builds a minimal statusgen root: docs/streams/ plus, for the tracker root,
// the .assay-versions pin file nextup reads.
func makeRoot(t *testing.T, name string, withPin bool) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0o755); err != nil {
		t.Fatal(err)
	}
	if withPin {
		pin := "# pins\nstatusgen statusgen/v0.1.0 deadbeef  # linux-amd64\n"
		if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(pin), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// twoRoots wires DESK_ROOTS at a valid tracker root (with pin) and a valid toolkit root.
func twoRoots(t *testing.T) (tracker, toolkit string) {
	t.Helper()
	tracker = makeRoot(t, "tracker", true)
	toolkit = makeRoot(t, "toolkit", false)
	t.Setenv(deskkit.RootsEnv,
		"example-org/tracker="+tracker+",medici-finance/assay="+toolkit)
	return tracker, toolkit
}

// TestNextup_MergesAcrossRoots is the happy path: every configured root is read
// and its rows land on one board, attributed to their repo.
func TestNextup_MergesAcrossRoots(t *testing.T) {
	installFakeStatusgen(t)
	twoRoots(t)

	var out, errb bytes.Buffer
	if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("nextup exited %d, stderr=%s", code, errb.String())
	}
	var rep nextupReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("nextup output is not JSON: %v\n%s", err, out.String())
	}
	if len(rep.Roots) != 2 {
		t.Fatalf("report covers %d roots, want 2", len(rep.Roots))
	}
	if len(rep.Rows) != 2 {
		t.Fatalf("merged %d rows, want 2 (one per root): %+v", len(rep.Rows), rep.Rows)
	}
	repos := map[string]bool{}
	for _, r := range rep.Rows {
		repos[r.Repo] = true
		if r.Root == "" {
			t.Error("row carries no root — a merged board must stay attributable")
		}
	}
	if !repos["example-org/tracker"] || !repos["medici-finance/assay"] {
		t.Errorf("rows are not attributed to both repos: %+v", rep.Rows)
	}
	// The JSON path must stay clean for pipe consumers (main.go's banner rule).
	if errb.Len() != 0 {
		t.Errorf("JSON path wrote to stderr: %q", errb.String())
	}
}

// TestNextup_FailsClosedOnRootError is the brief's central requirement: any root
// error exits NON-ZERO. A warn-and-continue would emit a short board that reads
// as "nothing open", and the desk would act on it (PR #1303).
func TestNextup_FailsClosedOnRootError(t *testing.T) {
	t.Run("nonexistent root", func(t *testing.T) {
		installFakeStatusgen(t)
		tracker := makeRoot(t, "tracker", true)
		t.Setenv(deskkit.RootsEnv,
			"example-org/tracker="+tracker+
				",medici-finance/assay=/nope/does/not/exist")

		var out, errb bytes.Buffer
		code := run([]string{"nextup"}, &out, &errb)
		if code == deskkit.ExitOK {
			t.Fatalf("a nonexistent root exited 0 — the board failed OPEN. stdout=%s", out.String())
		}
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
		}
		if !strings.Contains(errb.String(), "/nope/does/not/exist") {
			t.Errorf("error does not name the offending root: %q", errb.String())
		}
		if out.Len() != 0 {
			t.Errorf("a failed run still emitted a board: %s", out.String())
		}
	})

	t.Run("root without docs/streams", func(t *testing.T) {
		installFakeStatusgen(t)
		tracker := makeRoot(t, "tracker", true)
		bare := t.TempDir() // exists, but is not a statusgen root
		t.Setenv(deskkit.RootsEnv,
			"example-org/tracker="+tracker+",medici-finance/assay="+bare)

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitUnverifiable {
			t.Fatalf("a root with no docs/streams exited %d, want %d", code, deskkit.ExitUnverifiable)
		}
		if !strings.Contains(errb.String(), "docs/streams") {
			t.Errorf("error does not explain what is missing: %q", errb.String())
		}
	})

	t.Run("statusgen fails for one root", func(t *testing.T) {
		installFakeStatusgen(t)
		_, toolkit := twoRoots(t)
		t.Setenv("FAKE_SG_FAIL_MATCH", filepath.Base(toolkit))

		var out, errb bytes.Buffer
		code := run([]string{"nextup"}, &out, &errb)
		if code == deskkit.ExitOK {
			t.Fatalf("a statusgen failure on one root exited 0 — partial board. stdout=%s", out.String())
		}
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit %d, want %d", code, deskkit.ExitUnverifiable)
		}
		if !strings.Contains(errb.String(), "medici-finance/assay") {
			t.Errorf("error does not name the failing repo: %q", errb.String())
		}
	})

	t.Run("unparseable gate-scores output", func(t *testing.T) {
		installFakeStatusgen(t)
		_, toolkit := twoRoots(t)
		t.Setenv("FAKE_SG_GARBAGE_MATCH", filepath.Base(toolkit))

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitUnverifiable {
			t.Fatalf("unparseable statusgen output exited %d, want %d", code, deskkit.ExitUnverifiable)
		}
	})

	t.Run("statusgen binary missing", func(t *testing.T) {
		twoRoots(t)
		t.Setenv(statusgenBinEnv, filepath.Join(t.TempDir(), "no-such-statusgen"))

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitUnverifiable {
			t.Fatalf("a missing statusgen exited %d, want %d", code, deskkit.ExitUnverifiable)
		}
	})

	t.Run("missing .assay-versions pin", func(t *testing.T) {
		installFakeStatusgen(t)
		tracker := makeRoot(t, "tracker", false) // no pin file
		toolkit := makeRoot(t, "toolkit", false)
		t.Setenv(deskkit.RootsEnv,
			"example-org/tracker="+tracker+",medici-finance/assay="+toolkit)

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitUnverifiable {
			t.Fatalf("a missing statusgen pin exited %d, want %d — an unpinned run must not pass silently", code, deskkit.ExitUnverifiable)
		}
	})
}

// TestNextup_PinIsDiscoveredNotCompiledIn is the fix for #511: the statusgen
// pin used to require the compiled-in primary repo (example-org/tracker) to be
// among the configured roots, which made the board unrunnable
// for an adopter whose DESK_ROOTS names only their own repo (working around it
// meant naming tracker in DESK_ROOTS too, which in turn required widening
// ASSAY_ALLOWED_REPOS — the desk's WRITE authority — just to read a version
// pin). The pin must instead be discovered from WHICHEVER configured root
// actually carries a `.assay-versions` file.
func TestNextup_PinIsDiscoveredNotCompiledIn(t *testing.T) {
	t.Run("a single non-tracker root carrying the pin is sufficient", func(t *testing.T) {
		installFakeStatusgen(t)
		// Stand-in for an adopter whose roster names only their OWN repo — never
		// example-org/tracker. medici-finance/assay is
		// used here only because it is one of the two repos the fixed set
		// actually contains (so ConfiguredRoots accepts it without also
		// widening ASSAY_ALLOWED_REPOS); the point under test is that it is NOT
		// the tracker repo and the board must still run.
		root := makeRoot(t, "solo", true)
		t.Setenv(deskkit.RootsEnv, "medici-finance/assay="+root)

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("DESK_ROOTS naming only a non-tracker repo exited %d, want 0 (stderr=%s) — "+
				"the pin must not require the tracker repo to be configured", code, errb.String())
		}
		var rep nextupReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatal(err)
		}
		if rep.StatusgenPinned != "statusgen/v0.1.0" {
			t.Errorf("statusgenPinned = %q, want statusgen/v0.1.0", rep.StatusgenPinned)
		}
		if rep.StatusgenPinRepo != "medici-finance/assay" {
			t.Errorf("statusgenPinRepo = %q, want medici-finance/assay (the only configured root)", rep.StatusgenPinRepo)
		}
	})

	t.Run("a pin on a later root is still found", func(t *testing.T) {
		installFakeStatusgen(t)
		// example-org sorts before medici-finance, so this exercises "keep looking
		// past a root with no pin" with the pin on the SECOND root.
		tracker := makeRoot(t, "tracker", false) // no pin
		toolkit := makeRoot(t, "toolkit", true)  // has the pin
		t.Setenv(deskkit.RootsEnv,
			"example-org/tracker="+tracker+",medici-finance/assay="+toolkit)

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("nextup exited %d, stderr=%s", code, errb.String())
		}
		var rep nextupReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatal(err)
		}
		if rep.StatusgenPinRepo != "medici-finance/assay" {
			t.Errorf("statusgenPinRepo = %q, want the toolkit root — it is the only one with a pin", rep.StatusgenPinRepo)
		}
	})

	t.Run("a present but malformed pin fails closed, never falls through", func(t *testing.T) {
		installFakeStatusgen(t)
		tracker := makeRoot(t, "tracker", false)
		bad := filepath.Join(tracker, ".assay-versions")
		if err := os.WriteFile(bad, []byte("statusgen only-a-tag\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(deskkit.RootsEnv, "example-org/tracker="+tracker)

		var out, errb bytes.Buffer
		code := run([]string{"nextup"}, &out, &errb)
		if code == deskkit.ExitOK {
			t.Fatalf("a malformed pin file exited 0 — it must fail closed, not be skipped. stdout=%s", out.String())
		}
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit %d, want %d", code, deskkit.ExitUnverifiable)
		}
	})

	t.Run("no configured root carries a pin names all of them", func(t *testing.T) {
		installFakeStatusgen(t)
		tracker := makeRoot(t, "tracker", false)
		toolkit := makeRoot(t, "toolkit", false)
		t.Setenv(deskkit.RootsEnv,
			"example-org/tracker="+tracker+",medici-finance/assay="+toolkit)

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitUnverifiable {
			t.Fatalf("exit %d, want %d", code, deskkit.ExitUnverifiable)
		}
		for _, want := range []string{"example-org/tracker", "medici-finance/assay"} {
			if !strings.Contains(errb.String(), want) {
				t.Errorf("error does not name %s among the roots that were checked: %q", want, errb.String())
			}
		}
	})
}

// TestNextup_RepoAttributionMustAgreeWithConfiguration closes the gap where a
// root's own `repo:` declaration was trusted verbatim: a checkout could
// re-attribute its briefs to any repo string it liked — including one roots.go
// would have refused at configuration time. Agreement is fine; a
// disagreement is fail-closed, because silent misattribution is precisely what
// the cross-repo board exists to eliminate.
func TestNextup_RepoAttributionMustAgreeWithConfiguration(t *testing.T) {
	// trackerOnly configures a single root so FAKE_SG_REPO (which applies to every
	// root the shim is asked about) is unambiguous.
	trackerOnly := func(t *testing.T) {
		t.Helper()
		tracker := makeRoot(t, "tracker", true)
		t.Setenv(deskkit.RootsEnv, "example-org/tracker="+tracker)
	}

	t.Run("declaration agreeing with configuration is used", func(t *testing.T) {
		installFakeStatusgen(t)
		trackerOnly(t)
		t.Setenv("FAKE_SG_REPO", "example-org/tracker")

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("an agreeing repo declaration exited %d, stderr=%s", code, errb.String())
		}
		var rep nextupReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatal(err)
		}
		if len(rep.Rows) != 1 || rep.Rows[0].Repo != "example-org/tracker" {
			t.Errorf("rows = %+v, want one row attributed to the configured repo", rep.Rows)
		}
	})

	t.Run("declaration disagreeing with configuration fails closed", func(t *testing.T) {
		installFakeStatusgen(t)
		trackerOnly(t)
		// In the fixed set, but NOT the repo this root was configured under.
		t.Setenv("FAKE_SG_REPO", "medici-finance/assay")

		var out, errb bytes.Buffer
		code := run([]string{"nextup"}, &out, &errb)
		if code == deskkit.ExitOK {
			t.Fatalf("a root re-attributed its briefs to another repo and the board exited 0. stdout=%s", out.String())
		}
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
		}
		for _, want := range []string{"example-org/tracker", "medici-finance/assay"} {
			if !strings.Contains(errb.String(), want) {
				t.Errorf("error does not name %s — both sides of the disagreement must be visible: %q", want, errb.String())
			}
		}
		if out.Len() != 0 {
			t.Errorf("a misattributing run still emitted a board: %s", out.String())
		}
	})

	t.Run("a repo outside the fixed set cannot arrive via the data", func(t *testing.T) {
		installFakeStatusgen(t)
		trackerOnly(t)
		t.Setenv("FAKE_SG_REPO", "attacker/evil")

		var out, errb bytes.Buffer
		if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitUnverifiable {
			t.Fatalf("a row declaring an out-of-set repo exited %d, want %d — DESK_ROOTS refuses "+
				"that repo at configuration time and the data path must not be a way around it",
				code, deskkit.ExitUnverifiable)
		}
		if strings.Contains(out.String(), "attacker/evil") {
			t.Errorf("an out-of-set repo reached the board: %s", out.String())
		}
	})
}

// TestNextup_RootsReportResolvedPaths pins the coverage lines to the directory
// the rows actually came from. The configured path may be a relative or
// uncleaned spelling (the tracker default is literally "."), and "root tracker ." is a
// weak statement on the one line a reader checks when the queue looks short.
func TestNextup_RootsReportResolvedPaths(t *testing.T) {
	installFakeStatusgen(t)
	tracker := makeRoot(t, "tracker", true)
	// An uncleaned spelling of the same directory, built by concatenation because
	// filepath.Join would clean it away — resolution has to be observable.
	configured := tracker + "/../" + filepath.Base(tracker)
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+configured)

	var out, errb bytes.Buffer
	if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("nextup exited %d, stderr=%s", code, errb.String())
	}
	var rep nextupReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Roots) != 1 || len(rep.Rows) != 1 {
		t.Fatalf("roots=%+v rows=%+v, want one of each", rep.Roots, rep.Rows)
	}
	if rep.Roots[0].Path != tracker {
		t.Errorf("roots[0].path = %q, want the resolved %q", rep.Roots[0].Path, tracker)
	}
	if rep.Roots[0].Path != rep.Rows[0].Root {
		t.Errorf("roots[0].path = %q but the row's root = %q — the coverage field must agree with the rows",
			rep.Roots[0].Path, rep.Rows[0].Root)
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"nextup", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("--table exited %d", code)
	}
	if !strings.Contains(out.String(), tracker) {
		t.Errorf("--table coverage line does not carry the resolved root %q:\n%s", tracker, out.String())
	}
	// `configured` has `tracker` as a prefix, so the positive check above cannot tell
	// the two apart on its own — this is the assertion that actually bites.
	if strings.Contains(out.String(), configured) {
		t.Errorf("--table coverage line prints the configured spelling %q rather than the resolved path:\n%s",
			configured, out.String())
	}
}

// TestNextup_NeverRunsTheFrozenTree guards the venue rule: tracker's tools/statusgen
// is FROZEN, so nextup must exec a pinned BINARY and must never `go run` the
// local copy (the defect in PR #1303's version).
func TestNextup_NeverRunsTheFrozenTree(t *testing.T) {
	raw, err := os.ReadFile("nextup.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, forbidden := range []string{`"go", "run"`, `exec.Command("go"`} {
		if strings.Contains(src, forbidden) {
			t.Errorf("nextup.go contains %q — it must exec the pinned statusgen binary, never build/run the frozen in-repo copy", forbidden)
		}
	}
	if !strings.Contains(src, statusgenBinEnv) {
		t.Error("nextup.go does not honour STATUSGEN_BIN — there is no way to point it at the pinned binary")
	}
}

// TestNextup_StatusgenSkewIsReportedNotFatal pins the version contract: skew
// against the .assay-versions pin is surfaced (JSON field + --table WARN), and
// does NOT fail the board — --gate-scores is per-root regardless of statusgen
// version, so refusing would cost availability for no correctness gain.
func TestNextup_StatusgenSkewIsReportedNotFatal(t *testing.T) {
	installFakeStatusgen(t)
	twoRoots(t)
	t.Setenv("FAKE_SG_VERSION", "statusgen/v9.9.9") // pin file says v0.1.0

	var out, errb bytes.Buffer
	if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("version skew failed the board (exit %d); it must be reported, not fatal", code)
	}
	var rep nextupReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if !rep.StatusgenSkew {
		t.Error("statusgenSkew is false despite a version/pin mismatch — the skew would rot unnoticed")
	}
	if rep.StatusgenPinned != "statusgen/v0.1.0" || rep.StatusgenVersion != "statusgen/v9.9.9" {
		t.Errorf("pinned=%q running=%q, want v0.1.0 / v9.9.9", rep.StatusgenPinned, rep.StatusgenVersion)
	}

	// The --table view must SAY so; a machine-only signal nobody reads is no signal.
	out.Reset()
	errb.Reset()
	if code := run([]string{"nextup", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("--table exited %d", code)
	}
	if !strings.Contains(out.String(), "WARN statusgen") {
		t.Errorf("--table view carries no skew warning:\n%s", out.String())
	}
}

// TestNextup_EmptyBoardSaysWhatItRead closes the "reads as nothing open" gap from
// the other side: when every root really is empty, the table must say the roots
// were READ, not just print nothing.
func TestNextup_EmptyBoardSaysWhatItRead(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "statusgen")
	empty := "#!/bin/sh\n[ \"$1\" = \"--version\" ] && { echo statusgen/v0.1.0; exit 0; }\necho '[]'\n"
	if err := os.WriteFile(bin, []byte(empty), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(statusgenBinEnv, bin)
	twoRoots(t)

	var out, errb bytes.Buffer
	if code := run([]string{"nextup", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("nextup exited %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "2 configured root(s) read successfully") {
		t.Errorf("empty board does not state its coverage:\n%s", out.String())
	}
}

// TestConfiguredRoots covers the roots configuration: defaults, the DESK_ROOTS
// path override, and the refusals that keep the env from widening the fixed
// repo set.
func TestConfiguredRoots(t *testing.T) {
	t.Run("defaults are sorted and inside the fixed set", func(t *testing.T) {
		t.Setenv(deskkit.RootsEnv, "")
		roots, err := deskkit.ConfiguredRoots()
		if err != nil {
			t.Fatal(err)
		}
		if len(roots) < 2 {
			t.Fatalf("default roots = %+v, want at least tracker + assay", roots)
		}
		for i, r := range roots {
			if !deskkit.IsAllowedRepo(r.Repo) {
				t.Errorf("default root %s is outside the fixed repo set", r.Repo)
			}
			if i > 0 && roots[i-1].Repo >= r.Repo {
				t.Errorf("default roots are not sorted: %+v", roots)
			}
		}
	})

	t.Run("override replaces paths", func(t *testing.T) {
		t.Setenv(deskkit.RootsEnv, "medici-finance/assay=/tmp/tk")
		roots, err := deskkit.ConfiguredRoots()
		if err != nil {
			t.Fatal(err)
		}
		if len(roots) != 1 || roots[0].Path != "/tmp/tk" {
			t.Fatalf("override = %+v, want exactly the one named root", roots)
		}
		if got := deskkit.RootForRepo("medici-finance/assay"); got != "/tmp/tk" {
			t.Errorf("RootForRepo = %q, want /tmp/tk", got)
		}
		if got := deskkit.RootForRepo("example-org/agents"); got != "" {
			t.Errorf("RootForRepo for an unconfigured repo = %q, want empty", got)
		}
	})

	t.Run("refuses a repo outside the fixed set", func(t *testing.T) {
		t.Setenv(deskkit.RootsEnv, "attacker/evil=/tmp/evil")
		_, err := deskkit.ConfiguredRoots()
		if err == nil {
			t.Fatal("DESK_ROOTS widened the repo set — the fixed set must be compiled in")
		}
		if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
			t.Errorf("exit %d, want %d (refused)", code, deskkit.ExitRefused)
		}
	})

	t.Run("refuses malformed and duplicate entries", func(t *testing.T) {
		for _, spec := range []string{
			"no-equals-sign",
			"example-org/tracker=",
			"=/tmp/x",
			"medici-finance/assay=/a,medici-finance/assay=/b",
		} {
			t.Setenv(deskkit.RootsEnv, spec)
			if _, err := deskkit.ConfiguredRoots(); err == nil {
				t.Errorf("DESK_ROOTS=%q was accepted; it must be refused", spec)
			}
		}
	})
}

// TestStatusgenPin covers the pin reader: a good pin parses, and every unreadable
// or malformed case is Unverifiable rather than a silent default.
func TestStatusgenPin(t *testing.T) {
	t.Run("reads tag and sha", func(t *testing.T) {
		root := makeRoot(t, "tracker", true)
		tag, sha, err := deskkit.StatusgenPin(root)
		if err != nil {
			t.Fatal(err)
		}
		if tag != "statusgen/v0.1.0" || sha != "deadbeef" {
			t.Errorf("pin = %q %q, want statusgen/v0.1.0 deadbeef", tag, sha)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, _, err := deskkit.StatusgenPin(t.TempDir()); err == nil {
			t.Fatal("a missing .assay-versions returned no error")
		}
	})

	t.Run("no statusgen line", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte("other v1 abc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := deskkit.StatusgenPin(root); err == nil {
			t.Fatal("a pin file without a statusgen line returned no error")
		}
	})

	t.Run("malformed statusgen line", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte("statusgen only-a-tag\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := deskkit.StatusgenPin(root); err == nil {
			t.Fatal("a malformed statusgen pin returned no error")
		}
	})
}

// TestNextup_PinFlowFromConfiguredRoot is distribution/04's FLOW row: reading the
// pin from a configured root, resolving the statusgen binary, and rendering the
// board still work as one chain — against the REAL checked-in golden pin file
// (the live consumer's `.assay-versions`), not a synthesised one-liner. It also
// proves version skew is REPORTED: the golden pins `statusgen/v0.8.2` while the
// fake binary answers `statusgen/v0.1.0`, and a skew the desk can see is the
// cross-component behaviour this brief must not break.
//
// It is a NEW, named flow test on purpose. `TestStatusgenPin` (above) is a unit
// test of the parser and passes today; a Verify row matching `-run Pin` would go
// green without this end-to-end chain ever running.
func TestNextup_PinFlowFromConfiguredRoot(t *testing.T) {
	installFakeStatusgen(t)

	// A tracker root whose `.assay-versions` IS the checked-in live golden.
	tracker := makeRoot(t, "tracker", false)
	golden, err := os.ReadFile(filepath.Join(
		"..", "..", "internal", "deskkit", "testdata", "assay-versions-live.golden"))
	if err != nil {
		t.Fatalf("reading golden fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tracker, ".assay-versions"), golden, 0o644); err != nil {
		t.Fatal(err)
	}
	toolkit := makeRoot(t, "toolkit", false)
	t.Setenv(deskkit.RootsEnv,
		"example-org/tracker="+tracker+",medici-finance/assay="+toolkit)

	var out, errb bytes.Buffer
	if code := run([]string{"nextup"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("nextup exited %d, stderr=%s", code, errb.String())
	}
	var rep nextupReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("nextup output is not JSON: %v\n%s", err, out.String())
	}
	if len(rep.Rows) < 1 {
		t.Fatalf("board has no rows — the pin→binary→board chain did not complete: %+v", rep)
	}
	// Read straight from the golden, through the generalised reader.
	if rep.StatusgenPinned != "statusgen/v0.8.2" {
		t.Errorf("pinned = %q, want statusgen/v0.8.2 (read from the golden fixture)", rep.StatusgenPinned)
	}
	if !rep.StatusgenSkew {
		t.Errorf("skew not reported though pinned %q != running %q",
			rep.StatusgenPinned, rep.StatusgenVersion)
	}
	if rep.StatusgenPinRepo != "example-org/tracker" {
		t.Errorf("pin repo = %q, want example-org/tracker (the root carrying .assay-versions)", rep.StatusgenPinRepo)
	}
}
