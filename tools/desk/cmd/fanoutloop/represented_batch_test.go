package main

import (
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"reflect"
	"strings"
	"testing"
)

func withBatchFiles(t *testing.T, read func(string, []int) map[int]deskkit.PendingChangeObservation) {
	t.Helper()
	old := representedPRBatchFiles
	representedPRBatchFiles = read
	t.Cleanup(func() { representedPRBatchFiles = old })
}

// regression: #5022
func TestRepresentedBatchLazyFrontier(t *testing.T) {
	prs := []deskkit.PRRef{
		{Number: 12, State: "MERGED", Body: "Brief: example-port/11"},
		{Number: 11, State: "OPEN", Body: "Brief: example-port/11"},
		{Number: 22, State: "OPEN", Body: "Brief: example-port/12"},
		{Number: 21, State: "MERGED", Body: "Brief: example-port/12"},
		{Number: 31, State: "MERGED", Body: "Brief: example-port/13"},
	}
	withPlanTransports(t, prs, func(string, int) ([]deskkit.ChangedFile, error) {
		t.Fatal("serial transport used for batch frontier")
		return nil, nil
	})
	var frontiers [][]int
	withBatchFiles(t, func(_ string, ns []int) map[int]deskkit.PendingChangeObservation {
		frontiers = append(frontiers, append([]int(nil), ns...))
		out := map[int]deskkit.PendingChangeObservation{}
		for _, n := range ns {
			out[n] = deskkit.PendingChangeObservation{Files: []deskkit.ChangedFile{{Filename: "tools/x.go", Status: "added"}}}
		}
		if _, ok := out[12]; ok {
			out[12] = deskkit.PendingChangeObservation{Files: planAuthoringFiles()}
		}
		return out
	})
	got, err := representedSourceFor("example-org/example-repo", func() (map[string]bool, error) {
		return map[string]bool{"example-port/11": true, "example-port/12": true}, nil
	})()
	if err != nil {
		t.Fatal(err)
	}
	if got["example-port/11"].Number != 11 || got["example-port/12"].Number != 22 || got["example-port/13"].Number != 31 {
		t.Fatalf("wrong representation: %+v", got)
	}
	if !reflect.DeepEqual(frontiers, [][]int{{12, 22}, {11}}) {
		t.Fatalf("frontiers expanded/skipped lazy scope: %v", frontiers)
	}
}

// regression: #5022
func TestRepresentedBatchUncertainty(t *testing.T) {
	prs := []deskkit.PRRef{{Number: 11, State: "OPEN", Body: "Brief: example-port/11"}, {Number: 12, State: "OPEN", Body: "Brief: example-port/12"}, {Number: 13, State: "OPEN", Body: "Brief: example-port/13"}}
	withPlanTransports(t, prs, func(string, int) ([]deskkit.ChangedFile, error) {
		t.Fatal("must not replace uncertain batch with another observation")
		return nil, nil
	})
	withBatchFiles(t, func(string, []int) map[int]deskkit.PendingChangeObservation {
		return map[int]deskkit.PendingChangeObservation{11: {Err: errors.New("partial metadata"), Files: planAuthoringFiles()}, 13: {Files: []deskkit.ChangedFile{{Filename: "docs/streams/example-port/brief-13-test.md", Status: "removed"}}}}
	})
	got, err := representedSourceFor("example-org/example-repo", func() (map[string]bool, error) {
		return map[string]bool{"example-port/11": true, "example-port/12": true, "example-port/13": true}, nil
	})()
	if err != nil || len(got) != 3 {
		t.Fatalf("uncertainty/deletion became exemption: %+v %v", got, err)
	}
}

type ancestryForge struct {
	deskkit.Forge
	pr                       *deskkit.PullRequest
	files                    []deskkit.ChangedFile
	err                      error
	metadataCalls, fileCalls int
}

func (f *ancestryForge) GetPullRequest(deskkit.ForgeRepo, int) (*deskkit.PullRequest, error) {
	f.metadataCalls++
	return f.pr, f.err
}
func (f *ancestryForge) ListChangedFiles(deskkit.ForgeRepo, int) ([]deskkit.ChangedFile, error) {
	f.fileCalls++
	return f.files, f.err
}

// regression: #5022
func TestRepresentedBatchAncestryFallback(t *testing.T) {
	for _, status := range []string{"renamed", "copied"} {
		t.Run(status, func(t *testing.T) {
			files := []deskkit.ChangedFile{{Filename: "docs/streams/example-port/brief-11-test.md", Status: "added"}, {Filename: "docs/streams/example-port/README.md", Status: status, PreviousFilename: "tools/old.go"}}
			pr := &deskkit.PullRequest{Number: 11, HeadSHA: strings.Repeat("a", 40), ChangedFiles: 2}
			f := &ancestryForge{pr: pr, files: files}
			batch := func(deskkit.ForgeRepo, []int) map[int]deskkit.PendingChangeObservation {
				compact := append([]deskkit.ChangedFile(nil), files...)
				compact[1].PreviousFilename = ""
				return map[int]deskkit.PendingChangeObservation{11: {PR: pr, Files: compact}}
			}
			ob := representedFilesBatch(f, deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}, []int{11}, batch)[11]
			if ob.Err != nil || deskkit.BriefAuthoringOnly("example-port/11", ob.Files) || f.metadataCalls != 1 || f.fileCalls != 1 {
				t.Fatalf("ancestry lost or wrong native scope: %+v calls=%d/%d", ob, f.metadataCalls, f.fileCalls)
			}
		})
	}
}

// regression: #5022
func TestRepresentedBatchBinding(t *testing.T) {
	for _, kind := range []string{"head", "count", "partial", "error", "missing", "nil", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			pr := &deskkit.PullRequest{Number: 11, HeadSHA: strings.Repeat("a", 40), ChangedFiles: 1}
			native := *pr
			f := &ancestryForge{pr: &native, files: []deskkit.ChangedFile{{Filename: "docs/streams/example-port/README.md", Status: "renamed"}}}
			ob := deskkit.PendingChangeObservation{PR: pr, Files: f.files}
			switch kind {
			case "head":
				native.HeadSHA = strings.Repeat("b", 40)
			case "count":
				native.ChangedFiles = 2
				f.files = append(f.files, deskkit.ChangedFile{Filename: "tools/code.go", Status: "added"})
			case "partial":
				f.files = nil
			case "error":
				f.err = errors.New("native unavailable")
			case "nil":
				ob.PR = nil
			case "unknown":
				ob.Err = errors.New("unknown change kind")
			}
			batch := func(deskkit.ForgeRepo, []int) map[int]deskkit.PendingChangeObservation {
				if kind == "missing" {
					return nil
				}
				return map[int]deskkit.PendingChangeObservation{11: ob}
			}
			got := representedFilesBatch(f, deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}, []int{11}, batch)[11]
			if got.Err == nil {
				t.Fatalf("%s became complete: %+v", kind, got)
			}
			if (kind == "missing" || kind == "nil" || kind == "unknown") && f.metadataCalls != 0 {
				t.Fatal("replaced unknown observation with new native vote")
			}
		})
	}
}

// regression: #5022
func TestRepresentedBatchNativeBackend(t *testing.T) {
	files := planAuthoringFiles()
	f := &ancestryForge{pr: &deskkit.PullRequest{Number: 11, ChangedFiles: len(files)}, files: files}
	got := representedFilesBatch(f, deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}, []int{11}, nil)[11]
	if got.Err != nil || !reflect.DeepEqual(got.Files, files) || f.metadataCalls != 1 || f.fileCalls != 1 {
		t.Fatalf("native fallback changed: %+v", got)
	}
}
