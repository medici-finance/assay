package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #5915
func TestLegacyFieldWhitespace(t *testing.T) {
	catalog, _ := claimWorkFixture(t)
	wire, err := deskkit.EncodeClaimSubject("dispatch-claim repo--stream--08 owner=worker state=claimed branch=-", catalog.Records[0].Subject)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Fields(wire)[5]
	f := newStore()
	run, _, se := harness(t, f)
	const key = "repo--stream--08"
	if rc := run("acquire", key, "--repo", "medici-finance/assay", "--owner", "worker", "--branch", "feat/work"); rc != exitOK {
		t.Fatalf("valid legacy control %d %s", rc, se.String())
	}
	original := f.claims[key]
	if original.msg != "dispatch-claim repo--stream--08 owner=worker state=claimed branch=feat/work" {
		t.Fatalf("valid legacy wire changed: %q", original.msg)
	}
	for _, bad := range []string{"worker two", "worker\tother", "worker\nother", "worker\rother", "worker\u00a0other", "worker\u2003other"} {
		for _, args := range [][]string{
			{"acquire", "repo--stream--09", "--owner", bad},
			{"progress", key, "--owner", bad, "--branch", "feat/work"},
			{"steal", key, "--owner", bad, "--reason", "explicit reason"},
			{"acquire", "repo--stream--09", "--owner", "worker", "--branch", bad},
			{"progress", key, "--owner", "worker", "--branch", bad},
			{"release", "repo--stream--08" + bad},
		} {
			args = append(args, "--repo", "medici-finance/assay")
			if rc := run(args...); rc != exitRefused {
				t.Fatalf("structural whitespace accepted: rc=%d %s", rc, se.String())
			}
			if len(f.claims) != 1 || f.claims[key] != original {
				t.Fatal("invalid wire field changed a claim")
			}
		}
	}
	for _, args := range [][]string{
		{"acquire", "repo--stream--09", "--owner", "worker state=claimed branch=- " + token},
		{"acquire", "repo--stream--09", "--branch", "feat/work " + token},
		{"steal", key, "--owner", "worker", "--reason", "old\t" + token},
	} {
		args = append(args, "--repo", "medici-finance/assay")
		if rc := run(args...); rc != exitRefused || f.claims[key] != original || len(f.claims) != 1 {
			t.Fatalf("forged subject changed legacy claim: rc=%d %s", rc, se.String())
		}
	}
	if _, err := claimMessage(key, "worker other", "claimed", "", ""); err == nil {
		t.Fatal("builder bypass admitted whitespace")
	}
	if rc := run("progress", key, "--repo", "medici-finance/assay", "--owner", "worker", "--branch", "feat/next"); rc != exitOK {
		t.Fatalf("valid legacy progress %d %s", rc, se.String())
	}
	if fieldOf(f.claims[key].msg, "owner") != "worker" || fieldOf(f.claims[key].msg, "branch") != "feat/next" {
		t.Fatal("valid legacy fields changed")
	}
}

// regression: #5915
func TestAdmittedSourceProgress(t *testing.T) {
	_, peers := claimWorkFixture(t)
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"docs/streams/graph-repos.yaml":        "schema: graph-repos-v1\ncell: cell\nrepos:\n  repo: {cell: cell, repo: medici-finance/assay}\n",
		"docs/streams/stream/brief-08-work.md": "---\nbrief: cell:repo:stream:08\nid: 11cf440e-4e8e-4e51-98d6-211210dbefab\nversion: 2\nschema: brief-v2\n---\n# Work\n",
	}
	put := func(name, body string) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := tree.Add(name); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range files {
		put(name, body)
	}
	commit := func(message string) string {
		t.Helper()
		hash, err := tree.Commit(message, &git.CommitOptions{Author: &object.Signature{Name: "Fixture", Email: "fixture@example.invalid", When: time.Unix(1000, 0)}})
		if err != nil {
			t.Fatal(err)
		}
		return hash.String()
	}
	source := commit("admitted source")
	admitted, err := deskkit.LocalWorkCatalog(root, "medici-finance/assay")
	if err != nil {
		t.Fatal(err)
	}
	work, err := admitted.Select("stream:b-08@2")
	if err != nil {
		t.Fatal(err)
	}
	current := admitted
	old := loadClaimWork
	t.Cleanup(func() { loadClaimWork = old })
	loadClaimWork = func(string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		return current, peers, true, nil
	}
	f := newStore()
	run, _, se := harness(t, f)
	if rc := run("acquire", "stream:b-08@2", "--repo", "medici-finance/assay", "--owner", "worker"); rc != exitOK {
		t.Fatalf("acquire %d %s", rc, se.String())
	}
	original := f.claims[work.ClaimKey]
	put("worker.txt", "worker commit")
	if workerHead := commit("worker progress"); workerHead == source {
		t.Fatal("fixture HEAD did not move")
	}
	current, err = deskkit.LocalWorkCatalog(root, "medici-finance/assay")
	if err != nil {
		t.Fatal(err)
	}
	if rc := run("progress", "stream:b-08@2", "--repo", "medici-finance/assay", "--owner", "worker", "--branch", "feat/work"); rc != exitUnverifiable || f.claims[work.ClaimKey] != original {
		t.Fatalf("reader HEAD replaced immutable admission: %d %s", rc, se.String())
	}
	// The admitted source is supplied independently, not recovered as authority
	// from the payload and not replaced by the worker's newer execution commit.
	current = admitted
	if rc := run("progress", "stream:b-08@2", "--repo", "medici-finance/assay", "--owner", "other", "--branch", "feat/work"); rc != exitRefused || f.claims[work.ClaimKey] != original {
		t.Fatal("admitted snapshot granted another actor custody")
	}
	if rc := run("progress", "stream:b-08@2", "--repo", "medici-finance/assay", "--owner", "worker", "--branch", "feat/work"); rc != exitOK {
		t.Fatalf("admitted-source continuation %d %s", rc, se.String())
	}
	subject, present, err := deskkit.DecodeClaimSubject(f.claims[work.ClaimKey].msg)
	if err != nil || !present || subject != work.Subject || subject.Commit != source {
		t.Fatalf("continuation changed exact admitted subject: %v", err)
	}
}
