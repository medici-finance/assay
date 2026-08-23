package deskkit

import (
	"errors"
	"testing"
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
