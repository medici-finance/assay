package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func writeCustodyRoster(t *testing.T, home, slug string) string {
	t.Helper()
	config := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	body := "ASSAY_BLESS_LOGIN=example-human:1\nASSAY_TRUSTED_LOGINS=example-human:1\nASSAY_ALLOWED_REPOS=" + slug + "\nASSAY_REPO_FORGES=" + slug + "=github\n"
	if err := os.WriteFile(filepath.Join(config, "roster.env"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestDeskdCustodyOverridesCachedOperatorRosterAndRestores(t *testing.T) {
	operator := t.TempDir()
	operatorCfg := writeCustodyRoster(t, operator, "operator-org/repo")
	t.Setenv("HOME", operator)
	t.Setenv("USERPROFILE", operator)
	t.Setenv("ASSAY_CONFIG_HOME", operatorCfg)
	deskkit.SetToolClass(deskkit.ClassWrite)
	t.Cleanup(deskkit.ReloadConfig)
	if !deskkit.EffectiveConfig().Configured() {
		t.Fatal("operator fixture invalid")
	}
	wd, _ := os.Getwd()
	home := t.TempDir()
	c := &Cell{Name: "demo", Home: home, Config: writeCustodyRoster(t, home, "cell-org/project"), Repo: t.TempDir(), Env: envWith(nil)}
	restore := c.enterDeskCustody()
	t.Cleanup(restore) // A failed assertion must not strand later tests in this temporary directory.
	cfg := deskkit.EffectiveConfig()
	if !cfg.Configured() || cfg.RepoForges["cell-org/project"] != "github" || cfg.RepoForges["operator-org/repo"] != "" {
		t.Fatal("cell roster not selected")
	}
	if dirs := deskkit.ConfigHomeDirs(); len(dirs) != 1 || dirs[0] != c.Config {
		t.Fatalf("credential homes = %v", dirs)
	}
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	gotDir, gotErr := os.Stat(here)
	wantDir, wantErr := os.Stat(c.Repo)
	if gotErr != nil || wantErr != nil || !os.SameFile(gotDir, wantDir) {
		t.Fatalf("cwd = %s, want the cell repository %s (stat errors: %v, %v)", here, c.Repo, gotErr, wantErr)
	}
	restore()
	if os.Getenv("HOME") != operator || os.Getenv("ASSAY_CONFIG_HOME") != operatorCfg {
		t.Fatal("operator environment not restored")
	}
	if here, _ := os.Getwd(); here != wd {
		t.Fatal("cwd not restored")
	}
	if deskkit.EffectiveConfig().RepoForges["operator-org/repo"] != "github" {
		t.Fatal("roster cache not restored")
	}
	// A missing cell roster must never be answered by the configured operator.
	c.Home = t.TempDir()
	c.Config = filepath.Join(c.Home, ".config", "assay")
	restore = c.enterDeskCustody()
	defer restore()
	if deskkit.EffectiveConfig().Configured() {
		t.Fatal("missing cell roster inherited operator grants")
	}
}

func TestDeskdCoordinateUsesCellOriginNotCallerDirectory(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{{"init", repo}, {"-C", repo, "remote", "add", "origin", "git@github.com:example-org/actual-project.git"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s: %v", out, err)
		}
	}
	c := &Cell{Repo: repo, Env: envWith(nil)}
	if got := c.deskdRepo().Slug(); got != "example-org/actual-project" {
		t.Fatalf("coordinate = %s", got)
	}
	c.Env = envWith(map[string]string{"CELL_REPO_SLUG": "configured-org/explicit"})
	if got := c.deskdRepo().Slug(); got != "configured-org/explicit" {
		t.Fatalf("explicit coordinate = %s", got)
	}
	c.Env = envWith(map[string]string{"CELL_REPO_SLUG": "invalid"})
	assertDies(t, "invalid coordinate", func() { c.deskdRepo() })
	c.Env = envWith(nil)
	c.Repo = t.TempDir()
	assertDies(t, "missing origin", func() { c.deskdRepo() })
}

func TestDeskdMintUsesCellCustodyAndFailsClosed(t *testing.T) {
	operator := t.TempDir()
	t.Setenv("HOME", operator)
	t.Setenv("USERPROFILE", operator)
	t.Setenv("ASSAY_CONFIG_HOME", writeCustodyRoster(t, operator, "operator-org/repo"))
	deskkit.SetToolClass(deskkit.ClassWrite)
	t.Cleanup(deskkit.ReloadConfig)
	_ = deskkit.EffectiveConfig()
	home := t.TempDir()
	c := &Cell{Name: "demo", Home: home, Config: writeCustodyRoster(t, home, "cell-org/project"), Repo: t.TempDir(),
		Env: envWith(map[string]string{"CELL_REPO_SLUG": "cell-org/project", "ORGS": "cell-org,second-org"})}
	tokenPath := filepath.Join(t.TempDir(), "fixture-token")
	if err := os.WriteFile(tokenPath, []byte("fixture-token-value"), 0600); err != nil {
		t.Fatal(err)
	}
	var owners []string
	reset := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		if role != "desk" || os.Getenv("HOME") != c.Home || os.Getenv("ASSAY_CONFIG_HOME") != c.Config {
			t.Fatal("mint received wrong custody")
		}
		owners = append(owners, owner)
		return tokenPath, "", nil
	})
	defer reset()
	restore := c.enterDeskCustody()
	defer restore()
	env := c.deskdMintGitHub()
	if len(env) != 2 || len(owners) != 2 || owners[0] != "cell-org" || owners[1] != "second-org" {
		t.Fatalf("mint owners %v, env count %d", owners, len(env))
	}
	if err := os.Remove(filepath.Join(c.Config, "roster.env")); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	assertDies(t, "missing cell roster", func() { c.deskdMintGitHub() })
	if len(owners) != 2 {
		t.Fatal("missing roster reached custody")
	}
	// A contradictory forge entry must stop before any GitHub minter call.
	config := writeCustodyRoster(t, home, "cell-org/project")
	file := filepath.Join(config, "roster.env")
	body, _ := os.ReadFile(file)
	body = []byte(strings.ReplaceAll(string(body), "=github", "=gitlab"))
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	assertDies(t, "forge mismatch", func() { c.deskdMintGitHub() })
	if len(owners) != 2 {
		t.Fatal("forge mismatch reached GitHub custody")
	}
}

func TestDeskHelperEnvReplacesCustodyWithoutChangingParent(t *testing.T) {
	parent := []string{"HOME=operator", "HOME=duplicate", "home=case-variant", "DESK_ROOTS=operator/repo=/operator", "KEEP=value"}
	before := append([]string(nil), parent...)
	c := &Cell{Home: "/cell/home", Config: "/cell/config", Env: envWith(nil)}
	got := c.deskHelperEnv(parent, "worker-desk", "demo-worker-codex")
	if !reflect.DeepEqual(parent, before) {
		t.Fatal("helper custody changed parent slice")
	}
	counts := map[string]int{}
	for _, pair := range got {
		key, value, _ := strings.Cut(pair, "=")
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		counts[key]++
		switch key {
		case "HOME", "USERPROFILE":
			if value != c.Home {
				t.Fatal("helper inherited operator home")
			}
		case "DESK_ROOTS":
			if value != "" {
				t.Fatal("empty cell roots inherited operator roots")
			}
		case "KEEP":
			if value != "value" {
				t.Fatal("unrelated parent key changed")
			}
		}
	}
	for _, key := range []string{"HOME", "USERPROFILE", "ASSAY_CONFIG_HOME", "DESK_LOOP", "DESK_SESSION", "DESK_ROOTS", "KEEP"} {
		if counts[key] != 1 {
			t.Fatalf("%s count = %d", key, counts[key])
		}
	}
	if runtime.GOOS != "windows" && counts["home"] != 1 {
		t.Fatal("Unix case-sensitive key removed")
	}
}
