package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// npmFixturePkg is the package directory of the npm staging fixtures: a
// tracked package.json and package-lock.json, like the issue's bridge package.
const npmFixturePkg = "pkg"

// npmStagingFixture is a prepared, stamped verifier home whose attested tree
// holds one npm package, with the npm runner replaced by a fake that writes a
// fixed dependency set (a file, an executable and a .bin link) and, for the
// build script, a fixed dist output.
func npmStagingFixture(t *testing.T) (string, *verifierForge, VerifierReceipt) {
	t.Helper()
	root, f := verifierFixtureWith(t, "gpt-6-astra", "brief.md", map[string]string{
		npmFixturePkg + "/package.json":      `{"name":"fixture","scripts":{"build":"tsc"}}`,
		npmFixturePkg + "/package-lock.json": `{"name":"fixture","lockfileVersion":3}`,
		npmFixturePkg + "/src/main.ts":       "export const x = 1\n",
	})
	receipt, err := IssueVerifierAttestation(root, f)
	if err != nil {
		t.Fatal(err)
	}
	prev := npmStageCommand
	t.Cleanup(func() { npmStageCommand = prev })
	npmStageCommand = func(dir string, env []string, args ...string) error {
		write := func(rel, body string, mode os.FileMode) {
			p := filepath.Join(dir, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), mode); err != nil {
				t.Fatal(err)
			}
		}
		switch args[0] {
		case "ci":
			write("node_modules/lib/package.json", `{"name":"lib","version":"1.0.0"}`, 0o644)
			write("node_modules/lib/bin/tool.js", "#!/usr/bin/env node\n", 0o755)
			if err := os.MkdirAll(filepath.Join(dir, "node_modules/.bin"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../lib/bin/tool.js", filepath.Join(dir, "node_modules/.bin/tool")); err != nil {
				t.Fatal(err)
			}
		case "run":
			write("dist/main.js", "export const x = 1;\n", 0o644)
		}
		return nil
	}
	return root, f, receipt
}

func npmFixtureCache(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func writeNpmProbe(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"name":"zz-probe"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// regression: #7572
// The issue's probe: with no npm staging record, one file under the package's
// node_modules still makes pre-work admission refuse with exit 5, before rows
// and at Evidence, and the receipt carries no staging digest.
func TestNpmProbeRefusesUnstaged(t *testing.T) {
	root, f, receipt := npmStagingFixture(t)
	if r, err := verifierLoad(root); err != nil || r.NpmStaging != nil {
		t.Fatalf("fixture run already has a staging record: %+v %v", r.NpmStaging, err)
	}
	probe := npmFixturePkg + "/node_modules/zz-probe/package.json"
	writeNpmProbe(t, root, probe)
	got, err := CheckVerifierAttestationWithForge(root, "brief.md", f)
	if err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "unattested worktree file: "+probe) {
		t.Fatalf("unstaged node_modules probe: want exit %d refusal naming %s, got exit %d: %v", ExitRefused, probe, ExitCodeOf(err), err)
	}
	if got.NpmStaging != nil {
		t.Fatalf("unstaged run's receipt carries a staging digest: %+v", got.NpmStaging)
	}
	setEvidence(t, root, receipt)
	if _, err := CheckVerifierEvidenceWithForge(root, "example-org/one", "brief.md", f); err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "unattested worktree file: "+probe) {
		t.Fatalf("unstaged node_modules probe admitted at Evidence: %v", err)
	}
	// A dist directory a row wrote without staging refuses the same way.
	if err := os.RemoveAll(filepath.Join(root, npmFixturePkg, "node_modules")); err != nil {
		t.Fatal(err)
	}
	writeNpmProbe(t, root, npmFixturePkg+"/dist/main.js")
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "unattested worktree file: "+npmFixturePkg+"/dist/main.js") {
		t.Fatalf("unstaged dist admitted: %v", err)
	}
}

func setEvidence(t *testing.T, root string, receipt VerifierReceipt) {
	t.Helper()
	path := filepath.Join(root, "brief.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), "Pending.", receipt.EvidenceBinding(), 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// regression: #7572
// A tree staged by the admitted step, node_modules and the built dist, admits
// under the recorded digest before rows and at Evidence; a row that rebuilds
// dist to the same bytes stays admitted. Every other unattested file, a staged
// directory's sibling included, still refuses.
func TestNpmStagedTreeAdmits(t *testing.T) {
	root, f, receipt := npmStagingFixture(t)
	staged, err := StageVerifierNpm(root, "brief.md", npmFixturePkg, npmFixtureCache(t), true)
	if err != nil {
		t.Fatalf("staging refused: %v", err)
	}
	if staged.NpmStaging == nil || !npmStagingDigestRe.MatchString(staged.NpmStaging.Digest) || !staged.NpmStaging.Built || staged.NpmStaging.Package != npmFixturePkg {
		t.Fatalf("staging recorded no digest: %+v", staged.NpmStaging)
	}
	got, err := CheckVerifierAttestationWithForge(root, "brief.md", f)
	if err != nil {
		t.Fatalf("staged tree refused before rows: %v", err)
	}
	if got.NpmStaging == nil || got.NpmStaging.Digest != staged.NpmStaging.Digest {
		t.Fatalf("admission receipt does not carry the staged digest: %+v", got.NpmStaging)
	}
	out, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(out), `"NpmStaging":{`) || !strings.Contains(string(out), staged.NpmStaging.Digest) {
		t.Fatalf("printed receipt lacks the staging digest: %s %v", out, err)
	}
	// Row 1 rebuilds dist: identical bytes keep the digest.
	if err := os.WriteFile(filepath.Join(root, npmFixturePkg, "dist/main.js"), []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	setEvidence(t, root, receipt)
	if _, err := CheckVerifierEvidenceWithForge(root, "example-org/one", "brief.md", f); err != nil {
		t.Fatalf("staged tree refused at Evidence: %v", err)
	}
	for _, stray := range []string{npmFixturePkg + "/stray.txt", npmFixturePkg + "/node_modules2/x.json", "node_modules/x.json", npmFixturePkg + "/src/node_modules/x.json"} {
		writeNpmProbe(t, root, stray)
		if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "unattested worktree file: "+stray) {
			t.Fatalf("unattested %s admitted on a staged run: %v", stray, err)
		}
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(stray))); err != nil {
			t.Fatal(err)
		}
	}
}

// regression: #7572
// A staged tree changed after staging refuses: any added, changed, removed,
// re-moded or re-linked file under a staged directory, a record rewritten to
// drop dist, and a second staging of the same run.
func TestNpmMutatedStagingRefuses(t *testing.T) {
	nm := filepath.Join(npmFixturePkg, "node_modules")
	for _, mode := range []string{"changed", "added", "removed", "chmod", "relinked", "chained-link", "dangling-link", "dist-added", "dist-changed", "record-unbuilt", "record-lock", "restage"} {
		t.Run(mode, func(t *testing.T) {
			root, f, _ := npmStagingFixture(t)
			if _, err := StageVerifierNpm(root, "brief.md", npmFixturePkg, npmFixtureCache(t), true); err != nil {
				t.Fatalf("staging refused: %v", err)
			}
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err != nil {
				t.Fatalf("staged tree refused before mutation: %v", err)
			}
			at := func(rel string) string { return filepath.Join(root, rel) }
			want := "npm staging changed since it was staged"
			var err error
			switch mode {
			case "changed":
				err = os.WriteFile(at(nm+"/lib/package.json"), []byte(`{"name":"lib","version":"6.6.6"}`), 0o644)
			case "added":
				writeNpmProbe(t, root, npmFixturePkg+"/node_modules/zz-probe/package.json")
			case "removed":
				err = os.Remove(at(nm + "/lib/package.json"))
			case "chmod":
				err = os.Chmod(at(nm+"/lib/bin/tool.js"), 0o644)
			case "relinked":
				if err = os.Remove(at(nm + "/.bin/tool")); err == nil {
					err = os.Symlink("../lib/package.json", at(nm+"/.bin/tool"))
				}
			case "chained-link":
				// The review's repro: l1 resolves to node_modules itself, so
				// l2's target is inside lexically but resolves to the attested
				// brief at the home root.
				if err = os.MkdirAll(at(nm+"/a/b"), 0o755); err == nil {
					if err = os.Symlink("../..", at(nm+"/a/b/l1")); err == nil {
						err = os.Symlink("a/b/l1/../../brief.md", at(nm+"/l2"))
					}
				}
				want = "resolves outside its staged directory"
			case "dangling-link":
				err = os.Symlink("lib/absent.js", at(nm+"/dangling"))
				want = "does not resolve to an existing path"
			case "dist-added":
				writeNpmProbe(t, root, npmFixturePkg+"/dist/extra.js")
			case "dist-changed":
				err = os.WriteFile(at(npmFixturePkg+"/dist/main.js"), []byte("export const x = 2;\n"), 0o644)
			case "record-unbuilt":
				r, lerr := verifierLoad(root)
				if lerr != nil {
					t.Fatal(lerr)
				}
				r.NpmStaging.Built = false
				err = verifierSave(root, r)
				want = "unattested worktree file: " + npmFixturePkg + "/dist/main.js"
			case "record-lock":
				r, lerr := verifierLoad(root)
				if lerr != nil {
					t.Fatal(lerr)
				}
				r.NpmStaging.Lock = strings.Repeat("0", len(r.NpmStaging.Lock))
				err = verifierSave(root, r)
				want = "names a different package-lock.json"
			case "restage":
				_, serr := StageVerifierNpm(root, "brief.md", npmFixturePkg, npmFixtureCache(t), true)
				if serr == nil || ExitCodeOf(serr) != ExitRefused || !strings.Contains(serr.Error(), "already has an npm staging record") {
					t.Fatalf("second staging of one run not refused: %v", serr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s after staging: want exit %d refusal %q, got exit %d: %v", mode, ExitRefused, want, ExitCodeOf(err), err)
			}
		})
	}
}

// regression: #7572
// The staging step refuses before running npm when the home already holds an
// unattested file, the cache sits inside the home, or the package or a staged
// directory is not where an untracked staging can go.
func TestNpmStagingStepRefuses(t *testing.T) {
	for _, mode := range []string{"preexisting", "cache-inside", "cache-missing", "no-lock", "escape"} {
		t.Run(mode, func(t *testing.T) {
			root, _, _ := npmStagingFixture(t)
			ran := false
			prev := npmStageCommand
			npmStageCommand = func(dir string, env []string, args ...string) error { ran = true; return prev(dir, env, args...) }
			pkg, cache, want := npmFixturePkg, npmFixtureCache(t), ""
			switch mode {
			case "preexisting":
				writeNpmProbe(t, root, npmFixturePkg+"/node_modules/zz-probe/package.json")
				want = "unattested worktree file"
			case "cache-inside":
				cache = filepath.Join(root, "ignored")
				if err := os.MkdirAll(cache, 0o755); err != nil {
					t.Fatal(err)
				}
				want = "inside the home"
			case "cache-missing":
				cache = filepath.Join(cache, "absent")
				want = "not an existing directory"
			case "no-lock":
				pkg = npmFixturePkg + "/src"
				want = "no attested package.json"
			case "escape":
				pkg = "../" + npmFixturePkg
				want = "not a clean home-relative directory"
			}
			_, err := StageVerifierNpm(root, "brief.md", pkg, cache, false)
			if err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: want refusal %q, got %v", mode, want, err)
			}
			if ran {
				t.Fatalf("%s: npm ran before the step refused", mode)
			}
			if r, _ := verifierLoad(root); r.NpmStaging != nil {
				t.Fatalf("%s: a refused staging left a record", mode)
			}
		})
	}
}

// regression: #7572
// The staging step's npm commands run under an allow-list scrub with a fresh,
// empty HOME and TMPDIR, as an admitted Verify row does: no credential-shaped
// variable, npm config variable, NODE_OPTIONS or proxy login reaches either
// command, and the step's HOME and TMPDIR are gone once it ends.
func TestNpmStagingEnvScrubbed(t *testing.T) {
	root, _, _ := npmStagingFixture(t)
	realHome := t.TempDir()
	const sentinel = "sentinel-value-7572"
	t.Setenv("HOME", realHome)
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "NPM_TOKEN", "NODE_AUTH_TOKEN", "AWS_SECRET_ACCESS_KEY", "npm_config__authToken", "NPM_CONFIG_USERCONFIG", "NODE_OPTIONS", "SSH_AUTH_SOCK", "GIT_ASKPASS"} {
		t.Setenv(name, sentinel)
	}
	t.Setenv("HTTPS_PROXY", "http://user:"+sentinel+"@proxy.invalid:8080")
	t.Setenv("LANG", "C.UTF-8")
	var dirs []string
	prev := npmStageCommand
	npmStageCommand = func(dir string, env []string, args ...string) error {
		vars := map[string]string{}
		for _, kv := range env {
			k, v, _ := strings.Cut(kv, "=")
			if strings.Contains(v, sentinel) {
				t.Errorf("npm %s: %s reached the staging step", args[0], k)
			}
			if _, dup := vars[k]; dup {
				t.Errorf("npm %s: %s set twice", args[0], k)
			}
			vars[k] = v
		}
		if vars["LANG"] != "C.UTF-8" || vars["PATH"] == "" {
			t.Errorf("npm %s: allow-listed LANG or PATH dropped: %q %q", args[0], vars["LANG"], vars["PATH"])
		}
		for _, k := range []string{"HOME", "TMPDIR"} {
			d := vars[k]
			if d == "" || d == realHome || strings.HasPrefix(d, root) {
				t.Errorf("npm %s: %s=%q is not the step's own directory", args[0], k, d)
				continue
			}
			if ents, err := os.ReadDir(d); err != nil || len(ents) != 0 {
				t.Errorf("npm %s: %s=%s is not a fresh empty directory: %v %v", args[0], k, d, ents, err)
			}
			dirs = append(dirs, d)
		}
		return prev(dir, env, args...)
	}
	if _, err := StageVerifierNpm(root, "brief.md", npmFixturePkg, npmFixtureCache(t), true); err != nil {
		t.Fatalf("staging refused: %v", err)
	}
	if len(dirs) != 4 {
		t.Fatalf("want HOME and TMPDIR for both npm commands, got %v", dirs)
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("staging step's %s outlived the step: %v", d, err)
		}
	}
}

// regression: #7572
// A link the staging step's npm writes is resolved along its whole chain, not
// only lexically: the review's chained pair (l1 -> ../.., then
// l2 -> a/b/l1/../../brief.md) stays inside node_modules as written but
// resolves to the home root, and a dangling link resolves nowhere. Either
// refuses the staging and leaves no record, so a link out of the staged
// directory never enters the digest whose target could later change unseen.
func TestNpmStagingRefusesEscapingLinks(t *testing.T) {
	for _, mode := range []string{"chained-link", "dangling-link"} {
		t.Run(mode, func(t *testing.T) {
			root, _, _ := npmStagingFixture(t)
			prev := npmStageCommand
			npmStageCommand = func(dir string, env []string, args ...string) error {
				if err := prev(dir, env, args...); err != nil || args[0] != "ci" {
					return err
				}
				nm := filepath.Join(dir, "node_modules")
				if mode == "dangling-link" {
					return os.Symlink("lib/absent.js", filepath.Join(nm, "dangling"))
				}
				if err := os.MkdirAll(filepath.Join(nm, "a/b"), 0o755); err != nil {
					return err
				}
				if err := os.Symlink("../..", filepath.Join(nm, "a/b/l1")); err != nil {
					return err
				}
				return os.Symlink("a/b/l1/../../brief.md", filepath.Join(nm, "l2"))
			}
			want := map[string]string{"chained-link": "resolves outside its staged directory", "dangling-link": "does not resolve to an existing path"}[mode]
			_, err := StageVerifierNpm(root, "brief.md", npmFixturePkg, npmFixtureCache(t), false)
			if err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: want exit %d refusal %q, got exit %d: %v", mode, ExitRefused, want, ExitCodeOf(err), err)
			}
			if r, _ := verifierLoad(root); r.NpmStaging != nil {
				t.Fatalf("%s: a refused staging left a record", mode)
			}
		})
	}
}
