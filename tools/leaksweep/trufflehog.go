package main

// trufflehog engine — AGPL-3.0, invoked as an external CI tool and NOTHING else.
//
// No trufflehog code crosses into this repository: not embedded, not linked, not
// vendored, not copied from, not redistributed. This file writes a YAML config,
// runs a pinned binary, and reads the JSON lines it prints. That is the whole
// surface, and it is the only surface AGPL-3.0 permits without the obligations
// spreading to this tree.
//
// TRUFFLEHOG'S EXIT CODE IS NOT A VERDICT. By default it exits 0 whether or not
// it found anything; findings go to stdout as JSON lines and logs go to stderr.
// A wrapper that gated on `$?` would report every dirty tree as clean — the
// literal shape of the two false clean-certificates on this tool's record. So the
// findings are read from the report, and the exit code is used only to detect
// that the process itself failed.
//
// Verification is OFF (`--no-verification`). The gate path makes no network
// calls: a scanner that phones a third-party API mid-gate turns a deterministic
// local check into one that can hang, rate-limit, or leak the very strings it is
// scanning for to an outside service.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type trufflehogEngine struct{ binOverride string }

func (t *trufflehogEngine) name() string { return "trufflehog" }

func (t *trufflehogEngine) locate() (string, string, error) {
	bin, err := resolveBinary(t.binOverride, "LEAKSWEEP_TRUFFLEHOG_BIN", "trufflehog")
	if err != nil {
		return "", "", err
	}
	v, err := probeVersion(bin, "--version")
	if err != nil {
		return "", "", fmt.Errorf("trufflehog at %s did not answer a version probe: %v", bin, err)
	}
	return bin, v, nil
}

// trufflehogResult is the subset of one JSON line we consume.
//
// A custom-regex detector reports DetectorName "CustomRegex" for every rule and
// carries the rule's own name in ExtraData.name — so the rule identity MUST be
// read from ExtraData. Reading DetectorName instead would collapse all 130-odd
// house rules into one bucket and make the per-rule liveness proof meaningless.
type trufflehogResult struct {
	DetectorName   string            `json:"DetectorName"`
	Raw            string            `json:"Raw"`
	ExtraData      map[string]string `json:"ExtraData"`
	SourceMetadata struct {
		Data struct {
			Filesystem struct {
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"Filesystem"`
		} `json:"Data"`
	} `json:"SourceMetadata"`
}

func (t *trufflehogEngine) scan(bin, dir string, rules []rule) ([]finding, error) {
	cfgText, err := trufflehogConfig(rules)
	if err != nil {
		return nil, err
	}
	cfg, err := os.CreateTemp("", "leaksweep-trufflehog-*.yaml")
	if err != nil {
		return nil, err
	}
	defer os.Remove(cfg.Name())
	if _, err := cfg.WriteString(cfgText); err != nil {
		cfg.Close()
		return nil, err
	}
	cfg.Close()
	return t.run(bin, dir, cfg.Name())
}

func (t *trufflehogEngine) scanStock(bin, dir string) ([]string, error) {
	f, err := t.run(bin, dir, "")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	for _, x := range f {
		if !seen[x.ruleID] {
			seen[x.ruleID] = true
			ids = append(ids, x.ruleID)
		}
	}
	return ids, nil
}

func (t *trufflehogEngine) run(bin, dir, cfgPath string) ([]finding, error) {
	args := []string{"filesystem", dir, "--json", "--no-verification", "--no-update"}
	if cfgPath != "" {
		args = append(args, "--config", cfgPath)
	}
	cmd := exec.Command(bin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		return nil, fmt.Errorf("trufflehog exited %d: %v: %s", code, runErr,
			strings.TrimSpace(lastLines(stderr.String(), 4)))
	}
	var findings []finding
	for _, ln := range strings.Split(string(out), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var r trufflehogResult
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			// One unparseable line means we did not read the whole report, and a
			// partially-read report cannot support a clean certificate.
			return nil, fmt.Errorf("trufflehog report line did not parse as JSON: %v", err)
		}
		id := r.DetectorName
		if n, ok := r.ExtraData["name"]; ok && n != "" {
			id = n
		}
		p := r.SourceMetadata.Data.Filesystem.File
		if rel, err := filepath.Rel(dir, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = rel
		}
		findings = append(findings, finding{
			engine: "trufflehog", ruleID: id, path: filepath.ToSlash(p),
			line: r.SourceMetadata.Data.Filesystem.Line, match: r.Raw,
		})
	}
	return findings, nil
}

// trufflehogConfig renders the engine-neutral rules as trufflehog custom
// detectors.
//
// A trufflehog custom detector only runs on a chunk containing one of its
// KEYWORDS. A detector with no usable keyword therefore never fires — it is a
// dead rule that contributes silence indistinguishable from a pass — so an empty
// keyword is a hard error here rather than an omitted field.
func trufflehogConfig(rules []rule) (string, error) {
	var b strings.Builder
	b.WriteString("# GENERATED by tools/leaksweep — do not edit, do not commit.\n")
	b.WriteString("# Written to a temp file at run time; it contains the withheld token list.\n")
	b.WriteString("detectors:\n")
	for _, r := range rules {
		if r.keyword == "" {
			return "", fmt.Errorf("rule %s has no usable trufflehog keyword — a keywordless custom detector never fires, and a rule that never fires reports clean forever", r.id)
		}
		fmt.Fprintf(&b, "- name: %s\n", yamlQuote(r.id))
		fmt.Fprintf(&b, "  keywords:\n  - %s\n", yamlQuote(r.keyword))
		fmt.Fprintf(&b, "  regex:\n    adhoc: %s\n", yamlQuote(r.regex))
		if len(r.excepts) > 0 {
			b.WriteString("  exclude_regexes_match:\n")
			for _, x := range r.excepts {
				fmt.Fprintf(&b, "  - %s\n", yamlQuote("^"+regexp.QuoteMeta(x)+"$"))
			}
		}
	}
	return b.String(), nil
}

// yamlQuote emits a YAML double-quoted scalar. Every rule regex and token goes
// through it, so a token containing `:`, `#`, `{`, a leading `*`, or a backslash
// cannot silently restructure the document into a config that parses but means
// something else.
func yamlQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
