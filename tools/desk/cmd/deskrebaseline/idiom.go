package main

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

// idiom.go — the safe:idiom fact source. A row is idiom-stale only when the command it pins
// uses a tool idiom (one flag or command word) that a RECORDED RULING retired, and the list of
// such retirements is the checked-in file retired-idioms.json beside this package. Nothing is
// inferred from a pattern match alone: an idiom the list does not name is never safe:idiom,
// and a list entry that names no ruling reference is ignored. Adding an entry is a reviewed,
// human-merged change to that file, carrying the ruling it rests on.
//
// The list ships empty. No ruling retiring an idiom a Verify row spells has been recorded for
// this verb yet; the first entry is added with its ruling reference, never on this code's
// authority.

//go:embed retired-idioms.json
var retiredIdiomsJSON []byte

// retiredIdiom is one entry of the checked-in list: the retired idiom, the form that
// replaces it, and the reference to the ruling that retired it.
type retiredIdiom struct {
	Idiom       string `json:"idiom"`
	Replacement string `json:"replacement"`
	Ruling      string `json:"ruling"`
}

// retiredIdioms is the usable part of the checked-in list. It is a package var so a test can
// supply a fixture list; production reads only the embedded file.
var retiredIdioms = loadRetiredIdioms(retiredIdiomsJSON)

// rulingRef is the shape a ruling reference must have: an issue or pull-request reference
// (`#N` or `owner/repo#N`) or a GitHub issue/pull URL. Prose is not a reference.
var rulingRef = regexp.MustCompile(`^(?:(?:[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)?#[0-9]+|https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/(?:issues|pull)/[0-9]+)$`)

// idiomWord is the shape of an idiom or its replacement: one shell word with no quoting,
// spacing or shell metacharacter, so it can only ever match and replace a whole token.
var idiomWord = regexp.MustCompile(`^[A-Za-z0-9_./:@+-]+$`)

// loadRetiredIdioms parses the list and keeps only entries that can be acted on. It fails
// closed at every step: an unreadable file yields no entries; an entry without a well-formed
// ruling reference, with an idiom or replacement that is not a single plain word, or with a
// replacement equal to the idiom, is dropped; and an idiom listed twice is dropped entirely,
// since two entries leave the replacement ambiguous.
func loadRetiredIdioms(raw []byte) []retiredIdiom {
	var doc struct {
		Idioms []retiredIdiom `json:"idioms"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	count := map[string]int{}
	for _, e := range doc.Idioms {
		count[strings.TrimSpace(e.Idiom)]++
	}
	var out []retiredIdiom
	for _, e := range doc.Idioms {
		e.Idiom, e.Replacement, e.Ruling = strings.TrimSpace(e.Idiom), strings.TrimSpace(e.Replacement), strings.TrimSpace(e.Ruling)
		if !rulingRef.MatchString(e.Ruling) {
			continue
		}
		if !idiomWord.MatchString(e.Idiom) || !idiomWord.MatchString(e.Replacement) || e.Idiom == e.Replacement {
			continue
		}
		if count[e.Idiom] != 1 {
			continue
		}
		out = append(out, e)
	}
	return out
}

// idiomToken reports whether one shell token IS the idiom, or is the idiom carrying a value
// (`--flag=value`). A longer word that merely starts with the idiom never matches.
func idiomToken(token, idiom string) bool {
	return token == idiom || strings.HasPrefix(token, idiom+"=")
}

// retiredIdiomIn finds the one listed idiom command uses. It reports ok=false when the
// command uses none, or more than one (which replacement applies is then not a single fact),
// or when the command cannot be rewritten token by token (rewriteIdiom).
func retiredIdiomIn(command string) (retiredIdiom, bool) {
	var hits []retiredIdiom
	for _, e := range retiredIdioms {
		for _, tok := range renameTokens.FindAllString(command, -1) {
			if idiomToken(tok, e.Idiom) {
				hits = append(hits, e)
				break
			}
		}
	}
	if len(hits) != 1 {
		return retiredIdiom{}, false
	}
	if _, ok := rewriteIdiom(command, hits[0]); !ok {
		return retiredIdiom{}, false
	}
	return hits[0], true
}

// retiredIdiomFor returns the list entry for idiom, the lookup --open uses to write the edit.
func retiredIdiomFor(idiom string) (retiredIdiom, bool) {
	for _, e := range retiredIdioms {
		if e.Idiom == idiom {
			return e, true
		}
	}
	return retiredIdiom{}, false
}

// rewriteIdiom replaces each whole token that is the idiom with its replacement, keeping a
// `=value` suffix. The re-run proof and --open both write the row through this one function,
// so the row proven is the row written. Like renamed, it refuses (ok=false) a command whose
// text is not wholly made of tokens it recognises, and a command it would not change.
func rewriteIdiom(command string, e retiredIdiom) (string, bool) {
	spans := renameTokens.FindAllStringIndex(command, -1)
	end := 0
	for _, span := range spans {
		if strings.Trim(command[end:span[0]], " \t\r\n;|&()<>") != "" {
			return command, false
		}
		end = span[1]
	}
	if strings.Trim(command[end:], " \t\r\n;|&()<>") != "" {
		return command, false
	}
	out := renameTokens.ReplaceAllStringFunc(command, func(tok string) string {
		if idiomToken(tok, e.Idiom) {
			return e.Replacement + tok[len(e.Idiom):]
		}
		return tok
	})
	return out, out != command
}
