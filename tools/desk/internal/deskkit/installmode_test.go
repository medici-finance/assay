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
// mode with no set-id bit for every build-context install outside build
// scratch. Scratch is exempt only in a build stage: a file staged there is
// still checked when a later stage copies it out with COPY --from, and a
// COPY --from that names no earlier stage of the same file is held to the
// same mode as a build-context install, since this file cannot vouch for it.

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

// installModeSiteExempt names single instructions the sweep skips, keyed
// "<Dockerfile>: <instruction>", and why. Each must still match a finding.
var installModeSiteExempt = map[string]string{
	".oss-scanner/Dockerfile: COPY . /src": "the scanner template's contract puts the checkout at /src in the final stage; the image builds and scans it as root and installs nothing from it for another user",
}

// installScratchRoots are destinations that only feed a later build step in
// the same stage (source to compile, or scanner input); nothing under them is
// installed on PATH or shared with a runtime user. They are exempt only in a
// stage that is not the final one.
var installScratchRoots = []string{"/src", "/tmp"}

// installGate is the build-time half of one image: the RUN that ends its final
// stage tests each root exists, then carries each condition as
// `<cond>; then … exit 1; fi`. A find on a missing root writes only to stderr
// and the pipeline's status is grep's, so without the `test -d` the gate
// would pass on a tree it never read.
type installGate struct {
	roots []string
	conds []string
}

// installModeGates pins the gate of each image that has a RUN layer to carry
// one. Every installed path must be root:root, carry no set-id bit and be
// writable by root alone (no bit of mode 6022). A symlink fails outright,
// since its own mode says nothing about its target, except under the base
// image's /usr/local: that root legitimately holds the node archive's in-root
// links (npm, npx, corepack) and base-files' /usr/local/man, so there a link
// fails only if it dangles or resolves outside /usr/local, whose files the
// first condition already checks.
var installModeGates = map[string]installGate{
	"containers/base/Dockerfile": {
		roots: []string{"/usr/local", "/opt/assay"},
		conds: []string{
			`if find /usr/local -xdev ! -type l \( -perm /6022 -o ! -user 0 -o ! -group 0 \) -print | grep .`,
			`if find /usr/local -xdev -xtype l -print | grep .`,
			`if find /usr/local -xdev -type l -exec realpath -m {} + | grep -v '^/usr/local/'`,
			`if find /opt/assay -xdev \( -type l -o -perm /6022 -o ! -user 0 -o ! -group 0 \) -print | grep .`,
		},
	},
	"images/assay-harness/Dockerfile": {
		roots: []string{"/usr/local/bin", "/opt/assay"},
		conds: []string{`if find /usr/local/bin /opt/assay -xdev \( -type l -o -perm /6022 -o ! -user 0 -o ! -group 0 \) -print | grep .`},
	},
	"Dockerfile": {
		roots: []string{"/usr/local/bin"},
		conds: []string{`if find /usr/local/bin -xdev \( -type l -o -perm /6022 -o ! -user 0 -o ! -group 0 \) -print | grep .`},
	},
}

// chmodModeProblem returns why a --chmod value could leave a file writable by
// its group or others or set a set-id bit, or "" when it cannot. It accepts an
// octal mode and the symbolic form (`u=rwX,go=rX`), and refuses anything it
// cannot read.
func chmodModeProblem(mode string) string {
	if mode == "" {
		return "has an empty --chmod"
	}
	if strings.Trim(mode, "01234567") == "" {
		m, err := strconv.ParseUint(mode, 8, 32)
		if err != nil || m > 07777 {
			return "has an unreadable --chmod=" + mode
		}
		if m&06000 != 0 {
			return "--chmod=" + mode + " sets a set-id bit"
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
		// The op that grants: a '+' or '=' run, up to the next op.
		op := byte(0)
		for i := 0; i < len(who); i++ {
			switch c := who[i]; c {
			case '+', '-', '=':
				op = c
			case 's':
				if op != '-' {
					return "--chmod=" + mode + " sets a set-id bit"
				}
			case 'w':
				if op != '-' && (whoSet == "" || strings.ContainsAny(whoSet, "goa")) {
					return "--chmod=" + mode + " leaves the file writable by its group or others"
				}
			}
		}
	}
	return ""
}

// pathsOverlap reports whether a and b are the same path or one holds the other.
func pathsOverlap(a, b string) bool {
	return a == b || underAny(a, []string{b}) || underAny(b, []string{a}) || a == "/" || b == "/"
}

// installModeProblems returns one line per COPY or ADD in a Dockerfile that
// can install a file whose mode the checkout chose: a build-context install
// outside build-stage scratch, a COPY --from of a file a build stage staged
// from the build context, or a COPY --from naming no earlier stage, each
// without a root-only-writable --chmod.
func installModeProblems(dockerfile string) []string {
	ins := parseDockerfile(dockerfile)
	lastFrom := -1
	for i, in := range ins {
		if in.op == "FROM" {
			lastFrom = i
		}
	}
	var out []string
	workdir := ""
	// staged maps an earlier stage, by name and by index, to the scratch paths
	// it filled from the build context without a safe --chmod.
	staged := map[string][]string{}
	var cur []string
	curName, idx := "", -1
	save := func() {
		if idx >= 0 {
			staged[strconv.Itoa(idx)] = cur
			if curName != "" {
				staged[curName] = cur
			}
		}
	}
	for i, in := range ins {
		switch in.op {
		case "FROM":
			save()
			idx++
			// A stage inherits its base's WORKDIR, which this file cannot see:
			// a relative destination before a WORKDIR is treated as unknown.
			workdir, curName, cur = "", "", nil
			var words []string
			for _, a := range in.args {
				if !strings.HasPrefix(a, "--") {
					words = append(words, a)
				}
			}
			if len(words) == 3 && strings.EqualFold(words[1], "AS") {
				curName = strings.ToLower(words[2])
			}
			if len(words) > 0 {
				if from, ok := staged[strings.ToLower(words[0])]; ok {
					cur = append(cur, from...)
				} else if strings.Contains(words[0], "$") {
					// A stage chosen by a build arg may be any earlier stage.
					for _, from := range staged {
						cur = append(cur, from...)
					}
				}
			}
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
			mode, from, hasChmod := "", "", false
			for _, f := range flags {
				switch {
				case strings.HasPrefix(f, "--from="):
					from = strings.ToLower(strings.TrimPrefix(f, "--from="))
				case strings.HasPrefix(f, "--chmod="):
					mode, hasChmod = strings.TrimPrefix(f, "--chmod="), true
				}
			}
			if len(rest) < 2 {
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
			unsafe := !hasChmod || chmodModeProblem(mode) != ""
			if dest != "" && i < lastFrom && underAny(path.Clean(dest), installScratchRoots) {
				if from == "" && unsafe {
					cur = append(cur, path.Clean(dest))
				}
				continue
			}
			why := "installs from the build context without --chmod, so the checkout's mode ships"
			if from != "" {
				paths, isStage := staged[from]
				if isStage {
					touches := false
					for _, src := range rest[:len(rest)-1] {
						for _, p := range paths {
							touches = touches || pathsOverlap(path.Clean("/"+src), p)
						}
					}
					if !touches {
						continue
					}
					why = "copies a file stage " + from + " staged from the build context, without --chmod, so the checkout's mode ships"
				} else {
					why = "copies from " + from + ", which is no earlier stage of this file, without --chmod, so its mode is not this file's to vouch for"
				}
			}
			switch {
			case !hasChmod:
				out = append(out, line+": "+why)
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
// in a fail-closed RUN carrying the install-mode gate g, or "".
func installGateProblem(dockerfile string, g installGate) string {
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
		if in.op == "RUN" && segAt(in.segs, g.conds[0]) >= 0 {
			gate = i
		}
	}
	switch {
	case gate < 0:
		return "the final stage has no RUN carrying the install-mode gate `" + g.conds[0] + "`"
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
	segs := ins[gate].segs
	first := len(segs)
	for _, cond := range g.conds {
		if p := gateProblem(segs, cond, "install-mode"); p != "" {
			return p + " `" + cond + "`"
		}
		if at := segAt(segs, cond); at < first {
			first = at
		}
	}
	for _, root := range g.roots {
		if at := segAt(segs, "test -d "+root); at < 0 || at > first {
			return "the install-mode gate does not run `test -d " + root + "` before its find, so a missing root passes"
		}
	}
	return ""
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
			site := rel + ": " + p
			exempt := false
			for key := range installModeSiteExempt {
				if strings.HasPrefix(site, key+": ") {
					seen[key], exempt = true, true
				}
			}
			if !exempt {
				t.Errorf("%s", site)
			}
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
	for key := range installModeSiteExempt {
		if !seen[key] {
			t.Errorf("site exemption %q matches no finding: drop it", key)
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
	for rel, g := range installModeGates {
		t.Run(rel, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if p := installGateProblem(text, g); p != "" {
				t.Fatalf("%s: %s", rel, p)
			}
			exitLine := "        exit 1; \\\n"
			firstLine := g.conds[0] + "; then \\\n"
			at := strings.Index(text, firstLine)
			runAt := strings.LastIndex(text[:at], "\nRUN ") + 1
			mutants := map[string]string{
				"COPY after gate":  text + "COPY tools/ /usr/local/bin/\n",
				"USER before gate": text[:runAt] + "USER nobody\n" + text[runAt:],
			}
			for _, root := range g.roots {
				rootLine := "    test -d " + root + "; \\\n"
				if strings.Count(text, rootLine) != 1 {
					t.Fatalf("the mutants below need %q exactly once", rootLine)
				}
				mutants["root test dropped "+root] = strings.Replace(text, rootLine, "", 1)
			}
			for n, cond := range g.conds {
				// The gate's `if` line as written: cond with its escapes, then `; then \`.
				gateLine := cond + "; then \\\n"
				if strings.Count(text, gateLine) != 1 {
					t.Fatalf("the mutants below need %q exactly once", gateLine)
				}
				c := strings.Index(text, gateLine)
				if strings.Index(text[c:], exitLine) < 0 {
					t.Fatalf("the mutants below need %q after the gate", exitLine)
				}
				swap := func(from, to string) string {
					return strings.Replace(text, gateLine, strings.Replace(gateLine, from, to, 1), 1)
				}
				tag := "gate " + strconv.Itoa(n) + " "
				mutants[tag+"dropped"] = strings.Replace(text, gateLine, "if false; then \\\n", 1)
				mutants[tag+"masked"] = strings.Replace(text, gateLine, cond+" || true; then \\\n", 1)
				mutants[tag+"exits 0"] = text[:c] + strings.Replace(text[c:], exitLine, "        exit 0; \\\n", 1)
				if strings.Contains(cond, "-perm /6022") {
					mutants[tag+"drops set-id"] = swap("-perm /6022", "-perm /022")
					mutants[tag+"drops other-w"] = swap("-perm /6022", "-perm /6020")
					mutants[tag+"drops group-w"] = swap("-perm /6022", "-perm /6002")
				}
				if strings.Contains(cond, ` -o ! -user 0 -o ! -group 0`) {
					mutants[tag+"drops owner"] = swap(` -o ! -user 0 -o ! -group 0`, "")
				}
				if strings.Contains(cond, `\( -type l -o `) {
					mutants[tag+"skips symlinks"] = swap(`\( -type l -o `, `! -type l \( `)
				}
				if strings.Contains(cond, `'^/usr/local/'`) {
					mutants[tag+"widened"] = swap(`'^/usr/local/'`, `'^/'`)
				}
				if dirs := strings.Fields(strings.SplitN(strings.TrimPrefix(cond, "if find "), " -xdev", 2)[0]); len(dirs) > 1 {
					mutants[tag+"narrowed"] = swap(dirs[len(dirs)-1]+" -xdev", "-xdev")
				}
			}
			for name, m := range mutants {
				if m == text {
					t.Fatalf("mutant %q did not change the Dockerfile", name)
				}
				if p := installGateProblem(m, g); p == "" {
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
		"RUN go build -o /out/out .\n" +
		"FROM debian:bookworm-slim\nCOPY --from=build /out/out /usr/local/bin/out\n"
	if p := installModeProblems(head); len(p) != 0 {
		t.Fatalf("clean fixture flagged: %v", p)
	}
	bad := map[string]string{
		"setuid octal":         "COPY --chmod=4755 run.sh /usr/local/bin/run\n",
		"setgid octal":         "COPY --chmod=2755 run.sh /usr/local/bin/run\n",
		"symbolic u+s":         "COPY --chmod=u+s run.sh /usr/local/bin/run\n",
		"symbolic set-id =":    "COPY --chmod=u=rwxs,go=rx run.sh /usr/local/bin/run\n",
		"final-stage scratch":  "COPY . /src\n",
		"final-stage tmp":      "WORKDIR /tmp/b\nCOPY x ./x\n",
		"staged in scratch":    "COPY --from=build /src/run.sh /usr/local/bin/run\n",
		"staged scratch tree":  "COPY --from=build /src /opt/x\n",
		"from no stage":        "COPY --from=ctx run.sh /usr/local/bin/run\n",
		"from later stage":     "COPY --from=later run.sh /usr/local/bin/run\nFROM scratch AS later\n",
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
		"octal exec":           "COPY --chmod=0755 run.sh /usr/local/bin/run\n",
		"octal data":           "COPY --chmod=0644 a.txt /usr/local/share/a.txt\n",
		"symbolic tree":        "COPY --chmod=u=rwX,go=rX plugins/ /opt/assay/plugin/\n",
		"symbolic drops w":     "COPY --chmod=go-w plugins/ /opt/assay/plugin/\n",
		"symbolic drops s":     "COPY --chmod=u=rwX,go=rX,u-s plugins/ /opt/assay/plugin/\n",
		"from build output":    "COPY --from=build /out/x /usr/local/bin/x\n",
		"from stage index":     "COPY --from=0 /out/x /usr/local/bin/x\n",
		"from staged, moded":   "COPY --from=build --chmod=0755 /src/run.sh /usr/local/bin/run\n",
		"from no stage, moded": "COPY --from=ctx --chmod=0755 run.sh /usr/local/bin/run\n",
		"build-stage scratch":  "FROM debian:bookworm-slim AS more\nCOPY . /src\nFROM debian:bookworm-slim\n",
		"build-stage tmp":      "FROM debian:bookworm-slim AS more\nWORKDIR /tmp/b\nCOPY x ./x\nFROM debian:bookworm-slim\n",
	}
	for name, plant := range good {
		if p := installModeProblems(head + plant); len(p) != 0 {
			t.Errorf("%s: want none, got %v", name, p)
		}
	}
}
