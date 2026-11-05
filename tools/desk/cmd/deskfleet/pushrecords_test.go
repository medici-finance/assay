package main

// pushrecords_test.go — `push-token rotate|check` (#2910 part B) refuses a token record it
// cannot judge: a list record that is null, empty, unnamed, missing its id, active or revoked
// field, or carries an id that is not positive, on any page and in every mode, before any
// write; plain rotate never reaches the create path; the containment confirming read must be
// the new token's own complete record. Also the remaining fail-closed paths without a
// dedicated test: check-mode by-id could-not-check, a push-probe transport failure and a
// variable-preflight 403. Every value is an obviously fake placeholder; nothing contacts a
// real forge.

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ptBadRecords are list records that decode without a JSON error yet cannot be judged.
var ptBadRecords = []struct{ name, raw string }{
	{"null record", `null`},
	{"empty record", `{}`},
	{"named id 0", `{"id":0,"name":"` + ptName + `","active":true,"revoked":false,` +
		`"scopes":["write_repository"],"access_level":40,"expires_at":"2099-01-01"}`},
	{"named no id", `{"name":"` + ptName + `","active":true,"revoked":false,` +
		`"scopes":["write_repository"],"access_level":40,"expires_at":"2099-01-01"}`},
	{"named negative id", `{"id":-4,"name":"` + ptName + `","active":true,"revoked":false,` +
		`"scopes":["write_repository"],"access_level":40,"expires_at":"2099-01-01"}`},
	{"no name", `{"id":6100,"active":true,"revoked":false}`},
	{"empty name", `{"id":6100,"name":"","active":true,"revoked":false}`},
	{"named no active", `{"id":6100,"name":"` + ptName + `","revoked":false,` +
		`"scopes":["write_repository"],"access_level":40,"expires_at":"2099-01-01"}`},
	{"named no revoked", `{"id":6100,"name":"` + ptName + `","active":true,` +
		`"scopes":["write_repository"],"access_level":40,"expires_at":"2099-01-01"}`},
	{"named active null", `{"id":6100,"name":"` + ptName + `","active":null,"revoked":false}`},
}

// regression: #2910
func TestPushTokenRecordUnjudgeable(t *testing.T) {
	modes := []struct {
		name      string
		args      []string
		delistOld bool // the predecessor is not on the list (the case a bad record would turn into a create)
	}{
		{"rotate, predecessor live", nil, false},
		{"rotate, predecessor not listed", nil, true},
		{"rotate --create, predecessor not listed", []string{"--create"}, true},
		{"rotate --create, predecessor live", []string{"--create"}, false},
	}
	for _, rec := range ptBadRecords {
		for _, page := range []int{1, 2} {
			for _, m := range modes {
				t.Run(rec.name+"/page "+string(rune('0'+page))+"/"+m.name, func(t *testing.T) {
					f := newPTFake(t)
					if page == 2 {
						ptHistory(f, 150)
					}
					if m.delistOld {
						f.tok(ptOldID).active = false
					}
					f.listRaw, f.listRawPage = rec.raw, page
					h := newPTHarness(t, f)
					want := "page " + string(rune('0'+page)) + " record"
					rc := h.rotate(m.args...)
					if rc != exitUnverifiable || !strings.Contains(h.text(), "cannot be judged") ||
						!strings.Contains(h.text(), want) {
						t.Fatalf("rc=%d, want 6, %q and \"cannot be judged\"\n%s", rc, want, h.text())
					}
					if n := f.writes(); n != 0 {
						t.Errorf("%d writes, want 0", n)
					}
					if n := f.count("POST", "/access_tokens"); n != 0 {
						t.Errorf("%d token creates, want 0", n)
					}
					ptNoSentinel(t, h, rec.name)
				})
			}
			t.Run(rec.name+"/page "+string(rune('0'+page))+"/check", func(t *testing.T) {
				f := newPTFake(t)
				if page == 2 {
					ptHistory(f, 150)
				}
				f.listRaw, f.listRawPage = rec.raw, page
				h := newPTHarness(t, f)
				if rc := h.check(); rc != exitUnverifiable || !strings.Contains(h.text(), "cannot be judged") {
					t.Fatalf("check rc=%d, want 6 and \"cannot be judged\"\n%s", rc, h.text())
				}
				if strings.Contains(h.text(), "outcome: healthy") {
					t.Errorf("check reported healthy on an unjudgeable list\n%s", h.text())
				}
			})
		}
	}
	t.Run("a complete unrelated record is still skipped", func(t *testing.T) {
		f := newPTFake(t)
		f.listRaw, f.listRawPage = `{"id":6100,"name":"example-other","active":true,"revoked":false}`, 1
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d want 0\n%s", rc, h.text())
		}
		ptOneActive(t, f)
	})
}

// regression: #2910
func TestPushTokenPlainRotateNeverCreates(t *testing.T) {
	// No token id and no --create: issue() refuses before any request, whatever preflight did.
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)
	var errb bytes.Buffer
	r := &pushRun{o: pushOpts{tokenName: ptName}, e: &env{stdout: io.Discard, stderr: &errb},
		gl: newGitLabClient(srv.URL+"/api/v4", srv.Client(), "example-placeholder"), pp: "/projects/1",
		want: time.Now().AddDate(0, 0, 30)}
	if rc := r.issue(); rc != exitUnverifiable || !strings.Contains(errb.String(), "--create was not given") {
		t.Fatalf("issue() rc=%d, want 6 and the --create refusal\n%s", rc, errb.String())
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("%d requests, want 0 (no create, no rotate)", n)
	}
	// With no live token at all, plain rotate refuses and creates nothing.
	f := newPTFake(t)
	f.tok(ptOldID).active = false
	h := newPTHarness(t, f)
	if rc := h.rotate(); rc == exitOK {
		t.Fatalf("rc=0 with no live token and no --create\n%s", h.text())
	}
	if n := f.count("POST", "/access_tokens"); n != 0 || f.writes() != 0 {
		t.Errorf("%d creates and %d writes, want 0 and 0", n, f.writes())
	}
}

// regression: #2910
func TestPushTokenContainConfirmUnjudgeable(t *testing.T) {
	for name, raw := range map[string]string{
		"null":        `null`,
		"empty":       `{}`,
		"another id":  `{"id":9,"name":"` + ptName + `","active":false,"revoked":true}`,
		"no active":   `{"id":7001,"name":"` + ptName + `","revoked":true}`,
		"not an obj":  `[]`,
		"id 0 record": `{"id":0,"name":"` + ptName + `","active":false,"revoked":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newPTFake(t)
			f.readBackFault = "502"
			f.deadRaw = raw
			h := newPTHarness(t, f)
			rc := h.rotate()
			if rc != exitUnverifiable || !strings.Contains(h.text(), "containment-failed: revoke 7001 by hand") {
				t.Fatalf("rc=%d, want 6 and containment-failed\n%s", rc, h.text())
			}
			ptNoSentinel(t, h, name)
		})
	}
	t.Run("the token's own dead record confirms", func(t *testing.T) {
		f := newPTFake(t)
		f.readBackFault = "502"
		f.deadRaw = `{"id":7001,"name":"` + ptName + `","active":false,"revoked":true}`
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "contained: revoked 7001") {
			t.Fatalf("rc=%d, want 1 and contained\n%s", rc, h.text())
		}
	})
}

// regression: #2910
func TestPushTokenRemainingFailClosed(t *testing.T) {
	t.Run("check by-id could-not-check", func(t *testing.T) {
		f := newPTFake(t)
		f.byIDStatus, f.byIDAll = 500, true
		h := newPTHarness(t, f)
		if rc := h.check(); rc != exitUnverifiable {
			t.Fatalf("check rc=%d, want 6\n%s", rc, h.text())
		}
		if strings.Contains(h.text(), "outcome: healthy") {
			t.Errorf("check reported healthy with the by-id read failing\n%s", h.text())
		}
	})
	t.Run("push probe transport failure", func(t *testing.T) {
		f := newPTFake(t)
		f.pushDrop = true
		h := newPTHarness(t, f)
		rc := h.rotate()
		if rc == exitOK || !strings.Contains(h.text(), "push: FAIL — the receive-pack probe failed in transport") {
			t.Fatalf("rc=%d, want non-zero and the transport FAIL line\n%s", rc, h.text())
		}
		if strings.Contains(h.text(), "outcome: rotated") {
			t.Errorf("reported rotated with the push unproven\n%s", h.text())
		}
		ptNoSentinel(t, h, "push drop")
	})
	for _, args := range [][]string{nil, {"--create"}} {
		t.Run("variable preflight 403 "+strings.Join(args, " "), func(t *testing.T) {
			f := newPTFake(t)
			f.varPreStatus = 403
			h := newPTHarness(t, f)
			if rc := h.rotate(args...); rc != exitRefused {
				t.Fatalf("rc=%d, want 5\n%s", rc, h.text())
			}
			if n := f.writes(); n != 0 {
				t.Errorf("%d writes, want 0", n)
			}
		})
	}
}
