package deskkit

import (
	"errors"
	"testing"

	"github.com/medici-finance/assay/statusgen/workref"
)

func TestParseTrailersValid(t *testing.T) {
	body := []byte("Fix the thing.\n\nBrief: example-stream/02\n\nSigned-off notes.")
	trs, err := ParseTrailers(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trs) != 1 || trs[0].Kind != TrailerBrief || trs[0].Value != "example-stream/02" || trs[0].Line != 3 {
		t.Fatalf("got %+v", trs)
	}
}

func TestParseTrailersMissing(t *testing.T) {
	body := []byte("Fix the thing, no trailer at all.")
	trs, err := ParseTrailers(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trs) != 0 {
		t.Fatalf("want zero trailers, got %+v", trs)
	}
}

func TestParseTrailersDuplicate(t *testing.T) {
	body := []byte("Brief: a/01\nBrief: b/02\n")
	_, err := ParseTrailers(body)
	var dup *ErrTrailerDuplicate
	if !errors.As(err, &dup) {
		t.Fatalf("want ErrTrailerDuplicate, got %v", err)
	}
	if dup.Kind != TrailerBrief || dup.FirstLine != 1 || dup.Line != 2 {
		t.Fatalf("got %+v", dup)
	}
}

func TestParseTrailersIssueForm(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{"Issue: #42", "42"},
		{"Issue: # 42", "42"},
		{"Issue: 42", "42"},
	} {
		trs, err := ParseTrailers([]byte("x\n" + tc.line + "\n"))
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", tc.line, err)
		}
		if len(trs) != 1 || trs[0].Kind != TrailerIssue || trs[0].Value != tc.want {
			t.Fatalf("%q: got %+v", tc.line, trs)
		}
	}
}

func TestParseTrailersFencedBlockIgnored(t *testing.T) {
	body := []byte("Docs:\n\n```\nBrief: not/a/real/one\nIssue: #999\n```\n\nBrief: real/01\n")
	trs, err := ParseTrailers(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trs) != 1 || trs[0].Value != "real/01" {
		t.Fatalf("got %+v", trs)
	}
}

func TestParseTrailersBothKinds(t *testing.T) {
	_, err := ParseTrailers([]byte("Brief: a/01\nIssue: #7\n"))
	var both *ErrTrailerBoth
	if !errors.As(err, &both) {
		t.Fatalf("want ErrTrailerBoth, got %v", err)
	}
}

// regression: #4219
func TestSharedWorkrefTrailerParity(t *testing.T) {
	// This historical projection ignores prefixes and accepts numeric lengths the
	// strict QID API rejects. Preserve exact spelling here; CanonicalBriefID lowers it.
	for _, tc := range []struct{ ref, stream, number string }{
		{"build/11", "build", "11"}, {"build:11", "build", "11"},
		{"cell:repo:build:11", "build", "11"}, {"a:b:c:repo:build:1", "build", "1"},
		{" Build/00000 ", "Build", "00000"}, {":build:0", "build", "0"},
		{"arbitrary prefix:Build Stream:11", "Build Stream", "11"},
		{"repo:build/11", "repo:build", "11"},
	} {
		stream, number, ok := SplitBriefTrailer(tc.ref)
		// A colon always chooses colon reduction in the old API; a slash in its last
		// component is therefore refused (even though the typed adapter accepts it).
		if tc.ref == "repo:build/11" {
			if ok {
				t.Error("legacy colon/slash profile broadened")
			}
			continue
		}
		if !ok || stream != tc.stream || number != tc.number {
			t.Errorf("%q = %q/%q, %v", tc.ref, stream, number, ok)
		}
	}
	for _, bad := range []string{"build:b-11", "build:m-03@1", "build:11@1", "build/11a", "build/", "build//11", "build:11 extra"} {
		if _, _, ok := SplitBriefTrailer(bad); ok {
			t.Errorf("legacy trailer broadened: %s", bad)
		}
	}
	if got := CanonicalBriefID("CELL:REPO:Build:11"); got != "build/11" {
		t.Errorf("canonical ID = %s", got)
	}
	aliases, err := SplitAuthorsTrailer("Build/11, Build:12")
	if err != nil || len(aliases) != 2 || aliases[0] != "build/11" || aliases[1] != "build/12" {
		t.Errorf("authors = %v, %v", aliases, err)
	}
	if _, err := SplitAuthorsTrailer("Build/11 build:11"); err == nil {
		t.Error("author duplicate lost")
	}
	alias, stream, number, ok := SplitCrossRepoBriefTrailer("alias_name:Build/1")
	if !ok || alias != "alias_name" || stream != "Build" || number != "1" {
		t.Errorf("cross-repo = %s %s %s %v", alias, stream, number, ok)
	}
	reg, err := workref.NewRegistry([]workref.Root{{Cell: "example", Repo: "tools", Aliases: []string{"alias"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"build:b-11@2", "alias:build:b-11@2", "example:tools:build/b-11@2"} {
		got, err := ResolveWorkTrailer(ref, &workref.Context{Cell: "example", Repo: "tools"}, reg)
		if err != nil || got.String() != "example:tools:build:b-11" || got.Revision != 2 || got.Compatibility != "example:tools:build:11" {
			t.Errorf("typed adapter %s = %+v, %v", ref, got, err)
		}
	}
	if _, err := ResolveWorkTrailer("build:m-03@1", &workref.Context{Cell: "example", Repo: "tools"}, reg); err == nil {
		t.Error("milestone became delivered brief")
	}
}
