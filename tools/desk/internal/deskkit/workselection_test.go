package deskkit

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
)

func testWorkCatalog(t *testing.T) WorkCatalog {
	t.Helper()
	id := workref.Identity{Kind: workref.Brief, Cell: "cell", Repo: "repo", Stream: "stream", Number: "08"}
	uuid := "11cf440e-4e8e-4e51-98d6-211210dbefab"
	registry, err := workref.NewRegistry([]workref.Root{{Cell: "cell", Repo: "repo", Aliases: []string{"old"}}, {Cell: "cell", Repo: "other"}}, []workref.Binding{{Identity: id, Key: uuid}})
	if err != nil {
		t.Fatal(err)
	}
	return WorkCatalog{Declaring: workref.Context{Cell: "cell", Repo: "repo"}, Registry: registry, Records: []WorkRecord{{Identity: id, Label: "repo", Subject: BriefSubject{UUID: uuid, Version: 2, Repository: "owner/repo", Commit: strings.Repeat("a", 40), Path: "docs/streams/stream/brief-08-work.md"}}}}
}

// regression: #5704
func TestPrefixedTrailerIdentityJoin(t *testing.T) {
	catalog := testWorkCatalog(t)
	aliases := []string{"stream/08", "stream:08", "repo:stream:08", "cell:repo:stream:08", "repo:stream/08", "cell:repo:stream/08", "stream:b-08", "repo:stream:b-08@2", "cell:repo:stream:b-08@2", "stream/b-08@2", "cell:repo:stream/b-08@2", "old:stream:b-08@2"}
	var key string
	for _, alias := range aliases {
		kind, selected, err := ResolveTrailerWork([]byte("Brief: "+alias), catalog)
		if err != nil || kind != TrailerBrief || len(selected) != 1 {
			t.Fatalf("alias %s: %v %v", alias, selected, err)
		}
		if err := selected[0].Validate(); err != nil {
			t.Fatal(err)
		}
		if key == "" {
			key = selected[0].Reference.LogicalKey
		}
		if selected[0].Reference.LogicalKey != key || selected[0].ClaimKey != "repo--stream--08" {
			t.Fatalf("alias allocated another work/claim: %s", alias)
		}
		files := []ChangedFile{{Filename: catalog.Records[0].Subject.Path, Status: "added"}}
		if !BriefAuthoringOnly(alias, files, catalog) {
			t.Fatalf("authoring classification lost %s", alias)
		}
		files = append(files, ChangedFile{Filename: "code.go", Status: "modified"})
		if BriefAuthoringOnly(alias, files, catalog) {
			t.Fatalf("delivery misclassified %s", alias)
		}
		represented := RepresentingPRsByBrief([]PRRef{{Number: 1, State: "OPEN", Body: "Authors: " + alias}, {Number: 2, State: "OPEN", Body: "Brief: " + alias}}, catalog)
		if len(represented[key]) != 1 || represented[key][0].Number != 2 {
			t.Fatalf("Authors shadowed delivery: %v", represented)
		}
	}
	for _, bad := range []string{"other:stream:b-08", "unknown:stream:b-08", "stream:m-08@2", "stream:b-08@3", "stream:b-08@0", "stream:b-08@18446744073709551616", "STREAM:08", "repo:stream/b-08", "stream:08@2", " stream:b-08"} {
		if _, err := catalog.Select(bad); err == nil {
			t.Errorf("strict-refused input admitted: %q", bad)
		}
	}
	for _, body := range []string{"Authors: stream:b-08, repo:stream:b-08", "Brief: stream:b-08\nBrief: stream/08", "Brief: other:stream:b-08", "Cross-repo-brief: repo:stream:b-08"} {
		if _, _, err := ResolveTrailerWork([]byte(body), catalog); err == nil {
			t.Errorf("unsafe trailer admitted: %s", body)
		}
	}
	if kind, selected, err := ResolveTrailerWork([]byte("```\nBrief: stream:b-08\n```\nAuthors: stream:b-08"), catalog); err != nil || kind != TrailerAuthors || len(selected) != 1 {
		t.Fatalf("fence/kind lost: %v", err)
	}
	// Bypass the upper resolver: the selected value still fails at effect admission.
	work, _ := catalog.Select("stream:b-08@2")
	work.Reference.Kind = workref.Milestone
	if work.Validate() == nil {
		t.Fatal("lower boundary admitted milestone")
	}
	peers := []readercaps.Observation{{State: readercaps.Current, Advertisement: ReaderCapabilities("dev", "fixture")}}
	if CheckWorkOperation("claim", peers, true) == nil {
		t.Fatal("syntax-only peer granted claim")
	}
	peers[0].State = readercaps.Stale
	if CheckWorkOperation("claim", peers, true) == nil {
		t.Fatal("stale observation granted claim")
	}
}

// regression: #5704
func TestClaimSubjectTokens(t *testing.T) {
	subject := testWorkCatalog(t).Records[0].Subject
	base := "dispatch-claim repo--stream--08 owner=worker state=claimed branch=-"
	message, err := EncodeClaimSubject(base, subject)
	if err != nil {
		t.Fatal(err)
	}
	if got, present, err := DecodeClaimSubject(message); err != nil || !present || got != subject {
		t.Fatalf("subject roundtrip %v %v %v", got, present, err)
	}
	if !strings.HasPrefix(message, base+" ") {
		t.Fatal("historical five fields changed")
	}
	token := strings.Fields(message)[5]
	for _, bad := range []string{message + " " + token, base + " note=reason " + token, base + " note=" + token, base + " subject-v1=", base + " subject-v2=unknown"} {
		if _, _, err := DecodeClaimSubject(bad); err == nil {
			t.Errorf("duplicate/note/unknown token admitted: %s", bad)
		}
	}
	if _, present, err := DecodeClaimSubject(base + " note=discuss the subject"); err != nil || present {
		t.Fatal("historical payload no longer readable")
	}
	if _, err := EncodeClaimSubject(base+" note=reason "+token, subject); err == nil {
		t.Fatal("writer admitted note-injected token")
	}
}
