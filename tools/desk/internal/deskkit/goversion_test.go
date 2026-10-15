package deskkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The fleet's single Go version (#2918). Every go.mod `go` directive names
// fleetGoDirective; every workflow GO_VERSION / literal go-version, every
// Dockerfile golang base tag and GO_VERSION ARG names fleetGoToolchain; every
// workflow Go tarball digest is the reviewed linux-amd64 row for that
// toolchain in knownToolTarballs; and every actions/upload-artifact step uses
// fleetUploadArtifact. A bump changes these constants and every site in the
// same change, so a site left behind goes red here rather than drifting.
const (
	fleetGoToolchain    = "1.27.2"
	fleetGoDirective    = "1.27.0"
	fleetUploadArtifact = "cf430e030ddbb5b0abf93d22962f4752f3646cd9" // v7.0.2
)

// goPinExceptions are sites allowed to disagree, keyed "<path>|<kind>". The
// list can only shrink: an exception whose site now agrees (or is gone) is
// itself a failure, so it is removed in the change that fixes the site.
var goPinExceptions = map[string]string{
	// The go-analysis toolchains are policy inputs to the reviewed baseline
	// contract digest; moving them is a separately reviewed migration with
	// before/after evidence (docs/go-analysis.md "Maintain the ratchet"),
	// deferred out of #2918.
	"tools/go-analysis/pins.env|BUILD_GO":    "baseline migration pending",
	"tools/go-analysis/pins.env|ANALYSIS_GO": "baseline migration pending",
	".golangci.yml|golangci go":              "baseline migration pending",
}

// goPinSkipFiles are whole files outside the guard, with the reason.
var goPinSkipFiles = map[string]string{
	// A historical whole-file snapshot of an applied rollout; its banner
	// forbids promoting it, so it records the past rather than a pin.
	"tools/changelog/release.yml.proposed": "historical snapshot",
}

var (
	goModGoRe     = regexp.MustCompile(`^go\s+(\S+)\s*$`)
	goModTcRe     = regexp.MustCompile(`^toolchain\s+go(\S+)\s*$`)
	ymlGoVerRe    = regexp.MustCompile(`^\s*GO_VERSION:\s*["']?([^"'\s#]+)`)
	ymlSetupGoRe  = regexp.MustCompile(`^\s*go-version:\s*["']?([^"'\s#]+)`)
	ymlGoShaRe    = regexp.MustCompile(`^\s*(?:GO_SHA256|GO_LINUX_AMD64_SHA256):\s*["']?([^"'\s#]+)`)
	uploadArtRe   = regexp.MustCompile(`uses:\s*actions/upload-artifact@([^\s#]+)`)
	fromGolangRe  = regexp.MustCompile(`^\s*FROM\s+(?:--platform=\S+\s+)?golang:([^\s@]+)`)
	argGoVerRe    = regexp.MustCompile(`^\s*ARG\s+GO_VERSION=(\S+)`)
	argGoShaRe    = regexp.MustCompile(`^\s*ARG\s+GO_SHA256_(AMD64|ARM64)=(\S+)`)
	golangciGoRe  = regexp.MustCompile(`^\s+go:\s*["']?([^"'\s#]+)`)
	analysisGoRe  = regexp.MustCompile(`^(BUILD_GO|ANALYSIS_GO)=(\S+)\s*$`)
	goVersionTagR = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*`)
)

// goPin is one version-bearing site and the value the fleet requires there.
type goPin struct {
	path, kind, got, want string
	line                  int
}

func (p goPin) ok() bool { return p.got == p.want }

// goPinsIn returns every version-bearing site in one file, keyed by the
// repo-relative slash path (which selects the file kind).
func goPinsIn(rel, body string) []goPin {
	goTb := knownToolTarballs["GO"][fleetGoToolchain]
	base := filepath.Base(rel)
	isYml := strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml") ||
		strings.HasSuffix(base, ".yml.proposed")
	var pins []goPin
	add := func(n int, kind, got, want string) {
		pins = append(pins, goPin{path: rel, line: n, kind: kind, got: got, want: want})
	}
	for i, line := range strings.Split(strings.ReplaceAll(body, "\r", ""), "\n") {
		n := i + 1
		switch {
		case base == "go.mod":
			if m := goModGoRe.FindStringSubmatch(line); m != nil {
				add(n, "go.mod go", m[1], fleetGoDirective)
			}
			if m := goModTcRe.FindStringSubmatch(line); m != nil {
				add(n, "go.mod toolchain", m[1], fleetGoToolchain)
			}
		case strings.HasPrefix(base, "Dockerfile"):
			if m := fromGolangRe.FindStringSubmatch(line); m != nil {
				add(n, "FROM golang", goVersionTagR.FindString(m[1]), fleetGoToolchain)
			}
			if m := argGoVerRe.FindStringSubmatch(line); m != nil {
				add(n, "ARG GO_VERSION", m[1], fleetGoToolchain)
			}
			if m := argGoShaRe.FindStringSubmatch(line); m != nil {
				want := goTb.amd64
				if m[1] == "ARM64" {
					want = goTb.arm64
				}
				add(n, "ARG GO_SHA256", m[2], want)
			}
		case base == "pins.env":
			if m := analysisGoRe.FindStringSubmatch(line); m != nil {
				add(n, m[1], m[2], fleetGoToolchain)
			}
		case isYml:
			if base == ".golangci.yml" || base == ".golangci.yaml" {
				if m := golangciGoRe.FindStringSubmatch(line); m != nil {
					add(n, "golangci go", m[1], fleetGoDirective)
				}
				continue
			}
			if m := ymlGoVerRe.FindStringSubmatch(line); m != nil && !strings.HasPrefix(m[1], "$") {
				add(n, "GO_VERSION", m[1], fleetGoToolchain)
			}
			if m := ymlSetupGoRe.FindStringSubmatch(line); m != nil && !strings.HasPrefix(m[1], "$") {
				add(n, "go-version", m[1], fleetGoToolchain)
			}
			if m := ymlGoShaRe.FindStringSubmatch(line); m != nil && !strings.HasPrefix(m[1], "$") {
				add(n, "Go sha256", m[1], goTb.amd64)
			}
			if m := uploadArtRe.FindStringSubmatch(line); m != nil {
				add(n, "upload-artifact", m[1], fleetUploadArtifact)
			}
		}
	}
	return pins
}

// fleetGoPins walks root and returns every version-bearing site outside
// goPinSkipFiles.
func fleetGoPins(root string) ([]goPin, error) {
	var pins []goPin
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "testdata", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, skip := goPinSkipFiles[rel]; skip {
			return nil
		}
		base := d.Name()
		if base != "go.mod" && base != "pins.env" && !strings.HasPrefix(base, "Dockerfile") &&
			!strings.HasSuffix(base, ".yml") && !strings.HasSuffix(base, ".yaml") &&
			!strings.HasSuffix(base, ".yml.proposed") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pins = append(pins, goPinsIn(rel, string(b))...)
		return nil
	})
	return pins, err
}

// goPinProblems turns the sites into findings: every disagreeing site not
// excused, and every exception that no longer excuses a disagreeing site.
func goPinProblems(pins []goPin, exceptions map[string]string) []string {
	var out []string
	used := map[string]bool{}
	for _, p := range pins {
		if p.ok() {
			continue
		}
		key := p.path + "|" + p.kind
		if _, ok := exceptions[key]; ok {
			used[key] = true
			continue
		}
		out = append(out, p.path+":"+strconv.Itoa(p.line)+": "+p.kind+" "+p.got+", want "+p.want)
	}
	for key := range exceptions {
		if !used[key] {
			out = append(out, "stale exception "+key+": the site agrees or is gone; remove it")
		}
	}
	sort.Strings(out)
	return out
}

// regression: #2918
func TestGoSingleVersion(t *testing.T) {
	if _, ok := knownToolTarballs["GO"][fleetGoToolchain]; !ok {
		t.Fatalf("fleetGoToolchain %s has no reviewed tarball row in knownToolTarballs", fleetGoToolchain)
	}
	if mm := goVersionTagR.FindString(fleetGoDirective); !strings.HasPrefix(fleetGoToolchain+".", strings.Join(strings.Split(mm, ".")[:2], ".")+".") {
		t.Fatalf("fleetGoDirective %s and fleetGoToolchain %s name different minor releases", fleetGoDirective, fleetGoToolchain)
	}
	pins, err := fleetGoPins(fixtureRepoRoot)
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, p := range pins {
		count[p.kind]++
	}
	need := []string{"go.mod go"}
	if isFullCheckout(fixtureRepoRoot) {
		need = append(need, "GO_VERSION", "Go sha256", "upload-artifact", "FROM golang")
	}
	for _, k := range need {
		if count[k] == 0 {
			t.Errorf("vacuous: no %s site found under %s", k, fixtureRepoRoot)
		}
	}
	for _, p := range goPinProblems(pins, goPinExceptions) {
		t.Error(p)
	}
}

// regression: #2918
func TestGoSingleVersionControls(t *testing.T) {
	sha := knownToolTarballs["GO"][fleetGoToolchain].amd64
	good := map[string]string{
		"m/go.mod": "module x\n\ngo " + fleetGoDirective + "\n",
		".github/workflows/a.yml": "env:\n  GO_VERSION: \"" + fleetGoToolchain + "\"\n" +
			"  GO_SHA256: \"" + sha + "\"\n" +
			"      - uses: actions/upload-artifact@" + fleetUploadArtifact + " # v7.0.2\n" +
			"          go-version: ${{ env.GO_VERSION }}\n",
		"Dockerfile": "FROM golang:" + fleetGoToolchain + "-alpine AS b\nARG GO_VERSION=" + fleetGoToolchain + "\n",
	}
	for rel, body := range good {
		pins := goPinsIn(rel, body)
		if len(pins) == 0 {
			t.Errorf("%s: no site parsed from a good fixture", rel)
		}
		if p := goPinProblems(pins, nil); len(p) != 0 {
			t.Errorf("%s: good fixture flagged: %v", rel, p)
		}
	}
	bad := map[string]string{
		"m/go.mod":         "module x\n\ngo 1.25.0\n",
		"n/go.mod":         "module x\n\ngo " + fleetGoDirective + "\n\ntoolchain go1.27.1\n",
		"w/a.yml":          "env:\r\n  GO_VERSION: \"1.25.0\"\r\n",
		"w/b.yaml":         "        with:\n          go-version: '1.27.1'\n",
		"w/c.yml":          "  GO_LINUX_AMD64_SHA256: \"2852af0cb20a\"\n",
		"w/d.yml.proposed": "      uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0\n",
		"i/Dockerfile":     "FROM --platform=$BUILDPLATFORM golang:1.26-bookworm@sha256:00 AS g\n",
		"j/Dockerfile":     "ARG GO_VERSION=1.25.0\n",
		"k/Dockerfile.ci":  "ARG GO_SHA256_ARM64=" + sha + "\n",
		"tools/x/pins.env": "ANALYSIS_GO=1.25.0\n",
		"r/.golangci.yml":  "run:\n  go: \"1.25.0\"\n",
	}
	for rel, body := range bad {
		if p := goPinProblems(goPinsIn(rel, body), nil); len(p) != 1 {
			t.Errorf("%s: planted disagreement gave %d findings, want 1: %v", rel, len(p), p)
		}
	}
	stale := goPinProblems(goPinsIn("m/go.mod", good["m/go.mod"]), map[string]string{"m/go.mod|go.mod go": "x"})
	if len(stale) != 1 || !strings.HasPrefix(stale[0], "stale exception") {
		t.Errorf("an exception over an agreeing site must go red, got %v", stale)
	}
	excused := goPinProblems(goPinsIn("m/go.mod", bad["m/go.mod"]), map[string]string{"m/go.mod|go.mod go": "x"})
	if len(excused) != 0 {
		t.Errorf("an excused site must stay green, got %v", excused)
	}
}
