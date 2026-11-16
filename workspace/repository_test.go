package workspace

// repository_test.go — the store mapping's own schema string. Provisioning is a separately
// versioned protocol (cell-repository/v1), so a mapping is never read as a workspace v1 document.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// regression: F-cell-workspaces-02-c2
func TestStoreMappingSchema(t *testing.T) {
	const want = "cell-repository/v1" // spelled out so a rename of the constant cannot move this
	cell := CellRef{UUID: "00000000-7b4d-4e8a-9c21-5d6e7f8a9b0c", Root: "/srv/cells/alpha"}
	repo := RepoRef{Forge: "github.com", Repository: "example-org/widgets"}
	path := filepath.Join(cell.Root, ReposDir, RepoKey(repo), SourceDir)
	entry := StoreEntry{Repo: repo, RepoKey: RepoKey(repo), Path: path, CommonDir: filepath.Join(path, ".git"),
		Subject: "0123456789abcdef0123456789abcdef01234567"}

	m, err := NextMapping(nil, 0, cell, []StoreEntry{entry})
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != want {
		t.Fatalf("a published mapping carries schema %q, want %q", m.Schema, want)
	}
	b, err := EncodeStoreMapping(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeStoreMapping(b); err != nil {
		t.Fatalf("a mapping does not read back: %v", err)
	}

	// A mapping stamped with the workspace v1 string is refused as an unknown schema.
	old := bytes.Replace(b, []byte(want), []byte(Schema), 1)
	if bytes.Equal(old, b) {
		t.Fatal("fixture has no schema member to replace")
	}
	_, err = DecodeStoreMapping(old)
	wantReason(t, err, ReasonUnknownSchema)
	oldMapping := m
	oldMapping.Schema = Schema
	_, err = EncodeStoreMapping(oldMapping)
	wantReason(t, err, ReasonUnknownSchema)

	// And the workspace v1 reader refuses a mapping before any member rule.
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "request-worker.json"))
	if err != nil {
		t.Fatal(err)
	}
	stamped := bytes.Replace(raw, []byte(Schema), []byte(want), 1)
	if bytes.Equal(stamped, raw) {
		t.Fatal("fixture has no schema member to replace")
	}
	_, err = DecodeRequest(stamped)
	wantReason(t, err, ReasonUnknownSchema)
}
