package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// Author-identity classification (spec §3.1, §4.2). The partition is a
// CONFIGURABLE map supplied per target: each rule is a regular expression tested
// against the commit's raw author identity; the first matching rule's class
// wins. An author matching NO rule falls into the explicit IdentityUnclassified
// class — never silently dropped, and never merged into human/agent/automation
// (which would launder unknown provenance as known).
//
// The class strings are free-form so a target can name its own classes, but the
// three the churn readout is defined against are the conventional ones below.

// The conventional author-identity classes (spec §4.2).
const (
	IdentityHuman        = "human"
	IdentityAgent        = "agent"
	IdentityAutomation   = "automation"
	IdentityUnclassified = "unclassified"
)

// IdentityRule maps commits whose author matches Pattern to Class.
type IdentityRule struct {
	// Pattern is a Go regular expression tested against the author's raw
	// "Name <email>" line, name, and email.
	Pattern string `json:"pattern"`
	// Class is the author-identity class assigned on a match (e.g. "human",
	// "agent", "automation" — free-form so a target may name its own).
	Class string `json:"class"`

	re *regexp.Regexp
}

// IdentityMap is the ordered rule set. First match wins; an empty map (or no
// match) yields IdentityUnclassified.
type IdentityMap struct {
	Rules []IdentityRule `json:"rules"`
}

// compile pre-compiles every rule's pattern, failing loudly on a bad regex so a
// malformed identity map is a hard error rather than a silently-inert rule that
// would misclassify every commit it should have matched.
func (m *IdentityMap) compile() error {
	for i := range m.Rules {
		r := &m.Rules[i]
		if r.Class == "" {
			return fmt.Errorf("qualgen: identity rule %d has an empty class", i)
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return fmt.Errorf("qualgen: identity rule %d pattern %q: %w", i, r.Pattern, err)
		}
		r.re = re
	}
	return nil
}

// LoadIdentityMap reads and compiles an identity map from a JSON file. An empty
// path returns an empty map (every author is unclassified) rather than an error,
// so classification degrades honestly when no map is configured.
func LoadIdentityMap(path string) (*IdentityMap, error) {
	if path == "" {
		return &IdentityMap{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("qualgen: read identity map: %w", err)
	}
	var m IdentityMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("qualgen: parse identity map %s: %w", path, err)
	}
	if err := m.compile(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Classify returns the author-identity class for a commit. The first rule whose
// pattern matches the author's raw line, name, or email wins; an author matching
// no rule is IdentityUnclassified.
func (m *IdentityMap) Classify(c Commit) string {
	for i := range m.Rules {
		r := &m.Rules[i]
		if r.re == nil {
			// A map constructed in-process (tests) may not have been compiled;
			// compile lazily so an uncompiled rule never silently fails to match.
			if re, err := regexp.Compile(r.Pattern); err == nil {
				r.re = re
			} else {
				continue
			}
		}
		if r.re.MatchString(c.AuthorRaw) || r.re.MatchString(c.AuthorName) || r.re.MatchString(c.AuthorEmail) {
			return r.Class
		}
	}
	return IdentityUnclassified
}
