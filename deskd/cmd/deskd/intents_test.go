package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/config/intentkeys"
	"github.com/medici-finance/assay/deskd/config/schema"
)

// TestIntentWiringFailsClosed: the signed-intent path is ON only when the
// reviewed file enrols a key AND the gateway credential builds. Every other
// combination leaves the verifier nil — every signed intent refused — and the
// writer without a gateway.
func TestIntentWiringFailsClosed(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	enrolled := filepath.Join(dir, "enrolled.yaml")
	body := fmt.Sprintf("version: 1\nlogins:\n  alice:\n    - id: k\n      ed25519: %s\n", base64.StdEncoding.EncodeToString(pub))
	if err := os.WriteFile(enrolled, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(broken, []byte(body+"trust_all: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Gateway token files. Every token here is an obvious fixture, never a
	// real credential; the user-bound one exists only to be refused.
	tokFile := func(name, tok string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(tok+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := tokFile("gw-good", "ghs_gw_fixture_tok_0c1d")
	userBound := tokFile("gw-user", "ghp_gw_fixture_user_0c1d")
	gwEnv := func(file, login string) func(string) string {
		return func(k string) string {
			switch k {
			case intentGatewayTokenFileEnv:
				return file
			case intentGatewayLoginEnv:
				return login
			}
			return ""
		}
	}
	off := map[string]struct {
		path   string
		getenv func(string) string
	}{
		"missing file":          {filepath.Join(dir, "absent.yaml"), gwEnv(good, "gw[bot]")},
		"invalid file":          {broken, gwEnv(good, "gw[bot]")},
		"shipped file":          {filepath.Join("..", "..", "config", "intent-keys.yaml"), gwEnv(good, "gw[bot]")},
		"no gateway":            {enrolled, gwEnv("", "")},
		"human as gateway":      {enrolled, gwEnv(good, "operator")},
		"user-bound token":      {enrolled, gwEnv(userBound, "gw[bot]")},
		"missing gateway token": {enrolled, gwEnv(filepath.Join(dir, "gw-absent"), "gw[bot]")},
		"token file, no login":  {enrolled, gwEnv(good, "")},
	}
	for name, c := range off {
		t.Run(name, func(t *testing.T) {
			w := buildIntentWiring(c.path, c.getenv, nil)
			if w.Verifier != nil || w.Gateway != nil {
				t.Fatalf("signed intents ON for %s — must be OFF", name)
			}
			if !strings.Contains(w.Banner, "OFF") {
				t.Fatalf("banner %q does not say OFF", w.Banner)
			}
			if strings.Contains(w.Banner, "fixture_user") || strings.Contains(w.Banner, "fixture_tok") {
				t.Fatalf("the banner logs a token value: %q", w.Banner)
			}
		})
	}
	w := buildIntentWiring(enrolled, gwEnv(good, "gw[bot]"), nil)
	if w.Verifier == nil || w.Gateway == nil || !strings.Contains(w.Banner, "ON") {
		t.Fatalf("enrolled + gateway: %+v", w)
	}
	if strings.Contains(w.Banner, "fixture_tok") {
		t.Fatalf("the banner logs the gateway token: %q", w.Banner)
	}
}

// TestPollVerifiersPerCell: the poll's answer-record verifier (gap-3 item
// (5)) carries each cell's CURRENT answer_as and the loaded ring's keys,
// independent of whether the write path is on; an unknown cell, and a ring
// that failed to load, trust no bot-carried record.
func TestPollVerifiersPerCell(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := intentkeys.Parse([]byte(fmt.Sprintf("version: 1\nlogins:\n  alice:\n    - id: k\n      ed25519: %s\n", base64.StdEncoding.EncodeToString(pub))))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &schema.File{Cells: []schema.Cell{{Name: "one", AnswerAs: "alice"}, {Name: "two"}}}
	pick := pollAnswerVerifiers(cfg, ring)

	one := pick("one")
	if one.AnswerAs != "alice" || len(one.Keys("alice")) != 1 || !one.Keys("Alice")[0].Equal(pub) {
		t.Fatalf("cell one verifier = %+v", one)
	}
	if two := pick("two"); two.AnswerAs != schema.DefaultAnswerAs() || len(two.Keys(schema.DefaultAnswerAs())) != 0 {
		t.Fatalf("cell two verifier = answer_as %q, %d keys", two.AnswerAs, len(two.Keys(schema.DefaultAnswerAs())))
	}
	if unknown := pick("nope"); unknown.AnswerAs != "" || unknown.Keys != nil {
		t.Fatalf("unknown cell got a trusting verifier: %+v", unknown)
	}
	empty, _ := intentkeys.LoadOrEmpty(filepath.Join(t.TempDir(), "absent.yaml"))
	if k := pollAnswerVerifiers(cfg, empty)("one").Keys("alice"); len(k) != 0 {
		t.Fatalf("absent key file enrolled %d keys", len(k))
	}
}
