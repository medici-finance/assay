package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

// Real command and GitHubForge request encoder/decoder; only HTTP is substituted.
func TestQueueBridgeProbe(t *testing.T) {
	q := reviewtest.Read(t)
	q.Case = strings.TrimPrefix(q.Case, "queue-")
	legacyCost := q.Case == "legacy-cycle"
	if legacyCost {
		q.Case = "normal"
	}
	throughput := strings.HasPrefix(q.Case, "throughput-")
	q.Case = strings.TrimPrefix(q.Case, "throughput-")
	roster := `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=reviewer=github:reviewer:41987965
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
ASSAY_REPO_FORGES=example-org/tracker=github
`
	if q.Case == "untrusted-public" {
		roster = strings.ReplaceAll(roster, ":ci:private", ":ci:public")
		q.Case = "untrusted"
	}
	reviewtest.Home(t, q, roster)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	t.Setenv("DESKBOARD_GATE_SCORES_JSON", "[]")
	mintTokenFn = func(string, string) (string, string, error) { return "fixture-token", "", nil }
	sessionRoleFn = func(string) (string, string, error) { return "worker", "fixture", nil }
	forgeAPIBase = "https://fixture.invalid"
	transport := &reviewtest.Transport{Answer: func(r *http.Request) (int, string) {
		if r.Method == "POST" && r.URL.Path == "/graphql" {
			if q.Case == "denied" {
				return 403, `{"message":"Forbidden"}`
			}
			if q.Case == "untrusted" {
				var request struct{ Query string }
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request.Query == deskkit.PRTrustQuery {
					return 200, `{"data":{"repository":{"pullRequest":{"lastEditedAt":null,"comments":{"nodes":[],"pageInfo":{"hasNextPage":false}},"reviews":{"nodes":[],"pageInfo":{"hasNextPage":false}},"reviewThreads":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}}`
				}
			}
			nodes := []any{}
			size := q.Size
			if q.Case == "empty" {
				size = 0
			}
			for i := 1; i <= size; i++ {
				head := strings.Repeat("a", 40)
				if q.Case == "missing-head" {
					head = ""
				}
				reviews := map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{}}
				if q.Case == "lanes" {
					review := func(id int, state, body string) map[string]any {
						return map[string]any{"databaseId": id, "author": map[string]any{"login": "reviewer", "__typename": "Bot", "databaseId": 41987965}, "state": state, "commit": map[string]any{"oid": head}, "body": body, "submittedAt": "2026-10-02T00:00:00Z"}
					}
					switch i {
					case 1:
						reviews["nodes"] = []any{review(1, "APPROVED", "Security-Review: pass")}
					case 2:
						reviews["nodes"] = []any{review(2, "CHANGES_REQUESTED", "Verdict: request-changes"), review(3, "APPROVED", "Security-Review: pass")}
					case 3:
						reviews["nodes"] = []any{review(4, "APPROVED", "Verdict: approve")}
					case 4:
						reviews["nodes"] = []any{review(5, "CHANGES_REQUESTED", "Security-Review: fail")}
					}
				}
				if q.Case == "partial" {
					reviews["pageInfo"] = map[string]any{"hasNextPage": true}
				}
				author := "shared-agent"
				if q.Case == "untrusted" && i == 1 {
					author = "untrusted-contributor"
				}
				nodes = append(nodes, map[string]any{"number": i, "title": fmt.Sprintf("change %d", i), "body": "", "state": "OPEN", "isDraft": true, "createdAt": "2026-10-01T00:00:00Z", "updatedAt": "2026-10-22T00:00:00Z", "author": map[string]any{"login": author, "__typename": "User"}, "mergeStateStatus": "CLEAN", "headRefOid": head, "headRefName": "feat/x", "baseRefName": "main", "labels": map[string]any{"nodes": []any{}}, "reviews": reviews, "commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"oid": head, "committedDate": "2026-10-01T00:00:00Z", "statusCheckRollup": map[string]any{"contexts": map[string]any{"nodes": []any{map[string]any{"__typename": "CheckRun", "name": "build", "databaseId": i, "status": "COMPLETED", "conclusion": "SUCCESS"}}}}}}}}})
			}
			b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": nodes}}}})
			return 200, string(b)
		}
		if strings.HasSuffix(r.URL.Path, "/files") {
			return 200, `[{"filename":"example.go","status":"modified"}]`
		}
		if q.Case != "partial" && r.Method == "GET" && strings.Contains(r.URL.Path, "/pulls/") {
			if q.Case == "moved-head" {
				return 200, `{"number":1,"state":"open","changed_files":1,"head":{"sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`
			}
			return 200, `{"number":1,"state":"open","changed_files":1,"head":{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`
		}
		return 500, `{"message":"unexpected or unavailable enrichment"}`
	}}
	old := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = old })
	boardGuard = func() error { return nil }
	out := filepath.Join(q.Root, "observation.json")
	var stdout, stderr bytes.Buffer
	if legacyCost {
		hdr := Header{AsOf: time.Now().UTC().Format(time.RFC3339)}
		if _, _, err := fetchOpenPRs("example-org/tracker"); err != nil {
			t.Fatal(err)
		}
		if _, err := cmdActions(&hdr, 20*time.Minute, 30*time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, err := cmdPRs(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := cmdActions(&hdr, 20*time.Minute, 30*time.Minute); err != nil {
			t.Fatal(err)
		}
		reviewtest.Write(t, reviewtest.Response{Exit: 0, Attempts: transport.Snapshot(), HTTPComplete: true})
		return
	}
	exit := run([]string{"review-queue", "--out", out, "--rate-floor", "0"}, &stdout, &stderr)
	b, _ := os.ReadFile(out)
	facts := map[string]string{}
	if throughput && exit == 0 {
		before := len(transport.Snapshot())
		depth, _, err := frozenReviewDepth(out, time.Now().UTC())
		facts["depth"] = fmt.Sprint(depth)
		facts["depthError"] = fmt.Sprint(err)
		for _, bad := range []string{"stale", "scope", "policy", "schema"} {
			var o rf.Observation
			_ = json.Unmarshal(b, &o)
			now := time.Now().UTC()
			switch bad {
			case "stale":
				now = now.Add(time.Hour)
			case "scope":
				o.Inputs.Requested[0].Repository = "example-org/other"
			case "policy":
				o.Inputs.Policy = "changed"
			case "schema":
				o.Schema = "unsupported-v9"
			}
			raw, _ := json.Marshal(o)
			if bad == "scope" {
				raw = []byte(strings.ReplaceAll(string(b), "example-org/tracker", "example-org/other"))
			}
			_ = os.WriteFile(out, raw, 0600)
			_, _, e := frozenReviewDepth(out, now)
			if e == nil {
				t.Fatal("invalid frozen review depth accepted", bad)
			}
			want := map[string]string{"stale": "collection began before the caller cutoff", "scope": "review access scope changed", "policy": "unsupported evaluator or policy revision", "schema": "unsupported review observation schema"}[bad]
			if !strings.Contains(e.Error(), want) {
				t.Fatalf("wrong frozen depth refusal for %s: want %q, got %v", bad, want, e)
			}
			if bad != "scope" {
				kind := rf.SchemaError
				if bad == "stale" {
					kind = rf.StaleError
				}
				var input *rf.InputError
				if !errors.As(e, &input) || input.Kind != kind {
					t.Fatalf("wrong frozen depth error class for %s: %v", bad, e)
				}
			}
		}
		_ = os.WriteFile(out, b, 0600)
		var tp, te bytes.Buffer
		code := run([]string{"throughput", "--review-observation", out}, &tp, &te)
		var report throughputReport
		if code != 0 || json.Unmarshal(tp.Bytes(), &report) != nil {
			t.Fatal("throughput adapter failed", code, tp.String(), te.String())
		}
		found := false
		for _, stage := range report.Stages {
			if stage.Stage == "review" {
				found = stage.Depth != nil && *stage.Depth == q.Size && stage.Blind == ""
			}
		}
		if !found {
			t.Fatal("throughput lost frozen depth", report)
		}
		facts["depthAdditionalHTTP"] = fmt.Sprint(len(transport.Snapshot()) - before)
		if facts["depthAdditionalHTTP"] != "0" {
			t.Fatal("frozen depth recollected review")
		}
	}
	reviewtest.Write(t, reviewtest.Response{Exit: exit, Output: stdout.String() + stderr.String(), Snapshot: b, Facts: facts, Attempts: transport.Snapshot(), HTTPComplete: true})
}
