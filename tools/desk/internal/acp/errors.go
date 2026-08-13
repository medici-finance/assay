package acp

import "fmt"

// ErrClosed is returned by Client/conn operations after Close (or an
// unexpected agent-process exit) has torn down the connection.
var ErrClosed = fmt.Errorf("acp: connection closed")

// ErrNotInitialized is returned by NewSession, Prompt, and Cancel when
// called before Initialize has completed successfully. Initialize leaves
// the client in this state (rather than a usable one) when the agent
// negotiates an unsupported protocol version -- see
// ErrUnsupportedProtocolVersion.
var ErrNotInitialized = fmt.Errorf("acp: Initialize has not completed successfully")

// ErrUnsupportedProtocolVersion is returned by Initialize when the agent
// negotiates a protocol version outside Opts.SupportedProtocolVersions.
//
// Per the brief's Task 3, this is a fail-closed refusal: the client does not
// proceed to session/new (or any later call) on an unexpected version and
// guess at compatibility. Callers should treat this as terminal for the
// Client and Close it.
type ErrUnsupportedProtocolVersion struct {
	Negotiated int
	Supported  []int
}

func (e *ErrUnsupportedProtocolVersion) Error() string {
	return fmt.Sprintf("acp: agent negotiated protocol version %d, this client only supports %v", e.Negotiated, e.Supported)
}
