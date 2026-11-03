package serve_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/serve"
	"github.com/medici-finance/assay/deskd/internal/terminal"
	"github.com/medici-finance/assay/deskd/internal/terminal/container"
)

// v1_terminal_container_test.go — the service contract Verify rows 4 and 5: the
// send path over the REAL container driver (docker exec into the named
// fixture), end to end: HTTP → identity gate → registry → docker exec → ring →
// SSE. They need a local Docker daemon and the fixture container the brief's
// row 3 starts; with COCKPIT_VERIFY_CONTAINER unset they SKIP (could-not-check
// — never a pass).

const containerDesk = "v/dispatch"

type containerFx struct {
	srv      *httptest.Server
	reg      *terminal.Registry
	answerer *http.Cookie
	name     string
}

func newContainerFx(t *testing.T) *containerFx {
	t.Helper()
	name := os.Getenv("COCKPIT_VERIFY_CONTAINER")
	if name == "" {
		t.Skip("could-not-check: COCKPIT_VERIFY_CONTAINER is unset — this row needs a local Docker daemon and the named fixture container (brief row 3)")
	}
	cfg := &schema.File{Cells: []schema.Cell{{
		Name: "v", Repos: []string{"example-org/console"}, AnswerAs: "operator",
		Loops: []schema.Loop{{Role: schema.RoleDispatch, App: "assay-desk-app", Target: &schema.TargetSpec{Kind: schema.TargetContainer, Container: name}}},
	}}}
	reg := terminal.NewRegistry(cfg)
	if err := reg.Attach(context.Background(), containerDesk, container.NewDriver(), terminal.Size{Cols: 120, Rows: 40}); err != nil {
		t.Fatalf("attach the container driver: %v", err)
	}
	if !reg.Alive(containerDesk) {
		t.Fatalf("the container driver did not attach to %q (is the fixture running?)", name)
	}
	// The INJECTED answerer session: the same resolver seam deskd wires from
	// the viewer session store (identity.TerminalGate over identity.SessionStore).
	store := identity.NewSessionStore(nil)
	sess, _ := store.Begin("operator", "viewer-session-token")
	gate := &identity.TerminalGate{
		Sessions: store,
		Rollout:  actRolloutOn{},
		Cell: func(n string) (identity.TerminalCell, bool) {
			if n != "v" {
				return identity.TerminalCell{}, false
			}
			return identity.TerminalCell{Name: "v", Repos: cfg.Cells[0].Repos, AnswerAs: "operator"}, true
		},
	}
	srv := httptest.NewServer(serve.NewV1(nil, cfg, nil, &serve.NamedActions{
		Terminal: &serve.TerminalActions{Registry: reg, Authority: gate},
	}, nil, reg, nil, nil))
	t.Cleanup(srv.Close)
	t.Cleanup(srv.CloseClientConnections)
	fx := &containerFx{srv: srv, reg: reg, name: name, answerer: &http.Cookie{Name: identity.SessionCookie, Value: sess.ID}}
	fx.waitState(t, 15*time.Second, terminal.StateIdle)
	return fx
}

func (fx *containerFx) waitState(t *testing.T, within time.Duration, want terminal.AgentState) {
	t.Helper()
	waitFor(t, "desk state "+string(want), within, func() bool {
		h, _ := fx.reg.Resolve(containerDesk)
		return h.State == string(want)
	})
}

func (fx *containerFx) send(t *testing.T, req contract.SendRequest) (int, []byte) {
	t.Helper()
	raw, _ := json.Marshal(req)
	r, _ := http.NewRequest(http.MethodPost, fx.srv.URL+sendPath(containerDesk), bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(fx.answerer)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

func nonce(t *testing.T) string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b[:])
}

// TestSendRoundTripThroughSeam — row 4: a wait_for:prompt send of an echo
// through the seam lands in the container's shell, the result reports the
// desk idle after the write, and the nonce comes back on the SSE stream.
func TestSendRoundTripThroughSeam(t *testing.T) {
	fx := newContainerFx(t)
	n := nonce(t)
	code, body := fx.send(t, contract.SendRequest{Input: "echo cockpit-" + n + "\n", WaitFor: contract.SendWaitPrompt, TimeoutMS: 8000, Epoch: fx.reg.Epoch(containerDesk)})
	if code != http.StatusOK {
		t.Fatalf("send answered %d: %s", code, body)
	}
	var res contract.SendResult
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatal(err)
	}
	if res.Outcome != contract.SendSent || res.StateAfter != string(terminal.StateIdle) {
		t.Fatalf("result = %+v, want outcome sent and state_after idle", res)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	es, err := contract.NewClient(fx.srv.URL, contract.WithHTTPClient(&http.Client{})).TerminalStream(ctx, containerDesk, "")
	if err != nil {
		t.Fatal(err)
	}
	defer es.Close()
	var seen []byte
	for !bytes.Contains(seen, []byte("cockpit-"+n+"\r\n")) && !bytes.Contains(seen, []byte("\ncockpit-"+n)) {
		e, err := es.Next()
		if err != nil {
			t.Fatalf("the nonce never came back on the stream (%v); saw %q", err, tail(seen))
		}
		if e.Type == contract.EventTerminalOutput {
			var d contract.TerminalOutputData
			_ = json.Unmarshal(e.Data, &d)
			seen = append(seen, d.Bytes...)
		}
	}
}

// TestSendTimeoutWritesNothing — row 5: with the shell busy in `sleep 30`, a
// wait_for:prompt send with a short timeout is a 409 AND its bytes provably
// never reached the container: an INDEPENDENT docker exec lists every
// process's command line, and the tty echo (which would print typed-ahead
// bytes into the scrollback at once) never shows them either.
func TestSendTimeoutWritesNothing(t *testing.T) {
	fx := newContainerFx(t)
	if code, body := fx.send(t, contract.SendRequest{Input: "sleep 30\n", WaitFor: contract.SendWaitNone, Epoch: fx.reg.Epoch(containerDesk)}); code != http.StatusOK {
		t.Fatalf("sleep send answered %d: %s", code, body)
	}
	time.Sleep(time.Second) // let the shell start the sleep and fall silent
	n := nonce(t)
	code, body := fx.send(t, contract.SendRequest{Input: "echo leaked-" + n + "\n", WaitFor: contract.SendWaitPrompt, TimeoutMS: 1500, Epoch: fx.reg.Epoch(containerDesk)})
	if code != http.StatusConflict || !strings.Contains(string(body), `"timeout"`) {
		t.Fatalf("the busy-desk send answered %d %s, want 409 timeout", code, body)
	}
	time.Sleep(500 * time.Millisecond)
	out, err := exec.Command("docker", "exec", fx.name, "sh", "-c", "cat /proc/[0-9]*/cmdline 2>/dev/null | tr '\\000' ' '").Output()
	if err != nil {
		t.Fatalf("independent docker exec failed: %v", err)
	}
	if !strings.Contains(string(out), "sleep 30") {
		t.Fatalf("the independent process read does not even see the sleep (positive control failed): %q", tail(out))
	}
	if strings.Contains(string(out), "leaked-"+n) {
		t.Fatalf("a timed-out send's bytes reached the container: %q", tail(out))
	}
	if b, _ := fx.reg.Since(containerDesk, 0); bytes.Contains(b, []byte("leaked-"+n)) {
		t.Fatalf("the tty echoed the timed-out input — it was written: %q", tail(b))
	}
}

func tail(b []byte) []byte {
	if len(b) > 400 {
		return b[len(b)-400:]
	}
	return b
}
