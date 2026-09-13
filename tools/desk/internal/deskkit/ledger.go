package deskkit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LedgerRelPath is the ledger's location inside the boundary (component-model.md
// §5): an append-only record of what a component created OUTSIDE the boundary,
// so removal can find and compensate it. The ledger file itself lives inside
// the boundary (`.assay/`), even though every line it holds describes an
// outside effect.
const LedgerRelPath = ".assay/ledger.jsonl"

// LedgerEntry is one line of the append-only outside-effect record
// (component-model.md §5): {component, version, step, kind, id, created, by}.
// An outside apply step MUST write exactly one of these at the moment it
// creates the effect; an outside step with no ledger line is a
// `deskmanifest lint` PROBLEM (§5, "an outside effect with no ledger line is a
// defect the lint MUST flag").
type LedgerEntry struct {
	// Component is the manifest id that made the effect (e.g. "assay/labels").
	Component string `json:"component"`
	// Version is the component's version at the moment of creation.
	Version string `json:"version"`
	// Step is the apply-step id within the manifest (e.g. "label-set").
	Step string `json:"step"`
	// Kind is the manifest's `ledger:` value for that step (e.g. "label",
	// "app-installation", "roster-vars", "forge-binding").
	Kind string `json:"kind"`
	// ID names the concrete thing created (a label name, an installation id, a
	// variable name, …) — specific enough that a compensation or a human
	// checklist can find it later.
	ID string `json:"id"`
	// Created is an RFC3339 UTC timestamp. AppendLedger fills it in when empty.
	Created string `json:"created"`
	// By names the actor that created the effect (a role App slug, a human
	// login, …).
	By string `json:"by"`
}

// LedgerPath returns the ledger file's path under repo root.
func LedgerPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(LedgerRelPath))
}

// AppendLedger appends one line to root's .assay/ledger.jsonl, creating the
// .assay/ directory if it does not exist yet. It is append-only by
// construction: it opens with O_APPEND and never reads, rewrites, or
// truncates existing lines — "the ledger is the only memory of outside
// effects" (component-model.md §5) and losing a line the same way a rewrite
// could is exactly the failure that clause exists to rule out.
//
// Component, Step, Kind, and ID are required — a ledger line that cannot name
// what it recorded is useless to a later compensation or human checklist, so
// AppendLedger refuses rather than write a line missing one.
func AppendLedger(root string, e LedgerEntry) error {
	if strings.TrimSpace(e.Component) == "" || strings.TrimSpace(e.Step) == "" ||
		strings.TrimSpace(e.Kind) == "" || strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("ledger entry missing a required field (component/step/kind/id): %+v", e)
	}
	if e.Created == "" {
		e.Created = time.Now().UTC().Format(time.RFC3339)
	}
	path := LedgerPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating ledger directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening ledger %s: %w", path, err)
	}
	defer f.Close()
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encoding ledger entry: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("writing ledger line: %w", err)
	}
	return nil
}

// ReadLedger reads every entry from root's ledger, in file (append) order. A
// missing ledger file is zero entries, not an error — a fresh repo, or one
// whose components have made no outside effects yet, has nothing to read.
func ReadLedger(root string) ([]LedgerEntry, error) {
	path := LedgerPath(root)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening ledger %s: %w", path, err)
	}
	defer f.Close()

	var out []LedgerEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e LedgerEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("%s:%d: malformed ledger line: %w", path, lineNo, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading ledger %s: %w", path, err)
	}
	return out, nil
}

// LedgerForComponent filters entries belonging to one component id, preserving
// their original (append/LIFO-reversible) order.
func LedgerForComponent(entries []LedgerEntry, component string) []LedgerEntry {
	var out []LedgerEntry
	for _, e := range entries {
		if e.Component == component {
			out = append(out, e)
		}
	}
	return out
}
