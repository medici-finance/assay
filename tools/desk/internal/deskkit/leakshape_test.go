package deskkit

// leakshape_test.go — the shape stage of the disclosure sweep (leakshape.go, issue #3159).
//
// Every test is named TestS2… so the disclosure-sweep CI job (which runs
// `-run 'TestS2|TestCorpus'`) carries it with no workflow change.
//
// Every planted shape below is ASSEMBLED FROM FRAGMENTS, never a contiguous literal: a
// literal home path or alias remote in this file would itself trip TestS2ShapeRepoGate,
// the same discipline s2ForbiddenToken follows for the token stage.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

// leakShapeAllowRel is the committed allow file, repo-relative. It sits at the repo root
// beside the other tree-wide scanner config (.gitleaks.toml), not under tools/desk: it names
// paths across THIS repository, so it is the repo's configuration, not part of the tool.
const leakShapeAllowRel = ".leakshape-allow.yaml"

// Placeholder shapes, one per detector, built from fragments.
var (
	plantHomeMac  = "/Us" + "ers/" + "example-user"
	plantHomeLin  = "/ho" + "me/" + "example-user"
	plantHomeWin  = "C:" + `\` + "Users" + `\` + "example-user"
	plantNoreply  = "1234567" + "+" + "example-login" + "@" + "users.noreply.github.com"
	plantSSHScp   = "git" + "@" + "example-alias"
	plantSSHURL   = "ssh" + "://" + "git" + "@" + "example-alias"
	plantTilde    = "~" + "/" + "example-work"
	plantAltValue = "/Us" + "ers/" + "another-user"
)

// shapeTree is a temp tree that the TOKEN stage can check (it carries the token stage's
// control token) and that the token registry has no entry for — the registry is BLIND to
// whatever is planted in it.
func shapeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["config.go"] = "var " + s2ControlToken + " = map[string]int{}\n"
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// regression: #3159
func TestS2ShapeRepoGate(t *testing.T) {
	allow, err := LoadShapeAllow(filepath.Join(s2RepoRoot, leakShapeAllowRel), leakShapeAllowRel)
	if err != nil {
		t.Fatalf("allow file does not load — the shape stage is could-not-check: %v", err)
	}
	token, err := s2Sweep(s2RepoRoot, s2ForbiddenToken(), s2ControlToken, s2SweepExclusionSet(), s2SweepSelfPath)
	if err != nil {
		t.Fatalf("token stage: %v", err)
	}
	shape, err := ShapeSweep(s2RepoRoot, allow)
	if err != nil {
		t.Fatalf("shape stage: %v", err)
	}
	if shape.State == StageCouldNotCheck {
		t.Fatalf("shape stage could-not-check: %s", shape.Problem)
	}
	// Locations only — the matched value is never echoed into a CI log.
	for _, h := range shape.Hits {
		t.Errorf("unallowed leak shape at %s — replace it with a reserved example (an .example / "+
			".invalid host, a synthetic id) or add a reviewed allow entry naming the exact value", h.Location())
	}
	for _, s := range shape.Stale {
		t.Errorf("stale allow entry %s matches nothing — delete it", s)
	}
	if got := CertifyStages(token.state, shape.State); got != StageClean {
		t.Errorf("certify = %s (token %s, shape %s), want %s", got, token.state, shape.State, StageClean)
	}
	t.Logf("shape stage: %d text files, %d binary skipped, %d allowed matches", shape.Scanned, shape.Skipped, shape.Allowed)
}

// regression: #3159
func TestS2ShapeEachRedsAlone(t *testing.T) {
	cases := []struct {
		name, body string
		want       LeakShape
	}{
		{"home-mac", "cd " + plantHomeMac + "/src/repo\n", ShapeHomePath},
		{"home-linux", "WORKDIR=" + plantHomeLin + "/work\n", ShapeHomePath},
		{"home-windows", `root = "` + plantHomeWin + `\src"` + "\n", ShapeHomePath},
		{"home-windows-lower", `root = "` + "c:" + `\` + "users" + `\` + "example-user" + `\src"` + "\n", ShapeHomePath},
		{"home-mac-lower", "cd " + "/us" + "ers/" + "example-user" + "/src\n", ShapeHomePath},
		{"home-tilde-account", "cd " + "~" + "example-user" + "/src\n", ShapeHomePath},
		{"noreply", "Author: Example <" + plantNoreply + ">\n", ShapeNoreplyMail},
		{"ssh-scp", "origin\t" + plantSSHScp + ":owner/repo.git (push)\n", ShapeSSHAlias},
		{"ssh-scp-user", "origin\t" + "deploy" + "@" + "example-alias" + ":owner/repo.git (push)\n", ShapeSSHAlias},
		{"ssh-scp-upper", "origin\t" + "GIT" + "@" + "example-alias" + ":owner/repo.git (push)\n", ShapeSSHAlias},
		{"ssh-scp-bare", "url = " + "example-alias" + ":" + "owner/repo.git\n", ShapeSSHAlias},
		{"ssh-url", "url = " + plantSSHURL + "/owner/repo.git\n", ShapeSSHAlias},
		{"ssh-url-upper", "url = " + "SSH" + "://" + "git" + "@" + "example-alias" + "/owner/repo.git\n", ShapeSSHAlias},
		{"tilde", "cellctl new c --repo " + plantTilde + "/repo\n", ShapeTildeRoot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := shapeTree(t, map[string]string{"notes/planted.txt": tc.body})

			// The token registry is blind: the token stage certifies this tree.
			token, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if token.state != sweepCheckedClean {
				t.Fatalf("token stage = %s, want %s — the registry must be blind for this to "+
					"prove the shape stage stands on its own", token.state, sweepCheckedClean)
			}

			shape, err := ShapeSweep(dir, ShapeAllowList{})
			if err != nil {
				t.Fatal(err)
			}
			if shape.State != StageFailed {
				t.Fatalf("shape stage = %s (%s), want %s", shape.State, shape.Problem, StageFailed)
			}
			if len(shape.Hits) != 1 || shape.Hits[0].Shape != tc.want || shape.Hits[0].Path != "notes/planted.txt" || shape.Hits[0].Line != 1 {
				t.Fatalf("hits = %+v, want exactly one %s at notes/planted.txt:1", shape.Hits, tc.want)
			}
			if loc := shape.Hits[0].Location(); strings.Contains(loc, shape.Hits[0].Value) {
				t.Errorf("Location %q echoes the matched value", loc)
			}
			// A token-green, shape-red tree does not certify.
			if got := CertifyStages(token.state, shape.State); got != StageFailed {
				t.Fatalf("certify = %s, want %s", got, StageFailed)
			}
		})
	}
}

// regression: #3159
func TestS2ShapeNearMissesClean(t *testing.T) {
	// Path-like near misses are built from fragments too: a contiguous one reads as a
	// machine path to other public-surface scanners.
	body := strings.Join([]string{
		"remote: git@github.com:example-org/tracker.git",
		"remote: ssh://git@ssh.github.com:443/example-org/tracker.git",
		"remote: git@GitLab.com:example-group/proj.git",
		"remote: " + "git" + "@" + "bitbucket.org" + ":example-team/repo.git",
		"config: ~/.config/assay/roster.env",
		"cache: ~/.cache/go-build",
		"link: https://example.com/" + "home" + "/page",
		"path: /var/lib/" + "home" + "/x and ./" + "Users" + "/x",
		"bare: ~ / and ~5/10 of runs and ~int | ~string",
		"mail: noreply@github.com, 12345@users.noreply.github.com",
		"word: myhome/x and homes/users",
		// The widened scp grammar's near misses: a URL with a scheme (with or without a
		// user and a port), host:port, a Windows drive, prose with a colon, a user@host
		// with no path, a content digest, a search qualifier, a tilde in a URL path.
		"url: https://" + "deploy" + "@" + "example.com:8443/owner/repo.git",
		"url: https://" + "example.com:8443/owner/repo.git and " + "localhost:8080/owner/repo.git",
		"drive: " + "C:" + `\` + `src\repo.git and ` + "C:" + "/src/repo.git and " + "C:" + "src/repo.git",
		"note: foo/bar.git, see: owner/repo.git",
		"ping " + "alice" + "@" + "example-host: ready, and " + "alice" + "@" + "example-host:22",
		"image: alpine" + "@" + "sha256:" + strings.Repeat("ab", 32),
		"search: repo:owner/name is:open",
		"web: https://example.com/" + "~" + "someone/page",
	}, "\n") + "\n"
	dir := shapeTree(t, map[string]string{"near.txt": body})
	rep, err := ShapeSweep(dir, ShapeAllowList{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.State != StageClean {
		var locs []string
		for _, h := range rep.Hits {
			locs = append(locs, h.Location()+" "+h.Value)
		}
		t.Fatalf("state = %s (%s), want %s; hits: %v", rep.State, rep.Problem, StageClean, locs)
	}
}

// regression: #3159
func TestS2ShapeAllowIsExact(t *testing.T) {
	example := "doc: " + plantHomeMac + "/work\n"
	entry := ShapeAllow{Path: "docs/guide.md", Shape: string(ShapeHomePath), SHA256: []string{ShapeValueDigest(plantHomeMac)}, Reason: "documented example"}
	list := ShapeAllowList{Entries: []ShapeAllow{entry}}

	run := func(t *testing.T, files map[string]string, l ShapeAllowList) ShapeReport {
		t.Helper()
		rep, err := ShapeSweep(shapeTree(t, files), l)
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}

	t.Run("listed example is clean", func(t *testing.T) {
		rep := run(t, map[string]string{"docs/guide.md": example}, list)
		if rep.State != StageClean || rep.Allowed != 1 || len(rep.Stale) != 0 {
			t.Fatalf("state %s allowed %d stale %v hits %+v, want clean/1/none", rep.State, rep.Allowed, rep.Stale, rep.Hits)
		}
	})
	t.Run("unlisted value in the same file fails", func(t *testing.T) {
		rep := run(t, map[string]string{"docs/guide.md": example + "also: " + plantAltValue + "\n"}, list)
		if rep.State != StageFailed || len(rep.Hits) != 1 || rep.Hits[0].Value != plantAltValue {
			t.Fatalf("state %s hits %+v, want one hit on the unlisted value", rep.State, rep.Hits)
		}
	})
	t.Run("listed value in another file fails", func(t *testing.T) {
		rep := run(t, map[string]string{"docs/guide.md": example, "src/main.go": "// " + plantHomeMac + "\n"}, list)
		if rep.State != StageFailed || len(rep.Hits) != 1 || rep.Hits[0].Path != "src/main.go" {
			t.Fatalf("state %s hits %+v, want one hit in src/main.go", rep.State, rep.Hits)
		}
	})
	t.Run("entry covers only its own shape", func(t *testing.T) {
		rep := run(t, map[string]string{"docs/guide.md": example + "origin " + plantSSHScp + ":o/r.git\n"}, list)
		if rep.State != StageFailed || len(rep.Hits) != 1 || rep.Hits[0].Shape != ShapeSSHAlias {
			t.Fatalf("state %s hits %+v, want one %s hit", rep.State, rep.Hits, ShapeSSHAlias)
		}
	})
	t.Run("entry that matches nothing is stale", func(t *testing.T) {
		rep := run(t, map[string]string{"docs/guide.md": "nothing here\n"}, list)
		if len(rep.Stale) != 1 {
			t.Fatalf("stale = %v, want the one unmatched entry", rep.Stale)
		}
		// Stale is in the state itself, so a caller other than the repo gate reads it too.
		if rep.State != StageFailed || len(rep.Hits) != 0 {
			t.Fatalf("stale only: state %s hits %+v, want %s with no hits", rep.State, rep.Hits, StageFailed)
		}
		if got := CertifyStages(StageClean, rep.State); got != StageFailed {
			t.Fatalf("certify with a stale entry = %s, want %s", got, StageFailed)
		}
	})
	t.Run("allow file is swept like any other file", func(t *testing.T) {
		// The allow file names values by digest, so it carries no shape and has no
		// self-exclusion: a value written into it in the clear is a hit like anywhere else.
		src := "allow.yaml"
		digests := "sha256: [" + ShapeValueDigest(plantHomeMac) + "]\n"
		rep := run(t, map[string]string{"docs/guide.md": example, src: digests}, list)
		if rep.State != StageClean {
			t.Fatalf("digest-only allow file: state %s hits %+v, want clean", rep.State, rep.Hits)
		}
		rep = run(t, map[string]string{"docs/guide.md": example, src: digests + "# " + plantHomeMac + "\n"}, list)
		if rep.State != StageFailed || len(rep.Hits) != 1 || rep.Hits[0].Path != src {
			t.Fatalf("clear value in the allow file: state %s hits %+v, want one hit there", rep.State, rep.Hits)
		}
	})
	t.Run("location names the digest an entry needs", func(t *testing.T) {
		rep := run(t, map[string]string{"docs/guide.md": example + "also: " + plantAltValue + "\n"}, list)
		if len(rep.Hits) != 1 {
			t.Fatalf("hits %+v, want one", rep.Hits)
		}
		loc := rep.Hits[0].Location()
		if !strings.Contains(loc, "sha256:"+ShapeValueDigest(plantAltValue)) || strings.Contains(loc, plantAltValue) {
			t.Fatalf("Location %q: want the value's digest and never the value", loc)
		}
	})
}

// regression: #3159
func TestS2ShapeAllowValidation(t *testing.T) {
	okDigest := ShapeValueDigest("x")
	ok := "  - path: docs/guide.md\n    shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: documented example\n"
	cases := map[string]string{
		"no reason":      "  - path: docs/guide.md\n    shape: home-path\n    sha256: ['" + okDigest + "']\n",
		"blank reason":   "  - path: docs/guide.md\n    shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: '  '\n",
		"no values":      "  - path: docs/guide.md\n    shape: home-path\n    reason: r\n",
		"clear value":    "  - path: docs/guide.md\n    shape: home-path\n    sha256: ['x']\n    reason: r\n",
		"short digest":   "  - path: docs/guide.md\n    shape: home-path\n    sha256: ['" + okDigest[:63] + "']\n    reason: r\n",
		"upper digest":   "  - path: docs/guide.md\n    shape: home-path\n    sha256: ['" + strings.ToUpper(okDigest) + "']\n    reason: r\n",
		"old values key": "  - path: docs/guide.md\n    shape: home-path\n    values: ['x']\n    reason: r\n",
		"no path":        "  - shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: r\n",
		"unknown shape":  "  - path: docs/guide.md\n    shape: home-dir\n    sha256: ['" + okDigest + "']\n    reason: r\n",
		"glob path":      "  - path: docs/*.md\n    shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: r\n",
		"dir path":       "  - path: docs/\n    shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: r\n",
		"parent path":    "  - path: ../docs/guide.md\n    shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: r\n",
		"absolute path":  "  - path: /docs/guide.md\n    shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: r\n",
		"duplicate":      ok + ok,
		"unknown key":    "  - path: docs/guide.md\n    shape: home-path\n    sha256: ['" + okDigest + "']\n    reason: r\n    glob: true\n",
	}
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "allow.yaml")
		if err := os.WriteFile(p, []byte("allow:\n"+body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := LoadShapeAllow(write(ok), "allow.yaml"); err != nil {
		t.Fatalf("well-formed entry refused: %v", err)
	}
	for name, body := range cases {
		if _, err := LoadShapeAllow(write(body), "allow.yaml"); err == nil {
			t.Errorf("%s: loaded, want a refusal", name)
		}
	}
	if _, err := LoadShapeAllow(filepath.Join(dir, "absent.yaml"), "absent.yaml"); err == nil {
		t.Error("missing allow file loaded, want a refusal")
	}

	// An invalid list handed straight to the sweep is could-not-check, never clean.
	tree := shapeTree(t, map[string]string{"a.txt": "clean\n"})
	bad := ShapeAllowList{Entries: []ShapeAllow{{Path: "a.txt", Shape: string(ShapeHomePath), SHA256: []string{ShapeValueDigest("x")}}}}
	rep, err := ShapeSweep(tree, bad)
	if err != nil {
		t.Fatal(err)
	}
	if rep.State != StageCouldNotCheck {
		t.Fatalf("reasonless allow list: state %s, want %s", rep.State, StageCouldNotCheck)
	}
}

// regression: #3159
func TestS2ShapeCouldNotCheck(t *testing.T) {
	missing, err := ShapeSweep(filepath.Join(t.TempDir(), "no-such-tree"), ShapeAllowList{})
	if err != nil || missing.State != StageCouldNotCheck {
		t.Errorf("missing tree: state %s err %v, want %s", missing.State, err, StageCouldNotCheck)
	}
	empty, err := ShapeSweep(t.TempDir(), ShapeAllowList{})
	if err != nil || empty.State != StageCouldNotCheck {
		t.Errorf("empty tree: state %s err %v, want %s", empty.State, err, StageCouldNotCheck)
	}
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "blob.bin"), []byte{0x7f, 'E', 0, 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	binOnly, err := ShapeSweep(binDir, ShapeAllowList{})
	if err != nil || binOnly.State != StageCouldNotCheck {
		t.Errorf("binary-only tree: state %s err %v, want %s", binOnly.State, err, StageCouldNotCheck)
	}

	// A rule that cannot match its own canary is broken: its silence proves nothing, so the
	// stage is could-not-check — even over a tree that carries the shape it should catch.
	for i := range leakShapeRules {
		rules := append([]shapeRule(nil), leakShapeRules...)
		rules[i].re = regexp.MustCompile(`a-pattern-that-matches-no-canary-(?:zz)`)
		tree := shapeTree(t, map[string]string{"a.txt": "cd " + plantHomeMac + "\n"})
		rep, err := shapeSweep(tree, ShapeAllowList{}, rules)
		if err != nil {
			t.Fatal(err)
		}
		if rep.State != StageCouldNotCheck {
			t.Errorf("rule %d (%s) neutered: state %s, want %s", i, rules[i].shape, rep.State, StageCouldNotCheck)
		}
	}
}

// regression: #3159
func TestS2ShapeCertifyIsAnd(t *testing.T) {
	c, f, n := StageClean, StageFailed, StageCouldNotCheck
	cases := []struct {
		in   []string
		want string
	}{
		{nil, n},
		{[]string{c}, c},
		{[]string{c, c}, c},
		{[]string{c, f}, f},
		{[]string{f, c}, f},
		{[]string{c, n}, n},
		{[]string{n, f}, f},
		{[]string{c, ""}, n},
		{[]string{c, "clean"}, n},
	}
	for _, tc := range cases {
		if got := CertifyStages(tc.in...); got != tc.want {
			t.Errorf("CertifyStages(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// regression: #3159
func TestS2ShapeNoWordBoundary(t *testing.T) {
	for _, r := range leakShapeRules {
		src := r.re.String()
		if strings.Contains(src, `\b`) || strings.Contains(src, `\B`) {
			t.Errorf("%s rule uses a word-boundary escape: %s", r.shape, src)
		}
		if r.canary == "" {
			t.Errorf("%s rule has no canary — its silence would prove nothing", r.shape)
		}
	}
	seen := map[LeakShape]bool{}
	for _, r := range leakShapeRules {
		seen[r.shape] = true
	}
	for _, s := range AllLeakShapes {
		if !seen[s] {
			t.Errorf("shape %s has no rule", s)
		}
	}
}

// regression: #3159
func TestS2ShapeNonTextPaths(t *testing.T) {
	dir := shapeTree(t, map[string]string{})
	// UTF-16 with a BOM is text, not binary.
	units := utf16.Encode([]rune("cd " + plantTilde + "/repo\n"))
	raw := []byte{0xFF, 0xFE}
	for _, u := range units {
		raw = append(raw, byte(u), byte(u>>8))
	}
	if err := os.WriteFile(filepath.Join(dir, "wide.txt"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	// A symlink's target is checked-in text too.
	if err := os.Symlink(plantHomeLin+"/secret", filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	rep, err := ShapeSweep(dir, ShapeAllowList{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]LeakShape{}
	for _, h := range rep.Hits {
		got[h.Path] = h.Shape
	}
	if rep.State != StageFailed || got["wide.txt"] != ShapeTildeRoot || got["link"] != ShapeHomePath {
		t.Fatalf("state %s hits %+v, want a tilde hit in wide.txt and a home hit on link", rep.State, rep.Hits)
	}
}
