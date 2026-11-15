package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The verify-runner image's cluster stager (images/verify-runner/stage-cache.sh)
// screens every module path a clone's go.mod files declare before it runs
// `go mod download`. These tests run the real script against a fixture clone.
// No network is used and no target code runs: a stub `git` first on PATH
// answers the two pin checks with the fixture commit, and a stub `go` answers
// `go mod edit -json` with the fixture's parsed go.mod and records whether
// `go mod download` was reached.

const stageCacheEntrypoint = "../images/verify-runner/stage-cache.sh"

const stageCacheRepo = "medici-finance/assay"

// stageCacheRun stages one fixture clone whose single go.mod requires dep and
// returns the exit code, the stdout, and whether the download was reached.
func stageCacheRun(t *testing.T, script, dep string) (int, string, bool) {
	t.Helper()
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	clone := filepath.Join(cache, "clone")
	bin := filepath.Join(root, "bin")
	reached := filepath.Join(root, "download.reached")
	for _, d := range []string{filepath.Join(clone, ".git"), bin, filepath.Join(root, "mod"), filepath.Join(root, "gocache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	putFile(t, filepath.Join(clone, "go.mod"), "module github.com/medici-finance/assay/fixture\n")
	parsed, err := json.Marshal(map[string]any{
		"Module":  map[string]string{"Path": "github.com/medici-finance/assay/fixture"},
		"Require": []map[string]string{{"Path": dep, "Version": "v1.0.0"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(root, "parsed.json"), string(parsed))
	stubs := map[string]string{
		"git": "#!/bin/bash\n" +
			"for a in \"$@\"; do [ \"$a\" = rev-parse ] && { echo " + stageSourceSHA + "; exit 0; }; done\n" +
			"exit 99\n",
		"go": "#!/bin/bash\n" +
			"case \"$1 $2\" in\n" +
			"  'mod edit') cat " + shq(t, filepath.Join(root, "parsed.json")) + "; exit 0 ;;\n" +
			"  'mod download') : > " + shq(t, reached) + "; exit 0 ;;\n" +
			"esac\n" +
			"exit 99\n",
	}
	for name, body := range stubs {
		putFile(t, filepath.Join(bin, name), body)
		if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", script, "--repo="+stageCacheRepo, "--sha="+stageSourceSHA)
	cmd.Env = []string{
		"PATH=" + bin + string(os.PathListSeparator) + "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + root,
		"VR_CACHE_DIR=" + cache,
		"GOVCS=*:off",
		"GOPROXY=https://proxy.golang.org",
		"GOSUMDB=sum.golang.org",
		"GOTOOLCHAIN=local",
		"GOMODCACHE=" + filepath.Join(root, "mod"),
		"GOCACHE=" + filepath.Join(root, "gocache"),
	}
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("start %s: %v", script, err)
	}
	_, rerr := os.Stat(reached)
	return code, stdout.String() + stderr.String(), rerr == nil
}

func requireStageCacheHost(t *testing.T) {
	t.Helper()
	requireStageBash(t)
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("SKIP (could-not-check, not a pass): no python3 on this host")
	}
}

// The cluster lane has no public-repository proof. Its owner refusal remains
// in force even for public Assay modules; only the host bracket can prove them.
var (
	stageCacheAdmitted = []string{"github.com/example-org/public-lib"}
	stageCacheRefused  = []string{
		"github.com/medici-finance/assay/tools/desk",
		"github.com/medici-finance/assay-other/x",
		"github.com/medici-finance/assayx/y",
		"github.com/Medici-Finance/assay/tools/desk",
		"github.com/medici-finance/assay/../assay-other/x",
		"github.com/medici-finance/assay/./x",
		"github.com/medici-finance/assay//x",
		"github.com/medici-finance/assay/.x",
	}
)

// stageCacheFailures runs every case through script and returns one line per
// case it decided wrongly.
func stageCacheFailures(t *testing.T, script string) []string {
	t.Helper()
	var bad []string
	for _, p := range stageCacheAdmitted {
		code, out, reached := stageCacheRun(t, script, p)
		if code != 0 || !reached || !strings.Contains(out, "stage-done "+stageSourceSHA) {
			bad = append(bad, p+": refused: exit "+strconv.Itoa(code)+" "+strconv.Quote(out))
		}
	}
	for _, p := range stageCacheRefused {
		code, out, reached := stageCacheRun(t, script, p)
		if code != 3 || reached || !strings.Contains(out, "outside the approved public set") {
			bad = append(bad, p+": admitted: exit "+strconv.Itoa(code)+" "+strconv.Quote(out))
		}
	}
	return bad
}

// regression: #4683
// TestStageCachePublicOwned: without host-side identity/visibility proof,
// the cluster stager refuses every owner path before download.
func TestStageCachePublicOwned(t *testing.T) {
	requireStageCacheHost(t)
	if bad := stageCacheFailures(t, stageCacheEntrypoint); len(bad) != 0 {
		t.Fatalf("stage-cache.sh decided %d cases wrongly:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}

// regression: #4683
// TestStageCachePublicOwnedMutants: dropping the owner refusal or inserting
// an unproven public-prefix bypass makes the real script admit refused paths.
func TestStageCachePublicOwnedMutants(t *testing.T) {
	requireStageCacheHost(t)
	raw, err := os.ReadFile(stageCacheEntrypoint)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	const owner = `case $lc in "github.com/$OWNER_LC/"*) BAD="$BAD $p"; continue ;; esac`
	const anchor = `  lc=$(printf '%s' "$p" | tr '[:upper:]' '[:lower:]')`
	var publicPaths []string
	for _, p := range stageCacheRefused {
		if strings.HasPrefix(p, "github.com/medici-finance/assay/") {
			publicPaths = append(publicPaths, p)
		}
	}
	mutants := []struct {
		name, before, after string
		want                []string
	}{
		{"owner-refusal-dropped", owner, ":", stageCacheRefused},
		{"unproven-public-prefix", anchor, `  case $p in github.com/medici-finance/assay/*) continue ;; esac` + "\n" + anchor, publicPaths},
	}
	for _, m := range mutants {
		if n := strings.Count(src, m.before); n != 1 {
			t.Fatalf("%s: mutation anchor found %d times, want 1", m.name, n)
		}
		script := filepath.Join(t.TempDir(), "stage-cache.sh")
		putFile(t, script, strings.Replace(src, m.before, m.after, 1))
		bad := stageCacheFailures(t, script)
		if len(bad) != len(m.want) {
			t.Errorf("%s: %d cases failed, want exactly %v: %q", m.name, len(bad), m.want, bad)
			continue
		}
		for i, w := range m.want {
			if !strings.HasPrefix(bad[i], w+": ") {
				t.Errorf("%s: failure %d is %q, want case %s", m.name, i, bad[i], w)
			}
		}
	}
}
