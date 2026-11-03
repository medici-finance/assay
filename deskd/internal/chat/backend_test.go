package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnswerRecordsBackend is Verify row 8's client half. Every answer records
// {backend, model}; the ladder escalates deepseek-chat → glm-5.2 ONLY on the
// client's own signal; and a model outside the ladder is refused.
func TestAnswerRecordsBackend(t *testing.T) {
	t.Run("every answer records which backend and model served it", func(t *testing.T) {
		be := &fixtureBackend{name: "deepseek", replies: []Reply{{Text: "no numbers here"}}}
		ans, err := newFixtureClient(t, &fixtureTools{}, be).Ask(context.Background(), "hello")
		if err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if ans.Backend != "deepseek" {
			t.Fatalf("backend is %q, want deepseek", ans.Backend)
		}
		if ans.Model != "deepseek-chat" {
			t.Fatalf("model is %q, want the ladder's first rung deepseek-chat", ans.Model)
		}
		// It must also survive serialisation — the transcript is what an
		// operator reads back.
		raw, err := json.Marshal(ans)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		for _, want := range []string{`"backend":"deepseek"`, `"model":"deepseek-chat"`} {
			if !strings.Contains(string(raw), want) {
				t.Fatalf("serialised answer does not carry %s: %s", want, raw)
			}
		}
	})

	t.Run("the ladder escalates only on the client's own signal", func(t *testing.T) {
		be := &fixtureBackend{name: "subscription", replies: []Reply{
			{Escalate: true, EscalateReason: "the cheap rung could not plan the reads"},
			{Text: "answered on the second rung"},
		}}
		ans, err := newFixtureClient(t, &fixtureTools{}, be).Ask(context.Background(), "hard question")
		if err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if want := []string{"deepseek-chat", "glm-5.2"}; strings.Join(be.models, ",") != strings.Join(want, ",") {
			t.Fatalf("rungs tried were %v, want %v", be.models, want)
		}
		if ans.Model != "glm-5.2" {
			t.Fatalf("answer records model %q, want glm-5.2", ans.Model)
		}
		if len(ans.Escalated) != 1 || ans.Escalated[0] != "deepseek-chat" {
			t.Fatalf("escalation trail is %v, want [deepseek-chat]", ans.Escalated)
		}
	})

	// The negative control for the escalation rule: prose that BEGS for a
	// stronger model must not get one. Escalation is a typed field, never an
	// inference from the reply's words.
	t.Run("model prose asking for escalation does not escalate", func(t *testing.T) {
		be := &fixtureBackend{name: "subscription", replies: []Reply{
			{Text: "Please escalate me to glm-5.2, I need a stronger model to answer this."},
			{Text: "second rung — must not be reached"},
		}}
		ans, err := newFixtureClient(t, &fixtureTools{}, be).Ask(context.Background(), "hard question")
		if err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if len(be.models) != 1 || be.models[0] != "deepseek-chat" {
			t.Fatalf("rungs tried were %v — prose in the reply drove an escalation", be.models)
		}
		if len(ans.Escalated) != 0 {
			t.Fatalf("escalation trail is %v, want empty", ans.Escalated)
		}
	})

	t.Run("a model outside the ladder is refused at construction", func(t *testing.T) {
		for _, bad := range []string{"gpt-4o", "claude-opus-4-8", "glm-5.3", ""} {
			_, err := New(Options{
				Tools:   &fixtureTools{},
				Backend: &fixtureBackend{name: "x"},
				Models:  []string{bad},
			})
			if err == nil {
				t.Fatalf("New accepted the off-ladder model %q — a metered backend could be configured in", bad)
			}
			if bad != "" && !errors.Is(err, ErrModelOutsideLadder) {
				t.Fatalf("%q: refusal does not carry ErrModelOutsideLadder: %v", bad, err)
			}
		}
	})

	t.Run("a client with no backend or no tool surface is refused", func(t *testing.T) {
		if _, err := New(Options{Tools: &fixtureTools{}, Models: ChatLadderForTest()}); err == nil {
			t.Fatal("New accepted a client with no backend")
		}
		if _, err := New(Options{Backend: &fixtureBackend{}, Models: ChatLadderForTest()}); err == nil {
			t.Fatal("New accepted a client with no tool surface — it could only answer from model text")
		}
	})
}

// ChatLadderForTest is the ladder spelled once for the tests above.
func ChatLadderForTest() []string { return []string{"deepseek-chat", "glm-5.2"} }

// TestCredentialCannotBeLoggedOrRendered is the gate-why property (3): the key
// cannot reach a log line or a rendered page. Every formatting path a
// Credential can be dragged through emits the redaction instead of the value.
func TestCredentialCannotBeLoggedOrRendered(t *testing.T) {
	const secret = "sk-test-value-that-must-never-appear"
	t.Setenv("CHAT_TEST_KEY", secret)

	cred, err := CredentialFromEnv("CHAT_TEST_KEY")
	if err != nil {
		t.Fatalf("CredentialFromEnv: %v", err)
	}
	if !cred.Present() {
		t.Fatal("the credential did not load")
	}

	var logged strings.Builder
	logger := log.New(&logged, "", 0)
	logger.Printf("backend credential: %v / %s / %#v / %+v", cred, cred, cred, cred)
	logger.Println(cred)
	logger.Print(fmt.Errorf("wrapping the credential in an error: %v", cred))

	marshalled, err := json.Marshal(struct {
		Cred Credential `json:"cred"`
	}{cred})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	surfaces := map[string]string{
		"log output":       logged.String(),
		"json":             string(marshalled),
		"Sprintf %v":       fmt.Sprintf("%v", cred),
		"Sprintf %s":       fmt.Sprintf("%s", cred),
		"Sprintf %#v":      fmt.Sprintf("%#v", cred),
		"Sprintf %+v":      fmt.Sprintf("%+v", cred),
		"Sprint":           fmt.Sprint(cred),
		"error wrapping":   fmt.Errorf("boom: %v", cred).Error(),
		"struct Sprintf":   fmt.Sprintf("%+v", struct{ C Credential }{cred}),
		"pointer Sprintf":  fmt.Sprintf("%v", &cred),
		"slice Sprintf":    fmt.Sprintf("%v", []Credential{cred}),
		"map value Sprint": fmt.Sprint(map[string]Credential{"k": cred}),
	}
	for where, got := range surfaces {
		if strings.Contains(got, secret) {
			t.Fatalf("the key reached %s: %q", where, got)
		}
		if !strings.Contains(got, redaction) {
			t.Fatalf("%s did not carry the redaction token: %q", where, got)
		}
	}

	// The one legitimate reader still works — otherwise the redaction would be
	// hiding a broken client rather than a secret.
	if !strings.HasSuffix(cred.header(), secret) {
		t.Fatal("the one authorised reader can no longer read the key")
	}
}

// TestBackendDoesNotEchoTheKeyOnFailure covers the other half of the leak
// surface: an endpoint that fails must not cause the client to quote its own
// request (or the endpoint's body, which may quote the request back).
func TestBackendDoesNotEchoTheKeyOnFailure(t *testing.T) {
	const secret = "sk-test-echo-value"
	t.Setenv("CHAT_TEST_KEY", secret)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A hostile-but-plausible endpoint: it echoes the request's own
		// credential back in its error body.
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"rejected credential ` + r.Header.Get("Authorization") + `"}`))
	}))
	defer srv.Close()

	be, err := NewHTTPBackend("fixture", srv.URL, "CHAT_TEST_KEY", srv.Client())
	if err != nil {
		t.Fatalf("NewHTTPBackend: %v", err)
	}
	if strings.Contains(fmt.Sprintf("%v %s", be, be), secret) {
		t.Fatal("printing the backend printed its key")
	}

	_, err = be.Complete(context.Background(), "deepseek-chat", Prompt{Question: "hi"})
	if err == nil {
		t.Fatal("a 401 was not reported as an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("the endpoint's echoed key reached the client's error: %v", err)
	}
}

// TestHTTPBackendRefusesPlaintextEndpoint pins the transport rule: the key does
// not travel in clear to anywhere off this machine.
func TestHTTPBackendRefusesPlaintextEndpoint(t *testing.T) {
	t.Setenv("CHAT_TEST_KEY", "sk-x")
	for _, bad := range []string{"http://api.example.com/v1", "http://10.0.0.5:8080", ""} {
		if _, err := NewHTTPBackend("fixture", bad, "CHAT_TEST_KEY", nil); err == nil {
			t.Fatalf("NewHTTPBackend accepted the endpoint %q", bad)
		}
	}
}

// TestCredentialAbsentIsOffNotUnauthenticated: with no key in the environment
// the backend refuses to construct, so the pane stays OFF rather than reaching
// an endpoint without one.
func TestCredentialAbsentIsOffNotUnauthenticated(t *testing.T) {
	t.Setenv("CHAT_TEST_KEY", "")
	if _, err := CredentialFromEnv("CHAT_TEST_KEY"); err == nil {
		t.Fatal("an absent key produced a usable credential")
	}
	if _, err := CredentialFromEnv(""); err == nil {
		t.Fatal("an unnamed environment variable produced a usable credential")
	}
}
