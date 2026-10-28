package regression

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// The desk-base image ships every tools/desk/cmd binary, statusgen and bugs-gc in
// /usr/local/bin, and the release fails when containers/scripts/layer-secret-scan.sh
// finds key-shaped text in any of them. The Go linker packs unrelated string literals
// back to back, so a literal ending in an "sk-" run (desk-, task-, risk-) followed by
// alphanumeric neighbours reads as a generic model key: in v1.0.53 the flag literal
// perf-desk-role was packed before the runtime's userArenaStateGC. The tests below
// build that binary set the way the release does and apply the scanner's own patterns
// and exact exceptions, so a packing hit fails pull-request CI instead of the release.
// Nothing here restates a pattern: both are read from the script.

const releaseScanScript = "containers/scripts/layer-secret-scan.sh"

// releaseScan is the scanner's verdict for one file: its full pattern set and the
// exact "<image path>:<whole match>" pairs its known_label_match function exempts.
type releaseScan struct {
	pat    *regexp.Regexp
	exempt map[string]bool
}

var (
	scanSingleRE = regexp.MustCompile(`^([A-Z_]+)='([^']*)'$`)
	scanDoubleRE = regexp.MustCompile(`^([A-Z_]+)="((?:[^"$\x60\\]|\$\{[A-Z_]+\})*)"$`)
	scanRefRE    = regexp.MustCompile(`\$\{([A-Z_]+)\}`)
	scanCaseRE   = regexp.MustCompile(`^(/[^:\s]+):((?:"[^"]*"|[A-Za-z0-9_-])+)\s*(?:\|\s*\\|\)\s*return 0\s*;;)$`)
)

// loadReleaseScan parses the scanner script. It fails when PAT, LOOSE or
// MODELKEY_GENERIC is missing or empty, when PAT does not carry every LOOSE
// alternative, or when a known_label_match case line does not parse.
func loadReleaseScan(src string) (*releaseScan, error) {
	vars := map[string]string{}
	s := &releaseScan{exempt: map[string]bool{}}
	inCases := false
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(raw, "known_label_match()"):
			inCases = true
			continue
		case inCases && raw == "}":
			inCases = false
			continue
		case inCases:
			if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "/") {
				continue
			}
			m := scanCaseRE.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("known_label_match case does not parse: %q", line)
			}
			s.exempt[m[1]+":"+strings.ReplaceAll(m[2], `"`, "")] = true
			continue
		}
		if m := scanSingleRE.FindStringSubmatch(raw); m != nil {
			vars[m[1]] = m[2]
			continue
		}
		if m := scanDoubleRE.FindStringSubmatch(raw); m != nil {
			var missing string
			vars[m[1]] = scanRefRE.ReplaceAllStringFunc(m[2], func(ref string) string {
				name := scanRefRE.FindStringSubmatch(ref)[1]
				v, ok := vars[name]
				if !ok {
					missing = name
				}
				return v
			})
			if missing != "" {
				return nil, fmt.Errorf("%s refers to undefined %s", m[1], missing)
			}
		}
	}
	for _, name := range []string{"MODELKEY_GENERIC", "LOOSE", "PAT"} {
		if vars[name] == "" {
			return nil, fmt.Errorf("cannot read %s from the scanner", name)
		}
	}
	alts := map[string]bool{}
	for _, a := range strings.Split(vars["PAT"], "|") {
		alts[a] = true
	}
	for _, a := range strings.Split(vars["LOOSE"], "|") {
		if !alts[a] {
			return nil, fmt.Errorf("PAT does not carry the LOOSE alternative %q", a)
		}
	}
	if !alts[vars["MODELKEY_GENERIC"]] {
		return nil, fmt.Errorf("PAT does not carry MODELKEY_GENERIC")
	}
	// The scanner reports with grep -aoE: POSIX leftmost-longest, as CompilePOSIX.
	pat, err := regexp.CompilePOSIX(vars["PAT"])
	if err != nil {
		return nil, fmt.Errorf("scanner PAT: %v", err)
	}
	s.pat = pat
	return s, nil
}

// hits returns one line per match the scanner would fail on in a file at imagePath.
// A hit names its offset, the 12 bytes before it and a masked sample, never the value.
func (s *releaseScan) hits(imagePath string, data []byte) []string {
	var out []string
	for _, loc := range s.pat.FindAllIndex(data, -1) {
		match := string(data[loc[0]:loc[1]])
		if s.exempt[imagePath+":"+match] {
			continue
		}
		before := data[max(0, loc[0]-12):loc[0]]
		out = append(out, fmt.Sprintf("%s offset %d: after %q, %.4s… (%d chars)", imagePath, loc[0], before, match, len(match)))
	}
	return out
}

func readReleaseScan(t *testing.T, root string) *releaseScan {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(releaseScanScript)))
	if err != nil {
		t.Fatal(err)
	}
	s, err := loadReleaseScan(string(src))
	if err != nil {
		t.Fatalf("%s: %v", releaseScanScript, err)
	}
	return s
}

// TestReleaseScanPlantedHits is the positive control: the matcher must flag the
// v1.0.53 packing, keep scanning past an exempt match, and refuse a scanner it
// cannot read.
// regression: medici-finance/assay#3203
func TestReleaseScanPlantedHits(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	s := readReleaseScan(t, root)
	// Split so this source is not itself a key-shaped run.
	packed := "\x00runtime.perf-de" + "sk-role" + "userArenaStateGC\x00"
	if got := s.hits("/usr/local/bin/deskboot", []byte(packed)); len(got) != 1 {
		t.Errorf("v1.0.53 packing: want 1 hit, got %q", got)
	}
	if len(s.exempt) == 0 {
		t.Log("scanner has no known_label_match cases; exemption controls skipped")
	}
	for pair := range s.exempt {
		path, match, _ := strings.Cut(pair, ":")
		if got := s.hits(path, []byte("\x00"+match+"\x00")); len(got) != 0 {
			t.Errorf("exact exempt pair %s flagged: %q", path, got)
		}
		if got := s.hits("/usr/local/bin/other", []byte("\x00"+match+"\x00")); len(got) != 1 {
			t.Errorf("exempt match at another path: want 1 hit, got %q", got)
		}
		if got := s.hits(path, []byte("\x00"+match+"x\x00")); len(got) != 1 {
			t.Errorf("extended exempt match: want 1 hit, got %q", got)
		}
		if got := s.hits(path, []byte("\x00"+match+"\x00"+packed)); len(got) != 1 {
			t.Errorf("match after an exempt one: want 1 hit, got %q", got)
		}
		break
	}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(releaseScanScript)))
	if err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"no LOOSE":            regexp.MustCompile(`(?m)^LOOSE=.*$`).ReplaceAllString(string(src), ""),
		"no MODELKEY_GENERIC": regexp.MustCompile(`(?m)^MODELKEY_GENERIC=.*$`).ReplaceAllString(string(src), ""),
		"bad case line":       strings.Replace(string(src), `known_label_match() {`, "known_label_match() {\n    /usr/local/bin/x:$(echo y) | \\", 1),
	} {
		if _, err := loadReleaseScan(bad); err == nil {
			t.Errorf("%s: scanner parsed without error", name)
		}
	}
}

// TestReleaseBinariesScanClean builds the desk-base binary set for linux/amd64 with
// the release's flags and fails on any match the scanner would fail the image on.
// regression: medici-finance/assay#3203
func TestReleaseBinariesScanClean(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the release binary set")
	}
	if runtime.GOOS == "windows" {
		t.Skip("release images are built and scanned on Linux; this runs in build-test")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	s := readReleaseScan(t, root)
	out := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// The release workflow's desk-tools, statusgen and bugs-gc builds, with fixed
	// stamp values of the release's shape (short sha, UTC timestamp, tag).
	desk := "github.com/medici-finance/assay/tools/desk/internal/deskkit"
	ldflags := "-X " + desk + ".SourceSHA=0123456789ab -X " + desk + ".BuiltAt=2026-01-01T00:00:00Z" +
		" -X " + desk + ".ReleaseTag=v0.0.0 -X main.cellctlVersion=v0.0.0"
	builds := []struct{ dir, out string }{
		{"tools/desk", out + string(filepath.Separator)},
		{"statusgen", filepath.Join(out, "statusgen")},
		{"tools/bugs-gc", filepath.Join(out, "bugs-gc")},
	}
	args := [][]string{
		{"build", "-ldflags", ldflags, "-o", builds[0].out, "./cmd/..."},
		{"build", "-ldflags", "-X main.statusgenVersion=v0.0.0", "-o", builds[1].out, "."},
		{"build", "-trimpath", "-o", builds[2].out, "."},
	}
	for i, b := range builds {
		cmd := exec.CommandContext(ctx, "go", args[i]...)
		cmd.Dir = filepath.Join(root, filepath.FromSlash(b.dir))
		cmd.Env = FixtureEnv("CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
		cmd.WaitDelay = time.Second
		if o, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", b.dir, err, o)
		}
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 3 {
		t.Fatalf("built %d binaries, want the whole set", len(entries))
	}
	var (
		mu    sync.Mutex
		found []string
		wg    sync.WaitGroup
		slots = make(chan struct{}, runtime.NumCPU())
	)
	for _, e := range entries {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			data, err := os.ReadFile(filepath.Join(out, name))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				found = append(found, fmt.Sprintf("%s: could not read: %v", name, err))
				return
			}
			found = append(found, s.hits("/usr/local/bin/"+name, data)...)
		}(e.Name())
	}
	wg.Wait()
	sort.Strings(found)
	if len(found) > 0 {
		t.Errorf("%d release binaries scanned; %s would fail the image on:\n  %s\n"+
			"A string literal ending in an sk- run was packed next to alphanumeric data. Rename the literal; do not change the scanner.",
			len(entries), releaseScanScript, strings.Join(found, "\n  "))
		return
	}
	t.Logf("%d release binaries scanned clean", len(entries))
}
