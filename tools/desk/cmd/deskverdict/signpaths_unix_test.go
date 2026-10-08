//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Keyless-compose brief, Verify row 12 — on every --payload call the signer reads only a regular file
// reached without following a link, from a parent directory no other user can write, and
// never replaces or writes through its .out sibling.

// signDir is a fresh t.TempDir chmodded to 0700 (a chmod is not masked by the umask).
func signDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// signBounded runs cmdSign in a goroutine and waits at most 5s. On timeout it opens the FIFO's
// write side to release a signer blocked on it, and fails.
func signBounded(t *testing.T, fifo string, args []string) (string, int) {
	t.Helper()
	type res struct {
		out  string
		code int
	}
	done := make(chan res, 1)
	go func() {
		out, _, code := capture(func() int { return cmdSign(args) })
		done <- res{out, code}
	}()
	select {
	case r := <-done:
		return r.out, r.code
	case <-time.After(5 * time.Second):
		if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
		<-done
		t.Fatalf("cmdSign blocked on the FIFO %s for 5s — the payload open must not block", fifo)
		return "", -1
	}
}

func TestSignPathHandlingNeverFollowsLinks(t *testing.T) {
	keyDir := t.TempDir()
	privPath, _ := writePrivPEM(t, keyDir)
	t.Cleanup(func() { afterPayloadLstat = nil })

	refused := func(t *testing.T, out string, code int) {
		t.Helper()
		if code != deskkit.ExitRefused {
			t.Fatalf("exit %d, want 5", code)
		}
		if out != "" {
			t.Fatalf("refusal printed to stdout: %q", out)
		}
	}
	sign := func(payload string) (string, int) {
		out, _, code := capture(func() int { return cmdSign([]string{"--payload", payload, "--pem", privPath}) })
		return out, code
	}

	t.Run("i-payload-symlink", func(t *testing.T) {
		dir := signDir(t)
		writeFile(t, filepath.Join(dir, "real.json"), bindPayload)
		link := filepath.Join(dir, "link.json")
		if err := os.Symlink(filepath.Join(dir, "real.json"), link); err != nil {
			t.Fatal(err)
		}
		out, code := sign(link)
		refused(t, out, code)
	})

	t.Run("ii-payload-fifo", func(t *testing.T) {
		p := filepath.Join(signDir(t), "p.json")
		if err := syscall.Mkfifo(p, 0o600); err != nil {
			t.Fatal(err)
		}
		out, code := signBounded(t, p, []string{"--payload", p, "--pem", privPath})
		refused(t, out, code)
	})

	t.Run("iii-out-symlink-to-victim", func(t *testing.T) {
		dir := signDir(t)
		p := filepath.Join(dir, "p.json")
		writeFile(t, p, bindPayload)
		victim := filepath.Join(signDir(t), "victim.txt")
		writeFile(t, victim, "victim-bytes")
		outLink := filepath.Join(dir, "p.out")
		if err := os.Symlink(victim, outLink); err != nil {
			t.Fatal(err)
		}
		if _, code := sign(p); code != deskkit.ExitRefused {
			t.Fatalf("exit %d, want 5", code)
		}
		if got, _ := os.ReadFile(victim); string(got) != "victim-bytes" {
			t.Fatalf("victim written through the .out link: %q", got)
		}
		if fi, err := os.Lstat(outLink); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf(".out link is no longer a link (err %v)", err)
		}
	})

	t.Run("iv-out-existing-regular", func(t *testing.T) {
		dir := signDir(t)
		p := filepath.Join(dir, "p.json")
		writeFile(t, p, bindPayload)
		out := filepath.Join(dir, "p.out")
		writeFile(t, out, "leftover")
		if _, code := sign(p); code != deskkit.ExitRefused {
			t.Fatalf("exit %d, want 5", code)
		}
		if got, _ := os.ReadFile(out); string(got) != "leftover" {
			t.Fatalf("existing .out replaced: %q", got)
		}
	})

	t.Run("v-fresh-dir", func(t *testing.T) {
		dir := signDir(t)
		p := filepath.Join(dir, "p.json")
		writeFile(t, p, bindPayload)
		body, code := sign(p)
		if code != 0 {
			t.Fatalf("exit %d, want 0", code)
		}
		fi, err := os.Lstat(filepath.Join(dir, "p.out"))
		if err != nil || !fi.Mode().IsRegular() {
			t.Fatalf(".out is not a new regular file (err %v)", err)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "p.out")); string(got) != body {
			t.Fatalf(".out does not hold exactly the stdout body")
		}
	})

	t.Run("vi-swapped-to-fifo-after-lstat", func(t *testing.T) {
		p := filepath.Join(signDir(t), "p.json")
		writeFile(t, p, bindPayload)
		afterPayloadLstat = func(path string) {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Error(err)
			}
		}
		defer func() { afterPayloadLstat = nil }()
		out, code := signBounded(t, p, []string{"--payload", p, "--pem", privPath})
		refused(t, out, code)
	})

	t.Run("vii-swapped-to-symlink-after-lstat", func(t *testing.T) {
		dir := signDir(t)
		p := filepath.Join(dir, "p.json")
		writeFile(t, p, bindPayload)
		other := filepath.Join(dir, "other.json")
		writeFile(t, other, bindPayload)
		afterPayloadLstat = func(path string) {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
			if err := os.Symlink(other, path); err != nil {
				t.Error(err)
			}
		}
		defer func() { afterPayloadLstat = nil }()
		out, code := sign(p)
		refused(t, out, code)
	})

	t.Run("viii-dir-world-writable", func(t *testing.T) {
		dir := signDir(t)
		p := filepath.Join(dir, "p.json")
		writeFile(t, p, bindPayload)
		if err := os.Chmod(dir, 0o777); err != nil {
			t.Fatal(err)
		}
		out, code := sign(p)
		refused(t, out, code)
	})

	t.Run("ix-dir-symlink", func(t *testing.T) {
		realDir := signDir(t)
		writeFile(t, filepath.Join(realDir, "p.json"), bindPayload)
		ld := filepath.Join(signDir(t), "ld")
		if err := os.Symlink(realDir, ld); err != nil {
			t.Fatal(err)
		}
		out, code := sign(filepath.Join(ld, "p.json"))
		refused(t, out, code)
	})
}
