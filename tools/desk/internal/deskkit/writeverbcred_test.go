package deskkit

// writeverbcred_test.go — WriteVerbRoleCredential's behaviour by forge (#5457). The source
// half (no write verb forks desktoken) is writeverbguard_test.go.

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// regression: #5457
func TestWriteVerbCredGitLabReadOnly(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	rosterWithForge(t, repo.Slug(), "gitlab")
	custody := plantCustody(t, "worker", "gitlab-pat-stub")
	before, err := os.ReadFile(custody)
	if err != nil {
		t.Fatal(err)
	}
	mints := minterDetector(t, plantAppToken(t, "app-token-stub"))

	cred, err := WriteVerbRoleCredential("deskreply", "worker", " "+repo.Slug()+" ")
	if err != nil {
		t.Fatalf("WriteVerbRoleCredential on a GitLab-bound repo: %v", err)
	}
	if cred.Resolution.Kind != ForgeGitLab || cred.Path != custody || cred.Token != "gitlab-pat-stub" {
		t.Fatalf("got kind=%q path=%q; want the provisioned GitLab custody %s", cred.Resolution.Kind, cred.Path, custody)
	}
	if *mints != 0 {
		t.Fatalf("a write verb's GitLab read reached the GitHub App minter %d time(s)", *mints)
	}
	after, err := os.ReadFile(custody)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("the custody file changed under a write verb's read (err=%v) — a read never rotates", err)
	}
}

// regression: #5457
func TestWriteVerbCredGitLabRefuses(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	rosterWithForge(t, repo.Slug(), "gitlab")
	t.Setenv(EnvConfigHome, t.TempDir()) // nothing provisioned
	mints := minterDetector(t, plantAppToken(t, "app-token-stub"))

	cred, err := WriteVerbRoleCredential("deskclose", "worker", repo.Slug())
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("missing custody: exit %d (%v); want the refusal's own exit %d kept", ExitCodeOf(err), err, ExitRefused)
	}
	for _, want := range []string{"deskclose", "worker", repo.Slug(), "gitlab-worker.token"} {
		if !strings.Contains(errString(err), want) {
			t.Fatalf("refusal %q does not name %q", errString(err), want)
		}
	}
	if cred.Token != "" || *mints != 0 {
		t.Fatalf("missing GitLab custody yielded a credential (%t) or a mint (%d)", cred.Token != "", *mints)
	}
}

// regression: #5457
func TestWriteVerbCredGitHubMints(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
	rosterWithForge(t, repo.Slug(), "github")
	plantCustody(t, "worker", "gitlab-pat-stub") // present, and must NOT be read
	app := plantAppToken(t, "app-token-stub")
	mints := minterDetector(t, app)

	cred, err := WriteVerbRoleCredential("deskpr", "worker", repo.Slug())
	if err != nil {
		t.Fatalf("WriteVerbRoleCredential on a GitHub repo: %v", err)
	}
	if *mints != 1 || cred.Resolution.Kind != ForgeGitHub || cred.Token != "app-token-stub" {
		t.Fatalf("got kind=%q mints=%d; want the App credential from one in-process mint", cred.Resolution.Kind, *mints)
	}
}

// regression: #5457
func TestWriteVerbCredEmptyRefused(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
	rosterWithForge(t, repo.Slug(), "github")
	minterDetector(t, plantAppToken(t, "   "))

	_, err := WriteVerbRoleCredential("desklabel", "worker", repo.Slug())
	if err == nil || ExitCodeOf(err) == ExitOK {
		t.Fatal("an empty role credential was handed to a write verb; want a fail-closed refusal")
	}
}

// regression: #5457
func TestWriteVerbCredEmptyGitLab(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	rosterWithForge(t, repo.Slug(), "gitlab")
	plantCustody(t, "worker", "   \n")
	minterDetector(t, plantAppToken(t, "app-token-stub"))

	cred, err := WriteVerbRoleCredential("desklabel", "worker", repo.Slug())
	if err == nil || ExitCodeOf(err) == ExitOK || cred.Token != "" {
		t.Fatal("a blank GitLab custody file was handed to a write verb; want a fail-closed refusal")
	}
}
