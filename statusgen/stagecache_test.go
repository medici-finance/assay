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

// stageCacheAdmitted and stageCacheRefused are the stager's half of
// TestModEgressPublicOwned: the public Assay module passes under its own
// owner; neighbours, another case and non-canonical paths under the public
// prefix refuse with nothing downloaded.
var (
	stageCacheAdmitted = []string{"github.com/medici-finance/assay/tools/desk"}
	stageCacheRefused  = []string{
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
// TestStageCachePublicOwned: the stager admits the public Assay module under
// its own owner and refuses its neighbours, another case and non-canonical
// paths before any download.
func TestStageCachePublicOwned(t *testing.T) {
	requireStageCacheHost(t)
	if bad := stageCacheFailures(t, stageCacheEntrypoint); len(bad) != 0 {
		t.Fatalf("stage-cache.sh decided %d cases wrongly:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}

// regression: #4683
// TestStageCachePublicOwnedMutants: a stager with the canonical check dropped
// admits exactly the non-canonical cases, and one with no exemption refuses
// the public module, so the cases above can fail.
func TestStageCachePublicOwnedMutants(t *testing.T) {
	requireStageCacheHost(t)
	raw, err := os.ReadFile(stageCacheEntrypoint)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	const exempt = `if printf '%s' "$p" | grep -q -E -e "$OWNPUB" && printf '%s' "$p" | grep -q -E -e "$CANON"; then continue; fi`
	mutants := []struct {
		name, after string
		want        []string
	}{
		{"no-canon", `if printf '%s' "$p" | grep -q -E -e "$OWNPUB"; then continue; fi`, []string{
			"github.com/medici-finance/assay/../assay-other/x",
			"github.com/medici-finance/assay/./x",
			"github.com/medici-finance/assay//x",
			"github.com/medici-finance/assay/.x",
		}},
		{"no-exemption", `:`, []string{"github.com/medici-finance/assay/tools/desk"}},
	}
	if n := strings.Count(src, exempt); n != 1 {
		t.Fatalf("mutation anchor found %d times, want 1", n)
	}
	for _, m := range mutants {
		script := filepath.Join(t.TempDir(), "stage-cache.sh")
		putFile(t, script, strings.Replace(src, exempt, m.after, 1))
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
