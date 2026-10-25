package serve_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/serve"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// workflow_named_actions_test.go — the service contract's task-workflow amendment
// (2026-10-02): the closed terminal named-action allow-list stays exactly
// {answer, ack, send, session}; a workflow work, node or attempt id is NOT a
// terminal target; and workflow start / resume / handback, or a human-decision
// authorization, cannot be routed through `send` or `session`. Terminal chat
// and lifecycle control remain distinct authenticated boundaries.
//
// Every HTTP case runs through the REAL /v1 route table (serve.NewV1, the
// handler deskd mounts) with BOTH halves of the named actions wired — the
// answer/ack decision path with a recording platform writer, and the terminal
// path with the real identity.TerminalGate over a registry whose configured
// desks hold live recording sessions — and with the caller holding BOTH
// identities: the answerer's decision session and the answerer's viewer
// session. Refusals are therefore never a missing credential or a lower tier;
// they are the allow-list (the route does not exist) or the per-action scope
// (strict body, closed query, the registry's desk lookup). No docker, no
// network.

// wfFx is the act fixture with the decision path wired beside the terminal.
type wfFx struct {
	*actFx
	writer   *recordingWriter
	decision *http.Cookie // the answerer's answer-session cookie
	epochs   map[string]string
}

func newWorkflowFx(t *testing.T) *wfFx {
	t.Helper()
	fx := newActFx(t, nil)
	cfg := actCfg()
	// The same gate wireActHandler builds, over the same viewer session store
	// (so fx.answerer stays the cell's signed-in answerer).
	gate := &identity.TerminalGate{
		Sessions: fx.store,
		Rollout:  actRolloutOn{},
		Cell: func(name string) (identity.TerminalCell, bool) {
			for _, c := range cfg.Cells {
				if c.Name == name {
					return identity.TerminalCell{Name: c.Name, Repos: c.Repos, AnswerAs: c.AnswerAsLogin()}, true
				}
			}
			return identity.TerminalCell{}, false
		},
	}
	driver := func(k schema.TargetKind) (terminal.Target, error) {
		fx.askedKind = append(fx.askedKind, k)
		return fx.drivers, nil
	}
	w := &recordingWriter{}
	svc := answer.NewService(w, nil, nil, nil)
	svc.SetAnswerAs("operator")
	sessions := answer.NewSessionStore(nil, func() bool { return true })
	sess, err := sessions.BeginAsPresent("operator", true)
	if err != nil {
		t.Fatalf("begin answer session: %v", err)
	}
	fx.h = serve.NewV1(nil, cfg, nil, &serve.NamedActions{
		Service:    svc,
		Sessions:   sessions,
		Lookup:     fixtureLookup{},
		Credential: func(*http.Request) (answer.Credential, bool) { return fixtureCred{fixtureToken}, true },
		Terminal:   &serve.TerminalActions{Registry: fx.reg, Authority: gate, Driver: driver},
	}, nil, fx.reg, nil, nil)
	atPrompt(t, fx, fx.desk, actDesk)
	atPrompt(t, fx, fx.tracker, oitDesk)
	return &wfFx{
		actFx:    fx,
		writer:   w,
		decision: &http.Cookie{Name: answer.SessionCookie, Value: sess.ID},
		epochs:   map[string]string{actDesk: fx.epoch(actDesk), oitDesk: fx.epoch(oitDesk)},
	}
}

// req sends one request as the fully-identified answerer: JSON content type,
// loopback, the decision cookie AND the viewer cookie.
func (fx *wfFx) req(method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:5555"
	r.AddCookie(fx.answerer)
	r.AddCookie(fx.decision)
	rec := httptest.NewRecorder()
	fx.h.ServeHTTP(rec, r)
	return rec
}

// validSend is a send body that WOULD be delivered to the assay dispatch desk
// if a request carrying it were routed to that desk's send handler.
func (fx *wfFx) validSend(input string) string {
	raw, _ := json.Marshal(contract.SendRequest{Input: input, WaitFor: contract.SendWaitNone, Epoch: fx.epochs[actDesk]})
	return string(raw)
}

// noEffect asserts that nothing reached a lifecycle or decision effect: no
// platform write (comment, close, reaction), no byte into either configured
// desk, no driver resolved, no custom session opened, and both configured
// desks still attached at the epoch the fixture recorded (no detach, restart
// or re-attach).
func (fx *wfFx) noEffect(t *testing.T, sentToDesk ...string) {
	t.Helper()
	if n := fx.writer.comments + fx.writer.closes + fx.writer.reactions; n != 0 {
		t.Errorf("DECISION EFFECT: %d platform write(s) (comments=%d closes=%d reactions=%d)", n, fx.writer.comments, fx.writer.closes, fx.writer.reactions)
	}
	var got []string
	for _, b := range fx.desk.written() {
		got = append(got, string(b))
	}
	if !slices.Equal(got, sentToDesk) {
		t.Errorf("the assay desk received %q, want exactly %q", got, sentToDesk)
	}
	if w := fx.tracker.written(); len(w) != 0 {
		t.Errorf("the tracker desk received %q, want nothing", w)
	}
	if len(fx.askedKind) != 0 || len(fx.drivers.sess) != 0 {
		t.Errorf("LIFECYCLE EFFECT: a driver was resolved (%v) or attached (%d session(s))", fx.askedKind, len(fx.drivers.sess))
	}
	for _, cell := range []string{"assay", "tracker"} {
		if s := fx.reg.Sessions(cell); len(s) != 0 {
			t.Errorf("LIFECYCLE EFFECT: cell %s gained custom session(s) %v", cell, s)
		}
	}
	for id, s := range map[string]*actSession{actDesk: fx.desk, oitDesk: fx.tracker} {
		if !s.Alive() {
			t.Errorf("LIFECYCLE EFFECT: configured desk %s was detached", id)
		}
		if ep := fx.epoch(id); ep != fx.epochs[id] {
			t.Errorf("LIFECYCLE EFFECT: configured desk %s moved from epoch %q to %q", id, fx.epochs[id], ep)
		}
	}
}

// inCellWorkflowIDs are workflow work / node / attempt ids written the way a
// client might try to aim them at a terminal: inside the caller's own cell (so
// the Answerer tier passes and only the target check can refuse), including
// ids that EMBED a real configured desk id or mimic a custom-session id.
var inCellWorkflowIDs = []string{
	"assay/work/wi-0042",
	"assay/wi-0042",
	"assay/node/implement",
	"assay/attempt/7f3c9a",
	"assay/dispatch/attempt/7f3c9a",
	"assay/dispatch/node/implement",
	"assay/dispatch/work/wi-0042/node/implement/attempt/3",
	"assay/custom/attempt-7f3c9a",
}

// cellLessWorkflowIDs carry no cell segment: they authorize nothing.
var cellLessWorkflowIDs = []string{
	"wi-0042",
	"assay:at:service contract:18",
}

// workflowControlTokens are path segments a workflow-control or decision
// route would carry. None may appear in a human-write route.
var workflowControlTokens = []string{
	"workflow", "workflows", "work", "node", "nodes", "attempt", "attempts",
	"start", "resume", "handback", "restart", "attach", "detach",
	"decision", "decisions", "authorize", "approve", "ratify",
}

// TestNamedActionsRejectWorkflowControl — the service contract Verify row 102.
func TestNamedActionsRejectWorkflowControl(t *testing.T) {
	t.Run("the allow-list stays the four and no write route is workflow-shaped", func(t *testing.T) {
		want := []contract.Action{contract.ActionAnswer, contract.ActionAck, contract.ActionSend, contract.ActionSession}
		if !slices.Equal(contract.NamedActions, want) {
			t.Fatalf("NamedActions = %v, want exactly %v (the amendment keeps the closed set unchanged)", contract.NamedActions, want)
		}
		if got := contract.HumanWriteActions(); !slices.Equal(got, want) {
			t.Fatalf("Routes declare human writes %v, want exactly %v", got, want)
		}
		for _, r := range contract.Routes {
			if r.Method == contract.GET {
				continue
			}
			for _, seg := range strings.Split(strings.ToLower(r.Path), "/") {
				if slices.Contains(workflowControlTokens, seg) {
					t.Errorf("write route %s %s carries the workflow/decision control segment %q", r.Method, r.Path, seg)
				}
			}
		}
		// The two terminal wire bodies are pinned field-for-field: neither can
		// carry a work/node/attempt id, a control verb or a decision.
		for _, c := range []struct {
			typ  any
			want []string
		}{
			{contract.SendRequest{}, []string{"input", "wait_for", "timeout_ms", "epoch", "request_id"}},
			{contract.SessionRequest{}, []string{"on", "skill", "prompt", "title"}},
		} {
			typ := reflect.TypeOf(c.typ)
			var got []string
			for i := 0; i < typ.NumField(); i++ {
				name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
				got = append(got, name)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("%s fields = %v, want exactly %v", typ.Name(), got, c.want)
			}
		}
	})

	t.Run("a workflow id is not a terminal target", func(t *testing.T) {
		fx := newWorkflowFx(t)
		for _, id := range inCellWorkflowIDs {
			// Escaped (the contract client's form) and raw.
			for _, p := range []string{sendPath(id), contract.Prefix + "/desks/" + id + "/terminal/send"} {
				if rec := fx.req(http.MethodPost, p, fx.validSend("deploy\n")); rec.Code != http.StatusNotFound {
					t.Errorf("send to workflow id %s (%s): %d %s, want 404 no such desk", id, p, rec.Code, rec.Body.String())
				}
			}
			body, _ := json.Marshal(contract.SessionRequest{On: id})
			if rec := fx.req(http.MethodPost, contract.Prefix+"/sessions", string(body)); rec.Code != http.StatusNotFound {
				t.Errorf("session open on workflow id %s: %d %s, want 404 no such desk", id, rec.Code, rec.Body.String())
			}
			if rec := fx.req(http.MethodDelete, contract.Prefix+"/sessions/"+url.PathEscape(id), ""); rec.Code != http.StatusNotFound {
				t.Errorf("session close on workflow id %s: %d %s, want 404 no such session", id, rec.Code, rec.Body.String())
			}
		}
		for _, id := range cellLessWorkflowIDs {
			body, _ := json.Marshal(contract.SessionRequest{On: id})
			for _, rec := range []*httptest.ResponseRecorder{
				fx.req(http.MethodPost, sendPath(id), fx.validSend("deploy\n")),
				fx.req(http.MethodPost, contract.Prefix+"/sessions", string(body)),
				fx.req(http.MethodDelete, contract.Prefix+"/sessions/"+url.PathEscape(id), ""),
			} {
				if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
					t.Errorf("cell-less workflow id %s: %d %s, want 403 or 404", id, rec.Code, rec.Body.String())
				}
			}
		}
		// An attempt id offered as the epoch binds nothing: the send names the
		// real desk but was not composed against its live session.
		raw, _ := json.Marshal(contract.SendRequest{Input: "deploy\n", WaitFor: contract.SendWaitNone, Epoch: "wi-0042.attempt-3"})
		if rec := fx.req(http.MethodPost, sendPath(actDesk), string(raw)); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "epoch changed") {
			t.Errorf("send with an attempt id as its epoch: %d %s, want 409 epoch changed", rec.Code, rec.Body.String())
		}
		fx.noEffect(t)
	})

	t.Run("workflow lifecycle and decision control routes do not exist", func(t *testing.T) {
		fx := newWorkflowFx(t)
		d := url.PathEscape(actDesk)
		body := fx.validSend("deploy\n")
		for _, c := range []struct{ method, path string }{
			{http.MethodPost, "/v1/workflows/wi-0042/start"},
			{http.MethodPost, "/v1/work/wi-0042/resume"},
			{http.MethodPost, "/v1/nodes/implement/start"},
			{http.MethodPost, "/v1/attempts/7f3c9a/handback"},
			{http.MethodPost, "/v1/desks/" + d + "/terminal/start"},
			{http.MethodPost, "/v1/desks/" + d + "/terminal/resume"},
			{http.MethodPost, "/v1/desks/" + d + "/terminal/handback"},
			{http.MethodPost, "/v1/desks/" + d + "/terminal/send/resume"},
			{http.MethodPost, "/v1/desks/" + d + "/workflow/start"},
			{http.MethodPost, "/v1/desks/" + d + "/attach"},
			{http.MethodPost, "/v1/desks/" + d + "/restart"},
			{http.MethodPost, "/v1/desks/" + d + "/handback"},
			{http.MethodPut, sendPath(actDesk)},
			{http.MethodPatch, sendPath(actDesk)},
			{http.MethodDelete, "/v1/desks/" + d},
			{http.MethodDelete, "/v1/desks/" + d + "/terminal"},
			{http.MethodPost, "/v1/sessions/resume"},
			{http.MethodPost, "/v1/sessions/" + d + "/handback"},
			{http.MethodPost, "/v1/sessions/" + url.PathEscape("assay/attempt/7f3c9a") + "/start"},
			{http.MethodPut, "/v1/sessions"},
			{http.MethodPost, "/v1/escalations/IT1/approve"},
			{http.MethodPost, "/v1/escalations/IT1/authorize"},
			{http.MethodPost, "/v1/escalations/IT1/ratify"},
			{http.MethodPost, "/v1/escalations/IT1/terminal/send"},
			{http.MethodPost, "/v1/decisions/IT1/authorize"},
		} {
			rec := fx.req(c.method, c.path, body)
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: %d %s, want 404 or 405 — the route must not exist", c.method, c.path, rec.Code, rec.Body.String())
			}
		}
		// Closing a CONFIGURED desk through `session` is not a lifecycle stop.
		if rec := fx.req(http.MethodDelete, contract.Prefix+"/sessions/"+d, ""); rec.Code != http.StatusForbidden {
			t.Errorf("session close on the configured desk: %d %s, want 403 configured desk", rec.Code, rec.Body.String())
		}
		fx.noEffect(t)
	})

	t.Run("control payloads in the body are refused", func(t *testing.T) {
		fx := newWorkflowFx(t)
		ep := fx.epochs[actDesk]
		base := `"input":"deploy\n","wait_for":"none","epoch":"` + ep + `"`
		for _, extra := range []string{
			`"work_id":"wi-0042"`,
			`"node_id":"implement"`,
			`"attempt_id":"7f3c9a"`,
			`"workflow":{"id":"wi-0042","action":"resume"}`,
			`"action":"handback"`,
			`"control":"start"`,
			`"target":"assay/dispatch/attempt/7f3c9a"`,
			`"decision":{"id":"IT1","option":"1"}`,
			`"answer":"1"`,
			`"approve":true`,
			`"authorize":"human:operator"`,
		} {
			if rec := fx.req(http.MethodPost, sendPath(actDesk), "{"+base+","+extra+"}"); rec.Code != http.StatusBadRequest {
				t.Errorf("send carrying {%s}: %d %s, want 400", extra, rec.Code, rec.Body.String())
			}
			if rec := fx.req(http.MethodPost, contract.Prefix+"/sessions", `{"on":"assay/dispatch",`+extra+`}`); rec.Code != http.StatusBadRequest {
				t.Errorf("session open carrying {%s}: %d %s, want 400", extra, rec.Code, rec.Body.String())
			}
		}
		// The one verb-like field a send has is closed: prompt | idle | none.
		for _, wait := range []string{"start", "resume", "handback", "approval", "decision"} {
			if rec := fx.req(http.MethodPost, sendPath(actDesk), `{"input":"deploy\n","wait_for":"`+wait+`","epoch":"`+ep+`"}`); rec.Code != http.StatusBadRequest {
				t.Errorf("send with wait_for %q: %d %s, want 400", wait, rec.Code, rec.Body.String())
			}
		}
		fx.noEffect(t)
	})

	t.Run("control payloads in the query are refused", func(t *testing.T) {
		fx := newWorkflowFx(t)
		for _, q := range []string{
			"action=resume&attempt=7f3c9a",
			"work_id=wi-0042",
			"node=implement&action=start",
			"handback=1",
			"decision=IT1&approve=1",
			"target=" + url.QueryEscape("assay/dispatch/attempt/7f3c9a"),
		} {
			if rec := fx.req(http.MethodPost, sendPath(actDesk)+"?"+q, fx.validSend("deploy\n")); rec.Code != http.StatusBadRequest {
				t.Errorf("send with ?%s: %d %s, want 400", q, rec.Code, rec.Body.String())
			}
			if rec := fx.req(http.MethodPost, contract.Prefix+"/sessions?"+q, `{"on":"assay/dispatch"}`); rec.Code != http.StatusBadRequest {
				t.Errorf("session open with ?%s: %d %s, want 400", q, rec.Code, rec.Body.String())
			}
			if rec := fx.req(http.MethodDelete, contract.Prefix+"/sessions/"+url.PathEscape("assay/custom/x")+"?"+q, ""); rec.Code != http.StatusBadRequest {
				t.Errorf("session close with ?%s: %d %s, want 400", q, rec.Code, rec.Body.String())
			}
		}
		fx.noEffect(t)
	})

	t.Run("the registry refuses a workflow id with the HTTP layers bypassed", func(t *testing.T) {
		// Lower layer (d), alone: no route table, no strict decode, no tier —
		// the registry is called directly, as the right cell, at the real desk's
		// live epoch. The id is still not a terminal.
		fx := newWorkflowFx(t)
		driver := func(k schema.TargetKind) (terminal.Target, error) {
			fx.askedKind = append(fx.askedKind, k)
			return fx.drivers, nil
		}
		for _, id := range inCellWorkflowIDs {
			_, err := fx.reg.WaitThenSend(context.Background(), id, []byte("deploy\n"), contract.SendWaitNone, 0, terminal.SendBinding{
				Cell: "assay", Epoch: fx.epochs[actDesk], RequestID: "wf-bypass-1",
			})
			if !errors.Is(err, terminal.ErrNoSuchDesk) {
				t.Errorf("registry send to workflow id %s: %v, want ErrNoSuchDesk", id, err)
			}
			if _, err := fx.reg.OpenSession(context.Background(), id, "assay", "operator", "", driver, terminal.Size{Cols: 80, Rows: 24}); !errors.Is(err, terminal.ErrNoSuchDesk) {
				t.Errorf("registry session open on workflow id %s: %v, want ErrNoSuchDesk", id, err)
			}
			if err := fx.reg.CloseSession(id, "assay"); !errors.Is(err, terminal.ErrNoSuchSession) {
				t.Errorf("registry session close on workflow id %s: %v, want ErrNoSuchSession", id, err)
			}
		}
		fx.noEffect(t)
	})

	t.Run("terminal chat carrying control text is only chat", func(t *testing.T) {
		// The FLOW: the same identities, the real desk, a well-formed send whose
		// TEXT names a workflow start, a handback and an approval. It is
		// delivered verbatim as terminal input — and that is all it is.
		fx := newWorkflowFx(t)
		lines := []string{
			"/workflow start wi-0042 --node implement\n",
			"handback attempt 7f3c9a\n",
			"APPROVED: IT1 option 1 — ratified by human:operator\n",
		}
		allowed := map[string]bool{"desk_id": true, "request_id": true, "epoch": true, "outcome": true, "waited_ms": true, "cursor_after": true, "state_after": true, "duplicate": true}
		for _, line := range lines {
			rec := fx.req(http.MethodPost, sendPath(actDesk), fx.validSend(line))
			if rec.Code != http.StatusOK {
				t.Fatalf("chat send %q: %d %s", line, rec.Code, rec.Body.String())
			}
			m := bodyMap(t, rec)
			if m["outcome"] != string(contract.SendSent) || m["desk_id"] != actDesk {
				t.Errorf("chat send %q: %v, want outcome sent to %s", line, m, actDesk)
			}
			for k := range m {
				if !allowed[k] {
					t.Errorf("chat send result carries %q — a send reports its own delivery and nothing else", k)
				}
			}
		}
		fx.noEffect(t, lines...)
		// Positive control: the decision path IS wired in this fixture and IS
		// observable — the zero above is not a writer nobody could reach.
		if rec := post(fx.h, contract.Prefix+"/escalations/IT1/ack", fx.decision, ""); rec.Code != http.StatusOK || fx.writer.reactions != 1 {
			t.Fatalf("positive control: ack answered %d with %d reaction(s), want 200 and 1", rec.Code, fx.writer.reactions)
		}
	})
}
