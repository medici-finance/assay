//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// orderWriter records, in order, each output line that REPORTS a token file as usable, so a
// test can assert the custody evaluation ran on that file BEFORE its path was reported.
type orderWriter struct {
	buf    *bytes.Buffer
	events *[]string
}

func (w orderWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(string(p), "\n") {
		if strings.HasPrefix(line, "minted: ") {
			if i := strings.Index(line, " -> "); i >= 0 {
				rest := line[i+4:]
				if j := strings.Index(rest, " ("); j >= 0 {
					*w.events = append(*w.events, "reported:"+rest[:j])
				}
			}
		}
	}
	return w.buf.Write(p)
}

// Brief row 7: each token file is CREATED restricted — mode 0600 at the instant the create
// call returns, before a single byte is written, never widened then tightened — and the
// deskkit owner-only evaluation runs on the file as written BEFORE its path is reported.
func TestFleetTokenFileCustody(t *testing.T) {
	f := newFakeForge(t)
	h := newHarness(t, f)
	var events []string
	h.e.stdout = orderWriter{buf: h.out, events: &events}
	h.e.createRestricted = func(path string) (*os.File, error) {
		fh, err := createRestricted(path)
		if err != nil {
			return nil, err
		}
		fi, serr := fh.Stat()
		if serr != nil {
			t.Fatalf("stat at create: %v", serr)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s was created with mode %04o — not restricted at creation", path, perm)
		}
		if fi.Size() != 0 {
			t.Errorf("%s already held data at creation", path)
		}
		events = append(events, "created:"+filepath.Base(path))
		return fh, nil
	}
	h.e.classifyCustody = func(path string) deskkit.CustodyVerdict {
		events = append(events, "checked:"+path)
		return deskkit.ClassifyCustodyOwnerOnly(path)
	}
	if code := run(h.provisionArgs(), h.e); code != exitOK {
		t.Fatalf("exit %d\n%s", code, h.err.String())
	}
	for _, r := range fleetRoles {
		final := filepath.Join(h.dir, tokenFileName(r.Role))
		fi, err := os.Stat(final)
		if err != nil {
			t.Fatalf("role %s: %v", r.Role, err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("role %s: final mode %04o", r.Role, fi.Mode().Perm())
		}
		ci, ri := indexOf(events, "checked:"+final), indexOf(events, "reported:"+final)
		if ci < 0 {
			t.Errorf("role %s: the deskkit custody evaluation was never called on %s", r.Role, final)
			continue
		}
		if ri < 0 || ri < ci {
			t.Errorf("role %s: path reported (event %d) before the custody check (event %d)", r.Role, ri, ci)
		}
	}
	if n := strings.Count(strings.Join(events, "\n"), "created:"); n != len(fleetRoles) {
		t.Errorf("%d restricted creates, want %d", n, len(fleetRoles))
	}
}

func indexOf(s []string, want string) int {
	for i, v := range s {
		if v == want {
			return i
		}
	}
	return -1
}

// Brief row 8 — NEGATIVE PATH, layer 1 bypassed: the fixture creates the token file
// PERMISSIVELY (0644), so only layer 2 — the REAL deskkit read-back — stands between the token
// and a report of it as usable. A definite failure is not survivable under the ruling: the run
// stops, names the credential for revocation, never reports its path as usable, and mints
// nothing further.
func TestFleetRefusesOnCustodyFailure(t *testing.T) {
	f := newFakeForge(t)
	h := newHarness(t, f)
	badRole := "worker" // the second role
	h.e.createRestricted = func(path string) (*os.File, error) {
		if strings.HasPrefix(filepath.Base(path), "."+tokenFileName(badRole)) {
			return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		}
		return createRestricted(path)
	}
	old := setUmask(0)
	defer setUmask(old)

	code := run(h.provisionArgs(), h.e)
	if code != exitRefused {
		t.Fatalf("exit %d, want %d (refused)\nstdout:\n%s\nstderr:\n%s", code, exitRefused, h.out.String(), h.err.String())
	}
	bad := filepath.Join(h.dir, tokenFileName(badRole))
	for _, line := range strings.Split(h.out.String(), "\n") {
		if strings.HasPrefix(line, "minted: ") && strings.Contains(line, bad) {
			t.Fatalf("the refused credential was reported as usable: %s", line)
		}
	}
	if f.mints != 2 {
		t.Fatalf("mints = %d, want 2 — the run must stop at the refused credential", f.mints)
	}
	if !strings.Contains(h.out.String(), "REFUSED") || !strings.Contains(h.err.String(), "REVOKED") {
		t.Fatalf("the refusal does not name the credential for revocation:\nstdout:\n%s\nstderr:\n%s",
			h.out.String(), h.err.String())
	}
	if !strings.Contains(h.out.String(), "role="+badRole) || !strings.Contains(h.out.String(), "role=reviewer") {
		t.Fatalf("the partial-run report does not name both minted tokens:\n%s", h.out.String())
	}
}
