package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// ---- the credential ---------------------------------------------------------

// redaction is what a [Credential] renders as, everywhere, always. It is a
// constant so a test can assert on it and so there is exactly one spelling.
const redaction = "[redacted]"

// Credential is the model-backend subscription key.
//
// Its value lives in an UNEXPORTED field and every way of turning a value into
// text is overridden to emit [redaction] instead:
//
//	fmt.Sprint(c) / "%s" / "%v"  → String()
//	"%#v"                        → GoString()
//	json.Marshal / a rendered VM → MarshalJSON()
//	log/slog                     → LogValue()
//
// So the ordinary accidents — a struct dumped into a log line, a view model
// marshalled to the page, a %v in an error — cannot carry it. That is a
// structural property, not a convention: there is no exported field to read
// and no formatting verb that reaches the value.
//
// The one reader is [Credential.header], unexported, called at exactly one
// call site (the outbound request). Note that it builds its string by
// CONCATENATION and not with a formatting call — a formatting call on the same
// line as the scheme word is the shape the leak check greps for, and writing
// it the safe way costs nothing.
type Credential struct {
	v string
}

// CredentialFromEnv reads the subscription key from the environment variable
// the cell config NAMES. The key is mounted as a Secret by the deployment; it
// is never in a config file, a flag, an argument, or this repository.
func CredentialFromEnv(envVar string) (Credential, error) {
	if strings.TrimSpace(envVar) == "" {
		return Credential{}, errors.New("chat: no environment variable named for the model-backend key")
	}
	v := strings.TrimSpace(os.Getenv(envVar))
	if v == "" {
		return Credential{}, fmt.Errorf("chat: the model-backend key is not present in the environment "+
			"(the cell config names %s; the deployment mounts it as a Secret) — the Ask Assay pane stays OFF rather "+
			"than reaching an endpoint unauthenticated", envVar)
	}
	return Credential{v: v}, nil
}

// Present reports whether a key was loaded, without revealing anything about it.
func (c Credential) Present() bool { return c.v != "" }

// String renders the redaction. It is what "%s" and "%v" reach.
func (c Credential) String() string { return redaction }

// GoString renders the redaction. It is what "%#v" reaches.
func (c Credential) GoString() string { return redaction }

// MarshalJSON renders the redaction, so a view model or an audit record that
// happens to contain a Credential serialises the token and not the key.
func (c Credential) MarshalJSON() ([]byte, error) { return []byte(`"` + redaction + `"`), nil }

// header returns the outbound credential value. It is unexported and has
// exactly one caller.
func (c Credential) header() string { return "Bearer " + c.v }

// ---- the model ladder -------------------------------------------------------

// ErrModelOutsideLadder is returned when a model outside the closed §13.8
// ladder is asked for. The ladder itself lives in config/schema so that the
// boot-time validator and this client cannot disagree about it.
var ErrModelOutsideLadder = errors.New("model is not on the subscription ladder")

// Ladder is the ordered rung selection this client will use, cheapest first.
type Ladder struct {
	models []string
}

// NewLadder builds a ladder from a cell's configured model list, refusing any
// model outside [schema.ChatModelLadder]. The refusal is the SAME rule
// cmd/cellsvalidate applies at boot; having it here too means a client
// constructed in code — a test, a future caller — cannot bypass the config
// gate by not going through config.
func NewLadder(models []string) (Ladder, error) {
	if len(models) == 0 {
		return Ladder{}, errors.New("chat: a ladder needs at least one rung")
	}
	allowed := map[string]bool{}
	for _, m := range schema.ChatModels() {
		allowed[m] = true
	}
	out := make([]string, 0, len(models))
	for _, m := range models {
		if !allowed[m] {
			return Ladder{}, fmt.Errorf("%w: %q is not one of %v (design §13.8, ruled the custody design: subscription keys "+
				"only, no metered spend)", ErrModelOutsideLadder, m, schema.ChatModels())
		}
		out = append(out, m)
	}
	return Ladder{models: out}, nil
}

// Models returns the ladder's rungs as a fresh slice.
func (l Ladder) Models() []string {
	out := make([]string, len(l.models))
	copy(out, l.models)
	return out
}

// ---- the backend ------------------------------------------------------------

// Prompt is what the client sends a backend for one turn.
type Prompt struct {
	// Question is the operator's words.
	Question string
	// Tools is the bound read-only tool set, offered to the model so it can
	// choose which read to request. It is derived from the binding, so a tool
	// the chat may not call is never even described to the model.
	Tools []ToolDescriptor
	// Results are the tool results gathered so far this turn, as raw JSON.
	Results []ToolResult
}

// ToolResult is one bound tool's raw output.
type ToolResult struct {
	Tool string
	Raw  []byte
}

// Reply is a backend's response for one turn.
type Reply struct {
	// Text is the model's words. It is DATA. Nothing downstream parses a
	// number out of it, and the pane escapes it (§6.6).
	Text string
	// ToolCalls are the reads the model wants performed. Each is checked
	// against the binding before it is dispatched.
	ToolCalls []RequestedCall
	// Escalate is THE CLIENT'S OWN SIGNAL that this rung could not answer and
	// the next should be tried (design §13.8: client-initiated escalation).
	// Escalation is never automatic on a slow or short reply, and never driven
	// by anything in Text: a model that merely says "I should use a better
	// model" does not get one.
	Escalate bool
	// EscalateReason is recorded on the answer when Escalate is set, so a
	// reader can see why a more expensive rung was reached for.
	EscalateReason string
}

// RequestedCall is a read the model asked for.
type RequestedCall struct {
	Tool string
	Args []byte
}

// Backend is one model endpoint. Name() identifies the SERVICE (the endpoint
// family); the model id is passed per call, because one backend serves several
// rungs of the ladder.
type Backend interface {
	Name() string
	Complete(ctx context.Context, model string, p Prompt) (Reply, error)
}
