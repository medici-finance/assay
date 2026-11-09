package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/medici-finance/assay/statusgen/domain"
	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
)

const msAt = "2026-11-09T12:00:00Z"

type msSelector []domain.Provenance

func (s msSelector) Select(w domain.WorkIdentity, c domain.Candidate, d, e string) (domain.Provenance, bool) {
	for _, p := range s {
		if p.Work == w && p.Candidate == c && p.Duty == d && p.Evidence == e {
			return p, true
		}
	}
	return domain.Provenance{}, false
}

type msRepo struct {
	evaluation.Repo
	tree string
}

func (msRepo) InRepository() bool { return false }
func (r msRepo) VerifyExpectAt(rev, path, row string) (string, bool) {
	return "exit 0", rev == r.tree && row == "1"
}

type msComment struct {
	body, login, typ, created, updated string
	parent                             int
	status                             int
}
type msSources struct {
	raw           map[domain.EvidencePointer][]byte
	protected     map[domain.EvidencePointer]domain.Completeness
	record        MilestoneRecord
	authority     domain.EvidencePointer
	owner         workref.Context
	registry      workref.Registry
	briefs        []domain.MilestoneBriefObservation
	prerequisites []domain.MilestonePrereqObservation
	comments      map[int64]msComment
	complete      bool
	missing       map[domain.EvidencePointer]bool
}

func (s *msSources) ReadAt(p domain.EvidencePointer) ([]byte, error) {
	if s.missing[p] {
		return nil, fmt.Errorf("unavailable")
	}
	b, ok := s.raw[p]
	if !ok {
		return nil, fmt.Errorf("not selected")
	}
	return b, nil
}
func (s *msSources) Protected(p domain.EvidencePointer) domain.Completeness { return s.protected[p] }
func (s *msSources) Contract(p domain.EvidencePointer) (MilestoneRecord, domain.EvidencePointer, workref.Context, workref.Registry, bool, error) {
	if _, ok := s.raw[p]; !ok {
		return MilestoneRecord{}, domain.EvidencePointer{}, s.owner, s.registry, false, fmt.Errorf("not selected")
	}
	return s.record, s.authority, s.owner, s.registry, true, nil
}
func (s *msSources) Brief(c domain.MilestoneContributor) (domain.MilestoneBriefObservation, error) {
	for _, o := range s.briefs {
		if o.Contributor.Brief == c.Brief {
			return o, nil
		}
	}
	return domain.MilestoneBriefObservation{}, fmt.Errorf("unavailable")
}
func (s *msSources) Prerequisite(p domain.MilestonePrerequisite) (domain.MilestonePrereqObservation, error) {
	for _, o := range s.prerequisites {
		if o.Receipt.Milestone == p.Milestone {
			return o, nil
		}
	}
	return domain.MilestonePrereqObservation{}, fmt.Errorf("unavailable")
}
func (s *msSources) DecisionLineage(actor domain.Actor) ([]string, error) {
	return []string{actor.ID, "principal:driver"}, nil
}
func (s *msSources) Decisions(repo string, issue int) ([]rulingLink, bool, error) {
	var links []rulingLink
	for id := range s.comments {
		links = append(links, rulingLink{Repo: repo, Issue: issue, CommentID: id})
	}
	slices.SortFunc(links, func(a, b rulingLink) int { return int(a.CommentID - b.CommentID) })
	return links, s.complete, nil
}
func (s *msSources) client(t *testing.T) *ghClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/example-org/widgets/issues/17" {
			_ = json.NewEncoder(w).Encode(map[string]any{"body": "<!-- milestone-decision: example:widgets:build:m-01@1 -->"})
			return
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/repos/example-org/widgets/issues/comments/"), 10, 64)
		c, ok := s.comments[id]
		if err != nil || !ok {
			w.WriteHeader(404)
			return
		}
		if c.status != 200 {
			w.WriteHeader(c.status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]string{"login": c.login, "type": c.typ}, "issue_url": fmt.Sprintf("https://api.github.com/repos/example-org/widgets/issues/%d", c.parent), "body": c.body, "created_at": c.created, "updated_at": c.updated})
	}))
	t.Cleanup(srv.Close)
	return &ghClient{doer: srv.Client(), base: srv.URL, token: "fixture"}
}

type msFixture struct {
	m   domain.MilestoneManifest
	s   *msSources
	req milestoneAcceptanceRequest
}

func msPointer(path string, raw []byte) domain.EvidencePointer {
	return domain.EvidencePointer{Repository: "example-org/widgets", Commit: strings.Repeat("3", 40), Path: path, Digest: domain.ContentDigest(raw)}
}
func msJSON(v any) []byte {
	b, _ := json.Marshal(v)
	var tree any
	_ = json.Unmarshal(b, &tree)
	b, _ = json.Marshal(tree)
	return b
}
func msSetup(t *testing.T) *msFixture {
	t.Helper()
	owner := workref.Context{Cell: "example", Repo: "widgets"}
	reg, e := workref.NewRegistry([]workref.Root{{Cell: "example", Repo: "widgets"}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	s := &msSources{raw: map[domain.EvidencePointer][]byte{}, protected: map[domain.EvidencePointer]domain.Completeness{}, owner: owner, registry: reg, complete: true, comments: map[int64]msComment{}, missing: map[domain.EvidencePointer]bool{}}
	contract, e := os.ReadFile("testdata/milestone-support/06/contract.md")
	if e != nil {
		t.Fatal(e)
	}
	cp := msPointer("docs/streams/build/milestones/m-01-build.md", contract)
	s.raw[cp] = contract
	s.protected[cp] = domain.Present
	policy := domain.MilestoneAuthority{Schema: domain.MilestoneAuthoritySchema, Milestone: "example:widgets:build:m-01@1", DecisionIssue: 17, Humans: []string{"human:driver"}, Duty: domain.DutyPolicy{Duty: "verify", Against: []string{"author"}, MinProof: "attested"}, SeparateAcceptor: true}
	pb := msJSON(policy)
	ap := msPointer("evidence/authority.json", pb)
	s.raw[ap] = pb
	s.protected[ap] = domain.Present
	s.authority = ap
	plan := &MilestonePlan{}
	s.record, _ = parseMilestoneRecord(plan, MilestoneSource{Stream: "build", Owner: &owner, Registry: reg, Repo: fstest.MapFS{"src.go": {Data: []byte("source")}, "evidence/receipt.json": {Data: []byte("manifest")}}}, "milestones/m-01-build.md", contract)
	if len(plan.Diagnostics) > 0 || len(plan.Unknowns) > 0 {
		t.Fatalf("contract loader: %+v %+v", plan.Diagnostics, plan.Unknowns)
	}
	var r domain.Record
	b, e := os.ReadFile("testdata/domain/valid/candidate-a.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	env := domain.ContentDigest([]byte("environment"))
	m := domain.MilestoneManifest{Schema: domain.MilestoneReceiptSchema, Milestone: s.record.Ref(), Contract: cp, Authority: ap, ObservedAt: msAt, Evaluator: "fixture-evaluator@1", Subject: domain.MilestoneSubject{Candidates: []domain.Candidate{r.Candidate}, Artifacts: []string{}, Environment: env, Configuration: domain.ContentDigest([]byte("configuration")), Interface: "build-api@1", Profile: "production"}, Contributors: []domain.MilestoneContributor{}, Criteria: []domain.MilestoneCriterionResult{}, Prerequisites: []domain.MilestonePrerequisite{}}
	for i, pin := range s.record.Contributes {
		rr := r
		rr.Work.Brief = pin.Compatibility
		num := pin.Number
		srcRaw := []byte("brief " + num)
		src := msPointer("docs/streams/build/brief-"+num+".md", srcRaw)
		s.raw[src] = srcRaw
		evRaw := []byte("independent verification " + num)
		ep := msPointer("evidence/verify-"+num+".json", evRaw)
		ep.Anchor = "1"
		s.raw[ep] = evRaw
		c := domain.MilestoneContributor{Brief: pin.Identity.String(), UUID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1), Revision: 1, Source: src, Evidence: []domain.EvidencePointer{ep}}
		m.Contributors = append(m.Contributors, c)
		o := domain.Observations{Work: rr.Work, Candidate: rr.Candidate, Generation: rr.Generation.Recorded, AcceptanceVersion: rr.Acceptance.Version, AcceptanceDigest: rr.Acceptance.Digest, Eligibility: evaluation.EligibilityInput{Streams: []evaluation.Stream{{Name: "build", Briefs: []evaluation.Brief{{Num: num, Status: "done"}}}}}, Coverage: evaluation.CoverageInput{Briefs: []evaluation.BriefObservation{{ID: "build/" + num, BriefRel: src.Path, VerifyRows: []evaluation.VerifyRow{{ID: "1", Command: "test -f src.txt", Expect: "exit 0"}}, Witnesses: map[string][]evaluation.Witness{"1": {{Command: "test -f src.txt", State: evaluation.StatePass, Tree: rr.Candidate.Tree}}}}}}, Repo: msRepo{tree: rr.Candidate.Tree}}
		verify := domain.Actor{ID: "human:verifier", Kind: "human"}
		p := domain.Provenance{Schema: domain.ProvenanceSchema, Requires: []string{"provenance@1"}, Work: rr.Work, Candidate: rr.Candidate, Contract: domain.ContractBinding{Version: rr.Acceptance.Version, Digest: rr.Acceptance.Digest}, Duty: "verify", Evidence: ep.Digest, Requester: verify, Represented: verify, Executor: verify, Responsible: "principal:verifier", Delegation: []string{}, Sponsor: "principal:sponsor", Issuer: "app:fixture", Key: "key:fixture", Capture: domain.Capture{Actor: verify, Source: "witness", ObservedAt: msAt}, Environment: domain.Environment{ManifestDigest: env}, DeclaredProof: "attested", Corroboration: domain.Corroboration{Attribution: "corroborated", Lineage: "corroborated", Execution: "corroborated", Signature: "claimed", Custody: "missing"}}
		author := p
		author.Duty = "author"
		author.Executor = domain.Actor{ID: "agent:author", Kind: "agent-session"}
		author.Requester = author.Executor
		author.Represented = author.Executor
		author.Responsible = "principal:author"
		author.Capture.Actor = author.Executor
		claims := []domain.Provenance{p, author}
		s.briefs = append(s.briefs, domain.MilestoneBriefObservation{Contributor: c, Status: "done", Record: rr, Observations: o, Claims: claims, Selector: msSelector(append([]domain.Provenance(nil), claims...))})
		m.Criteria = append(m.Criteria, domain.MilestoneCriterionResult{ID: fmt.Sprintf("C%02d", i+1), Result: "pass", ObservedAt: msAt, Verifier: verify, Provenance: ep, Evidence: []domain.EvidencePointer{ep}})
	}
	prereq := domain.MilestonePrerequisite{Milestone: "example:widgets:build:m-02@1", Manifest: domain.ContentDigest([]byte("prerequisite receipt")), Decision: domain.DecisionPointer{Repository: "example-org/widgets", Issue: 18, Comment: 180, Digest: domain.ContentDigest([]byte("prerequisite decision"))}, Environment: env, Interface: m.Subject.Interface, Profile: m.Subject.Profile}
	m.Prerequisites = append(m.Prerequisites, prereq)
	s.prerequisites = []domain.MilestonePrereqObservation{{Receipt: prereq, Historical: true, State: "supported"}}
	f := &msFixture{m: m, s: s}
	f.req = milestoneAcceptanceRequest{DecisionIssue: 17, Current: domain.MilestoneInputs{CurrentContract: cp, CurrentMilestone: m.Milestone, CurrentSubject: m.Subject, ObservedAt: msAt, ValidUntil: "2026-11-10T12:00:00Z", Evaluator: m.Evaluator, Requirements: readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{}}}}
	f.publish()
	f.decision(170, "accept", nil, "driver-login")
	return f
}
func (f *msFixture) publish() {
	raw, e := f.m.Canonical()
	if e != nil {
		raw = msJSON(f.m)
	}
	p := msPointer("evidence/receipt.json", raw)
	f.s.raw[p] = raw
	f.req.Manifest = p
}
func (f *msFixture) decision(id int64, disposition string, replaces *domain.DecisionPointer, login string) {
	p := domain.MilestoneDecisionPayload{Schema: domain.MilestoneDecisionSchema, Milestone: f.m.Milestone, Contract: f.m.Contract, Manifest: f.req.Manifest.Digest, Disposition: disposition, Replaces: replaces}
	f.s.comments[id] = msComment{body: string(msJSON(p)), login: login, typ: "User", created: msAt, updated: msAt, parent: 17, status: 200}
}
func (f *msFixture) run(t *testing.T) domain.MilestoneAssessment {
	return readMilestoneAcceptance(f.req, f.s, f.s.client(t), map[string]string{"driver": "driver-login", "outsider": "other-login"})
}
func msSupported(t *testing.T, a domain.MilestoneAssessment) {
	t.Helper()
	if a.State != "supported" || len(a.History) != 1 || a.AcceptanceOutstanding {
		t.Fatalf("want supported exact receipt: %+v", a)
	}
}
func msNotSupported(t *testing.T, a domain.MilestoneAssessment) {
	t.Helper()
	if a.State == "supported" {
		t.Fatalf("unsafe acceptance: %+v", a)
	}
}

func msReason(t *testing.T, a domain.MilestoneAssessment, code string) {
	t.Helper()
	msNotSupported(t, a)
	if !slices.Contains(a.Blockers, code) && !slices.Contains(a.Unknowns, code) {
		t.Fatalf("want reason %s: %+v", code, a)
	}
}

// regression: milestone-support/06
func TestMilestoneAcceptSubject(t *testing.T) {
	msSupported(t, msSetup(t).run(t))
	cases := map[string]func(*msFixture){
		"no-receipt": func(f *msFixture) { f.s.comments = map[int64]msComment{} },
		"reject":     func(f *msFixture) { f.decision(170, "reject", nil, "driver-login") },
		"hold":       func(f *msFixture) { f.decision(170, "hold", nil, "driver-login") },
		"profile":    func(f *msFixture) { f.req.Current.CurrentSubject.Profile = "staging" },
		"environment": func(f *msFixture) {
			f.req.Current.CurrentSubject.Environment = domain.ContentDigest([]byte("other environment"))
		},
		"configuration": func(f *msFixture) {
			f.req.Current.CurrentSubject.Configuration = domain.ContentDigest([]byte("other config"))
		},
		"interface": func(f *msFixture) { f.req.Current.CurrentSubject.Interface = "build-api@2" },
		"candidate": func(f *msFixture) {
			f.req.Current.CurrentSubject.Candidates = append([]domain.Candidate(nil), f.m.Subject.Candidates...)
			f.req.Current.CurrentSubject.Candidates[0].Commit = strings.Repeat("7", 40)
		},
		"version":       func(f *msFixture) { f.req.Current.CurrentMilestone = "example:widgets:build:m-01@2" },
		"source-commit": func(f *msFixture) { f.req.Current.CurrentContract.Commit = strings.Repeat("7", 40) },
		"source-path":   func(f *msFixture) { f.req.Current.CurrentContract.Path = "other.md" },
		"prerequisite-profile": func(f *msFixture) {
			f.m.Prerequisites[0].Profile = "staging"
			f.s.prerequisites[0].Receipt = f.m.Prerequisites[0]
		},
		"swapped-criterion-proof": func(f *msFixture) {
			f.m.Criteria[1].Provenance = f.m.Criteria[0].Provenance
			f.m.Criteria[1].Evidence = f.m.Criteria[0].Evidence
		},
		"missing-criterion":   func(f *msFixture) { f.m.Criteria = f.m.Criteria[:1] },
		"extra-criterion":     func(f *msFixture) { c := f.m.Criteria[0]; c.ID = "C03"; f.m.Criteria = append(f.m.Criteria, c) },
		"duplicate-criterion": func(f *msFixture) { f.m.Criteria = append(f.m.Criteria, f.m.Criteria[0]) },
		"failed-criterion":    func(f *msFixture) { f.m.Criteria[0].Result = "fail" },
		"criterion-verifier":  func(f *msFixture) { f.m.Criteria[0].Verifier.ID = "human:outsider" },
		"brief-revision":      func(f *msFixture) { f.m.Contributors[0].Revision++ },
		"brief-uuid":          func(f *msFixture) { f.m.Contributors[0].UUID = "00000000-0000-4000-8000-000000000099" },
		"latest-second-contributor": func(f *msFixture) {
			f.s.briefs[1].Contributor.Evidence = append([]domain.EvidencePointer(nil), f.s.briefs[1].Contributor.Evidence...)
			f.s.briefs[1].Contributor.Evidence[0].Commit = strings.Repeat("8", 40)
		},
		"wrong-prerequisite": func(f *msFixture) { f.m.Prerequisites[0].Manifest = domain.ContentDigest([]byte("other receipt")) },
		"work-remaining":     func(f *msFixture) { f.s.briefs[0].Status = "verified" },
		"withdrawn":          func(f *msFixture) { f.s.briefs[1].Status = "withdrawn" },
		"wrong-actor":        func(f *msFixture) { c := f.s.comments[170]; c.login = "other-login"; f.s.comments[170] = c },
		"bot":                func(f *msFixture) { c := f.s.comments[170]; c.typ = "Bot"; f.s.comments[170] = c },
		"edited":             func(f *msFixture) { c := f.s.comments[170]; c.updated = "2026-11-09T13:00:00Z"; f.s.comments[170] = c },
		"deleted":            func(f *msFixture) { c := f.s.comments[170]; c.status = 404; f.s.comments[170] = c },
		"wrong-issue":        func(f *msFixture) { c := f.s.comments[170]; c.parent = 19; f.s.comments[170] = c },
		"failed-brief-witness": func(f *msFixture) {
			o := &f.s.briefs[0].Observations
			o.Coverage.Briefs[0].Witnesses["1"][0].Tree = strings.Repeat("9", 40)
		},
		"missing-selector": func(f *msFixture) { f.s.briefs[0].Selector = nil },
		"forged-provenance": func(f *msFixture) {
			f.s.briefs[0].Claims = append([]domain.Provenance(nil), f.s.briefs[0].Claims...)
			f.s.briefs[0].Claims[0].Responsible = "principal:forged"
		},
		"shared-lineage": func(f *msFixture) {
			for i := range f.s.briefs {
				claims := append([]domain.Provenance(nil), f.s.briefs[i].Claims...)
				claims[0].Responsible = claims[1].Responsible
				f.s.briefs[i].Claims = claims
				f.s.briefs[i].Selector = msSelector(claims)
			}
		},
		"acceptor-is-verifier": func(f *msFixture) {
			for i := range f.s.briefs {
				claims := append([]domain.Provenance(nil), f.s.briefs[i].Claims...)
				claims[0].Delegation = []string{"human:driver"}
				f.s.briefs[i].Claims = claims
				f.s.briefs[i].Selector = msSelector(claims)
			}
		},
		"unmerged-policy":    func(f *msFixture) { f.s.protected[f.m.Authority] = domain.Missing },
		"unmerged-contract":  func(f *msFixture) { f.s.protected[f.m.Contract] = domain.Missing },
		"unknown-protection": func(f *msFixture) { delete(f.s.protected, f.m.Contract) },
		"unsupported-semantics": func(f *msFixture) {
			f.req.Current.Requirements.Required = []readercaps.Operation{{Operation: "accept", Schemas: []string{domain.MilestoneReceiptSchema}, Semantics: []string{"future@9"}}}
		},
	}
	expected := map[string]string{"no-receipt": "acceptance-outstanding", "reject": "acceptance-outstanding", "hold": "acceptance-outstanding", "profile": "current-subject", "environment": "current-subject", "configuration": "current-subject", "interface": "current-subject", "candidate": "current-subject", "version": "current-subject", "source-commit": "current-subject", "source-path": "current-subject", "prerequisite-profile": "prerequisite-profile", "swapped-criterion-proof": "criterion-provenance", "missing-criterion": "criterion-missing", "extra-criterion": "contract-set", "duplicate-criterion": "receipt-canonical", "failed-criterion": "criterion-failed", "criterion-verifier": "criterion-provenance", "brief-revision": "contributor-revision", "brief-uuid": "contributor-binding", "latest-second-contributor": "contributor-binding", "wrong-prerequisite": "prerequisite-receipt", "work-remaining": "work-remaining", "withdrawn": "work-remaining", "wrong-actor": "acceptance-outstanding", "bot": "acceptance-outstanding", "edited": "decision-unavailable", "deleted": "decision-unavailable", "wrong-issue": "acceptance-outstanding", "failed-brief-witness": "brief-evidence-support", "missing-selector": "provenance-uncorroborated", "forged-provenance": "provenance-not-selected", "shared-lineage": "principal-overlap", "acceptor-is-verifier": "acceptance-outstanding", "unmerged-policy": "authority-unmerged", "unmerged-contract": "contract-unmerged", "unknown-protection": "contract-protected-source-unknown", "unsupported-semantics": "receipt-capability"}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := msSetup(t)
			change(f)
			before := f.s.comments[170]
			f.publish()
			if before.body != "" && before.login == "driver-login" && before.typ == "User" && before.created == before.updated && before.status == 200 && before.parent == 17 && name != "reject" && name != "hold" {
				f.decision(170, "accept", nil, "driver-login")
			}
			a := f.run(t)
			if expected[name] == "" {
				t.Fatalf("case has no reason assertion: %s", name)
			}
			msReason(t, a, expected[name])
			if name == "wrong-issue" || name == "bot" || name == "wrong-actor" {
				if len(a.Quarantined) != 1 || slices.Contains(a.Unknowns, "decision-unavailable") {
					t.Fatalf("refusal mapping changed: %+v", a)
				}
			}
			if name == "no-receipt" && !a.AcceptanceOutstanding {
				t.Fatalf("completion became acceptance: %+v", a)
			}
		})
	}
	// Bypass host admission: core must independently retain both protected-source
	// checks even when all producer claims and the upper validation were accepted.
	for _, kind := range []string{"contract", "authority"} {
		t.Run("inner-"+kind, func(t *testing.T) {
			f := msSetup(t)
			in := msCoreInputs(t, f)
			if kind == "contract" {
				in.ContractProtected = domain.Missing
			} else {
				in.AuthorityProtected = domain.Missing
			}
			a := domain.AssessMilestone(f.m, in)
			msReason(t, a, kind+"-unmerged")
			if len(a.History) != 0 {
				t.Fatalf("unmerged source established authority: %+v", a)
			}
		})
	}
	t.Run("inner-policy-subject", func(t *testing.T) {
		f := msSetup(t)
		in := msCoreInputs(t, f)
		in.Policy.Milestone = "example:widgets:build:m-02@1"
		msReason(t, domain.AssessMilestone(f.m, in), "authority-policy")
	})
	t.Run("inner-policy-issue", func(t *testing.T) {
		f := msSetup(t)
		in := msCoreInputs(t, f)
		in.Decisions[0].Pointer.Issue = 99
		msReason(t, domain.AssessMilestone(f.m, in), "decision-binding")
	})
	t.Run("coverage-selection", func(t *testing.T) {
		f := msSetup(t)
		f.req.Manifest.Path = "evidence/unlinked.json"
		f.s.raw[f.req.Manifest] = msJSON(f.m)
		msReason(t, f.run(t), "receipt-not-in-coverage")
	})

	t.Run("inner-missing-lineage", func(t *testing.T) {
		f := msSetup(t)
		in := msCoreInputs(t, f)
		in.Decisions[0].Lineage = nil
		a := domain.AssessMilestone(f.m, in)
		msNotSupported(t, a)
		if !slices.Contains(a.Unknowns, "decision-lineage") {
			t.Fatalf("missing identity lineage treated as independence: %+v", a)
		}
	})
	t.Run("unknown-payload", func(t *testing.T) {
		f := msSetup(t)
		c := f.s.comments[170]
		c.body = strings.Replace(c.body, "milestone-decision-v1", "milestone-decision-v9", 1)
		f.s.comments[170] = c
		msReason(t, f.run(t), "decision-binding")
	})
	t.Run("discussion-is-not-accept", func(t *testing.T) {
		f := msSetup(t)
		c := f.s.comments[170]
		c.body = "Approved example:widgets:build:m-01@1 in discussion"
		f.s.comments[170] = c
		msReason(t, f.run(t), "acceptance-outstanding")
	})
	t.Run("duplicate-payload-key", func(t *testing.T) {
		f := msSetup(t)
		c := f.s.comments[170]
		c.body = strings.Replace(c.body, `"disposition":"accept"`, `"disposition":"reject","disposition":"accept"`, 1)
		f.s.comments[170] = c
		msReason(t, f.run(t), "decision-unavailable")
	})
	t.Run("untrusted-current-envelope", func(t *testing.T) {
		f := msSetup(t)
		in := msCoreInputs(t, f)
		f.req.Current = in
		f.s.comments = map[int64]msComment{}
		a := f.run(t)
		msNotSupported(t, a)
		if !a.AcceptanceOutstanding {
			t.Fatal("inherited producer decision")
		}
	})

	t.Run("pointer-path-class", func(t *testing.T) {
		for _, bad := range []string{"..", ".", "../file", "dir/../../file", "/absolute", "dir\\file"} {
			p := msSetup(t).m.Contract
			p.Path = bad
			if e := p.Validate(); e == nil {
				t.Fatalf("unsafe pointer path accepted: %q", bad)
			}
		}
		f := msSetup(t)
		f.m.Contributors[0].Source.Path = ".."
		f.publish()
		msReason(t, f.run(t), "receipt-canonical")
	})

}

func msCoreInputs(t *testing.T, f *msFixture) domain.MilestoneInputs {
	t.Helper()
	in := f.req.Current
	in.Definition = domain.MilestoneDefinition{Milestone: f.m.Milestone, Source: f.m.Contract, Authority: f.m.Authority, Contributors: f.s.record.Contributes, Prerequisites: []string{f.m.Prerequisites[0].Milestone}, Criteria: []string{"C01", "C02"}, CoverageKnown: true, CriterionContributors: map[string][]string{"C01": {f.m.Contributors[0].Brief}, "C02": {f.m.Contributors[1].Brief}}}
	if e := json.Unmarshal(f.s.raw[f.m.Authority], &in.Policy); e != nil {
		t.Fatal(e)
	}
	in.Policy.Source = f.m.Authority
	in.Owner = f.s.owner
	in.Registry = f.s.registry
	in.ContractProtected = domain.Present
	in.AuthorityProtected = domain.Present
	in.Briefs = f.s.briefs
	in.Prerequisites = f.s.prerequisites
	in.DecisionsComplete = true
	for _, p := range domain.MilestoneEvidencePointers(f.m) {
		in.Evidence = append(in.Evidence, domain.MilestoneEvidenceObservation{Pointer: p, State: domain.Present})
	}
	c := f.s.comments[170]
	var payload domain.MilestoneDecisionPayload
	_ = json.Unmarshal([]byte(c.body), &payload)
	in.Decisions = []domain.MilestoneDecision{{Pointer: domain.DecisionPointer{Repository: "example-org/widgets", Issue: 17, Comment: 170, Digest: domain.ContentDigest([]byte(c.body))}, Payload: payload, Actor: domain.Actor{ID: "human:driver", Kind: "human"}, Authenticated: true, Lineage: []string{"human:driver", "principal:driver"}}}
	return in
}

// regression: milestone-support/06
func TestMilestoneRevocation(t *testing.T) {
	f := msSetup(t)
	baseline := f.run(t)
	msSupported(t, baseline)
	ptr := baseline.History[0].Decision
	f.decision(171, "revoke", &ptr, "driver-login")
	a := f.run(t)
	msNotSupported(t, a)
	if len(a.History) != 1 || !a.History[0].Revoked || a.History[0].Decision != ptr || a.History[0].Manifest != baseline.History[0].Manifest || a.State != "blocked" {
		t.Fatalf("revocation erased history: %+v", a)
	}
	cases := map[string]func(*msFixture){
		"stale":                func(f *msFixture) { f.req.Current.ObservedAt = "2026-11-11T12:00:00Z" },
		"missing-evidence":     func(f *msFixture) { f.s.missing[f.m.Contributors[0].Evidence[0]] = true },
		"changed-profile":      func(f *msFixture) { f.req.Current.CurrentSubject.Profile = "staging" },
		"new-version":          func(f *msFixture) { f.req.Current.CurrentMilestone = "example:widgets:build:m-01@2" },
		"partial-thread":       func(f *msFixture) { f.s.complete = false },
		"unknown-prerequisite": func(f *msFixture) { f.s.prerequisites[0].State = "unknown" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := msSetup(t)
			change(f)
			a := f.run(t)
			msNotSupported(t, a)
			if len(a.History) != 1 || a.History[0].Revoked || len(a.Unknowns) == 0 {
				t.Fatalf("history/current conflated: %+v", a)
			}
		})
	}
	for _, disposition := range []string{"revoke", "accept", "supersede"} {
		t.Run("unauthorized-"+disposition, func(t *testing.T) {
			f := msSetup(t)
			f.decision(171, disposition, &ptr, "other-login")
			a := f.run(t)
			msSupported(t, a)
			if len(a.Quarantined) != 1 {
				t.Fatalf("unauthorized successor not quarantined: %+v", a)
			}
		})
	}
	t.Run("unauthorized-malformed", func(t *testing.T) {
		f := msSetup(t)
		f.decision(171, "revoke", &ptr, "other-login")
		c := f.s.comments[171]
		c.body = strings.Replace(c.body, `"disposition":"revoke"`, `"unknown":"revoke"`, 1)
		f.s.comments[171] = c
		msSupported(t, f.run(t))
	})

	for _, disposition := range []string{"revoke", "supersede"} {
		changes := map[string]func(*msComment){
			"edited":         func(c *msComment) { c.updated = "2026-11-09T13:00:00Z" },
			"edited-content": func(c *msComment) { c.updated = "2026-11-09T13:00:00Z"; c.body = "corrected discussion" },
			"deleted":        func(c *msComment) { c.status = 404 },
			"malformed": func(c *msComment) {
				c.body = strings.Replace(c.body, `"disposition":`, `"unexpected":true,"disposition":`, 1)
			},
			"missing-record": func(c *msComment) {
				c.body = strings.Replace(c.body, "example:widgets:build:m-01@1", "example:widgets:build:m-99@1", 1)
			},
		}
		for name, change := range changes {
			t.Run(disposition+"-"+name, func(t *testing.T) {
				f := msSetup(t)
				f.decision(171, disposition, &ptr, "driver-login")
				c := f.s.comments[171]
				change(&c)
				f.s.comments[171] = c
				a := f.run(t)
				msReason(t, a, "decision-unavailable")
				if a.State != "unknown" || len(a.History) != 1 || a.History[0].Decision != ptr || a.History[0].Revoked || a.History[0].Superseded {
					t.Fatalf("untrusted successor rewrote acceptance history: %+v", a)
				}
			})
		}
		t.Run(disposition+"-outsider-edited", func(t *testing.T) {
			f := msSetup(t)
			f.decision(171, disposition, &ptr, "other-login")
			c := f.s.comments[171]
			c.updated = "2026-11-09T13:00:00Z"
			f.s.comments[171] = c
			msSupported(t, f.run(t))
		})
		t.Run(disposition+"-overlap", func(t *testing.T) {
			f := msSetup(t)
			in := msCoreInputs(t, f)
			f.decision(171, disposition, &ptr, "driver-login")
			c := f.s.comments[171]
			var payload domain.MilestoneDecisionPayload
			_ = json.Unmarshal([]byte(c.body), &payload)
			successor := in.Decisions[0]
			successor.Pointer.Comment = 171
			successor.Pointer.Digest = domain.ContentDigest([]byte(c.body))
			successor.Payload = payload
			successor.Lineage = []string{"principal:author"}
			in.Decisions = append(in.Decisions, successor)
			a := domain.AssessMilestone(f.m, in)
			msReason(t, a, "successor-lineage-conflict")
			if a.State != "unknown" || len(a.History) != 1 || a.History[0].Decision != ptr {
				t.Fatalf("overlap restriction erased history: %+v", a)
			}
			wrong := ptr
			wrong.Comment = 999
			in.Decisions[1].Payload.Replaces = &wrong
			msSupported(t, domain.AssessMilestone(f.m, in))
		})
	}
	t.Run("stable-reasons", func(t *testing.T) {
		f := msSetup(t)
		in := msCoreInputs(t, f)
		in.ContractProtected = domain.CouldNotCheck
		in.AuthorityProtected = domain.CouldNotCheck
		in.Evidence = nil
		want := []string{"acceptance-outstanding", "authority-protected-source-unknown", "contract-protected-source-unknown", "evidence-unavailable"}
		for i := 0; i < 16; i++ {
			a := domain.AssessMilestone(f.m, in)
			if !reflect.DeepEqual(a.Unknowns, want) {
				t.Fatalf("unknown order/duplicates: %+v", a)
			}
		}
		in.ContractProtected = domain.Missing
		in.AuthorityProtected = domain.Missing
		for i := 0; i < 16; i++ {
			a := domain.AssessMilestone(f.m, in)
			if !reflect.DeepEqual(a.Blockers, []string{"authority-unmerged", "contract-unmerged"}) {
				t.Fatalf("blocker order/duplicates: %+v", a)
			}
		}
	})

	t.Run("wrong-target", func(t *testing.T) {
		f := msSetup(t)
		wrong := ptr
		wrong.Comment++
		f.decision(171, "revoke", &wrong, "driver-login")
		msSupported(t, f.run(t))
	})
	t.Run("conflicting-successors", func(t *testing.T) {
		f := msSetup(t)
		f.decision(171, "supersede", &ptr, "driver-login")
		f.decision(172, "accept", &ptr, "driver-login")
		a := f.run(t)
		msNotSupported(t, a)
		if !slices.Contains(a.Unknowns, "conflicting-successors") || len(a.History) != 1 {
			t.Fatalf("conflict not retained: %+v", a)
		}
	})
	t.Run("blocked-and-unknown", func(t *testing.T) {
		f := msSetup(t)
		f.decision(171, "revoke", &ptr, "driver-login")
		f.s.complete = false
		a := f.run(t)
		if len(a.Blockers) == 0 || len(a.Unknowns) == 0 || len(a.History) != 1 {
			t.Fatalf("three-state detail lost: %+v", a)
		}
	})
	t.Run("no-mutation", func(t *testing.T) {
		f := msSetup(t)
		before := msJSON(f.m)
		_ = f.run(t)
		if !reflect.DeepEqual(before, msJSON(f.m)) {
			t.Fatal("assessment mutated its manifest")
		}
	})
}
