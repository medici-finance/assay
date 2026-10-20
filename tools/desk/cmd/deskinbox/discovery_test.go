package main

import (
	"bytes"
	"encoding/json"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
	"net/http"
	"strings"
	"testing"
	"time"
)

// regression: class issue-projection
func TestDiscoveryJourney(t *testing.T) {
	withForge(t, nil)
	for _, denied := range []bool{false, true} {
		transport := &reviewtest.Transport{Answer: func(req *http.Request) (int, string) {
			if req.Method != "GET" || !strings.HasSuffix(req.URL.Path, "/issues") {
				t.Errorf("unexpected enrichment request %s %s", req.Method, req.URL.Path)
				return 500, `{}`
			}
			if denied {
				return 403, `{"message":"fixture permission loss"}`
			}
			return 200, `[{"number":1,"title":"question-one","state":"open","created_at":"2026-01-01T00:00:00Z","user":{"login":"external","id":99},"labels":[{"name":"question"}]},{"number":2,"title":"urgent-two","state":"open","created_at":"2026-01-02T00:00:00Z","user":{"login":"external","id":99},"labels":[{"name":"urgent"},{"name":"question"}]},{"number":3,"title":"unrelated-three","state":"open","user":{"login":"external","id":99},"labels":[]}]`
		}}
		forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
			return &deskkit.GitHubForge{Token: "local-fixture", BaseURL: "https://fixture.invalid", Client: &http.Client{Transport: transport}}, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
		}
		var out, errb bytes.Buffer
		code := run([]string{"discovery", "example-org/tracker"}, &out, &errb, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
		var rows []struct {
			Number int
			Labels []struct{ Name string }
		}
		if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
			t.Fatal(err, out.String())
		}
		if denied {
			if code != 2 || len(rows) != 0 || !strings.Contains(errb.String(), "QUERY FAILED") {
				t.Fatal("permission loss became complete", code, out.String(), errb.String())
			}
		} else {
			if code != 0 || len(rows) != 2 || rows[0].Number != 2 || rows[1].Number != 1 || rows[0].Labels[0].Name != "urgent" {
				t.Fatal("human inbox selection/order/wire changed", code, out.String(), errb.String())
			}
		}
		if got := len(transport.Snapshot()); got != 1 {
			t.Fatal("unexpected HTTP attempts", got)
		}
	}
}
