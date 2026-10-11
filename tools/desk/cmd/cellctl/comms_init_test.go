//go:build darwin || linux

package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommsInitNativePrivateAndIdempotent(t *testing.T) {
	dir, err := os.MkdirTemp("", "ci-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(dir, ".codex"), 0700)
	os.WriteFile(filepath.Join(dir, ".codex", "auth.json"), []byte("{\"fixture\":true}\n"), 0600)
	bin := filepath.Join(dir, "bin")
	os.Mkdir(bin, 0700)
	for _, name := range []string{"codex", "commsgw", "commsloop"} {
		os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\necho 'codex-cli fixture-version'\n"), 0700)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	c := &Cell{Name: "fixture", Ref: "fixture", Kind: "house", Dir: dir, Env: envWith(map[string]string{"HOME": dir, "CELLS_ROOT": dir, "CELL": "fixture", "CELL_KIND": "house", "CELL_REPO_SLUG": "example/repo", "ROLES": strings.Join(knownRoles, " ")})}
	os.WriteFile(filepath.Join(dir, "cell.env"), []byte("CELL=fixture\nCELL_KIND=house\n"), 0600)
	opts := commsInitOptions{Model: "fixture-model", Effort: "low"}
	c.Env.Put("DRY_RUN", "1")
	before := snapshotTree(t, dir)
	if err := c.initComms(opts); err != nil {
		t.Fatal(err)
	}
	if after := snapshotTree(t, dir); after != before {
		t.Fatal("dry init changed files")
	}
	c.Env.Put("DRY_RUN", "")
	if err := c.initComms(opts); err != nil {
		t.Fatal(err)
	}
	cfg, err := c.loadDeskComms()
	if err != nil || cfg == nil {
		t.Fatal("invalid generated manifest", err)
	}
	raw, _ := os.ReadFile(cfg.SigningKeys["the-desk"])
	seed, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(seed) != 32 {
		t.Fatal("bad seed")
	}
	var trust map[string]string
	b, _ := os.ReadFile(cfg.TrustStore)
	json.Unmarshal(b, &trust)
	if trust[c.Name] != base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)) {
		t.Fatal("trust/key mismatch")
	}
	if !strings.Contains(string(cfg.Decider), "comms-reader") || !strings.Contains(string(cfg.Decider), "fixture-version") {
		t.Fatal("native reader not pinned")
	}
	before = snapshotTree(t, dir)
	if err = c.initComms(opts); err != nil {
		t.Fatal(err)
	}
	if after := snapshotTree(t, dir); after != before {
		t.Fatal("repeated init rotated state")
	}
	c.Env.Put("CELL_COMMS_CONFIG", "")
	if err = c.initComms(opts); err != nil {
		t.Fatal("manifest selection recovery", err)
	}
	for _, path := range []string{cfg.TrustStore, cfg.SigningKeys["the-desk"], c.Env.Get("CELL_COMMS_CONFIG")} {
		fi, err := os.Stat(path)
		if err != nil || fi.Mode().Perm() != 0600 {
			t.Fatal("private custody", path, err)
		}
	}
}
