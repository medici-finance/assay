package deskkit

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// CVE-2026-98432 stopgap (#6213), the two sites mptcp_test.go does not reach:
// Compose services (they hold no pod spec) and kustomize patches (they carry
// only what they change, and the rendered check needs a kustomize binary this
// test cannot assume). The value rule is the runtime's, godebugDisablesMPTCP.

var composeFile = regexp.MustCompile(`(^|/)(docker-)?compose[^/]*\.ya?ml$`)

// composeMptcpExempt lists Compose files whose services run no Go program.
var composeMptcpExempt = map[string]string{
	"containers/postgres/compose.yaml": "upstream PostgreSQL image (a C server); GODEBUG is read only by Go runtimes",
	"workflow/testdata/compose.yaml":   "disposable PostgreSQL fixture; GODEBUG is read only by Go runtimes",
}

// yamlDocs decodes every non-empty document of a YAML (or JSON) stream into a map.
// A stream that does not decode is an error: an unreadable file is never a pass.
func yamlDocs(raw []byte) ([]map[string]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var out []map[string]any
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
		if len(doc) != 0 {
			out = append(out, doc)
		}
	}
}

// composeMptcpProblems lists the services of one Compose document that do not
// leave MPTCP off, in either the map or the KEY=value list form of `environment`,
// and counts the services it read.
func composeMptcpProblems(doc map[string]any) (problems []string, services int) {
	svcs, _ := doc["services"].(map[string]any)
	names := make([]string, 0, len(svcs))
	for n := range svcs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		svc, _ := svcs[n].(map[string]any)
		services++
		var values []string
		switch env := svc["environment"].(type) {
		case map[string]any:
			if v, ok := env["GODEBUG"]; ok {
				s, isString := v.(string)
				if !isString {
					s = "\x00not a string"
				}
				values = append(values, s)
			}
		case []any:
			for _, e := range env {
				if s, _ := e.(string); strings.HasPrefix(s, "GODEBUG=") {
					values = append(values, strings.TrimPrefix(s, "GODEBUG="))
				}
			}
		}
		if len(values) != 1 || !godebugDisablesMPTCP(values[0]) {
			problems = append(problems, "service "+n+": environment needs exactly one GODEBUG="+mptcpOff)
		}
	}
	return problems, services
}

// patchMptcpProblems lists what a kustomize patch does to GODEBUG. A patch may
// leave the base entry alone; it may not carry one that is anything but the
// literal setting, and it may not carry a strategic-merge directive ($patch,
// $retainKeys, ...) that could delete or replace the base entry unseen.
func patchMptcpProblems(node any, where string) []string {
	var out []string
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.HasPrefix(k, "$") {
				out = append(out, where+": patch directive "+k+" can drop the base GODEBUG entry unseen")
			}
			out = append(out, patchMptcpProblems(v[k], where)...)
		}
		if v["name"] == "GODEBUG" {
			if s, ok := v["value"].(string); !ok || s != mptcpOff || len(v) != 2 {
				out = append(out, where+": a GODEBUG entry in a patch must be exactly name+value "+mptcpOff)
			}
		}
	case []any:
		for _, item := range v {
			out = append(out, patchMptcpProblems(item, where)...)
		}
	}
	return out
}

// regression: #6213
func TestMptcpOffOnEveryComposeSvc(t *testing.T) {
	const svc = "services:\n  a:\n    image: i\n"
	for _, c := range []struct {
		name, text string
		bad        bool
	}{
		{"no environment", svc, true},
		{"enabled", svc + "    environment:\n      GODEBUG: multipathtcp=1\n", true},
		{"other setting only", svc + "    environment:\n      GODEBUG: http2debug=1\n", true},
		{"list form twice", svc + "    environment: [\"GODEBUG=multipathtcp=0\", \"GODEBUG=multipathtcp=1\"]\n", true},
		{"space after comma", svc + "    environment:\n      GODEBUG: \"multipathtcp=1, multipathtcp=0\"\n", true},
		{"trailing space", svc + "    environment:\n      GODEBUG: \"multipathtcp=0 \"\n", true},
		{"not a string", svc + "    environment:\n      GODEBUG: 0\n", true},
		{"own environment replaces the shared one", "x-c: &c\n  environment:\n    GODEBUG: multipathtcp=0\nservices:\n  a:\n    <<: *c\n    environment:\n      OTHER: x\n", true},
		{"map form", svc + "    environment:\n      GODEBUG: multipathtcp=0\n", false},
		{"list form", svc + "    environment: [\"GODEBUG=multipathtcp=0\"]\n", false},
		{"merged setting", svc + "    environment:\n      GODEBUG: http2debug=1,multipathtcp=0\n", false},
		{"inherited through a merge key", "x-c: &c\n  environment:\n    GODEBUG: multipathtcp=0\nservices:\n  a:\n    <<: *c\n", false},
	} {
		docs, err := yamlDocs([]byte(c.text))
		if err != nil || len(docs) != 1 {
			t.Fatalf("control %q does not parse: %v", c.name, err)
		}
		if problems, n := composeMptcpProblems(docs[0]); n != 1 || (len(problems) > 0) != c.bad {
			t.Errorf("control %q: services=%d problems=%v, want defect=%v", c.name, n, problems, c.bad)
		}
	}

	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/compose.yaml", "the Compose files ship only in a full checkout")
	manifests, _, _, _, _ := repoSweepInputs(t, fixtureRepoRoot)
	services, files := 0, map[string]bool{}
	seenExempt := map[string]bool{}
	for _, rel := range manifests {
		if !composeFile.MatchString(rel) || strings.HasSuffix(rel, ".json") {
			continue
		}
		if _, ok := composeMptcpExempt[rel]; ok {
			seenExempt[rel] = true
			continue
		}
		raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		docs, err := yamlDocs(raw)
		if err != nil {
			t.Errorf("%s: does not decode: %v", rel, err)
			continue
		}
		files[rel] = true
		for _, doc := range docs {
			problems, n := composeMptcpProblems(doc)
			services += n
			for _, p := range problems {
				t.Errorf("%s: %s", rel, p)
			}
		}
	}
	for rel := range composeMptcpExempt {
		if !seenExempt[rel] {
			t.Errorf("exemption %s names no Compose file in the tree: drop it", rel)
		}
	}
	if !files["containers/compose.yaml"] || services == 0 {
		t.Fatalf("the sweep did not reach containers/compose.yaml (files %v, services %d)", files, services)
	}
	t.Logf("%d Compose services in %d files, %d exempt", services, len(files), len(seenExempt))
}

// regression: #6213
func TestMptcpPatchesKeepBaseEntry(t *testing.T) {
	for _, c := range []struct {
		name, text string
		bad        bool
	}{
		{"patch without GODEBUG", `{"spec":{"containers":[{"name":"c","env":[{"name":"X","value":"1"}]}]}}`, false},
		{"literal setting", `{"env":[{"name":"GODEBUG","value":"multipathtcp=0"}]}`, false},
		{"enabled", `{"env":[{"name":"GODEBUG","value":"multipathtcp=1"}]}`, true},
		{"merged setting", `{"env":[{"name":"GODEBUG","value":"http2debug=1,multipathtcp=0"}]}`, true},
		{"valueFrom", `{"env":[{"name":"GODEBUG","valueFrom":{"secretKeyRef":{"name":"s","key":"k"}}}]}`, true},
		{"delete directive", `{"env":[{"name":"GODEBUG","$patch":"delete"}]}`, true},
		{"replace directive", `{"containers":[{"name":"c","$patch":"replace"}]}`, true},
		{"retainKeys", `{"env":[{"$retainKeys":["name"],"name":"X"}]}`, true},
	} {
		docs, err := yamlDocs([]byte(c.text))
		if err != nil || len(docs) != 1 {
			t.Fatalf("control %q does not parse: %v", c.name, err)
		}
		if got := patchMptcpProblems(docs[0], c.name); (len(got) > 0) != c.bad {
			t.Errorf("control %q: problems=%v, want defect=%v", c.name, got, c.bad)
		}
	}

	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/k8s", "the desk manifests ship only in a full checkout")
	_, _, _, _, patches := repoSweepInputs(t, fixtureRepoRoot)
	const compat = "containers/k8s/components/compatibility-cellctl/workloads.yaml"
	if !patches[compat] {
		t.Fatalf("the sweep did not find %s among the kustomize patches %v", compat, patches)
	}
	paths := make([]string, 0, len(patches))
	for p := range patches {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, rel))
		if err != nil {
			t.Errorf("%s: patch listed by a kustomization is unreadable: %v", rel, err)
			continue
		}
		docs, err := yamlDocs(raw)
		if err != nil {
			t.Errorf("%s: does not decode: %v", rel, err)
			continue
		}
		for _, doc := range docs {
			for _, p := range patchMptcpProblems(doc, rel) {
				t.Error(p)
			}
		}
	}
	t.Logf("%d kustomize patches swept", len(paths))
}
