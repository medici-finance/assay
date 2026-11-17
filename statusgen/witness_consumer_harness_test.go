//go:build witnessflow

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Preserve the invoking Go cache/toolchain profile for compilation only.
// Executed subject tests retain fixture authority and their own TestMain.
var consumerInvocationEnv = os.Environ()

const consumerDelivery = "3b2235af1af6dd42118cbf975eb4ac2b8e8ab409"
const consumerBase = "c288fd881e7fe5532614ad25a4610be7c6acf8a3"
const consumerLegacy = "0f2b208cebc30af31269e7482b8b9159b602867d"
const consumerProbe = "^TestV2(FoldConsumerFlow|ConsumerImplicitDeps|ConsumerFixtureWrite|ConsumerInvalidRecords|ConsumerLatestBad)$"

func consumerRequired(t *testing.T, key string) string {
	t.Helper()
	p := os.Getenv(key)
	if p == "" || !filepath.IsAbs(p) {
		t.Fatalf("%s must name an absolute path", key)
	}
	return p
}

func consumerCommand(t *testing.T, dir string, args ...string) ([]byte, error) {
	t.Helper()
	return consumerCommandEnv(t, dir, os.Environ(), args...)
}

func consumerCommandEnv(t *testing.T, dir string, environ []string, args ...string) ([]byte, error) {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		t.Fatal("consumer harness requires a bounded test timeout")
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(-time.Second))
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(append([]string(nil), environ...), "GOWORK=off")
	cmd.WaitDelay = 2 * time.Second
	return cmd.CombinedOutput()
}

func consumerGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := consumerCommand(t, root, append([]string{"git", "-C", root}, args...)...)
	if err != nil {
		t.Fatalf("source audit could not read Git: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func consumerBuildIdentity(t *testing.T, bin, source string) {
	t.Helper()
	out, err := consumerCommand(t, "", "go", "version", "-m", bin)
	if err != nil || !bytes.Contains(out, []byte("vcs.revision="+source+"\n")) || !bytes.Contains(out, []byte("vcs.modified=false\n")) {
		t.Fatalf("binary does not bind the clean subject %s: %v\n%s", source, err, out)
	}
	b, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	t.Logf("binary source=%s sha256=%s", source, hex.EncodeToString(sum[:]))
}

func consumerHistoryAudit(t *testing.T, root, bin string) {
	t.Helper()
	if got := consumerGit(t, root, "rev-parse", consumerDelivery+"^1"); got != consumerBase {
		t.Fatalf("historical first parent changed: %s", got)
	}
	objects := []struct{ path, blob string }{
		{"docs/streams/statusgen/brief-36-content-bound-witness-schema-v2-reader.md", "051ddc7b265a8d29681c8552b9721019b8a2afcd"},
		{"statusgen/autoflip.go", "5b7e7576d8f1ebfa0adf62465eb4ece8d7438fb3"},
		{"statusgen/reconcilefold.go", "580cf396ff70323b26071d02aada41bf5bf01ca5"},
	}
	for i, o := range objects {
		if got := consumerGit(t, root, "rev-parse", consumerDelivery+":"+o.path); got != o.blob {
			t.Fatalf("historical object changed: %s", o.path)
		}
		if i > 0 && consumerGit(t, root, "rev-parse", consumerBase+":"+o.path) != o.blob {
			t.Fatalf("original caller was not unchanged: %s", o.path)
		}
	}
	brief := consumerGit(t, root, "show", consumerDelivery+":"+objects[0].path)
	declaration := "statusgen/autoflip.go and statusgen/reconcilefold.go (two of the three evaluateCoverage call sites, none of which passes Dependencies today; the third, coverage.go, is routed on the line above): fixed-here"
	if !strings.Contains(brief, declaration) {
		t.Fatal("exact original combined declaration is absent")
	}
	for _, o := range objects[1:] {
		body := consumerGit(t, root, "show", consumerDelivery+":"+o.path)
		if !strings.Contains(body, "coverage := evaluateCoverage(root, streams, coverageOptions{})") {
			t.Fatalf("original consumer edge changed: %s", o.path)
		}
	}
	out, err := consumerCommand(t, root, bin, "--root", root, "--consumers", "--brief", "statusgen/36", "--delivery", consumerDelivery, "--base", consumerBase)
	exit, ok := err.(*exec.ExitError)
	summary := "summary: 7 corroborated, 1 disproved, 1 unchecked, 0 brief(s) claiming nothing"
	if !ok || exit.ExitCode() != 1 || !bytes.Contains(out, []byte(summary)) || !bytes.Contains(out, []byte("claims fixed-here but fails the diff-lookup check")) {
		t.Fatalf("strict historical report changed; not accepted as a pass: %v\n%s", err, out)
	}
	t.Logf("strict historical audit remains disproved (exit=1):\n%s", out)
}

// consumerSubject runs the exact frozen test bytes against the subject's real
// product code. Only test-only overlays differ in the ordinary arm. A legacy
// control changes exactly one caller's evaluator identifier and adds a helper
// that invokes the actual pre-v2 binary; it never edits source on disk.
func consumerSubject(t *testing.T, root, packet, legacy, caller string) ([]byte, error) {
	t.Helper()
	tmp := t.TempDir()
	replace := map[string]string{filepath.Join(root, "statusgen", "consumerflow_test.go"): packet}
	if caller != "" {
		path := filepath.Join(root, "statusgen", caller)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		old := []byte("coverage := evaluateCoverage(")
		if bytes.Count(body, old) != 1 {
			t.Fatalf("legacy control cannot select exactly one real caller in %s", caller)
		}
		mutated := bytes.Replace(body, old, []byte("coverage := legacyConsumerCoverage("), 1)
		target := filepath.Join(tmp, "caller.go")
		if err := os.WriteFile(target, mutated, 0600); err != nil {
			t.Fatal(err)
		}
		replace[path] = target
		shim := `package main
import("encoding/json";"os";"os/exec";"fmt")
func legacyConsumerCoverage(root string,streams []*Stream,opts coverageOptions)map[string]Coverage{
 out,err:=exec.Command(os.Getenv("SG_LEGACY_BIN"),"--root",root,"--coverage","--json").Output();if err!=nil{panic(fmt.Sprintf("legacy control read failed: %v %s",err,out))}
 var all []Coverage;if err:=json.Unmarshal(out,&all);err!=nil{panic(err)}
 result:=map[string]Coverage{};for _,c:=range all{result[c.BriefID]=c}
 c,ok:=result["wv/01"];if !ok||len(c.Claims)!=2||c.Released{panic("legacy control did not read and refuse the actual two v2 claims")}
 return result
}
`
		shimPath := filepath.Join(tmp, "legacy.go")
		if err := os.WriteFile(shimPath, []byte(shim), 0600); err != nil {
			t.Fatal(err)
		}
		replace[filepath.Join(root, "statusgen", "consumer_legacy_control.go")] = shimPath
	}
	raw, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlay, raw, 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(tmp, "subject.test")
	out, err := consumerCommandEnv(t, filepath.Join(root, "statusgen"), consumerInvocationEnv, "go", "test", "-c", "-overlay", overlay, "-o", binary, ".")
	if err != nil {
		t.Fatalf("behavioral subject did not compile; this is NOT semantic fail-first: %v\n%s", err, out)
	}
	selector := consumerProbe
	if caller != "" {
		selector = "^TestV2FoldConsumerFlow$"
	}
	cmdEnv := os.Getenv("SG_LEGACY_BIN")
	t.Setenv("SG_LEGACY_BIN", legacy)
	out, err = consumerCommand(t, filepath.Join(root, "statusgen"), binary, "-test.count=1", "-test.timeout=120s", "-test.v", "-test.run", selector)
	t.Setenv("SG_LEGACY_BIN", cmdEnv)
	return out, err
}

// regression: #5684
func TestWitnessConsumerFlow(t *testing.T) {
	root := consumerRequired(t, "SG_CONSUMER_ROOT")
	candidate := consumerRequired(t, "SG_NEW_BIN")
	legacy := consumerRequired(t, "SG_LEGACY_BIN")
	source := consumerGit(t, root, "rev-parse", "HEAD")
	if consumerGit(t, root, "status", "--porcelain") != "" {
		t.Fatal("consumer subject must be a clean immutable source")
	}
	consumerBuildIdentity(t, candidate, source)
	consumerBuildIdentity(t, legacy, consumerLegacy)
	// Qualify actual nonempty v2 behavior before compiling cross-revision tests:
	// an old source with no new fixture/type must fail on its real held claims.
	fixture := newGitV2Fixture(t).build()
	coverage := fixture.cliCoverage(candidate)
	if len(coverage) != 1 || !coverage[0].Released || len(coverage[0].Claims) != 2 {
		t.Fatalf("candidate did not release the actual two v2 claims: %+v", coverage)
	}
	for _, claim := range coverage[0].Claims {
		if !claim.Released || claim.Witness == nil || claim.Witness.Schema != 2 {
			t.Fatalf("candidate did not execute a real v2 record: %+v", claim)
		}
	}
	consumerHistoryAudit(t, root, candidate)
	helper, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	packet := filepath.Join(helper, "consumerflow_test.go")
	data, err := os.ReadFile(packet)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	t.Logf("subject=%s immutable behavioral packet sha256=%x", source, digest)
	out, err := consumerSubject(t, root, packet, legacy, "")
	t.Logf("actual caller behavior:\n%s", out)
	if err != nil {
		t.Fatalf("actual caller flow failed semantically: %v", err)
	}
	for _, name := range []string{"TestV2FoldConsumerFlow", "TestV2ConsumerImplicitDeps", "TestV2ConsumerFixtureWrite", "TestV2ConsumerInvalidRecords", "TestV2ConsumerLatestBad"} {
		if !bytes.Contains(out, []byte("--- PASS: "+name+" ")) {
			t.Fatalf("missing actual named PASS: %s", name)
		}
	}
	for _, caller := range []string{"autoflip.go", "reconcilefold.go"} {
		out, err := consumerSubject(t, root, packet, legacy, caller)
		reason := "actual fold did not consume released v2"
		if caller == "autoflip.go" {
			reason = "actual autoflip did not complete its decision"
		}
		if err == nil || !bytes.Contains(out, []byte("--- FAIL: TestV2FoldConsumerFlow ")) || !bytes.Contains(out, []byte(reason)) {
			t.Fatalf("legacy rewire did not fail at its real caller %s: %v\n%s", caller, err, out)
		}
		t.Logf("independent legacy rewire %s rejected by actual flow:\n%s", caller, out)
	}
	if got := consumerGit(t, root, "rev-parse", "HEAD"); got != source || consumerGit(t, root, "status", "--porcelain") != "" {
		t.Fatal("subject changed during behavioral proof")
	}
	fmt.Fprintln(os.Stdout, "current consumer flow proved; original historical declaration remains disproved")
}
