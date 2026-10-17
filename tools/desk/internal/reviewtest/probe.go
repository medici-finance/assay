// Package reviewtest contains local qualification fixtures, never review policy.
// Only command tests import it. No shipped command gains a flag or a runtime store.
package reviewtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const RequestEnv = "ASSAY_REVIEW_TEST_REQUEST"
const ResponseEnv = "ASSAY_REVIEW_TEST_RESPONSE"

// Request selects a bounded test-only adapter in a command's package main.
type Request struct {
	Case    string          `json:"case"`
	Size    int             `json:"size"`
	Root    string          `json:"root"`
	Home    string          `json:"home,omitempty"`
	Head    string          `json:"head,omitempty"`
	Base    string          `json:"base,omitempty"`
	Actions json.RawMessage `json:"actions,omitempty"`
	PRs     json.RawMessage `json:"prs,omitempty"`
	Now     string          `json:"now,omitempty"`
}

// Row is observed output, not a second classifier. Order is significant.
type Row struct {
	ID          string `json:"id"`
	Head        string `json:"head"`
	Action      string `json:"action"`
	Disposition string `json:"disposition"`
}

type Response struct {
	Actions      json.RawMessage   `json:"actions,omitempty"`
	PRs          json.RawMessage   `json:"prs,omitempty"`
	Rows         []Row             `json:"rows"`
	Depth        map[string]int    `json:"depth"`
	Idle         string            `json:"idle"`
	Exit         int               `json:"exit"`
	Facts        map[string]string `json:"facts"`
	Calls        []string          `json:"calls"`
	Attempts     []Attempt         `json:"attempts"`
	HTTPComplete bool              `json:"httpComplete"`
}

func Read(t *testing.T) Request {
	t.Helper()
	path := os.Getenv(RequestEnv)
	if path == "" {
		t.Skip("test-only adapter; run internal/reviewtest qualification")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r Request
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.Size < 1 || r.Size > 200 {
		t.Fatal("fixture size must be 1..200")
	}
	return r
}

func Write(t *testing.T, r Response) {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv(ResponseEnv), b, 0600); err != nil {
		t.Fatal(err)
	}
}

// Home keeps local fixture state across steady cycles. Every concurrent slot has
// its own root; no test ever imports the operator's config or credential files.
func Home(t *testing.T, r Request, roster string) {
	t.Helper()
	if r.Home == "" {
		t.Fatal("fixture home is required")
	}
	dir := filepath.Join(r.Home, ".config", "assay")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("HOME", r.Home)
}
