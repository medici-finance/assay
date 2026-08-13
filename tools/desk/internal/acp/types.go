package acp

import "encoding/json"

// Wire-shape structs for the subset of the ACP schema this spike-grade
// client speaks (github.com/agentclientprotocol/sdk's schema/schema.json,
// protocol version 1 -- see README.md for how that was confirmed against
// the live adapter). Unexported: SessionUpdate.Raw and PermissionRequest.Raw
// hand callers the untyped json.RawMessage for anything this package
// doesn't model itself.

type wireImplementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type wireFSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile"`
	WriteTextFile bool `json:"writeTextFile"`
}

type wireClientCapabilities struct {
	FS       wireFSCapabilities `json:"fs"`
	Terminal bool               `json:"terminal"`
}

type wireInitializeParams struct {
	ProtocolVersion    int                    `json:"protocolVersion"`
	ClientCapabilities wireClientCapabilities `json:"clientCapabilities"`
	ClientInfo         *wireImplementation    `json:"clientInfo,omitempty"`
}

type wireInitializeResult struct {
	ProtocolVersion int                `json:"protocolVersion"`
	AgentInfo       wireImplementation `json:"agentInfo"`
	AuthMethods     json.RawMessage    `json:"authMethods"`
}

type wireNewSessionParams struct {
	Cwd        string            `json:"cwd"`
	McpServers []json.RawMessage `json:"mcpServers"`
}

type wireNewSessionResult struct {
	SessionID string `json:"sessionId"`
}

type wireContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type wirePromptParams struct {
	SessionID string             `json:"sessionId"`
	Prompt    []wireContentBlock `json:"prompt"`
}

type wirePromptResult struct {
	StopReason string `json:"stopReason"`
}

type wireCancelParams struct {
	SessionID string `json:"sessionId"`
}

type wireSessionNotification struct {
	SessionID string          `json:"sessionId"`
	Update    json.RawMessage `json:"update"`
}

// wireSessionUpdateKind probes only the discriminator field of a
// session/update notification's "update" object.
type wireSessionUpdateKind struct {
	SessionUpdate string `json:"sessionUpdate"`
}

type wireSetSessionModeParams struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type wirePermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type wireToolCallUpdate struct {
	ToolCallID string `json:"toolCallId"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
}

type wireRequestPermissionParams struct {
	SessionID string                 `json:"sessionId"`
	ToolCall  wireToolCallUpdate     `json:"toolCall"`
	Options   []wirePermissionOption `json:"options"`
}

type wireReadTextFileParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
}

type wireReadTextFileResult struct {
	Content string `json:"content"`
}

type wireWriteTextFileParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}
