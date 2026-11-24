package main

import (
	"strings"
	"testing"
)

// regression: F-record-pr-app
//
// TestRecordPRIsNotWiredToWriteIssues pins the record-PR credential away from
// the write-issues App. The record PR creates a ref, commits cells.yaml and
// opens a pull request, which needs Contents: write and Pull requests: write;
// the write-issues App is specified to hold neither. So the
// variable the record PR reads must not be the write-issues one, and the
// write-issues one must be reported as no longer read rather than silently
// ignored.
func TestRecordPRIsNotWiredToWriteIssues(t *testing.T) {
	if recordTokenEnv == legacyRecordTokenEnv {
		t.Fatalf("recordTokenEnv = %q is the write-issues App's variable: the record PR would authenticate as an App specified to hold neither Contents nor Pull requests write", recordTokenEnv)
	}
	if strings.Contains(strings.ToUpper(recordTokenEnv), "WRITE_ISSUES") || strings.Contains(strings.ToUpper(recordAppEnv), "WRITE_ISSUES") {
		t.Fatalf("record-PR variables %q / %q must not name the write-issues App", recordTokenEnv, recordAppEnv)
	}

	set := func(k string) string {
		if k == legacyRecordTokenEnv {
			return "x"
		}
		return ""
	}
	notice := legacyRecordTokenNotice(set)
	if !strings.Contains(notice, legacyRecordTokenEnv) || !strings.Contains(notice, recordTokenEnv) {
		t.Fatalf("a set legacy variable must be reported and name its replacement, got %q", notice)
	}
	if got := legacyRecordTokenNotice(func(string) string { return "" }); got != "" {
		t.Fatalf("no legacy variable set, want no notice, got %q", got)
	}
}
