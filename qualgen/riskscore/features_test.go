package riskscore

import (
	"math"
	"testing"
	"time"
)

// TestExtractJIT_DiffusionAndSize checks the Kamei diffusion + size features are
// extracted from a change's metadata: subsystems, directories, files, and the
// raw add/delete/total counts.
func TestExtractJIT_DiffusionAndSize(t *testing.T) {
	c := Change{
		ID:                  "c1",
		CommitTime:          time.Unix(0, 0),
		Files:               []string{"a/x.go", "a/y.go", "b/z.go"},
		LinesAdded:          30,
		LinesDeleted:        5,
		LinesTotal:          1000,
		NDev:                4,
		PriorChangesToFiles: 9,
		PriorDefectsToFiles: 2,
		RecentChurn:         12,
		AgeDays:             40,
		AuthorClass:         AuthorAgent,
		AuthorExp:           7,
	}
	f := ExtractJIT(c)
	if f.NF != 3 {
		t.Errorf("NF: want 3 files, got %d", f.NF)
	}
	if f.ND != 2 {
		t.Errorf("ND: want 2 dirs (a, b), got %d", f.ND)
	}
	if f.NS != 2 {
		t.Errorf("NS: want 2 subsystems (a, b), got %d", f.NS)
	}
	if f.LA != 30 || f.LD != 5 || f.LT != 1000 {
		t.Errorf("size: got LA=%d LD=%d LT=%d", f.LA, f.LD, f.LT)
	}
	if f.NUC != 9 || f.NPD != 2 || f.NDEV != 4 || f.RC != 12 {
		t.Errorf("history: got NUC=%d NPD=%d NDEV=%d RC=%d", f.NUC, f.NPD, f.NDEV, f.RC)
	}
	if f.Class != AuthorAgent || f.EXP != 7 {
		t.Errorf("author-class: got class=%q exp=%d", f.Class, f.EXP)
	}
}

// TestChangeEntropy_ConcentratedVsSpread checks the diffusion-entropy feature: a
// change concentrated in one file has entropy 0; one spread evenly across files
// approaches 1; a skewed spread lands strictly between.
func TestChangeEntropy_ConcentratedVsSpread(t *testing.T) {
	concentrated := Change{Files: []string{"a.go", "b.go"}, ChangedLinesPerFile: []int{100, 0}}
	even := Change{Files: []string{"a.go", "b.go", "c.go", "d.go"}, ChangedLinesPerFile: []int{10, 10, 10, 10}}
	skewed := Change{Files: []string{"a.go", "b.go", "c.go"}, ChangedLinesPerFile: []int{80, 10, 10}}

	if e := changeEntropy(concentrated); e != 0 {
		t.Errorf("concentrated change entropy: want 0, got %.3f", e)
	}
	if e := changeEntropy(even); math.Abs(e-1) > 1e-9 {
		t.Errorf("evenly-spread change entropy: want 1, got %.3f", e)
	}
	sk := changeEntropy(skewed)
	if !(sk > 0 && sk < 1) {
		t.Errorf("skewed change entropy: want strictly in (0,1), got %.3f", sk)
	}
}

// TestVector_ClassOneHotAndOrder pins the numeric feature vector's fixed order
// and the author-class one-hot encoding the learned model depends on.
func TestVector_ClassOneHotAndOrder(t *testing.T) {
	names := FeatureNames()
	agent := ExtractJIT(Change{Files: []string{"a/x.go"}, AuthorClass: AuthorAgent}).Vector()
	auto := ExtractJIT(Change{Files: []string{"a/x.go"}, AuthorClass: AuthorAutomation}).Vector()
	human := ExtractJIT(Change{Files: []string{"a/x.go"}, AuthorClass: AuthorHuman}).Vector()

	if len(names) != len(agent) {
		t.Fatalf("FeatureNames (%d) and Vector (%d) must match in length", len(names), len(agent))
	}
	idxAgent, idxAuto := -1, -1
	for i, n := range names {
		switch n {
		case "class_agent":
			idxAgent = i
		case "class_automation":
			idxAuto = i
		}
	}
	if idxAgent < 0 || idxAuto < 0 {
		t.Fatalf("class one-hot columns not found in %v", names)
	}
	if agent[idxAgent] != 1 || agent[idxAuto] != 0 {
		t.Errorf("agent one-hot wrong: agent=%v auto=%v", agent[idxAgent], agent[idxAuto])
	}
	if auto[idxAgent] != 0 || auto[idxAuto] != 1 {
		t.Errorf("automation one-hot wrong: agent=%v auto=%v", auto[idxAgent], auto[idxAuto])
	}
	// Human is the reference level: both one-hot columns zero.
	if human[idxAgent] != 0 || human[idxAuto] != 0 {
		t.Errorf("human reference level should be all-zero one-hot, got agent=%v auto=%v", human[idxAgent], human[idxAuto])
	}
}
