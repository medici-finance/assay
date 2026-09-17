package main

// scanissues_nativeread_test.go — the #1223 acceptance proof.
//
// #1223 retires the `--scan-issues` OPEN-issue read's dependency on shelling out to `gh` and
// routes it through the native forge (via the `deskread` verb). The proof the shell-out is gone
// is a read that runs authenticated with NO working `gh` on PATH: the migrated path reaches the
// forge through `deskread`, so it succeeds where the `gh` shell-out cannot.
//
// FAIL-FIRST. Before #1223 the production --scan-issues lister was ghIssueLister, which shells
// `gh issue list`. With `gh` stubbed to fail (below), that read errors — so the production
// assertion (defaultScanIssueLister succeeds) FAILS. The mutation that re-reddens this test on
// the fixed tree is reverting the production default back to ghIssueLister
// (`var defaultScanIssueLister issueLister = ghIssueLister`): the gh stub then fails the read
// again.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeStubBin writes an executable stub script into dir under name.
func writeStubBin(t *testing.T, dir, name, script string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub %s: %v", name, err)
	}
}

// TestScanIssueReadUsesNativeForgeNotGH proves the --scan-issues open-issue read no longer
// depends on `gh`: with `gh` stubbed to fail and `deskread` serving a valid envelope, the
// production lister still reads the repo's open issues.
func TestScanIssueReadUsesNativeForgeNotGH(t *testing.T) {
	bin := t.TempDir()

	// `gh` on PATH, but stubbed to FAIL — the runtime condition #1223 exists to survive
	// (a HOME override / child-process naming / single-installation GH_TOKEN that leaves the
	// gh shell-out unauthenticated). Any read that reaches `gh` errors.
	writeStubBin(t, bin, "gh", "#!/bin/sh\necho 'gh: stubbed to fail (native-read test)' >&2\nexit 1\n")

	// `deskread` on PATH, serving the native-forge envelope for whatever repo it is asked for
	// ($3 is the --repo value: `deskread issues --repo <repo>`). Unquoted heredoc so $repo
	// interpolates; the body carries no command substitution.
	writeStubBin(t, bin, "deskread", "#!/bin/sh\n"+
		"repo=\"$3\"\n"+
		"cat <<EOF\n"+
		"{\"schema\":1,\"kind\":\"issues\",\"repos\":[{\"repo\":\"$repo\",\"issues\":"+
		"[{\"number\":4242,\"title\":\"native read works\",\"state\":\"open\","+
		"\"authorLogin\":\"deviant-ozzie\",\"labels\":[\"bug\"],"+
		"\"createdAt\":\"2026-09-16T00:00:00Z\",\"url\":\"https://example.test/$repo/issues/4242\"}]}],"+
		"\"partial\":[]}\n"+
		"EOF\n")

	// Stub dir FIRST on PATH so the stubs shadow any real `gh`/`deskread`; the rest of PATH
	// stays only so the stub scripts can resolve `cat`. The `gh` on PATH is the failing stub,
	// so a read that succeeds here succeeded WITHOUT any working `gh`.
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	const repo = "example-org/alpha"

	// Guard: the gh stub is effective — the PRE-migration lister (ghIssueLister, which shells
	// `gh`) errors. Without this a defaultScanIssueLister that silently fell back to gh could
	// pass the assertion below on a broken stub.
	if _, err := ghIssueLister(repo); err == nil {
		t.Fatal("ghIssueLister succeeded with `gh` stubbed to fail — the gh stub is not effective, " +
			"so the native-read assertion below would prove nothing")
	}

	// The PRODUCTION --scan-issues lister reads the same repo successfully with no working
	// `gh` — because it routes through `deskread` on the native forge seam. This is the line
	// that reddens on the unmigrated tree (ghIssueLister as the default).
	issues, err := defaultScanIssueLister(repo)
	if err != nil {
		t.Fatalf("defaultScanIssueLister(%s) errored with a working deskread and no working gh: %v — "+
			"the read still depends on the gh shell-out", repo, err)
	}
	if len(issues) != 1 {
		t.Fatalf("defaultScanIssueLister(%s) returned %d issues, want 1 (the deskread envelope's one issue)", repo, len(issues))
	}
	got := issues[0]
	if got.Number != 4242 {
		t.Errorf("issue number = %d, want 4242", got.Number)
	}
	if got.Author.Login != "deviant-ozzie" {
		t.Errorf("issue author login = %q, want %q (carried through from the native forge's authorLogin)", got.Author.Login, "deviant-ozzie")
	}
	if names := labelNames(got.Labels); len(names) != 1 || names[0] != "bug" {
		t.Errorf("issue labels = %v, want [bug]", names)
	}
}

// TestScanIssueReadCouldNotCheckIsAnError proves the migrated lister preserves ghIssueLister's
// per-repo contract: a repo `deskread` reports as unreadable (in the envelope's `partial` list)
// comes back as an ERROR, which planScan degrades to a could-not-check skip rather than reading
// as an empty (clean) board — the property that keeps a failed read from retiring live
// placeholders.
func TestScanIssueReadCouldNotCheckIsAnError(t *testing.T) {
	bin := t.TempDir()
	// deskread reports the repo in `partial` (could-not-check), with an empty `repos` list —
	// exactly deskread's exit-0 partial shape.
	writeStubBin(t, bin, "deskread", "#!/bin/sh\n"+
		"repo=\"$3\"\n"+
		"cat <<EOF\n"+
		"{\"schema\":1,\"kind\":\"issues\",\"repos\":[],"+
		"\"partial\":[{\"repo\":\"$repo\",\"reason\":\"the App installation cannot read it\"}]}\n"+
		"EOF\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := defaultScanIssueLister("example-org/beta"); err == nil {
		t.Fatal("defaultScanIssueLister returned nil error for a repo deskread reported as could-not-check — " +
			"a could-not-check must be an error, never an empty clean read")
	}
}
