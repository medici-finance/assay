package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/packet"
)

// usePacketBuilder undoes the harness's default (no packet) and pins the build time, so a
// test drives the real builder. Call it AFTER stub.install.
func usePacketBuilder(t *testing.T) time.Time {
	t.Helper()
	old := buildPacketFn
	buildPacketFn = buildDispatchPacket
	t.Cleanup(func() { buildPacketFn = old })
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	oldNow := packetNow
	packetNow = func() time.Time { return at }
	t.Cleanup(func() { packetNow = oldNow })
	return at
}

// withPacketProvider binds a provider to kit for one test, restoring whatever was there.
func withPacketProvider(t *testing.T, kit string, p packetProvider) {
	t.Helper()
	old, had := packetProviders[kit]
	packetProviders[kit] = p
	t.Cleanup(func() {
		if had {
			packetProviders[kit] = old
		} else {
			delete(packetProviders, kit)
		}
	})
}

func fixedPacket(head string) packetProvider {
	return func(packetInput) (packet.Spec, error) {
		return packet.Spec{Head: head, Sections: []packet.Section{
			packet.NewSection("Change", func() (packet.Content, error) {
				var c packet.Content
				c.Text("- number: 7")
				c.Untrusted("description", []byte("author text\n"))
				return c, nil
			}),
		}}, nil
	}
}

// packetLines returns the assignment's `Packet:` lines — the lines, in the part of the
// prompt above the standing clauses, that start with the prefix a kit clause keys on.
func packetLines(t *testing.T, prompt string) []string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(assignmentSection(t, prompt), "\n") {
		if strings.HasPrefix(l, packet.AssignmentPrefix) {
			out = append(out, l)
		}
	}
	return out
}

// TestPacketWrittenBesideTheAssignmentWithOneLine: a kit with a provider gets ONE owner-only
// file beside its prompt file, with the head commit and the build time at the top, and the
// assignment gains exactly one `Packet: <absolute path>` line.
func TestPacketWrittenBesideTheAssignmentWithOneLine(t *testing.T) {
	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	withPacketProvider(t, "worker", fixedPacket("0123abcd"))
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	usePacketBuilder(t)
	rc := run([]string{"example-stream--07", "--root", root, "--kit", "worker", "--tier", "strong",
		"--prompt-file", promptFile, "--quiet"})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	want := filepath.Join(filepath.Dir(promptFile), "prompt.packet.md")
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("no packet beside the prompt file: %v", err)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(want); st.Mode().Perm() != 0o600 {
			t.Errorf("packet mode = %v, want 0600", st.Mode().Perm())
		}
	}
	for _, w := range []string{"# Dispatch packet — worker — ", "- **Head commit:** `0123abcd`",
		"- **Built:** 2026-03-04T05:06:07Z", "## Change", "author text"} {
		if !strings.Contains(string(body), w) {
			t.Errorf("packet lacks %q:\n%s", w, body)
		}
	}
	prompt, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := packetLines(t, string(prompt)); len(got) != 1 || got[0] != "Packet: "+want {
		t.Fatalf("assignment `Packet:` lines = %q, want exactly [%q]", got, "Packet: "+want)
	}
	if n := strings.Count(string(prompt), packet.AssignmentPrefix+want); n != 1 {
		t.Errorf("the packet path appears %d times in the prompt, want 1", n)
	}
}

// TestPacketBuildFailureNeverFailsTheDispatch is the table of ways a packet can fail to
// exist. In every one the dispatch still exits 0 with its claim, worktree and prompt, the
// assignment carries NO `Packet:` line, and — unless the provider declined quietly —
// stderr says why even under --quiet.
func TestPacketBuildFailureNeverFailsTheDispatch(t *testing.T) {
	failing := packet.NewSection("A", func() (packet.Content, error) { return packet.Content{}, errors.New("unreadable") })
	cases := []struct {
		name     string
		provider packetProvider
		cacheErr bool // print the prompt (no --prompt-file) and make the cache dir unusable
		wantSaid string
		silent   bool
	}{
		{name: "the provider fails", provider: func(packetInput) (packet.Spec, error) {
			return packet.Spec{}, errors.New("the forge would not answer")
		}, wantSaid: "the forge would not answer"},
		{name: "the provider panics", provider: func(packetInput) (packet.Spec, error) {
			panic("index out of range")
		}, wantSaid: "stopped unexpectedly: index out of range"},
		{name: "the provider declines", provider: func(packetInput) (packet.Spec, error) {
			return packet.Spec{}, errNoPacket
		}, silent: true},
		{name: "no head commit", provider: fixedPacket(""), wantSaid: "no head commit"},
		{name: "every section fails", provider: func(packetInput) (packet.Spec, error) {
			return packet.Spec{Head: "abc", Sections: []packet.Section{failing}}, nil
		}, wantSaid: "no section could be built"},
		{name: "the head moved during the build", provider: func(in packetInput) (packet.Spec, error) {
			s, _ := fixedPacket("abc")(in)
			s.Recheck = func() error { return errors.New("the head moved from abc to def while the packet was read") }
			return s, nil
		}, wantSaid: "the head moved from abc to def"},
		{name: "a section panics", provider: func(packetInput) (packet.Spec, error) {
			return packet.Spec{Head: "abc", Sections: []packet.Section{packet.NewSection("A", func() (packet.Content, error) {
				var files []string
				_ = files[3]
				return packet.Content{}, nil
			})}}, nil
		}, wantSaid: "stopped unexpectedly"},
		{name: "nowhere to write it", provider: fixedPacket("abc"), cacheErr: true, wantSaid: "no place to write it"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withPacketProvider(t, "worker", tc.provider)
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
			usePacketBuilder(t)
			dir := t.TempDir()
			promptFile := filepath.Join(dir, "prompt.md")
			args := []string{"example-stream--07", "--root", root, "--kit", "worker", "--tier", "strong", "--quiet"}
			var prompt string
			var rc int
			var stderr string
			if tc.cacheErr {
				old := packetCacheDir
				packetCacheDir = func() (string, error) { return "", errors.New("no cache directory here") }
				t.Cleanup(func() { packetCacheDir = old })
				prompt = captureStdout(t, func() { rc, stderr = runCapturingStderr(t, args) })
			} else {
				rc, stderr = runCapturingStderr(t, append(args, "--prompt-file", promptFile))
				raw, err := os.ReadFile(promptFile)
				if err != nil {
					t.Fatalf("the prompt was not written: %v", err)
				}
				prompt = string(raw)
			}
			if rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0 — a packet failure must not fail the dispatch\n%s", rc, stderr)
			}
			if !s.ran("dispatch-claim.sh acquire example-stream--07") || s.ran("dispatch-claim.sh release") {
				t.Errorf("the claim was not left held by the dispatch: %v", s.calls)
			}
			if got := packetLines(t, prompt); len(got) != 0 {
				t.Errorf("assignment carries %q with no packet behind it", got)
			}
			if strings.Contains(prompt, packet.AssignmentPrefix) {
				t.Errorf("the prompt mentions a packet that does not exist")
			}
			if _, err := os.Stat(filepath.Join(dir, "prompt.packet.md")); !os.IsNotExist(err) {
				t.Errorf("a packet file was left behind (stat err = %v)", err)
			}
			said := strings.Contains(stderr, "packet: NOT built")
			if tc.silent && said {
				t.Errorf("a declined packet was reported: %s", stderr)
			}
			if !tc.silent && (!said || !strings.Contains(stderr, tc.wantSaid)) {
				t.Errorf("stderr does not say why there is no packet (want %q):\n%s", tc.wantSaid, stderr)
			}
		})
	}
}

// TestPacketNotBuiltOnDryRunOrWithoutAProvider: --dry-run touches nothing, and a kit with no
// provider gets no packet, no line and no message.
func TestPacketNotBuiltOnDryRunOrWithoutAProvider(t *testing.T) {
	t.Run("dry run", func(t *testing.T) {
		called := 0
		withPacketProvider(t, "worker", func(in packetInput) (packet.Spec, error) {
			called++
			return fixedPacket("abc")(in)
		})
		s := &stub{}
		_, root := s.install(t)
		plantScripts(t, root)
		usePacketBuilder(t)
		promptFile := filepath.Join(t.TempDir(), "prompt.md")
		rc, _ := runCapturingStderr(t, []string{"example-stream--07", "--root", root, "--repo", allowedRepo,
			"--kit", "worker", "--tier", "strong", "--dry-run", "--prompt-file", promptFile})
		if rc != deskkit.ExitOK || called != 0 {
			t.Fatalf("rc = %d, provider calls = %d; a dry run builds no packet", rc, called)
		}
		if prompt, _ := os.ReadFile(promptFile); strings.Contains(string(prompt), packet.AssignmentPrefix) {
			t.Error("a dry-run prompt names a packet")
		}
	})
	t.Run("no provider", func(t *testing.T) {
		old, had := packetProviders["worker"]
		delete(packetProviders, "worker")
		t.Cleanup(func() {
			if had {
				packetProviders["worker"] = old
			}
		})
		s := &stub{}
		_, root := s.install(t)
		plantScripts(t, root)
		s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
		usePacketBuilder(t)
		promptFile := filepath.Join(t.TempDir(), "prompt.md")
		rc, stderr := runCapturingStderr(t, []string{"example-stream--07", "--root", root, "--kit", "worker",
			"--tier", "strong", "--prompt-file", promptFile})
		if rc != deskkit.ExitOK || strings.Contains(stderr, "packet:") {
			t.Fatalf("rc = %d; a kit with no provider must be silent about packets:\n%s", rc, stderr)
		}
		prompt, _ := os.ReadFile(promptFile)
		if strings.Contains(string(prompt), packet.AssignmentPrefix) {
			t.Error("the prompt names a packet nothing built")
		}
	})
}

// TestPacketPathFor is the placement table.
func TestPacketPathFor(t *testing.T) {
	cache := t.TempDir()
	old := packetCacheDir
	packetCacheDir = func() (string, error) { return cache, nil }
	t.Cleanup(func() { packetCacheDir = old })
	dir := t.TempDir()
	cases := []struct {
		name, promptFile, claimKey, want string
		wantErr                          bool
	}{
		{name: "beside the prompt file", promptFile: filepath.Join(dir, "prompt.md"), claimKey: "k",
			want: filepath.Join(dir, "prompt.packet.md")},
		{name: "a prompt file with no extension", promptFile: filepath.Join(dir, "assignment"), claimKey: "k",
			want: filepath.Join(dir, "assignment.packet.md")},
		{name: "printed prompt goes to the cache directory", claimKey: "example--pr-7--security",
			want: filepath.Join(cache, "assay", "packets", "example--pr-7--security.packet.md")},
		{name: "a claim key that is not a file name", claimKey: "../../etc/x", wantErr: true},
		{name: "an empty claim key", claimKey: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := packetPathFor(dispatchOpts{promptFile: tc.promptFile}, dispatchPlan{claimKey: tc.claimKey})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got %q, want an error", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// TestPrunePacketsRemovesOnlyOldPackets: the cache directory is swept of this tool's own
// stale packets and of nothing else.
func TestPrunePacketsRemovesOnlyOldPackets(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	write := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		at := now.Add(-age)
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
		return p
	}
	stale := write("a.packet.md", 8*24*time.Hour)
	fresh := write("b.packet.md", time.Hour)
	other := write("notes.txt", 30*24*time.Hour)
	sub := filepath.Join(dir, "old.packet.md")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	prunePackets(dir, now.Add(-packetMaxAge))
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a week-old packet was kept")
	}
	for _, keep := range []string{fresh, other, sub} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s was removed: %v", filepath.Base(keep), err)
		}
	}
}

// TestRegisterPacketProviderRefusesASecondProvider: two files claiming one kit is a build
// defect that must not resolve silently to whichever init ran last.
func TestRegisterPacketProviderRefusesASecondProvider(t *testing.T) {
	const kit = "example-kit-for-the-registry-test"
	t.Cleanup(func() { delete(packetProviders, kit) })
	registerPacketProvider(" Example-Kit-For-The-Registry-Test ", fixedPacket("abc"))
	if _, ok := packetProviders[kit]; !ok {
		t.Fatal("the kit key was not normalised")
	}
	defer func() {
		if recover() == nil {
			t.Error("a second provider for the same kit was accepted")
		}
	}()
	registerPacketProvider(kit, fixedPacket("def"))
}
