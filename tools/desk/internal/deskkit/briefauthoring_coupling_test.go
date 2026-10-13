package deskkit

import (
	"encoding/json"
	"os"
	"testing"
)

// briefauthoring_coupling_test.go — the deskkit half of the briefs-authoring cross-tree
// binding (#1502). statusgen (a separate Go module) duplicates BriefAuthoringOnly for its
// reconcile reader; the SAME vector file is read here cross-module and run through the
// original, and in-module by statusgen's TestAuthoringCoupling. If either copy drifts, one half
// goes red. DO NOT relax an assertion to green it.
//
// This reads statusgen/testdata/brief_authoring_vectors.json, OUTSIDE this Go module, so it is
// registered in citrigger_test.go's ciCrossModuleRegistry.

const briefAuthoringVectorPath = "../../../../statusgen/testdata/brief_authoring_vectors.json"

// regression: #1502
func TestAuthoringCoupling(t *testing.T) {
	raw, err := os.ReadFile(briefAuthoringVectorPath)
	if err != nil {
		t.Fatalf("cannot read the shared vectors at %s: %v — if statusgen moved, re-point this "+
			"coupling test, do NOT delete it", briefAuthoringVectorPath, err)
	}
	var v struct {
		Cases []struct {
			Why     string `json:"why"`
			BriefID string `json:"briefID"`
			Files   []struct {
				Filename         string `json:"filename"`
				Status           string `json:"status"`
				PreviousFilename string `json:"previousFilename"`
			} `json:"files"`
			AuthoringOnly bool `json:"authoringOnly"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s does not parse: %v", briefAuthoringVectorPath, err)
	}
	if len(v.Cases) == 0 {
		t.Fatal("the shared vector holds no cases")
	}
	for _, c := range v.Cases {
		var files []ChangedFile
		for _, f := range c.Files {
			files = append(files, ChangedFile{Filename: f.Filename, Status: f.Status, PreviousFilename: f.PreviousFilename})
		}
		if got := BriefAuthoringOnly(c.BriefID, files); got != c.AuthoringOnly {
			t.Errorf("%s: BriefAuthoringOnly(%q) = %v, want %v", c.Why, c.BriefID, got, c.AuthoringOnly)
		}
	}
}
