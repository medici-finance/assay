package main

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

// Keep all other issue reads on the existing fail-closed fake, but enumerate with
// the REAL GitHub adapter. Counts are HTTP attempts, not ListOpenIssues calls.
type qualificationForge struct {
	*fakeForge
	gh *deskkit.GitHubForge
}

func (f *qualificationForge) ListOpenIssues(fr deskkit.ForgeRepo) ([]deskkit.IssueSummary, error) {
	return f.gh.ListOpenIssues(fr)
}

func TestQualificationProbe(t *testing.T) {
	q := reviewtest.Read(t)
	installForge(t, map[string]*repoFixture{})
	reviewtest.Home(t, q, fixtureRoster)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	transport := &reviewtest.Transport{Answer: func(req *http.Request) (int, string) {
		if req.Method != "GET" || !strings.HasSuffix(req.URL.Path, "/issues") {
			return 500, `{"message":"unexpected fixture endpoint"}`
		}
		if q.Case == "access-failure" && strings.Contains(req.URL.Path, "/tracker/") {
			return 403, `{"message":"fixture access denied"}`
		}
		if q.Case == "empty" || !strings.Contains(req.URL.Path, "/tracker/") {
			return 200, "[]"
		}
		if q.Case != "open" {
			return 500, `{"message":"unknown fixture case"}`
		}
		var rows []string
		for i := 1; i <= q.Size; i++ {
			rows = append(rows, fmt.Sprintf(`{"number":%d,"title":"fixture","state":"open","user":{"login":"shared-agent","id":2002},"labels":[]}`, i))
		}
		// Explicit terminal pagination: page 2 is empty, including cap-sized cohorts.
		if req.URL.Query().Get("page") != "1" {
			return 200, "[]"
		}
		return 200, "[" + strings.Join(rows, ",") + "]"
	}}
	gh := &deskkit.GitHubForge{Token: "local-fixture", BaseURL: "https://fixture.invalid", Client: &http.Client{Transport: transport}}
	previous := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		f, fr, err := previous(repo)
		if err != nil {
			return nil, fr, err
		}
		return &qualificationForge{fakeForge: f.(*fakeForge), gh: gh}, fr, nil
	}
	t.Cleanup(func() { forgeFor = previous })
	var out, errb bytes.Buffer
	code := run([]string{"--root", q.Root, "issues"}, &out, &errb)
	reviewtest.Write(t, reviewtest.Response{Exit: code, Facts: map[string]string{"output": out.String()}, Attempts: transport.Snapshot(), HTTPComplete: q.Case != "open"})
}
