package main

import (
	"encoding/json"
	"os"
	"testing"
)

// briefauthoring_coupling_test.go — the statusgen half of the briefs-authoring cross-tree
// binding (#1502). statusgen is its own Go module and cannot import deskkit, so
// briefauthoring.go duplicates deskkit.BriefAuthoringOnly. The SAME vector file is read here
// and by tools/desk/internal/deskkit/briefauthoring_coupling_test.go; if either implementation
// drifts, one half goes red. Do not relax an assertion to green it.

const briefAuthoringVectorPath = "testdata/brief_authoring_vectors.json"

type briefAuthoringVectors struct {
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

// regression: #1502
func TestAuthoringCoupling(t *testing.T) {
	raw, err := os.ReadFile(briefAuthoringVectorPath)
	if err != nil {
		t.Fatal(err)
	}
	var v briefAuthoringVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s does not parse: %v", briefAuthoringVectorPath, err)
	}
	if len(v.Cases) == 0 {
		t.Fatal("the shared vector holds no cases")
	}
	for _, c := range v.Cases {
		var files []prFile
		for _, f := range c.Files {
			files = append(files, prFile{Filename: f.Filename, Status: f.Status, PreviousFilename: f.PreviousFilename})
		}
		if got := briefAuthoringOnly(c.BriefID, files); got != c.AuthoringOnly {
			t.Errorf("%s: briefAuthoringOnly(%q) = %v, want %v", c.Why, c.BriefID, got, c.AuthoringOnly)
		}
	}
}
