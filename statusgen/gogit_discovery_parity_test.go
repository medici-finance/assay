package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
)

// These controls use existing adapter APIs, so their test-only patch can run
// against the reviewed head before the production fix is applied.
func coreDiscoveryAdapterParity(t *testing.T, root string) {
	t.Helper()
	oracle := func(args ...string) []byte {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
		if err != nil {
			t.Fatalf("native discovery oracle %q: %v", args, err)
		}
		return out
	}
	head := oracle("rev-parse", "HEAD")
	for _, check := range []struct {
		name string
		got  func(string) ([]byte, error)
		args []string
	}{
		{"HEAD", gitHeadOutput, []string{"rev-parse", "HEAD"}},
		{"shallow", gitShallowOutput, []string{"rev-parse", "--is-shallow-repository"}},
		{"prefix", gitPrefixOutput, []string{"rev-parse", "--show-prefix"}},
	} {
		want := oracle(check.args...)
		got, err := check.got(root)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: adapter=%q %v native=%q", check.name, got, err, want)
		}
	}
	wantTop := strings.TrimSpace(string(oracle("rev-parse", "--show-toplevel")))
	if top, err := repoTopLevel(root); err != nil || top != wantTop {
		t.Errorf("ordinary toplevel=%q %v native=%q", top, err, wantTop)
	}
	provenanceTop, err := historyGit(root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	if top, err := provenanceRepoTop(root); err != nil || top != strings.TrimSpace(string(provenanceTop)) {
		t.Errorf("provenance toplevel=%q %v native=%q", top, err, provenanceTop)
	}
	if !(coverageRepo{root: root}).InRepository() {
		t.Error("native repository disappeared from coverage")
	}
	if got := gitCurrentSHA(root); got != strings.TrimSpace(string(head)) {
		t.Errorf("current SHA=%q native=%q", got, head)
	}
	read := ordinaryCommitRead(root, strings.TrimSpace(string(head)))
	out, err, handled := commitReadOutput(read, strings.TrimSpace(string(head)))
	if err != nil || (handled && !bytes.Equal(out, head)) || (!handled && read.native == "") {
		t.Errorf("commit scalar read=%q %v handled=%v profile=%+v", out, err, handled, read)
	}
	if got := exactRefCommit(root, "refs/heads/main"); got != strings.TrimSpace(string(head)) {
		t.Errorf("exact main=%q native HEAD=%q", got, head)
	}
}

func coreReftableParity(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", "main", "--ref-format=reftable", root)
	cmd.Env = fixtureGitEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.ToLower(string(out))
		unsupportedFlag := strings.Contains(message, "unknown option") && strings.Contains(message, "ref-format")
		unsupportedBackend := strings.Contains(message, "unknown ref storage format") && strings.Contains(message, "reftable")
		if unsupportedFlag || unsupportedBackend {
			t.Skipf("local Git cannot create a reftable fixture: %s", strings.TrimSpace(string(out)))
		}
		t.Fatalf("reftable fixture failed for a reason other than local capability: %v: %s", err, out)
	}
	runGit(t, root, "config", "user.name", "T")
	runGit(t, root, "config", "user.email", "t@example.invalid")
	runGit(t, root, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "reftable")
	read := gitCoreFor(root)
	if read.err != nil || read.native == "" {
		t.Fatalf("reftable was not preselected native: %+v", read)
	}
	coreDiscoveryAdapterParity(t, root)
	nested := filepath.Join(root, "sub")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	coreDiscoveryAdapterParity(t, nested)
	// An ordinary locator must keep its historical redirect; provenance still
	// discovers the original reftable root through its stripped native policy.
	other := t.TempDir()
	gitInit(t, other, "Other", "other@example.invalid")
	runGit(t, other, "checkout", "-b", "main")
	runGit(t, other, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "ordinary redirect")
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	wantHead, wantErr := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	gotHead, gotErr := gitHeadOutput(root)
	if (wantErr == nil) != (gotErr == nil) || (wantErr == nil && !bytes.Equal(gotHead, wantHead)) {
		t.Fatalf("ordinary redirect changed: %q %v native=%q %v", gotHead, gotErr, wantHead, wantErr)
	}
	wantTop, wantErr := historyGit(root, "rev-parse", "--show-toplevel").Output()
	gotTop, gotErr := provenanceRepoTop(root)
	if (wantErr == nil) != (gotErr == nil) || (wantErr == nil && gotTop != strings.TrimSpace(string(wantTop))) {
		t.Fatalf("provenance locator isolation changed: %q %v native=%q %v", gotTop, gotErr, wantTop, wantErr)
	}
}

func coreNestedDotGitParity(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	runGit(t, root, "checkout", "-b", "main")
	runGit(t, root, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "parent")
	nested := filepath.Join(root, "sub")
	if err := os.MkdirAll(filepath.Join(nested, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	coreDiscoveryAdapterParity(t, nested)
	// Where the platform already supports canonical discovery, prove the
	// nested empty entry did not merely cause a new always-native shortcut.
	read := ordinaryGitCore(nested)
	if read.err != nil {
		t.Fatal(read.err)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	why, err := gitcore.NativeFilesystemReadReason(filepath.Join(physicalRoot, "sub"), physicalRoot)
	if err != nil {
		t.Fatal(err)
	}
	if why == "" {
		if read.native != "" {
			t.Fatalf("supported nested profile became always-native: %s", read.native)
		}
		wantHead := parityGitBytes(t, nested, "rev-parse", "HEAD")
		wantPrefix := parityGitBytes(t, nested, "rev-parse", "--show-prefix")
		t.Setenv("PATH", t.TempDir())
		if exec.Command("git", "--version").Run() == nil {
			t.Fatal("native poison positive control failed")
		}
		if got, err := gitHeadOutput(nested); err != nil || !bytes.Equal(got, wantHead) {
			t.Fatalf("nested supported HEAD reached native: %q %v", got, err)
		}
		if got, err := gitPrefixOutput(nested); err != nil || !bytes.Equal(got, wantPrefix) {
			t.Fatalf("nested supported prefix reached native: %q %v", got, err)
		}
	}
}

func coreRepositoryProfileErrors(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	runGit(t, root, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "parent")
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[extensions\n"), 0644); err != nil {
		t.Fatal(err)
	}
	read := openGitCoreRead(root)
	if read.err == nil || read.native != "" {
		t.Fatalf("genuine metadata error became an unsupported profile: %+v", read)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := gitHeadOutput(root); err == nil || errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("failed config read retried native: %v", err)
	}
}
