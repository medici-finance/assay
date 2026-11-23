package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptanceDefinitionBindsProtectedOriginalTable(t *testing.T) {
	o := vfDefaults()
	o.gate, o.evidence = "human", ""
	root, readme := vfFixture(t, o)
	vfGit(t, root, vfMailHuman, "remote", "add", "origin", "https://github.com/"+creditRepo+".git")
	path := vfBrief(root)
	base := vfRev(t, root, "HEAD")
	putFile(t, filepath.Join(root, "delivery.txt"), "delivery\n")
	vfCommit(t, root, vfMailWorker)
	delivery := vfRev(t, root, "HEAD")
	marker := fmt.Sprintf("\nVerification-Table: delivery=%s base=%s original=%s\n\n", delivery, base, base)
	vfAppend(t, root, marker+witnessTableFor(vfRow1, vfRow2)+"\n\n"+vfPass)
	vfCommit(t, root, vfMailVerifier)

	source, err := readAcceptanceSource(root, path)
	if err != nil {
		t.Fatal(err)
	}
	def, err := readAcceptanceDefinition(root, source)
	if err != nil {
		t.Fatal(err)
	}
	if def.version != 1 || def.original != base || len(def.tableHash) != 64 {
		t.Fatalf("unexpected protected definition: %+v", def)
	}
	m := acceptanceManifest{AcceptanceVersion: def.version, Original: def.original, ProtectedVerifyHash: def.tableHash}
	if err := m.checkDefinition(def); err != nil {
		t.Fatal(err)
	}
	m.AcceptanceVersion++
	if err := m.checkDefinition(def); err == nil {
		t.Fatal("changed acceptance version must refuse")
	}

	putFile(t, filepath.Join(root, "elsewhere.txt"), "unrelated main change\n")
	vfCommit(t, root, vfMailHuman)
	current, err := readAcceptanceSourceAt(root, path, source.Revision)
	if err != nil || current.Revision != source.Revision {
		t.Fatalf("unchanged source must survive an unrelated descendant: %+v (%v)", current, err)
	}
	putFile(t, readme, getFile(t, readme)+"\nCommitted board change\n")
	vfCommit(t, root, vfMailHuman)
	if _, err := readAcceptanceSourceAt(root, path, source.Revision); err == nil || !strings.Contains(err.Error(), "README.md differs") {
		t.Fatalf("changed committed board must refuse, got %v", err)
	}
}
