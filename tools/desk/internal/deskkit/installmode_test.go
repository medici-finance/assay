package deskkit

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// A COPY or ADD from the build context keeps the mode each file had in the
// checkout unless it names --chmod. A checkout made under umask 002 therefore
// ships group-writable files, and on PATH or in a shared directory that lets
// any member of the file's group replace what every user of the image runs.
// This sweep holds every image in the tree to an explicit, root-only-writable
// mode for every build-context install outside build scratch.

// installModeExempt names Dockerfiles the sweep skips, and why.
var installModeExempt = map[string]string{
	// Pinned on its own, with its own install-mode gate, by
	// verifyrunner_gitfloor_test.go.
	"images/verify-runner/Dockerfile": "held by the verify-runner floor test, which pins its modes and gate",
	// Qualification recipes: FROM scratch, never published, and their exact
	// bytes are part of the recorded qualification.
	"tools/desk/internal/forgeservice/auth/testdata/profile-probe/Dockerfile": "qualification recipe; never published, bytes are recorded",
	"tools/desk/internal/forgeservice/auth/testdata/runtime/Dockerfile":       "qualification recipe; never published, bytes are recorded",
}

// installScratchRoots are destinations that only feed a later build step in
// the same stage (source to compile, or scanner input); nothing under them is
// installed on PATH or shared with a runtime user.
var installScratchRoots = []string{"/src", "/tmp"}

// installModeGate is the build-time half: the final RUN of each image that has
// one, with the directories it must cover. The condition is the gate's exact
// `if` command; gateProblem requires it to be followed by `then … exit 1`.
var installModeGates = map[string]string{
	"containers/base/Dockerfile":      `if find /usr/local /opt/assay -xdev ! -type l \( -perm /022 -o ! -user 0 -o ! -group 0 \) -print | grep .`,
	"images/assay-harness/Dockerfile": `if find /usr/local/bin /opt/assay -xdev ! -type l \( -perm /022 -o ! -user 0 -o ! -group 0 \) -print | grep .`,
	"Dockerfile":                      `if find /usr/local/bin -xdev ! -type l \( -perm /022 -o ! -user 0 -o ! -group 0 \) -print | grep .`,
}

// chmodModeProblem returns why a --chmod value could leave a file writable by
// its group or others, or "" when it cannot. It accepts an octal mode and the
// symbolic form (`u=rwX,go=rX`), and refuses anything it cannot read.
func chmodModeProblem(mode string) string {
	if mode == "" {
		return "has an empty --chmod"
	}
	if strings.Trim(mode, "01234567") == "" {
		m, err := strconv.ParseUint(mode, 8, 32)
		if err != nil || m > 07777 {
			return "has an unreadable --chmod=" + mode
		}
		if m&022 != 0 {
			return "--chmod=" + mode + " leaves the file writable by its group or others"
		}
		return ""
	}
	for _, clause := range strings.Split(mode, ",") {
		who := strings.TrimLeft(clause, "ugoa")
		whoSet := clause[:len(clause)-len(who)]
		if who == "" || !strings.ContainsAny(who[:1], "+-=") {
			return "has an unreadable --chmod=" + mode
		}
		for _, r := range who[1:] {
			if !strings.ContainsRune("rwxXst+-=", r) {
				return "has an unreadable --chmod=" + mode
			}
		}
		if whoSet == "" || strings.ContainsAny(whoSet, "goa") {
			// The op that grants: a '+' or '=' run carrying w, up to the next op.
			op := byte(0)
			for i := 0; i < len(who); i++ {
				switch c := who[i]; c {
				case '+', '-', '=':
					op = c
				case 'w':
					if op != '-' {
						return "--chmod=" + mode + " leaves the file writable by its group or others"
					}
				}
			}
		}
	}
	return ""
}

// installModeProblems returns one line per build-context COPY or ADD in a
// Dockerfile whose destination is outside build scratch and which does not
// name a root-only-writable --chmod.
func installModeProblems(dockerfile string) []string {
	var out []string
	workdir := ""
	for _, in := range parseDockerfile(dockerfile) {
		switch in.op {
		case "FROM":
			// A stage inherits its base's WORKDIR, which this file cannot see:
			// a relative destination before a WORKDIR is treated as unknown.
			workdir = ""
		case "WORKDIR":
			if len(in.args) == 1 {
				if strings.HasPrefix(in.args[0], "/") || workdir == "" {
					workdir = path.Clean(in.args[0])
				} else {
					workdir = path.Join(workdir, in.args[0])
				}
			}
		case "COPY", "ADD":
			var flags, rest []string
			for _, a := range in.args {
				if strings.HasPrefix(a, "--") && len(rest) == 0 {
					flags = append(flags, a)
				} else {
					rest = append(rest, a)
				}
			}
			line := in.op + " " + strings.Join(in.args, " ")
			if len(rest) > 0 && strings.HasPrefix(rest[0], "[") {
				out = append(out, line+": JSON form is not modelled; write it in the plain form")
				continue
			}
			mode, fromStage := "", false
			for _, f := range flags {
				switch {
				case strings.HasPrefix(f, "--from="):
					fromStage = true
				case strings.HasPrefix(f, "--chmod="):
					mode = strings.TrimPrefix(f, "--chmod=")
				}
			}
			if fromStage || len(rest) < 2 {
				continue
			}
			dest := rest[len(rest)-1]
			if !strings.HasPrefix(dest, "/") {
				if workdir == "" {
					dest = ""
				} else {
					dest = path.Join(workdir, dest)
				}
			}
			if dest != "" && underAny(path.Clean(dest), installScratchRoots) {
				continue
			}
			hasChmod := false
			for _, f := range flags {
				hasChmod = hasChmod || strings.HasPrefix(f, "--chmod=")
			}
			switch {
			case !hasChmod:
				out = append(out, line+": installs from the build context without --chmod, so the checkout's mode ships")
			case chmodModeProblem(mode) != "":
				out = append(out, line+": "+chmodModeProblem(mode))
			}
		}
	}
	return out
}

func underAny(p string, roots []string) bool {
	for _, r := range roots {
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// installGateProblem returns why the final stage of a Dockerfile does not end
// in a fail-closed RUN carrying the install-mode gate cond, or "".
func installGateProblem(dockerfile, cond string) string {
	ins := parseDockerfile(dockerfile)
	lastFrom := -1
	for i, in := range ins {
		if in.op == "FROM" {
			lastFrom = i
		}
	}
	gate, lastLayer := -1, -1
	for i := lastFrom + 1; i < len(ins) && lastFrom >= 0; i++ {
		in := ins[i]
		if in.op == "RUN" || in.op == "COPY" || in.op == "ADD" {
			lastLayer = i
		}
		if in.op == "RUN" && segAt(in.segs, cond) >= 0 {
			gate = i
		}
	}
	switch {
	case gate < 0:
		return "the final stage has no RUN carrying the install-mode gate `" + cond + "`"
	case gate != lastLayer:
		return "a RUN, COPY or ADD follows the install-mode gate in the final stage, so its layer ships unchecked"
	}
	user := ""
	for _, in := range ins[lastFrom+1 : gate] {
		if in.op == "USER" {
			user = strings.Join(in.args, " ")
		}
	}
	if user != "" && user != "root" && user != "0" {
		return "the install-mode gate runs as USER " + user + ", so find cannot see every file"
	}
	if p := failClosedProblem(ins[gate].args); p != "" {
		return "the install-mode gate RUN " + p
	}
	return gateProblem(ins[gate].segs, cond, "install-mode")
}

// TestImageInstallModes sweeps every Dockerfile in the tree: no build-context
// install outside build scratch may rely on the checkout's mode.
//
// regression: #6788
func TestImageInstallModes(t *testing.T) {
	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/base/Dockerfile", "containers/ ships only in a full checkout")
	_, _, dockerfiles, _, _ := repoSweepInputs(t, fixtureRepoRoot)
	seen := map[string]bool{}
	swept := 0
	for _, rel := range dockerfiles {
		if strings.HasSuffix(rel, ".dockerignore") {
			continue
		}
		if _, ok := installModeExempt[rel]; ok {
			seen[rel] = true
			continue
		}
		raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		swept++
		for _, p := range installModeProblems(string(raw)) {
			t.Errorf("%s: %s", rel, p)
		}
	}
	if swept < 8 {
		t.Fatalf("swept %d Dockerfiles; the tree holds at least eight, so the sweep is not reading it", swept)
	}
	for rel := range installModeExempt {
		if !seen[rel] {
			t.Errorf("exemption %s names no Dockerfile in the tree: drop it", rel)
		}
	}
}

// TestImageInstallModeGates pins the build-time gate in each image that has a
// RUN layer to carry it, and catches each way of weakening it.
//
// regression: #6788
func TestImageInstallModeGates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the mutant anchors assume an LF checkout of Linux build inputs")
	}
	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/base/Dockerfile", "containers/ ships only in a full checkout")
	for rel, cond := range installModeGates {
		t.Run(rel, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if p := installGateProblem(text, cond); p != "" {
				t.Fatalf("%s: %s", rel, p)
			}
			// The gate's `if` line as written: cond with its escapes, then `; then \`.
			gateLine := cond + "; then \\\n"
			exitLine := "        exit 1; \\\n"
			if strings.Count(text, gateLine) != 1 {
				t.Fatalf("the mutants below need %q exactly once", gateLine)
			}
			at := strings.Index(text, gateLine)
			if strings.Index(text[at:], exitLine) < 0 {
				t.Fatalf("the mutants below need %q after the gate", exitLine)
			}
			runAt := strings.LastIndex(text[:at], "\nRUN ") + 1
			swap := func(from, to string) string {
				return strings.Replace(text, gateLine, strings.Replace(gateLine, from, to, 1), 1)
			}
			mutants := map[string]string{
				"gate dropped":       strings.Replace(text, gateLine, "    if false; then \\\n", 1),
				"gate masked":        swap("grep .", "grep . || true"),
				"gate drops other-w": swap("-perm /022", "-perm /020"),
				"gate drops group-w": swap("-perm /022", "-perm /002"),
				"gate drops owner":   swap(` -o ! -user 0 -o ! -group 0`, ""),
				"gate exits 0":       text[:at] + strings.Replace(text[at:], exitLine, "        exit 0; \\\n", 1),
				"COPY after gate":    text + "COPY tools/ /usr/local/bin/\n",
				"USER before gate":   text[:runAt] + "USER nobody\n" + text[runAt:],
			}
			if dirs := strings.Fields(strings.SplitN(strings.TrimPrefix(cond, "if find "), " -xdev", 2)[0]); len(dirs) > 1 {
				mutants["gate narrowed"] = swap(dirs[len(dirs)-1]+" -xdev", "-xdev")
			}
			for name, m := range mutants {
				if m == text {
					t.Fatalf("mutant %q did not change the Dockerfile", name)
				}
				if p := installGateProblem(m, cond); p == "" {
					t.Errorf("mutant %q: the weakened gate was not caught", name)
				} else {
					t.Logf("mutant %q caught: %s", name, p)
				}
			}
		})
	}
}

// TestInstallModeSweepCatches is the positive control: a planted install of
// each shape is caught, and the shapes that are not installs are not.
//
// regression: #6788
func TestInstallModeSweepCatches(t *testing.T) {
	const head = "FROM debian:bookworm-slim AS build\nWORKDIR /src\nCOPY . .\nCOPY go.mod ./\nCOPY tool.sh /tmp/tool.sh\n" +
		"FROM debian:bookworm-slim\nCOPY --from=build /src/out /usr/local/bin/out\n"
	if p := installModeProblems(head); len(p) != 0 {
		t.Fatalf("clean fixture flagged: %v", p)
	}
	bad := map[string]string{
		"no mode":              "COPY run.sh /usr/local/bin/run\n",
		"no mode, other flag":  "COPY --chown=0:0 run.sh /usr/local/bin/run\n",
		"ADD no mode":          "ADD run.sh /usr/local/bin/run\n",
		"tree no mode":         "COPY plugins/ /opt/assay/plugin/\n",
		"group-writable octal": "COPY --chmod=0775 run.sh /usr/local/bin/run\n",
		"other-writable octal": "COPY --chmod=0757 run.sh /usr/local/bin/run\n",
		"symbolic g+w":         "COPY --chmod=u=rwX,g=rwX,o=rX plugins/ /opt/assay/plugin/\n",
		"symbolic a+w":         "COPY --chmod=a+w run.sh /usr/local/bin/run\n",
		"symbolic no who":      "COPY --chmod=+w run.sh /usr/local/bin/run\n",
		"unreadable mode":      "COPY --chmod=$MODE run.sh /usr/local/bin/run\n",
		"relative, workdir":    "WORKDIR /usr/local/bin\nCOPY run.sh ./run\n",
		"relative, no workdir": "COPY run.sh run\n",
		"released stage":       "FROM scratch AS released\nCOPY .release-images/x/ /out/\n",
		"json form":            "COPY [\"run.sh\", \"/usr/local/bin/run\"]\n",
	}
	for name, plant := range bad {
		if p := installModeProblems(head + plant); len(p) != 1 {
			t.Errorf("%s: want one problem, got %v", name, p)
		}
	}
	good := map[string]string{
		"octal exec":        "COPY --chmod=0755 run.sh /usr/local/bin/run\n",
		"octal data":        "COPY --chmod=0644 a.txt /usr/local/share/a.txt\n",
		"symbolic tree":     "COPY --chmod=u=rwX,go=rX plugins/ /opt/assay/plugin/\n",
		"symbolic drops w":  "COPY --chmod=go-w plugins/ /opt/assay/plugin/\n",
		"from another":      "COPY --from=build /src/x /usr/local/bin/x\n",
		"scratch dest":      "COPY . /src\n",
		"relative, scratch": "WORKDIR /tmp/b\nCOPY x ./x\n",
	}
	for name, plant := range good {
		if p := installModeProblems(head + plant); len(p) != 0 {
			t.Errorf("%s: want none, got %v", name, p)
		}
	}
}
