package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeHooksFile plants a hooks.yaml in the state directory (the dirOverride setup() points
// deskkit at). This is the ONE place hooks are ever read from.
func writeHooksFile(t *testing.T, stateDir, body string) {
	t.Helper()
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "hooks.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write hooks.yaml: %v", err)
	}
}

// TestHooksIgnoreItemTreeFile is the negative control behind the source rule: a hooks.yaml
// placed in the cwd, in a worktree, or in a repo root is NEVER read. Only <StateDir>/hooks.yaml
// is. This is the layer that makes the state-dir-only rule real — an untrusted head that
// drops a hooks.yaml into the item's tree must not get it executed.
func TestHooksIgnoreItemTreeFile(t *testing.T) {
	stateDir := setup(t) // dirOverride → this dir; it has NO hooks.yaml
	poison := "after_create: touch " + filepath.Join(t.TempDir(), "SHOULD-NOT-EXIST") + "\n" +
		"before_run: exit 7\nafter_run: exit 7\nbefore_remove: exit 7\n"

	// Plant the poison file in three item-tree locations a caller might sit in.
	for _, name := range []string{"cwd", "worktree", "reporoot"} {
		dir := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "hooks.yaml"), []byte(poison), 0o600); err != nil {
			t.Fatal(err)
		}
		// Even when the hook runs WITH that dir as its worktree (cwd), the loader must not
		// have read the poison from it.
		ran, err := RunHook(HookBeforeRun, HookEnv{Worktree: dir})
		if err != nil {
			t.Fatalf("%s: RunHook returned an error, so a hooks.yaml in the item tree was read: %v", name, err)
		}
		if ran {
			t.Fatalf("%s: a hook RAN — the item-tree hooks.yaml was executed; the source rule is broken", name)
		}
	}

	// And with an explicit (harmless) hooks.yaml in the STATE dir, the state-dir one IS the
	// one that governs — proving the loader reads that path, not the item tree.
	writeHooksFile(t, stateDir, "before_run: exit 0\n")
	ran, err := RunHook(HookBeforeRun, HookEnv{Worktree: t.TempDir()})
	if err != nil {
		t.Fatalf("state-dir before_run should have exited 0: %v", err)
	}
	if !ran {
		t.Fatal("state-dir before_run should have RUN")
	}
}

// TestHookTimeoutKills proves a hung hook is killed and reported, rather than wedging the
// caller forever.
func TestHookTimeoutKills(t *testing.T) {
	stateDir := setup(t)
	writeHooksFile(t, stateDir, "before_run: sleep 30\ntimeout_ms: 200\n")

	start := time.Now()
	ran, err := RunHook(HookBeforeRun, HookEnv{})
	elapsed := time.Since(start)

	if !ran {
		t.Fatal("the hook should be recorded as having RUN before it timed out")
	}
	if err == nil {
		t.Fatal("a hook that outran its timeout must return an error, not succeed")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error does not name the timeout: %v", err)
	}
	// It must have been KILLED near the 200ms budget, not run to the 30s sleep.
	if elapsed.Seconds() > 5 {
		t.Fatalf("the hook was not killed at its timeout — it ran %s", elapsed)
	}
}

// TestHookTimeoutKillsProcessTree is the harder half of the timeout control, and the one
// that fails on a host whose /bin/sh does NOT exec the last command of `sh -c`.
//
// TestHookTimeoutKills above writes `sleep 30`, which BSD/macOS sh turns into an exec: the
// shell process IS the sleep, so killing the one process we started ends everything and the
// test passes even when the implementation only kills that one process. Debian's dash — the
// /bin/sh in the Linux CI image — forks instead, so `sleep 30` is a GRANDCHILD. Two separate
// things then go wrong if the hook is not run as a process group:
//
//   - the grandchild inherits the write end of the stdout/stderr pipe os/exec created for the
//     bytes.Buffer, so cmd.Wait blocks on the copying goroutine until the GRANDCHILD exits —
//     the hook "times out" after the full 30s, 150x its 200ms budget; and
//   - the grandchild is never signalled at all, so a hung hook leaks a live process per run.
//
// The script here reproduces both on ANY sh: the `&` guarantees a separate process, and the
// marker file it would create proves whether that process survived the timeout.
func TestHookTimeoutKillsProcessTree(t *testing.T) {
	stateDir := setup(t)
	marker := filepath.Join(t.TempDir(), "survivor")
	// A backgrounded child (which outlives a kill aimed only at the shell) plus a foreground
	// sleep to hold the hook open. Non-interactive sh runs both in the shell's OWN process
	// group, so a group-wide kill reaches them and a single-process kill does not.
	writeHooksFile(t, stateDir,
		"before_run: (sleep 1; touch "+marker+") & sleep 30\ntimeout_ms: 200\n")

	start := time.Now()
	ran, err := RunHook(HookBeforeRun, HookEnv{})
	elapsed := time.Since(start)

	if !ran {
		t.Fatal("the hook should be recorded as having RUN before it timed out")
	}
	if err == nil {
		t.Fatal("a hook that outran its timeout must return an error, not succeed")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error does not name the timeout: %v", err)
	}
	// (1) Wait must not block on a pipe the grandchild still holds open.
	if elapsed.Seconds() > 5 {
		t.Fatalf("RunHook blocked past the timeout waiting on an inherited pipe — it took %s", elapsed)
	}
	// (2) The whole tree must be dead. The child would touch the marker at ~1s; give it
	// well past that and require that it never got there.
	time.Sleep(2 * time.Second)
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatalf("a child of the hook SURVIVED the timeout and ran on (it created %s) — "+
			"the kill reached only the shell, not the process group", marker)
	}
}

// TestHookEnvScrubsSecrets proves a GH_TOKEN (and other secret-shaped names) set in the
// caller's environment does NOT reach the hook, while the fixed ASSAY_* vars and ordinary
// vars DO. A hook is operator shell, but it must never be the leak path for an App token.
func TestHookEnvScrubsSecrets(t *testing.T) {
	stateDir := setup(t)
	wt := t.TempDir()
	dump := filepath.Join(wt, "envdump")

	// Secret-shaped names that must be scrubbed, and one ordinary name that must survive.
	t.Setenv("GH_TOKEN", "fake-should-be-scrubbed") // scrub keys on the NAME; value is a harmless placeholder (no real token prefix)
	t.Setenv("MY_SECRET_KEY", "shhh")
	t.Setenv("APP_PEM", "-----BEGIN-----")
	t.Setenv("SOME_TOKEN_VALUE", "tok")
	t.Setenv("ORDINARY_VAR", "keepme")

	writeHooksFile(t, stateDir, "before_run: env > "+dump+"\n")

	ran, err := RunHook(HookBeforeRun, HookEnv{RunKey: "k1", Worktree: wt, Repo: "o/n", Role: "worker"})
	if err != nil {
		t.Fatalf("hook failed: %v", err)
	}
	if !ran {
		t.Fatal("hook did not run")
	}
	b, rerr := os.ReadFile(dump)
	if rerr != nil {
		t.Fatalf("read env dump: %v", rerr)
	}
	env := string(b)

	for _, dropped := range []string{"GH_TOKEN=", "MY_SECRET_KEY=", "APP_PEM=", "SOME_TOKEN_VALUE="} {
		if strings.Contains(env, dropped) {
			t.Errorf("secret-shaped var leaked into the hook environment: %s", dropped)
		}
	}
	for _, kept := range []string{
		"ORDINARY_VAR=keepme",
		"ASSAY_RUN_KEY=k1",
		"ASSAY_WORKTREE=" + wt,
		"ASSAY_REPO=o/n",
		"ASSAY_ROLE=worker",
		"ASSAY_HOOK=" + HookBeforeRun,
	} {
		if !strings.Contains(env, kept) {
			t.Errorf("expected var missing from the hook environment: %q", kept)
		}
	}
}

// TestHookFailureClass pins the Symphony-mirrored fatality of each hook name, so a caller
// reads the class from one place.
func TestHookFailureClass(t *testing.T) {
	fatal := map[string]bool{
		HookAfterCreate:  true,
		HookBeforeRun:    true,
		HookAfterRun:     false,
		HookBeforeRemove: false,
	}
	for name, want := range fatal {
		if got := HookFatalOnFailure(name); got != want {
			t.Errorf("HookFatalOnFailure(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestHooksAbsentIsNoop proves the zero-config case: no hooks.yaml ⇒ every hook a no-op,
// no error.
func TestHooksAbsentIsNoop(t *testing.T) {
	setup(t) // state dir exists conceptually but carries no hooks.yaml
	for _, name := range []string{HookAfterCreate, HookBeforeRun, HookAfterRun, HookBeforeRemove} {
		ran, err := RunHook(name, HookEnv{})
		if err != nil {
			t.Errorf("%s: absent hooks.yaml should be no error, got %v", name, err)
		}
		if ran {
			t.Errorf("%s: absent hooks.yaml should be a no-op, but it RAN", name)
		}
	}
}

// TestHooksMalformedFailsClosed proves a hooks.yaml the loader cannot parse is Unverifiable,
// never silently treated as empty.
func TestHooksMalformedFailsClosed(t *testing.T) {
	stateDir := setup(t)
	writeHooksFile(t, stateDir, "before_run: [this is: not, valid\n")
	_, err := RunHook(HookBeforeRun, HookEnv{})
	if err == nil {
		t.Fatal("a malformed hooks.yaml must fail closed, not be read as empty")
	}
	if !IsUnverifiable(err) {
		t.Fatalf("malformed hooks.yaml should be Unverifiable (exit 6), got %v", err)
	}
}
