// Package testenv builds explicit, scoped environments for repository and command
// fixtures. It is test support; production tools must not import it.
package testenv

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet"
)

// Options declares every executable and environment value a fixture needs.
// Resolve approved executables before Install; Tools never searches PATH.
type Options struct {
	Tools map[string]string
	Vars  map[string]string
}

// Fixture owns the home, executable directory and immutable child environment.
type Fixture struct {
	Home string
	Bin  string
	t    testing.TB
	base map[string]string
}

func keyFor(key, platform string) string {
	if platform == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func reserved(key string) bool {
	switch strings.ToUpper(key) {
	case "PATH", "HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "TMP", "TEMP", "TMPDIR":
		return true
	}
	return false
}

// Install replaces the test process environment until cleanup. Like t.Setenv,
// it cannot be used by parallel tests. Explicit hostile controls belong AFTER
// installation, so setup never erases the input an authority test must reject.
func Install(t testing.TB, opts Options) *Fixture {
	t.Helper()
	root := t.TempDir()
	f := &Fixture{Home: filepath.Join(root, "home"), Bin: filepath.Join(root, "bin"), t: t}
	config := filepath.Join(f.Home, ".config")
	cache := filepath.Join(f.Home, ".cache")
	data := filepath.Join(f.Home, ".local", "share")
	tmp := filepath.Join(root, "tmp")
	for _, dir := range []string{f.Home, f.Bin, config, cache, data, tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	global, system := filepath.Join(root, "git-global"), filepath.Join(root, "git-system")
	for _, path := range []string{global, system} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f.base = map[string]string{
		"PATH": f.Bin, "HOME": f.Home, "USERPROFILE": f.Home,
		"XDG_CONFIG_HOME": config, "XDG_CACHE_HOME": cache, "XDG_DATA_HOME": data,
		"TMP": tmp, "TEMP": tmp, "TMPDIR": tmp,
		"GIT_CONFIG_GLOBAL": global, "GIT_CONFIG_SYSTEM": system, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_TEMPLATE_DIR": gitquiet.TemplateDir(t), "GIT_TERMINAL_PROMPT": "0",
		"KUBECONFIG": os.DevNull, "LC_ALL": "C", "TZ": "UTC",
	}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SystemRoot", "WINDIR", "COMSPEC", "PATHEXT"} {
			if value, ok := os.LookupEnv(key); ok {
				f.base[keyFor(key, runtime.GOOS)] = value
			}
		}
	}
	toolNames := make([]string, 0, len(opts.Tools))
	for name := range opts.Tools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)
	for _, name := range toolNames {
		source := opts.Tools[name]
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00=") || !filepath.IsAbs(source) {
			t.Fatalf("testenv: tool %q must name an absolute approved executable", name)
		}
		info, err := os.Stat(source)
		if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0) {
			t.Fatalf("testenv: tool %q is not an existing executable", name)
		}
		if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
			name += filepath.Ext(source)
		}
		dest := filepath.Join(f.Bin, name)
		if _, err := os.Lstat(dest); !os.IsNotExist(err) {
			t.Fatalf("testenv: duplicate tool name %q", name)
		}
		if err := os.Symlink(source, dest); err != nil {
			if runtime.GOOS != "windows" {
				t.Fatalf("testenv: link tool %q: %v", name, err)
			}
			body, readErr := os.ReadFile(source)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if err := os.WriteFile(dest, body, info.Mode().Perm()); err != nil {
				t.Fatal(err)
			}
		}
	}
	varKeys := make([]string, 0, len(opts.Vars))
	for key := range opts.Vars {
		varKeys = append(varKeys, key)
	}
	sort.Strings(varKeys)
	seen := map[string]bool{}
	for _, key := range varKeys {
		canonical := keyFor(key, runtime.GOOS)
		if seen[canonical] {
			t.Fatalf("testenv: duplicate environment key %q", key)
		}
		seen[canonical] = true
		f.overlay(f.base, key, opts.Vars[key])
	}
	// Clear all undeclared inputs, not just today's known Git/token variables.
	// Register both presence and value restoration through testing's own cleanup.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, keep := f.base[keyFor(key, runtime.GOOS)]; !keep {
			t.Setenv(key, "")
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, entry := range f.Env() {
		key, value, _ := strings.Cut(entry, "=")
		t.Setenv(key, value)
	}
	return f
}

func (f *Fixture) overlay(env map[string]string, key, value string) {
	f.t.Helper()
	if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) || reserved(key) {
		f.t.Fatalf("testenv: invalid or owned environment key %q", key)
	}
	env[keyFor(key, runtime.GOOS)] = value
}

// Env returns a fresh, deterministic environment. Extras deliberately override
// prior values; later process mutations never enter this frozen snapshot.
func (f *Fixture) Env(extra ...string) []string {
	f.t.Helper()
	env := make(map[string]string, len(f.base)+len(extra))
	for key, value := range f.base {
		env[key] = value
	}
	for _, entry := range extra {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			f.t.Fatalf("testenv: expected KEY=value")
		}
		f.overlay(env, key, value)
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+env[key])
	}
	return result
}
