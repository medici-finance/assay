package deskkit

// Verify row 11 of desktools-v2/71: statusgen's container credit check reads the
// attestation record through deskread and applies ReadVerifierAttestation's
// rules itself, because statusgen does not link this package. This test holds
// the two readers to one verdict: every shared fixture in
// statusgen/testdata/credit-records/ (statusgen's TestCreditRecordFixtures reads
// the same files) is served through a fake forge, and ReadVerifierAttestation
// must admit or refuse it as the fixture's `expect` says.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type creditParityFixture struct {
	Why    string `json:"why"`
	Expect string `json:"expect"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Issue  struct {
		State  string   `json:"state"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		URL    string   `json:"url"`
		Labels []string `json:"labels"`
		Author struct {
			Login string `json:"login"`
			ID    int64  `json:"id"`
			Type  string `json:"type"`
		} `json:"author"`
	} `json:"issue"`
	Trust struct {
		Complete         bool   `json:"complete"`
		BodyHistoryKnown *bool  `json:"bodyHistoryKnown"`
		BodyEditedAt     string `json:"bodyEditedAt"`
		Events           []any  `json:"events"`
	} `json:"trust"`
	LabelEvents struct {
		Complete bool `json:"complete"`
		Events   []struct {
			Label      string `json:"label"`
			Removed    bool   `json:"removed"`
			CreatedAt  string `json:"createdAt"`
			ActorLogin string `json:"actorLogin"`
			ActorID    int64  `json:"actorId"`
		} `json:"events"`
	} `json:"labelEvents"`
}

// creditParityForge serves one fixture. Only the three reads
// ReadVerifierAttestation makes are implemented; any other call panics on the
// nil embedded Forge.
type creditParityForge struct {
	Forge
	t *testing.T
	f creditParityFixture
}

func (p *creditParityForge) want(repo ForgeRepo, n int) error {
	if repo.Slug() != p.f.Repo || n != p.f.Number {
		p.t.Errorf("read %s#%d, the fixture is %s#%d", repo.Slug(), n, p.f.Repo, p.f.Number)
		return errors.New("not this fixture")
	}
	return nil
}

func (p *creditParityForge) GetIssueTyped(repo ForgeRepo, n int, kind TargetKind) (*Issue, error) {
	if err := p.want(repo, n); err != nil {
		return nil, err
	}
	i := p.f.Issue
	return &Issue{Number: n, State: i.State, Title: i.Title, URL: i.URL, Body: i.Body,
		Labels: append([]string(nil), i.Labels...),
		Author: Account{Login: i.Author.Login, ID: i.Author.ID, Type: i.Author.Type}}, nil
}

func (p *creditParityForge) IssueTrustEvents(repo ForgeRepo, n int) (*TrustPayload, error) {
	if err := p.want(repo, n); err != nil {
		return nil, err
	}
	tr := &TrustPayload{Complete: p.f.Trust.Complete, BodyHistoryKnown: p.f.Trust.BodyHistoryKnown != nil && *p.f.Trust.BodyHistoryKnown}
	if p.f.Trust.BodyEditedAt != "" {
		at, err := time.Parse(time.RFC3339, p.f.Trust.BodyEditedAt)
		if err != nil {
			p.t.Fatalf("fixture %d bodyEditedAt: %v", p.f.Number, err)
		}
		tr.BodyEdited = at
	}
	if len(p.f.Trust.Events) != 0 {
		p.t.Fatalf("fixture %d: comment events are not modelled here", p.f.Number)
	}
	return tr, nil
}

func (p *creditParityForge) ListIssueLabelEvents(repo ForgeRepo, n int) ([]LabelEvent, error) {
	if err := p.want(repo, n); err != nil {
		return nil, err
	}
	if !p.f.LabelEvents.Complete {
		return nil, errors.New("incomplete label timeline")
	}
	var out []LabelEvent
	for _, e := range p.f.LabelEvents.Events {
		out = append(out, LabelEvent{Name: e.Label, AppliedBy: e.ActorLogin, Removed: e.Removed, CreatedAt: e.CreatedAt, AppliedByID: e.ActorID})
	}
	return out, nil
}

func TestVerifierAttestationCreditParity(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "statusgen", "testdata", "credit-records")
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no shared credit-record fixtures under %s: %v", dir, err)
	}
	admitted := 0
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var f creditParityFixture
			dec := json.NewDecoder(strings.NewReader(string(raw)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&f); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if f.Expect != "admit" && f.Expect != "refuse" {
				t.Fatalf("expect %q is neither admit nor refuse", f.Expect)
			}
			owner, repoName, _ := strings.Cut(f.Repo, "/")
			r, err := ReadVerifierAttestation(&creditParityForge{t: t, f: f}, ForgeRepo{Owner: owner, Name: repoName}, f.Number)
			switch {
			case f.Expect == "admit" && err != nil:
				t.Errorf("%s: refused (%v), want admitted", f.Why, err)
			case f.Expect == "refuse" && err == nil:
				t.Errorf("%s: admitted (%+v), want refused", f.Why, r.Binding)
			case f.Expect == "refuse" && !IsRefused(err):
				t.Errorf("%s: %v is not a refusal", f.Why, err)
			case err == nil:
				admitted++
			}
		})
	}
	if admitted == 0 {
		t.Fatal("no fixture was admitted; the parity check would pass on a reader that refuses everything")
	}
}
