//go:build unix

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

// The fake cockpits execute only the service command, never a desk/model. Their
// JSON responses follow the CLI contracts, including Orca's RuntimeRpcSuccess:
// stablyai/orca at 2b0ce175, src/cli/index-terminal-commands.test.ts.
func commsUpFixture(t *testing.T) (*repairFixture, *Cell, *deskCommsConfig, string, []string) {
	t.Helper()
	f := newRepairFixture(t)
	c, cfg, manifest := commsFixture(t)
	if err := os.Chmod(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	c.Dir, c.Session = f.cellDir, "example-cell"
	f.writeCellEnv(t, "ROLES=the-desk\n")
	bin := filepath.Join(t.TempDir(), "tools with spaces")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(bin, "calls")
	starts := filepath.Join(bin, "starts")
	for _, name := range []string{"commsgw", "commsloop"} {
		script := "#!/bin/sh\nprintf '%s\\n' " + cockpitQuote(name) + " >> " + cockpitQuote(starts) + "\nexec /bin/sleep 300\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$COMMS_CALLS"
case "$1 $2" in
 "has-session -t") test -f "$COMMS_CALLS.session"; exit $?;;
 "new-session -d") touch "$COMMS_CALLS.session"; exit 0;;
 "kill-session -t") rm -f "$COMMS_CALLS.session"; exit 0;;
 "tab --help") echo "create close list"; exit 0;;
 "pane --help") echo "run"; exit 0;;
 "workspace --help") echo "list create"; exit 0;;
 "workspace list") echo '{"result":{"workspaces":[{"workspace_id":"ws"}]}}'; exit 0;;
 "tab create") for word in "$@"; do label="$word"; done; case "$label" in *-comms-*) printf '%s' "$label" > "$COMMS_CALLS.label";; *) echo '{"result":{"tab":{"tab_id":"other-tab"},"root_pane":{"pane_id":"other-pane"}}}'; exit 0;; esac; echo '{"result":{"tab":{"tab_id":"owned-tab"},"root_pane":{"pane_id":"pane"}}}'; exit 0;;
 "tab list") printf '{"result":{"tabs":[{"tab_id":"owned-tab","label":"%s"}]}}\n' "$(cat "$COMMS_CALLS.label")"; exit 0;;
 "list-windows -a") printf '@42\t%s\n' "$(cat "$COMMS_CALLS.label")"; exit 0;;
 "terminal show") printf '{"ok":true,"result":{"terminal":{"handle":"term-owned","title":"%s"}},"_meta":{"runtimeId":"runtime"}}\n' "$(cat "$COMMS_CALLS.label")"; exit 0;;
 "terminal --help") echo "create close"; exit 0;;
 "terminal close") if test "$3" = --help; then echo '--terminal --worktree --all'; fi; exit 0;;
 "repo --help") echo add; exit 0;;
 "automations create") if test "$3" = --help; then echo '--name --repo --trigger --precheck --prompt'; fi; exit 0;;
esac
if test "$1 $2 $3" = 'terminal create --help'; then
 echo '--worktree --command --title --json'; exit 0
fi
command=''
if test "$1" = new-window; then
 last=''
 for word in "$@"; do
  if test "$last" = -n; then printf '%s' "$word" > "$COMMS_CALLS.label"; fi
  last="$word"; command="$word"
 done
 echo '@42'
elif test "$1 $2" = 'pane run'; then command="$4"
elif test "$1 $2" = 'terminal create'; then
 while test "$#" -gt 0; do
  if test "$1" = --command; then shift; command="$1"; fi
  if test "$1" = --title; then shift; printf '%s' "$1" > "$COMMS_CALLS.label"; fi
  shift
 done
 echo '{"id":"request","ok":true,"result":{"terminal":{"handle":"term-owned"}},"_meta":{"runtimeId":"runtime"}}'
fi
case "$command" in
 *" comms "*)
  /bin/sh -c "$command" > "$COMMS_CALLS.service" 2>&1 &
  ;;
esac
exit 0
`
	for _, cockpit := range []string{"tmux", "herdr", "orca"} {
		if err := os.WriteFile(filepath.Join(bin, cockpit), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	manifestSpace := filepath.Join(filepath.Dir(manifest), "comms config's $literal.json")
	writeCommsFixture(t, manifestSpace, cfg)
	r := f.run(t, nil, "set", "example", "CELL_COMMS_CONFIG="+manifestSpace)
	if r.code != 0 {
		t.Fatalf("setter: %+v", r)
	}
	loaded := envWith(map[string]string{})
	if err := parseCellEnv(loaded, filepath.Join(f.cellDir, "cell.env")); err != nil || loaded.Get("CELL_COMMS_CONFIG") != manifestSpace {
		t.Fatalf("setter did not round-trip path: %v", err)
	}
	env := []string{"PATH=" + bin + ":/usr/bin:/bin", "DESK_TOOLS_BIN=" + bin, "COMMS_CALLS=" + log}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(c.commsDir(), "STOP"), []byte("test cleanup"), 0600)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			l, err := cellcadence.Acquire(c.commsDir())
			if err == nil {
				l.Close()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
	return f, c, cfg, log, env
}

func TestCommsUpDown(t *testing.T) {
	for _, cockpit := range cockpitValues {
		if cockpit == "auto" {
			continue
		}
		t.Run(cockpit, func(t *testing.T) {
			f, c, cfg, log, env := commsUpFixture(t)
			args := []string{"up", "example", "--cockpit", cockpit, "--no-attach", "--no-the-desk"}
			if r := f.run(t, append(env, "DRY_RUN=1"), args...); r.code != 0 || strings.Count(r.stdout, "[dry-run] comms:") != 1 || !strings.Contains(r.stdout, "--cells-root") {
				t.Fatalf("dry run: %+v", r)
			}
			if _, err := os.Stat(log); !os.IsNotExist(err) {
				b, _ := os.ReadFile(log)
				if strings.Contains(string(b), "comms 'example' run") {
					t.Fatal("dry run created a service")
				}
			}
			if r := f.run(t, env, args...); r.code != 0 {
				t.Fatalf("up: %+v", r)
			}
			// Wait for both actual supervised stand-in processes, not just checkpoint creation.
			starts := filepath.Join(filepath.Dir(log), "starts")
			until := time.Now().Add(3 * time.Second)
			for {
				b, _ := os.ReadFile(starts)
				if len(strings.Fields(string(b))) == 2 {
					break
				}
				if time.Now().After(until) {
					t.Fatalf("gateway/drain not started: %s", b)
				}
				time.Sleep(10 * time.Millisecond)
			}
			if r := f.run(t, env, args...); r.code != 0 || !strings.Contains(r.stdout, "reusing the running service") {
				t.Fatalf("repeat up: %+v", r)
			}
			b, _ := os.ReadFile(starts)
			if len(strings.Fields(string(b))) != 2 {
				t.Fatalf("duplicate pair: %s", b)
			}
			if err := os.MkdirAll(cfg.QueueDir, 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"queued", "held", "acknowledged"} {
				if err := os.WriteFile(filepath.Join(cfg.QueueDir, name), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if r := f.run(t, append(env, "DRY_RUN=1"), "down", "example", "--cockpit", cockpit); r.code != 0 {
				t.Fatalf("dry down: %+v", r)
			}
			if owned, err := c.commsOwned(); !owned || err != nil {
				t.Fatalf("dry down stopped service: %v", err)
			}
			// Removing configuration cannot orphan the old supervisor.
			if r := f.run(t, env, "set", "example", "CELL_COMMS_CONFIG="); r.code != 0 {
				t.Fatal(r)
			}
			if r := f.run(t, env, "down", "example", "--cockpit", cockpit); r.code != 0 {
				t.Fatalf("down: %+v", r)
			}
			state, err := c.commsState()
			if err != nil || state.Running || state.Outcome != "ok" {
				t.Fatalf("unclean checkpoint: %+v %v", state, err)
			}
			for _, name := range []string{"queued", "held", "acknowledged"} {
				if b, err := os.ReadFile(filepath.Join(cfg.QueueDir, name)); err != nil || string(b) != "keep" {
					t.Fatalf("queue lost: %s %v", name, err)
				}
			}
			calls, _ := os.ReadFile(log)
			want := map[string]string{"tmux": "kill-window -t @42", "herdr": "tab close owned-tab", "orca": "terminal close --terminal term-owned"}[cockpit]
			if !strings.Contains(string(calls), want) {
				t.Fatalf("owned surface not closed: %s", calls)
			}
		})
	}
}

func TestCommsUpPreflight(t *testing.T) {
	for _, mode := range []string{"unset", "disabled", "bad", "missing-binary", "stale"} {
		t.Run(mode, func(t *testing.T) {
			f, c, cfg, log, env := commsUpFixture(t)
			e := envWith(map[string]string{})
			_ = parseCellEnv(e, filepath.Join(f.cellDir, "cell.env"))
			manifest := e.Get("CELL_COMMS_CONFIG")
			switch mode {
			case "unset":
				f.run(t, env, "set", "example", "CELL_COMMS_CONFIG=")
			case "disabled":
				cfg.Mode = "disabled"
				writeCommsFixture(t, manifest, cfg)
			case "bad":
				cfg.Mode = "full"
				writeCommsFixture(t, manifest, cfg)
			case "missing-binary":
				os.Remove(filepath.Join(filepath.Dir(log), "commsgw"))
			case "stale":
				l, err := cellcadence.Acquire(c.commsDir())
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now().UTC()
				b, _ := json.Marshal(cellcadence.State{Schema: cellcadence.Schema, Mode: "interactive", Cell: c.Name, Role: "comms", PID: 1, Running: true, Sequence: 1, NextDue: now, Heartbeat: now, LastStart: now, Outcome: "running"})
				os.WriteFile(filepath.Join(c.commsDir(), "checkpoint.json"), b, 0600)
				l.Close()
			}
			r := f.run(t, env, "up", "example", "--cockpit", "tmux", "--no-the-desk", "--no-attach")
			if mode == "unset" || mode == "disabled" {
				if r.code != 0 {
					t.Fatal(r)
				}
			} else if r.code == 0 {
				t.Fatalf("invalid config/state started: %+v", r)
			}
			b, _ := os.ReadFile(log)
			if strings.Contains(string(b), " comms ") {
				t.Fatalf("service launched: %s", b)
			}
			if mode != "unset" && mode != "disabled" && strings.Contains(string(b), "new-session") {
				t.Fatalf("partial startup before refusal: %s", b)
			}
		})
	}
}

func TestCommsOrcaAutomation(t *testing.T) {
	f, c, _, log, env := commsUpFixture(t)
	f.writeCellEnv(t, "ROLES=the-desk\nCELL_HARNESS=claude\n")
	_, cfg, manifest := commsFixture(t)
	if err := os.Chmod(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	cfg.Cell = "example"
	writeCommsFixture(t, manifest, cfg)
	f.run(t, env, "set", "example", "CELL_COMMS_CONFIG="+manifest)
	r := f.run(t, env, "up", "example", "--cockpit", "orca", "--automate", "hourly", "--no-attach")
	if r.code != 0 {
		t.Fatal(r)
	}
	b, _ := os.ReadFile(log)
	if strings.Count(string(b), "terminal create --worktree") != 1 || strings.Count(string(b), "automations create --name") != 1 {
		t.Fatalf("service duplicated into schedule: %s", b)
	}
	if r := f.run(t, env, "down", "example", "--cockpit", "orca"); r.code != 0 {
		t.Fatal(r)
	}
	if s, err := c.commsState(); err != nil || s.Running {
		t.Fatalf("service survived: %+v %v", s, err)
	}
}

func TestCommsForeignSurface(t *testing.T) {
	for _, cockpit := range []string{"tmux", "herdr", "orca"} {
		t.Run(cockpit, func(t *testing.T) {
			f, _, _, log, env := commsUpFixture(t)
			if r := f.run(t, env, "up", "example", "--cockpit", cockpit, "--no-attach", "--no-the-desk"); r.code != 0 {
				t.Fatal(r)
			}
			if err := os.WriteFile(log+".label", []byte("unrelated-terminal"), 0600); err != nil {
				t.Fatal(err)
			}
			r := f.run(t, env, "down", "example", "--cockpit", cockpit)
			if r.code == 0 || !strings.Contains(r.stderr, "identity changed") {
				t.Fatalf("foreign terminal accepted: %+v", r)
			}
			b, _ := os.ReadFile(log)
			for _, bad := range []string{"kill-window -t @42", "tab close owned-tab", "terminal close --terminal term-owned"} {
				if strings.Contains(string(b), bad) {
					t.Fatalf("foreign terminal closed: %s", b)
				}
			}
		})
	}
}

func TestCommsArgvRoundTrip(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "cellctl with ' and $.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	c := &Cell{Name: "example", Dir: filepath.Join(dir, "root with ' and $", "example")}
	out, err := exec.Command("/bin/sh", "-c", c.commsCmdIn(shellPOSIX, script)).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--cells-root", filepath.Dir(c.Dir), "comms", "example", "run"}
	if string(out) != strings.Join(want, "\n")+"\n" {
		t.Fatalf("argv changed: %s", out)
	}
}
