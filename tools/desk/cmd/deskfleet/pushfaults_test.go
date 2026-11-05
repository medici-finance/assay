package main

// pushfaults_test.go — `push-token rotate|check` (#2910 part B) fail-closed paths: the paged
// access-token list, read-back and by-id read failures, the expiry window, the hidden
// fallback and the recovery lines, against the stateful fake in pushfake_test.go. Every value
// is an obviously fake placeholder; nothing contacts a real forge.

import (
	"fmt"
	"strings"
	"testing"
)

// ptHistory prepends n revoked same-name records and n active records of another name, so the
// live token sits past the first page whether or not the list filters on state.
func ptHistory(f *ptFake, n int) {
	var hist []*ptTok
	for i := 0; i < n; i++ {
		hist = append(hist, &ptTok{id: int64(1000 + i), name: ptName, revoked: true, scopes: []string{"write_repository"},
			level: 40, expires: ptDate(-30)})
	}
	for i := 0; i < n; i++ {
		hist = append(hist, &ptTok{id: int64(3000 + i), name: "example-other", active: true, scopes: []string{"read_api"},
			level: 30, expires: ptDate(60)})
	}
	f.toks = append(hist, f.toks...)
}

// regression: #2910
func TestPushTokenPagedList(t *testing.T) {
	t.Run("live token past page one", func(t *testing.T) {
		f := newPTFake(t)
		ptHistory(f, 150)
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d want 0\n%s", rc, h.text())
		}
		ptOneActive(t, f)
		pages := 0
		for _, r := range f.requests() {
			if r.Method == "GET" && strings.HasSuffix(r.Path, "/access_tokens") {
				pages++
				if !strings.Contains(r.Query, "state=active") || !strings.Contains(r.Query, "per_page=100") {
					t.Errorf("list request %q does not ask for active records 100 per page", r.Query)
				}
			}
		}
		if pages < 2 {
			t.Errorf("%d list pages read, want every page (>= 2)", pages)
		}
		f.toks[len(f.toks)-1].expires = ptDate(20)
		if rc := h.check("--min-days", "7"); rc != exitOK {
			t.Fatalf("check rc=%d\n%s", rc, h.text())
		}
	})
	t.Run("--create never mints a second live token", func(t *testing.T) {
		f := newPTFake(t)
		ptHistory(f, 150)
		h := newPTHarness(t, f)
		if rc := h.rotate("--create"); rc != exitOK {
			t.Fatalf("rc=%d want 0\n%s", rc, h.text())
		}
		if n := f.count("POST", "/access_tokens"); n != 0 {
			t.Errorf("%d token creates, want 0 (the live token exists past page one)", n)
		}
		ptOneActive(t, f)
	})
	faults := []struct {
		name, want string
		setup      func(f *ptFake)
	}{
		{"page parameter ignored", "pagination not honoured", func(f *ptFake) { ptHistory(f, 150); f.ignorePage = true }},
		{"endless pages", "runs past", func(f *ptFake) { f.endlessPages = true }},
		{"page 2 answers 500", "could-not-check", func(f *ptFake) { ptHistory(f, 150); f.listFailPage = 2 }},
		{"page 2 answers 404", "page 2 answered HTTP 404", func(f *ptFake) {
			ptHistory(f, 150)
			f.listStatus, f.listFailPage = 404, 2
		}},
		{"list 500", "could-not-check", func(f *ptFake) { f.listStatus = 500 }},
		{"list 403", "could-not-check", func(f *ptFake) { f.listStatus = 403 }},
		{"list not an array", "could-not-check", func(f *ptFake) { f.listStatus = 200 }},
		{"list 200 null", "answered null", func(f *ptFake) { f.listNull = true }},
		{"page 2 answers 200 null", "page 2 answered null", func(f *ptFake) {
			ptHistory(f, 150)
			f.listNull, f.listFailPage = true, 2
		}},
	}
	for _, c := range faults {
		t.Run(c.name, func(t *testing.T) {
			f := newPTFake(t)
			f.toks[0].active = false // --create would mint on a short read
			c.setup(f)
			h := newPTHarness(t, f)
			if rc := h.rotate("--create"); rc != exitUnverifiable || !strings.Contains(h.text(), c.want) {
				t.Fatalf("rc=%d, want 6 and %q\n%s", rc, c.want, h.text())
			}
			if n := f.writes(); n != 0 {
				t.Errorf("%d writes, want 0", n)
			}
		})
	}
	t.Run("active and revoked record is not live", func(t *testing.T) {
		f := newPTFake(t)
		f.toks = append(f.toks, &ptTok{id: 6002, name: ptName, active: true, revoked: true,
			scopes: []string{"write_repository"}, level: 40, expires: ptDate(20)})
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d want 0\n%s", rc, h.text())
		}
	})
}

// regression: #2910
func TestPushTokenReadbackUnreadable(t *testing.T) {
	for _, fault := range []string{"502", "drop", "garbage"} {
		t.Run(fault, func(t *testing.T) {
			f := newPTFake(t)
			f.readBackFault = fault
			h := newPTHarness(t, f)
			rc := h.rotate()
			want := "contained: revoked 7001 — the read-back could not be read"
			if rc != exitFailed || !strings.Contains(h.text(), want) {
				t.Fatalf("rc=%d, want 1 and %q\n%s", rc, want, h.text())
			}
			if n := f.count("DELETE", "/access_tokens/7001"); n != 1 {
				t.Errorf("%d revokes of the new id, want 1", n)
			}
			if n := len(f.activeByName()); n != 0 {
				t.Errorf("%d active tokens by name afterwards, want 0", n)
			}
			ptNoProof(t, h, f)
			ptNoSentinel(t, h, fault)
		})
	}
	t.Run("unhidden variable omits its value", func(t *testing.T) {
		f := newPTFake(t)
		f.readNoValue = true
		h := newPTHarness(t, f)
		want := "contained: revoked 7001 — the read-back disagreed on value"
		if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), want) {
			t.Fatalf("rc=%d, want 1 and %q\n%s", rc, want, h.text())
		}
		ptNoProof(t, h, f)
	})
}

// regression: #2910
func TestPushTokenByIDUnreadable(t *testing.T) {
	for _, st := range []int{500, 404} {
		t.Run(fmt.Sprint(st), func(t *testing.T) {
			f := newPTFake(t)
			f.byIDStatus = st
			h := newPTHarness(t, f)
			if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "by-id: could-not-check") {
				t.Fatalf("rc=%d, want 1 and by-id: could-not-check\n%s", rc, h.text())
			}
			if strings.Contains(h.text(), "outcome:") {
				t.Errorf("printed an outcome:\n%s", h.text())
			}
			if n := f.count("GET", "/personal_access_tokens/self") + f.count("GET", ".git/info/refs"); n != 0 {
				t.Errorf("%d later probes, want 0", n)
			}
		})
	}
}

// regression: #2910
func TestPushTokenExpiryWindow(t *testing.T) {
	set := func(v any) func(f *ptFake) {
		return func(f *ptFake) {
			f.byIDMutate = func(id int64, m map[string]any) {
				if id != ptOldID {
					m["expires_at"] = v
				}
			}
		}
	}
	for name, setup := range map[string]func(f *ptFake){
		"null":          set(nil),
		"unparseable":   set("never"),
		"far future":    set(ptDate(400)),
		"5 days longer": set(ptDate(35)),
	} {
		t.Run(name, func(t *testing.T) {
			f := newPTFake(t)
			setup(f)
			h := newPTHarness(t, f)
			if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "by-id: FAIL field expires_at") {
				t.Fatalf("rc=%d, want 1 and expires_at\n%s", rc, h.text())
			}
		})
	}
	t.Run("one day either side passes", func(t *testing.T) {
		for _, d := range []int{29, 31} {
			f := newPTFake(t)
			set(ptDate(d))(f)
			h := newPTHarness(t, f)
			if rc := h.rotate(); rc != exitOK {
				t.Fatalf("day %d: rc=%d want 0\n%s", d, rc, h.text())
			}
		}
	})
	t.Run("check with null expiry", func(t *testing.T) {
		f := newPTFake(t)
		f.byIDMutate = func(_ int64, m map[string]any) { m["expires_at"] = nil }
		h := newPTHarness(t, f)
		if rc := h.check("--min-days", "7"); rc != exitFailed || !strings.Contains(h.text(), "by-id: FAIL field expires_at") {
			t.Fatalf("rc=%d, want 1 and expires_at\n%s", rc, h.text())
		}
	})
}

// regression: #2910
func TestPushTokenHiddenFallback(t *testing.T) {
	f := newPTFake(t)
	f.v = ptVar{}
	f.supportsHidden = false
	h := newPTHarness(t, f)
	if rc := h.rotate("--create"); rc != exitOK {
		t.Fatalf("rc=%d want 0\n%s", rc, h.text())
	}
	if f.v.hidden || !f.v.masked || !f.v.protected {
		t.Errorf("stored flags: %+v", f.v)
	}
	if !strings.Contains(h.text(), "hidden=false value=matches") {
		t.Errorf("the read-back did not record the unhidden fallback:\n%s", h.text())
	}
	ptOneActive(t, f)
}

// regression: #2910
func TestPushTokenRecoveryLines(t *testing.T) {
	t.Run("create path, write 500 x3", func(t *testing.T) {
		f := newPTFake(t)
		f.v = ptVar{}
		f.writeStatuses = []int{500, 500, 500}
		h := newPTHarness(t, f)
		if rc := h.rotate("--create"); rc != exitFailed || !strings.Contains(h.text(), "does not exist") ||
			!strings.Contains(h.text(), "re-run `push-token rotate --create`") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		if rc := h.rotate("--create"); rc != exitOK {
			t.Fatalf("re-run rc=%d\n%s", rc, h.text())
		}
		ptOneActive(t, f)
	})
	t.Run("create landed, answer lost", func(t *testing.T) {
		f := newPTFake(t)
		f.v = ptVar{}
		f.landThenFail = 1
		h := newPTHarness(t, f)
		if rc := h.rotate("--create"); rc != exitFailed || !strings.Contains(h.text(), "may now exist") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		if !f.v.exists {
			t.Fatal("the fake did not store the create")
		}
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("plain re-run rc=%d\n%s", rc, h.text())
		}
		ptOneActive(t, f)
	})
}

// regression: #2910
func TestPushTokenRedactsUnexported(t *testing.T) {
	const sentinel = "example-redact-sentinel"
	s := newPushSecret(sentinel)
	inner := struct {
		name string
		v    pushSecret
	}{"x", s}
	type outer struct {
		w any
		p *pushSecret
	}
	o := outer{inner, &s}
	decimal := strings.Trim(fmt.Sprint([]byte(sentinel)), "[]")
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		for _, x := range []any{inner, &inner, o, []any{inner}, map[string]any{"k": inner}} {
			r := fmt.Sprintf(verb, x)
			if strings.Contains(r, sentinel) || strings.Contains(r, decimal) {
				t.Errorf("%s of %T exposed the value: %s", verb, x, r)
			}
		}
	}
	s.wipe()
	if s.present() || s.reveal() != "" {
		t.Error("wipe left the value readable")
	}
}

// regression: #2910
func TestPushTokenRefusesPaths(t *testing.T) {
	for _, p := range []string{"example-group/../example-other", "./example-group/example-project",
		"example-group/./example-project", ".."} {
		t.Run(p, func(t *testing.T) {
			f := newPTFake(t)
			f.pathOverride = p
			h := newPTHarness(t, f)
			if rc := h.rotate(); rc != exitRefused || !strings.Contains(h.text(), "not a plain project path") {
				t.Fatalf("rc=%d want 5\n%s", rc, h.text())
			}
			if n := f.writes(); n != 0 {
				t.Errorf("%d writes, want 0", n)
			}
		})
	}
	t.Run("userinfo in the API base", func(t *testing.T) {
		f := newPTFake(t)
		h := newPTHarness(t, f)
		h.envs["GITLAB_API_BASE"] = strings.Replace(f.base(), "://", "://example-user@", 1)
		if rc := h.rotate(); rc != exitRefused {
			t.Fatalf("rc=%d want 5\n%s", rc, h.text())
		}
		if n := len(f.requests()); n != 0 {
			t.Errorf("%d requests, want 0", n)
		}
	})
}
