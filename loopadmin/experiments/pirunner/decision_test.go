//go:build pirunner

package pirunner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The decision is derived, never asserted: decide() reads results.json against
// the vetted source manifest and the hashes of the subject files as they are
// in this tree. A claimed verdict is accepted only when it matches the
// derivation, except that UNKNOWN may understate a derived GO.

const (
	verdictGO      = "GO"
	verdictNoGo    = "NO-GO"
	verdictUnknown = "UNKNOWN"
)

// Groups and negative controls the brief makes mandatory.
var (
	mandatoryGroups   = []string{"TestPiPilotContract", "TestPiPilotRestart", "TestPiPilotEffects", "TestPiPilotAuthority", "TestPiPilotStopAndObserve", "TestPiPilotAccounting", "TestPiPilotDecisionEvidence", "TestPiPilotNegativeControls"}
	mandatoryControls = []string{"no-start-dedupe", "no-exec-reauth", "unstable-effect-id", "no-unknown-stop-hold", "unknown-cost-zeroed"}
	mandatoryCorpus   = []int{100, 1000}
	// subjectFiles bind a verdict to the code that produced it, relative to
	// loopadmin/. The decision test itself and its fixtures are the judge,
	// not the subject.
	subjectFiles = []string{
		"experiments/pirunner/adapter.go",
		"experiments/pirunner/bridge.go",
		"experiments/pirunner/ledger.go",
		"experiments/pirunner/helpers_test.go",
		"experiments/pirunner/contract_test.go",
		"experiments/pirunner/recovery_test.go",
		"experiments/pirunner/negative_test.go",
		"experiments/pirunner/node/src/bridge.ts",
		"experiments/pirunner/node/package.json",
		"experiments/pirunner/node/package-lock.json",
		"experiments/pirunner/node/tsconfig.json",
		"testdata/pirunner/negative-controls.json",
	}
)

type Tuple struct {
	PiDurable  string `json:"pi_durable"`
	PiAI       string `json:"pi_ai"`
	Chord      string `json:"chord"`
	TypeScript string `json:"typescript"`
	Node       string `json:"node"`
	NodeSHA256 string `json:"node_sha256"`
	LockSHA256 string `json:"lock_sha256"`
}

type Manifest struct {
	Tuple     Tuple             `json:"tuple"`
	VettedBy  string            `json:"vetted_by"`
	Integrity map[string]string `json:"lock_integrity"`
}

type Case struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Group struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Exit    *int   `json:"exit"`
	Status  string `json:"status"`
	Cases   []Case `json:"cases"`
}

type Control struct {
	Name    string `json:"name"`
	Verdict string `json:"verdict"`
}

type Measure struct {
	ToolResults int   `json:"tool_results"`
	Entries     int   `json:"entries"`
	InputBytes  int   `json:"input_bytes"`
	LogBytes    int64 `json:"log_bytes"`
	CommitCount *int  `json:"commit_count"`
	ReopenMS    int   `json:"reopen_ms"`
	PeakRSS     int64 `json:"peak_rss_bytes"`
	PeakCovered bool  `json:"peak_continuous"`
}

type Deletion struct {
	Candidate   string   `json:"candidate"`
	Paths       []string `json:"paths"`
	Disposition string   `json:"disposition"`
	Offset      bool     `json:"offset_by_replacement"`
}

type Env struct {
	OS            string `json:"os"`
	NetworkDenied bool   `json:"network_denied"`
	Approved      bool   `json:"approved_environment"`
}

type Results struct {
	Verdict       string            `json:"verdict"`
	Reasons       []string          `json:"reasons"`
	Tuple         Tuple             `json:"tuple"`
	SubjectHashes map[string]string `json:"subject_hashes"`
	Environment   Env               `json:"environment"`
	Groups        []Group           `json:"groups"`
	Controls      []Control         `json:"negative_controls"`
	Measurements  []Measure         `json:"measurements"`
	Deletions     []Deletion        `json:"deletions"`
	Adverse       []string          `json:"adverse_findings"`
}

// decide derives the verdict and its reason codes. Adverse findings are a
// NO-GO; any missing or unproven evidence is UNKNOWN; GO needs none of either.
func decide(r Results, m Manifest, hashes map[string]string, cases map[string][]string) (string, []string) {
	var why []string
	add := func(f string, a ...any) { why = append(why, fmt.Sprintf(f, a...)) }
	if len(r.Adverse) > 0 {
		return verdictNoGo, append([]string(nil), r.Adverse...)
	}
	precise, net := 0, 0
	for _, d := range r.Deletions {
		if (d.Disposition == "omit" || d.Disposition == "remove") && len(d.Paths) > 0 {
			precise++
			if !d.Offset {
				net++
			}
		}
	}
	if precise > 0 && net == 0 {
		return verdictNoGo, []string{"simplification-offset"}
	}
	if r.Tuple != m.Tuple {
		add("tuple-mismatch")
	}
	if m.VettedBy == "" || strings.HasPrefix(m.VettedBy, "pending") {
		add("tuple-not-vetted")
	}
	if !reflect.DeepEqual(r.SubjectHashes, hashes) {
		add("subject-hash-mismatch")
	}
	if !(r.Environment.Approved && r.Environment.NetworkDenied && r.Environment.OS == "linux") {
		add("environment-not-approved")
	}
	byName := map[string]Group{}
	for _, g := range r.Groups {
		byName[g.Name] = g
	}
	for _, name := range mandatoryGroups {
		g, ok := byName[name]
		if !ok {
			add("group-missing:%s", name)
			continue
		}
		if g.Status != "pass" || g.Exit == nil || *g.Exit != 0 || g.Command == "" {
			add("group-not-passed:%s", name)
			continue
		}
		var got []string
		for _, c := range g.Cases {
			if c.Status != "pass" {
				add("case-not-passed:%s/%s", name, c.Name)
			}
			got = append(got, c.Name)
		}
		if want := cases[name]; !sameSet(got, want) {
			add("cases-incomplete:%s", name)
		}
	}
	ctl := map[string]string{}
	for _, c := range r.Controls {
		ctl[c.Name] = c.Verdict
	}
	for _, name := range mandatoryControls {
		if ctl[name] != "killed" {
			add("control-not-killed:%s", name)
		}
	}
	size := map[int]Measure{}
	for _, ms := range r.Measurements {
		size[ms.ToolResults] = ms
	}
	partial := false
	for _, n := range mandatoryCorpus {
		ms, ok := size[n]
		if !ok || ms.Entries < n || ms.InputBytes == 0 || ms.LogBytes == 0 || ms.ReopenMS == 0 {
			add("measurement-missing:%d", n)
			continue
		}
		if ms.CommitCount == nil || !ms.PeakCovered {
			partial = true
		}
	}
	if partial {
		add("measurement-coverage-partial")
	}
	if precise == 0 {
		add("no-precise-deletion")
	}
	if len(why) > 0 {
		return verdictUnknown, why
	}
	return verdictGO, nil
}

// accept reports whether a claimed verdict is truthful against the derived one.
func accept(claim, derived string) bool {
	return claim == derived || (claim == verdictUnknown && derived == verdictGO)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	return reflect.DeepEqual(a, b)
}

func readJSON(t *testing.T, path string, v any) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return raw
}

func sha(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

var (
	topFunc = regexp.MustCompile(`(?m)^func (TestPiPilot\w+)\(t \*testing\.T\) \{`)
	subRun  = regexp.MustCompile(`(?m)^\tt\.Run\("([^"]+)"`)
)

// sourceCases reads each mandatory group's first-level subtests from the test
// sources, so a results file cannot drop a case the code still runs.
func sourceCases(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	files, _ := filepath.Glob("*_test.go")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		locs := topFunc.FindAllStringSubmatchIndex(src, -1)
		for i, l := range locs {
			end := len(src)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			name := src[l[2]:l[3]]
			for _, m := range subRun.FindAllStringSubmatch(src[l[1]:end], -1) {
				out[name] = append(out[name], m[1])
			}
		}
	}
	return out
}

func subjectHashes(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range subjectFiles {
		out[f] = sha(t, filepath.Join("..", "..", filepath.FromSlash(f)))
	}
	return out
}

// patch sets dotted paths ("groups.0.status") in a JSON document.
func patch(t *testing.T, raw []byte, set map[string]any, v any) {
	t.Helper()
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	// Parents before children, and each value deep-copied, so a fixture's
	// "environment.os" refines the control's "environment" without leaking
	// into the next fixture.
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		di, dj := strings.Count(paths[i], "."), strings.Count(paths[j], ".")
		if di != dj {
			return di < dj
		}
		return paths[i] < paths[j]
	})
	for _, path := range paths {
		var val any
		b, _ := json.Marshal(set[path])
		_ = json.Unmarshal(b, &val)
		parts := strings.Split(path, ".")
		cur := doc
		for i, p := range parts {
			last := i == len(parts)-1
			switch c := cur.(type) {
			case map[string]any:
				if last {
					c[p] = val
				} else {
					cur = c[p]
				}
			case []any:
				var idx int
				if _, err := fmt.Sscanf(p, "%d", &idx); err != nil || idx >= len(c) {
					t.Fatalf("patch %s: bad index %s", path, p)
				}
				if last {
					c[idx] = val
				} else {
					cur = c[idx]
				}
			default:
				t.Fatalf("patch %s: cannot descend at %s", path, p)
			}
		}
	}
	b, _ := json.Marshal(doc)
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	Name     string         `json:"name"`
	Results  map[string]any `json:"results_patch"`
	Manifest map[string]any `json:"manifest_patch"`
	Claim    string         `json:"claim"`
	Accept   bool           `json:"accept"`
	Want     string         `json:"want"`
}

// TestPiPilotDecisionEvidence dereferences the published verdict against the
// locked tuple, the installed packages, the subject hashes and the test
// sources, then proves the derivation rejects fabricated GO claims.
func TestPiPilotDecisionEvidence(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "pirunner")
	var r Results
	var m Manifest
	rawR := readJSON(t, filepath.Join(dir, "results.json"), &r)
	rawM := readJSON(t, filepath.Join(dir, "source-manifest.json"), &m)
	hashes := subjectHashes(t)
	cases := sourceCases(t)

	t.Run("ManifestDereferences", func(t *testing.T) {
		if got := sha(t, filepath.Join("node", "package-lock.json")); got != m.Tuple.LockSHA256 {
			t.Fatalf("lockfile sha256 %s, manifest %s", got, m.Tuple.LockSHA256)
		}
		var lock struct {
			Packages map[string]struct {
				Version   string `json:"version"`
				Integrity string `json:"integrity"`
			} `json:"packages"`
		}
		readJSON(t, filepath.Join("node", "package-lock.json"), &lock)
		want := map[string]string{"@earendil-works/pi-durable": m.Tuple.PiDurable, "@earendil-works/pi-ai": m.Tuple.PiAI, "@earendil-works/chord": m.Tuple.Chord, "typescript": m.Tuple.TypeScript}
		for pkg, ver := range want {
			l := lock.Packages["node_modules/"+pkg]
			if l.Version != ver || l.Integrity == "" || l.Integrity != m.Integrity[pkg] {
				t.Errorf("%s: lock %s %s, manifest %s %s", pkg, l.Version, l.Integrity, ver, m.Integrity[pkg])
			}
			var inst struct {
				Version string `json:"version"`
			}
			readJSON(t, filepath.Join("node", "node_modules", filepath.FromSlash(pkg), "package.json"), &inst)
			if inst.Version != ver {
				t.Errorf("%s: installed %s, manifest %s", pkg, inst.Version, ver)
			}
		}
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(m.Tuple.NodeSHA256) || m.VettedBy == "" {
			t.Fatalf("manifest lacks the runtime checksum or vetting record")
		}
	})

	t.Run("CasesMatchSource", func(t *testing.T) {
		for _, g := range mandatoryGroups[:6] {
			if len(cases[g]) == 0 {
				t.Errorf("no subtests found for %s", g)
			}
		}
		var neg negMap
		readJSON(t, filepath.Join(dir, "negative-controls.json"), &neg)
		var names []string
		for _, mu := range neg.Mutations {
			names = append(names, mu.Name)
		}
		if !sameSet(names, mandatoryControls) {
			t.Fatalf("negative-control map %v, mandatory %v", names, mandatoryControls)
		}
	})

	t.Run("VerdictDerived", func(t *testing.T) {
		v, why := decide(r, m, hashes, cases)
		if !accept(r.Verdict, v) {
			t.Fatalf("published verdict %s, derived %s: %v", r.Verdict, v, why)
		}
		if !sameSet(r.Reasons, why) {
			t.Fatalf("published reasons %v\nderived reasons %v\nsubject hashes now: %v", r.Reasons, why, hashes)
		}
		report, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "reports", "agent-execution", "pi-runner-pilot.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(report), "Verdict: **"+r.Verdict+"**") {
			t.Fatalf("report does not state the machine verdict %s", r.Verdict)
		}
		for _, code := range why {
			if !strings.Contains(string(report), "`"+code+"`") {
				t.Errorf("report omits reason %s", code)
			}
		}
	})

	t.Run("FabricatedGORejected", func(t *testing.T) {
		var fx struct {
			Control  fixture   `json:"go_control"`
			Fixtures []fixture `json:"fixtures"`
		}
		readJSON(t, filepath.Join(dir, "cases.json"), &fx)
		if len(fx.Fixtures) < 8 {
			t.Fatalf("only %d adverse/unknown fixtures", len(fx.Fixtures))
		}
		// The positive control: the real results, completed by patches, must
		// derive GO; otherwise every rejection below proves nothing.
		ctlR, ctlM := fx.Control.Results, fx.Control.Manifest
		ctlR["subject_hashes"] = toAny(hashes)
		var gr Results
		var gm Manifest
		patch(t, rawR, ctlR, &gr)
		patch(t, rawM, ctlM, &gm)
		if v, why := decide(gr, gm, hashes, cases); v != verdictGO {
			t.Fatalf("positive GO control derived %s: %v", v, why)
		}
		for _, f := range fx.Fixtures {
			set := map[string]any{}
			for k, v := range ctlR {
				set[k] = v
			}
			for k, v := range f.Results {
				set[k] = v
			}
			mset := map[string]any{}
			for k, v := range ctlM {
				mset[k] = v
			}
			for k, v := range f.Manifest {
				mset[k] = v
			}
			var fr Results
			var fm Manifest
			patch(t, rawR, set, &fr)
			patch(t, rawM, mset, &fm)
			v, why := decide(fr, fm, hashes, cases)
			if v != f.Want {
				t.Errorf("%s: derived %s (%v), want %s", f.Name, v, why, f.Want)
			}
			if got := accept(f.Claim, v); got != f.Accept {
				t.Errorf("%s: claim %s accepted=%v, want %v", f.Name, f.Claim, got, f.Accept)
			}
		}
	})
}

func toAny(m map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
