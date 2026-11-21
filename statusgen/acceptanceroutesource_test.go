package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAcceptanceSource(t *testing.T) {
	fx := newCreditRepo(t)
	readme := filepath.Join(filepath.Dir(fx.brief), "README.md")
	putFile(t, readme, "# Stream\n")
	evidence := "**VERIFY: HELD**\n\n**VERIFY: COULD-NOT-CHECK**\n"
	fx.commit(t, vfMailVerifier, creditPreface+evidence)

	got, err := readAcceptanceSource(fx.root, fx.brief)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repository != creditRepo || got.BriefPath != creditBriefRel || !hex40Re.MatchString(got.Revision) ||
		!hex40Re.MatchString(got.BriefBlob) || !hex40Re.MatchString(got.ReadmeBlob) {
		t.Fatalf("source was not bound to both Git blobs: %+v", got)
	}
	if len(got.Refusals) != 2 || got.Refusals[0].Line >= got.Refusals[1].Line ||
		got.Refusals[0].Digest == got.Refusals[1].Digest {
		t.Fatalf("wanted two distinct refusals in source order, got %+v", got.Refusals)
	}
	putFile(t, readme, "# Edited stream\n")
	if _, err := readAcceptanceSource(fx.root, fx.brief); err == nil || !strings.Contains(err.Error(), "README.md differs") {
		t.Fatalf("uncommitted README must refuse, got %v", err)
	}
}
