package main

import (
	"strings"
	"testing"
)

func TestSharedTrailerCallers(t *testing.T) {
	body := "~~~md\nBrief: ff/99\n~~~\nBrief: ff/01\n"
	if ClassifyPRLink([]PRLinkRecord{{Number: 1, Body: body}})[1] != PRLinkLinked {
		t.Fatal("classifier counted fenced work")
	}
	if ref, ok := singleBriefTrailer(body); !ok || ref != "ff/01" {
		t.Fatal(ref, ok)
	}
	if n, refs := parseBriefLikeTrailers(body); n != 1 || len(refs) != 1 || refs[0] != "ff/01" {
		t.Fatal(n, refs)
	}
	if refs, _ := mergeTrailerKeys(body); len(refs) != 1 || refs[0] != "ff/01" {
		t.Fatal(refs)
	}
}
func TestSharedStrictBotID(t *testing.T) {
	scanWithRoster(t, map[string]string{"ASSAY_BLESS_LOGIN": "ada:1001", "ASSAY_TRUSTED_LOGINS": "ada:1001", "ASSAY_TRUSTED_BOT_SLUGS": "reviewer=example-reviewer-app:3003"})
	if trustedAuthorID("example-reviewer-app[bot]", 9999) {
		t.Fatal("wrong bot ID admitted")
	}
	if !trustedAuthorID("example-reviewer-app[bot]", 3003) {
		t.Fatal("matching bot ID refused")
	}
	if authorizedByIdentity("example-reviewer-app[bot]", 9999, "Bot") {
		t.Fatal("authorized-bot lane bypassed ID check")
	}
}
func TestSharedQuotedBrief(t *testing.T) {
	root := t.TempDir()
	ffStreamBrief(t, root, strings.Replace(ffBriefRaw("yes", "", ""), "regulatory: yes", `regulatory: "yes"`, 1))
	info, e := resolveBriefKey(root, "ff/01")
	if e != nil || info.Risk["regulatory"] != "yes" {
		t.Fatal(info, e)
	}
}
