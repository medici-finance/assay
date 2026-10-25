package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

// Hides only the optional batch capability; all original wrapped Forge methods
// remain the same. This is the serial reference arm, never a production fallback.
type serialPositiveForge struct{ deskkit.Forge }

type changingDuplicateForge struct {
	deskkit.Forge
	reads  int
	second string
}

func (f *changingDuplicateForge) GetIssue(repo deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	if n == 9 {
		f.reads++
		if f.reads == 2 && f.second == "open" {
			return &deskkit.Issue{Number: n, State: "open", Title: "reopened"}, nil
		}
		if f.reads == 2 && f.second == "denied" {
			return nil, deskkit.Unverifiable("second duplicate read denied", nil)
		}
	}
	return f.Forge.GetIssue(repo, n)
}

// regression: #4360
// A malformed local population may contain two placeholder files for one
// issue. Keep both original row reads rather than silently sharing a fact.
func TestPositiveStateDuplicatePlaceholdersKeepSerialReads(t *testing.T) {
	for _, second := range []string{"closed", "open", "denied"} {
		t.Run(second, func(t *testing.T) {
			root, calls := projectionFixture(t)
			_ = calls
			resolve := forgeFor
			var changing *changingDuplicateForge
			forgeFor = func(repo string, obs ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
				f, fr, err := resolve(repo, obs...)
				if repo == homeRepo {
					changing = &changingDuplicateForge{Forge: f, second: second}
					f = changing
				}
				return f, fr, err
			}
			dir := filepath.Join(root, issueLoopDir)
			if err := os.WriteFile(filepath.Join(dir, "second-issue-9.md"), []byte(placeholderFixture(homeRepo, "todo", "")), 0600); err != nil {
				t.Fatal(err)
			}
			code, out := projectionRun(t, root, "--projection", "intake")
			if changing == nil || changing.reads != 2 {
				t.Fatal("duplicate observations reused", changing)
			}
			if second == "closed" {
				if code != 0 || strings.Count(out, "closed-nine") != 2 {
					t.Fatal(code, out)
				}
			} else {
				if code != 6 || strings.Contains(out, "\nRETIRE") {
					t.Fatal("later duplicate failure was hidden", code, out)
				}
			}
		})
	}
}

// regression: #4360
// regression: class issue-positive-state-batch
func TestPositiveStateBatchNativeParity(t *testing.T) {
	counts := []int{0, 1, 21, 95}
	var qualification *reviewtest.Request
	if os.Getenv(reviewtest.RequestEnv) != "" {
		q := reviewtest.Read(t)
		qualification = &q
		counts = []int{q.Size}
	}
	for _, count := range counts {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			resolve := forgeFor
			installForge(t, nil)
			// Native adapter selection must be explicit, not inferred from the
			// checkout's remote (harness copies intentionally carry no Git home).
			config := filepath.Join(os.Getenv("HOME"), ".config", "assay")
			var forges []string
			for _, repo := range deskkit.ScanRepos() {
				forges = append(forges, repo+"=github")
			}
			roster := filepath.Join(config, "roster.env")
			rawRoster, err := os.ReadFile(roster)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(roster, append(rawRoster, []byte("\nASSAY_REPO_FORGES="+strings.Join(forges, ",")+"\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ASSAY_CONFIG_HOME", config)
			deskkit.ReloadConfig()
			t.Cleanup(deskkit.ReloadConfig)
			oldMint, oldRole, oldBase, oldTransport := mintTokenFn, sessionRoleFn, forgeAPIBase, http.DefaultTransport
			t.Cleanup(func() {
				mintTokenFn, sessionRoleFn, forgeAPIBase, http.DefaultTransport = oldMint, oldRole, oldBase, oldTransport
			})
			mintTokenFn = func(string, string) (string, string, error) { return "fixture-token", "", nil }
			sessionRoleFn = func(string) (string, string, error) { return "worker", "worker-desk", nil }
			forgeAPIBase = ""
			root := t.TempDir()
			dir := filepath.Join(root, issueLoopDir)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for n := 1; n <= count; n++ {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("issue-%d.md", n)), []byte(placeholderFixture(homeRepo, "todo", "")), 0600); err != nil {
					t.Fatal(err)
				}
			}
			requests, graphql, singles := 0, 0, 0
			var witness []string
			fault := ""
			view := "intake"
			http.DefaultTransport = perfWire(func(r *http.Request) (*http.Response, error) {
				requests++
				if qualification != nil {
					time.Sleep(8 * time.Millisecond)
				}
				witness = append(witness, r.Method+" "+r.URL.RequestURI())
				body := "[]"
				status := 200
				if r.URL.Path == "/graphql" {
					graphql++
					var in struct {
						Query     string         `json:"query"`
						Variables map[string]any `json:"variables"`
					}
					rawRequest, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatal(err)
					}
					witness[len(witness)-1] += " body=" + string(rawRequest)
					if err := json.Unmarshal(rawRequest, &in); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(in.Query, "mutation") || strings.Contains(in.Query, "issue(number:1)") {
						t.Fatal(in.Query)
					}
					rows := map[string]any{}
					for slot := 0; slot < 20; slot++ {
						if in.Variables[fmt.Sprintf("use%d", slot)] != true {
							continue
						}
						raw, ok := in.Variables[fmt.Sprintf("n%d", slot)].(float64)
						if !ok || raw < 1 || raw > float64(count) {
							t.Fatal(in.Variables)
						}
						n := int(raw)
						rows[fmt.Sprintf("a%d", slot)] = map[string]any{"number": n, "state": "CLOSED", "title": fmt.Sprintf("closed-%d", n)}
					}
					if len(rows) == 0 || len(rows) > 20 {
						t.Fatal(in.Variables)
					}
					env := map[string]any{"data": map[string]any{"repository": rows}}
					switch fault {
					case "null":
						rows["a0"] = nil
					case "missing":
						delete(rows, "a0")
					case "wrong-number":
						rows["a0"].(map[string]any)["number"] = 99999
					case "unknown-state":
						rows["a0"].(map[string]any)["state"] = "UNKNOWN"
					case "open":
						rows["a0"].(map[string]any)["state"] = "OPEN"
					case "data-and-errors":
						env["errors"] = []any{map[string]any{"message": "permission denied"}}
					case "second-page-null":
						if graphql == 2 {
							rows["a0"] = nil
						}
					case "permission":
						status = 403
						env = map[string]any{"message": "permission denied"}
					}
					b, _ := json.Marshal(env)
					body = string(b)
				} else if !strings.HasSuffix(r.URL.Path, "/issues") {
					singles++
					n, _ := strconv.Atoi(filepath.Base(r.URL.Path))
					if n < 1 || n > count {
						t.Fatal(r.URL.Path)
					}
					body = fmt.Sprintf(`{"number":%d,"state":"closed","title":"closed-%d"}`, n, n)
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			runArm := func(serial bool) (int, string, int, int, int) {
				requests, graphql, singles = 0, 0, 0
				witness = nil
				forgeFor = func(repo string, obs ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
					f, fr, err := resolve(repo, obs...)
					if err != nil {
						return nil, fr, err
					}
					f = deskkit.ReadOnly(f)
					if serial {
						return &serialPositiveForge{f}, fr, nil
					}
					return f, fr, nil
				}
				var out, errs bytes.Buffer
				args := []string{"issues", "--root", root}
				switch view {
				case "inbox":
					args = append(args, "--projection", "inbox", "--to", "worker")
				case "legacy-to":
					args = append(args, "--to", "worker")
				default:
					args = append(args, "--projection", "intake")
				}
				var code int
				if qualification != nil {
					t.Setenv("ASSAY_TELEMETRY", "1")
					code = measured(append(args, "--perf-profile=tool"), &out, &errs, nil)
				} else {
					code = run(args, &out, &errs)
				}
				return code, projectionStable(out.String() + errs.String()), requests, graphql, singles
			}
			if qualification != nil {
				if qualification.Case != "serial" && qualification.Case != "batch" {
					t.Fatal("unknown qualification case")
				}
				code, out, attempts, queries, gets := runArm(qualification.Case == "serial")
				h, err := deskkit.LoadPerfHistory(1)
				if err != nil || len(h.Records) != 1 {
					t.Fatal("native writer/reader", err, len(h.Records))
				}
				raw, err := json.Marshal(h.Records)
				if err != nil {
					t.Fatal(err)
				}
				reviewtest.Write(t, reviewtest.Response{Exit: code, Output: out, Snapshot: raw, Calls: append([]string(nil), witness...), HTTPComplete: true, Facts: map[string]string{"http": fmt.Sprint(attempts), "stateGraphQL": fmt.Sprint(queries), "stateGET": fmt.Sprint(gets), "delayMs": "8"}})
				return
			}
			bc, bo, br, bg, bs := runArm(true)
			ac, ao, ar, ag, as := runArm(false)
			wantBatches := 0
			wantSingles := 0
			if count == 1 {
				wantSingles = 1
			} else {
				wantBatches = (count + 19) / 20
				if count%20 == 1 {
					wantBatches--
					wantSingles = 1
				}
			}
			if bc != 0 || ac != 0 || bo != ao || bg != 0 || bs != count || ag != wantBatches || as != wantSingles || br-ar != count-wantBatches-wantSingles {
				t.Fatalf("count=%d serial=%d/%d/%d/%d candidate=%d/%d/%d/%d outputEqual=%v\n%s\n%s", count, bc, br, bg, bs, ac, ar, ag, as, bo == ao, bo, ao)
			}
			_, again, rr, rg, rs := runArm(false)
			if again != ao || rr != ar || rg != ag || rs != as {
				t.Fatal("later invocation reused facts", rr, rg, rs)
			}
			t.Logf("matched count=%d serialHTTP=%d candidateHTTP=%d stateGET=%d->%d stateGraphQL=%d->%d", count, br, ar, bs, as, bg, ag)
			if count == 95 {
				for _, bad := range []string{"null", "missing", "wrong-number", "unknown-state", "open", "data-and-errors", "second-page-null", "permission"} {
					fault = bad
					code, out, _, queries, gets := runArm(false)
					wantQueries := 1
					if bad == "second-page-null" {
						wantQueries = 2
					}
					if code != 6 || queries != wantQueries || gets != 0 || strings.Contains(out, "\nRETIRE") {
						t.Fatalf("fault=%s code=%d queries=%d GETfallback=%d output=%s", bad, code, queries, gets, out)
					}
				}
				fault = ""
				for _, mode := range []string{"inbox", "legacy-to"} {
					view = mode
					code, out, attempts, queries, gets := runArm(false)
					if code != 0 || queries != 0 || gets != 0 || attempts != len(deskkit.ScanRepos()) || strings.Contains(out, "closed-") {
						t.Fatal("unselected placeholders performed reads", mode, code, attempts, queries, gets, out)
					}
				}
			}
		})
	}
}
