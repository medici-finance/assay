package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeVGRegistry drops a graph-repos-v1 alias registry into a verifygate fixture root: cell
// "example", this tree's own alias "app" (the single published alias, so self is inferred), a
// withheld sibling "other", and nothing else.
func writeVGRegistry(t *testing.T, root string) {
	t.Helper()
	reg := `schema: graph-repos-v1
cell: example
repos:
  app:   {cell: example, repo: example-org/app}
  other: {cell: example, repo: null, unpublished: true}
`
	p := filepath.Join(root, "docs", "streams", "graph-repos.yaml")
	if err := os.WriteFile(p, []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func vgRowStatus(t *testing.T, root, num string) string {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range streams {
		if s.Name == "vg" {
			if row := findRow(s, num); row != nil {
				return row.Status
			}
		}
	}
	t.Fatalf("no vg/%s row", num)
	return ""
}

// TestCloseVerifyAliasRouting pins the close side of the topology contract: a ref that names a
// repo alias flips THIS tree's brief only when the alias resolves through the registry to this
// tree's own alias. Another repo's alias, an unregistered alias, a foreign cell and a tree with
// no registry all refuse, and vg/01 (verified, eligible) is left untouched by every one of them.
func TestCloseVerifyAliasRouting(t *testing.T) {
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	t.Run("no registry: aliased ref refuses", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		if err := closeVerify(root, "other:vg/01", now); err == nil {
			t.Fatal("an aliased ref in a tree with no registry must refuse, it flipped")
		}
		if got := vgRowStatus(t, root, "01"); got != "verified" {
			t.Errorf("vg/01 = %q, want verified (nothing may be written)", got)
		}
	})

	refused := []string{
		"example:other:vg:01", // brief-v2 id naming the sibling repo
		"other:vg/01",         // cross-repo ref naming the sibling repo
		"example:other:vg/01", // cross-cell form, sibling repo
		"ghost:vg/01",         // alias the registry does not define
		"elsewhere:app:vg:01", // this repo's alias under a foreign cell
	}
	for _, ref := range refused {
		t.Run("refuses "+ref, func(t *testing.T) {
			root, _ := loadVGStreams(t)
			writeVGRegistry(t, root)
			if err := closeVerify(root, ref, now); err == nil {
				t.Fatalf("close-verify %q must refuse — the alias is not this tree's", ref)
			}
			if got := vgRowStatus(t, root, "01"); got != "verified" {
				t.Errorf("vg/01 = %q after refused %q, want verified", got, ref)
			}
		})
	}

	for _, ref := range []string{"example:app:vg:01", "app:vg/01", "vg/01"} {
		t.Run("flips "+ref, func(t *testing.T) {
			root, _ := loadVGStreams(t)
			writeVGRegistry(t, root)
			if err := closeVerify(root, ref, now); err != nil {
				t.Fatalf("close-verify %q (this tree's own alias) must flip: %v", ref, err)
			}
			if got := vgRowStatus(t, root, "01"); got != "done" {
				t.Errorf("vg/01 = %q, want done", got)
			}
		})
	}
}
