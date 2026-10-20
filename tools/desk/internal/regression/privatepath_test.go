package regression

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// inheritedPathRE matches an assignment that puts a directory in front of the inherited PATH:
// `PATH="<dir>:$PATH"` or `PATH="<dir>:${PATH}"`, exported or as a one-command prefix.
var inheritedPathRE = regexp.MustCompile(`\bPATH="[^"]*:\$\{?PATH\}?"`)

// realCockpitSuites may prepend to the inherited PATH by hand, because the suite needs the
// host's REAL cockpit. Every entry names why.
var realCockpitSuites = map[string]string{
	"scrubbed-cell.test.sh": "proves private-socket isolation, which needs a real tmux server",
}

// inheritedPathFaults returns file:line for every line in src, outside comments, that prepends
// to the inherited PATH instead of calling private_path.
func inheritedPathFaults(name string, src []byte) []string {
	var faults []string
	for i, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if inheritedPathRE.MatchString(line) {
			faults = append(faults, fmt.Sprintf("%s:%d: prepends to the inherited PATH; use private_path (tools/cellctl/tests/private-path.sh)", name, i+1))
		}
	}
	return faults
}

// TestCellctlSuitesUsePrivatePath is the class guard for a cellctl suite whose verdict depends
// on the cockpits the host has installed: a suite that puts its stub directory in front of the
// inherited PATH without stubbing tmux and herdr lets the host decide `check`'s tmux row and the
// auto cockpit, so it passes on a workstation and fails on a clean runner. private_path is the
// one choke point that stubs both; a planted control keeps the matcher honest.
// regression: #1927
func TestCellctlSuitesUsePrivatePath(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "cellctl", "tests")
	files, err := filepath.Glob(filepath.Join(dir, "*.test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	callers := 0
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(file)
		if strings.Contains(string(src), `private_path "`) {
			callers++
		}
		if _, ok := realCockpitSuites[base]; ok {
			continue
		}
		if faults := inheritedPathFaults(base, src); len(faults) > 0 {
			t.Errorf("%s", strings.Join(faults, "\n"))
		}
	}
	if callers < 12 {
		t.Errorf("found %d suites calling private_path in %s, want at least 12", callers, dir)
	}
	planted := "#!/usr/bin/env bash\n" +
		"export PATH=\"$T/bin:$PATH\"\n" +
		"out=\"$(PATH=\"$T/x:${PATH}\" cellctl check c)\"\n" +
		"# export PATH=\"$T/bin:$PATH\" in a comment is not a site\n" +
		"private_path \"$T/bin\"\n" +
		"PATH=\"$T/bin-nocodex\" cellctl check c\n"
	got := strings.Join(inheritedPathFaults("plant.test.sh", []byte(planted)), "\n")
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, "plant.test.sh:2:") || !strings.Contains(got, "plant.test.sh:3:") {
		t.Errorf("planted controls: want exactly lines 2 and 3 flagged, got:\n%s", got)
	}
}
