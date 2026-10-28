package deskkit

import (
	"errors"
	"slices"
	"testing"
)

type verifierObservationForge struct {
	*verifierForge
	reads, histories, timelines int
	afterHistory                func()
	finalReadError              bool
	timelineError               bool
	incompleteHistory           bool
}

func (f *verifierObservationForge) GetIssueTyped(repo ForgeRepo, n int, kind TargetKind) (*Issue, error) {
	f.reads++
	if f.finalReadError && f.reads == 2 {
		return nil, errors.New("current state unavailable")
	}
	i, err := f.verifierForge.GetIssueTyped(repo, n, kind)
	if i == nil || err != nil {
		return i, err
	}
	copy := *i
	copy.Labels = slices.Clone(i.Labels)
	return &copy, nil
}
func (f *verifierObservationForge) IssueTrustEvents(repo ForgeRepo, n int) (*TrustPayload, error) {
	f.histories++
	got, err := f.verifierForge.IssueTrustEvents(repo, n)
	if got != nil && f.incompleteHistory {
		got.Complete = false
	}
	if f.afterHistory != nil {
		f.afterHistory()
	}
	return got, err
}
func (f *verifierObservationForge) ListIssueLabelEvents(repo ForgeRepo, n int) ([]LabelEvent, error) {
	f.timelines++
	if f.timelineError {
		return nil, errors.New("incomplete label timeline")
	}
	return f.verifierForge.ListIssueLabelEvents(repo, n)
}

func TestVerifierObservationPreservesProvenanceAndCurrentState(t *testing.T) {
	cases := []struct {
		name   string
		change func(*verifierObservationForge)
	}{
		{"valid", nil},
		{"edited-history", func(f *verifierObservationForge) { f.edited = true }},
		{"unknown-history", func(f *verifierObservationForge) { f.missingHistory = true }},
		{"incomplete-history", func(f *verifierObservationForge) { f.incompleteHistory = true }},
		{"incomplete-labels", func(f *verifierObservationForge) { f.timelineError = true }},
		{"current-read-unavailable", func(f *verifierObservationForge) { f.finalReadError = true }},
		{"body-moves-after-history", func(f *verifierObservationForge) {
			f.afterHistory = func() { f.issue.Body += "edited" }
		}},
		{"title-moves-after-history", func(f *verifierObservationForge) {
			f.afterHistory = func() { f.issue.Title += "edited" }
		}},
		{"reopens-after-history", func(f *verifierObservationForge) {
			f.afterHistory = func() { f.issue.State = "open" }
		}},
		{"kind-moves-after-history", func(f *verifierObservationForge) {
			f.afterHistory = func() { f.issue.IsPullRequest = true }
		}},
		{"author-moves-after-history", func(f *verifierObservationForge) {
			f.afterHistory = func() { f.issue.Author.Login = "untrusted" }
		}},
		{"labels-removed-after-history", func(f *verifierObservationForge) {
			f.afterHistory = func() { f.issue.Labels = nil }
		}},
		{"labels-reapplied-by-foreign-actor", func(f *verifierObservationForge) {
			f.afterHistory = func() {
				for _, label := range f.issue.Labels {
					f.events = append(f.events, LabelEvent{Name: label, Removed: true, AppliedBy: "example-desk[bot]"}, LabelEvent{Name: label, AppliedBy: "untrusted"})
				}
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, base := verifierFixture(t, "gpt-6.1-sol")
			want, err := IssueVerifierAttestation(root, base)
			if err != nil {
				t.Fatal(err)
			}
			f := &verifierObservationForge{verifierForge: base}
			if tc.change != nil {
				tc.change(f)
			}
			got, err := ReadVerifierAttestation(f, ForgeRepo{Owner: "example-org", Name: "one"}, 41)
			if tc.change == nil {
				if err != nil || got != want {
					t.Fatalf("receipt = %+v, %v; want %+v", got, err, want)
				}
				if f.reads != 2 || f.histories != 1 || f.timelines != 1 {
					t.Fatalf("issue/history/labels = %d/%d/%d; want 2/1/1", f.reads, f.histories, f.timelines)
				}
				// Scheduling reuse never reaches execution admission.
				base.issue.State = "open"
				if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
					t.Fatal("execution used an earlier scheduling receipt after reopen")
				}
			} else if err == nil {
				t.Fatal("changed or unverifiable attestation was admitted")
			}
		})
	}
}
