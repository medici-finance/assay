package deskkit

import (
	"bytes"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

// CellctlShimTemplate is the wrapper cellctl generates for one desk verb (cmd/cellctl genShims;
// the bash oracle carries a byte-identical copy, pinned by a cellctl parity test). It lives here
// so the generator and the instrument pin check share ONE copy of the format: the pin check admits
// a shim only when the file is byte-for-byte this template rendered for the fields it carries.
//
// Everything in it is LITERAL shell, evaluated when the generated shim itself runs under the
// caller's REAL, un-swapped HOME — only the `env HOME=` on the exec lines swaps it for the
// wrapped verb.
//
// assay#1145: `gh`'s ambient credential (keychain on macOS, hosts.yml-adjacent elsewhere) is
// keyed to the REAL HOME, so a `gh` subprocess a shimmed verb shells out to silently loses auth
// once HOME is swapped. It is resolved HERE, before the swap.
//
// assay#1631: it is NOT handed to the verb as GH_TOKEN. Desk verbs read an inherited GH_TOKEN as
// the operator's EXPLICIT choice of credential (deskdispatch skipped its role-App mint on it), so
// exporting the human login there made every self-minting verb act as the human. The token
// travels as CELLCTL_GH_AMBIENT instead — a name no desk verb reads — and only the cell's `gh`
// wrapper (cellctl's ghWrapTemplate, prepended to the verb's PATH) turns it back into GH_TOKEN,
// for a `gh` child with none of its own. An explicit GH_TOKEN/GH_ENTERPRISE_TOKEN already in the
// caller's env still passes through untouched and is never looked up against.
const CellctlShimTemplate = `#!/usr/bin/env bash
# cellctl shim (__CELL__): run this desk verb with the CELL config-home; the session keeps the
# real HOME. the ambient gh credential is keyed to the real HOME, so it is resolved HERE (before
# HOME is swapped below) and handed over as CELLCTL_GH_AMBIENT — never as GH_TOKEN, which desk
# verbs read as an explicit operator override (assay#1631). Only the cell's gh wrapper, first on
# the verb's PATH, turns it back into GH_TOKEN, for a ` + "`gh`" + ` child with none of its own (assay#1145).
gh_token="${CELLCTL_GH_AMBIENT:-}"
if [[ -z "$gh_token" && -z "${GH_TOKEN:-}" && -z "${GH_ENTERPRISE_TOKEN:-}" ]] && command -v gh >/dev/null 2>&1; then
  gh_token="$(gh auth token 2>/dev/null || true)"
fi
if [[ -n "$gh_token" ]]; then
  # exported, never an env(1) argument: argv is readable by other local users for as long as
  # env runs, the environment is not.
  export CELLCTL_GH_AMBIENT="$gh_token"
  exec env HOME="__CELL_HOME__" PATH="__GH_WRAP__:$PATH" "__BIN__" "$@"
else
  exec env HOME="__CELL_HOME__" "__BIN__" "$@"
fi
`

// RenderCellctlShim is CellctlShimTemplate for one cell and one desk verb binary. Substitution is
// by plain replacement, never sed: a path or cell name holding `&` or a regex metacharacter would
// otherwise corrupt it.
func RenderCellctlShim(cell, home, ghWrap, bin string) string {
	return strings.NewReplacer(
		"__CELL__", cell,
		"__CELL_HOME__", home,
		"__GH_WRAP__", ghWrap,
		"__BIN__", bin,
	).Replace(CellctlShimTemplate)
}

// CellctlShimMarker opens the header comment line of every desk-verb shim cellctl generates.
// A `#!` script carrying a line that starts with it is treated as a shim: its own bytes are never
// the pinned binary's, so it is either admitted as exactly the generated shim or not at all.
const CellctlShimMarker = "# cellctl shim ("

// shimPlaceholder matches one field slot of CellctlShimTemplate.
var shimPlaceholder = regexp.MustCompile(`__(?:CELL_HOME|CELL|GH_WRAP|BIN)__`)

// shimShape matches CellctlShimTemplate with every field slot captured, and shimSlots names the
// slot each capture group fills. Extraction only proposes field values; ShimExecTarget admits a
// shim on byte equality with the re-rendered template, never on this match alone.
var shimShape, shimSlots = func() (*regexp.Regexp, []string) {
	var pat strings.Builder
	var slots []string
	last := 0
	for _, m := range shimPlaceholder.FindAllStringIndex(CellctlShimTemplate, -1) {
		pat.WriteString(regexp.QuoteMeta(CellctlShimTemplate[last:m[0]]) + `([^"\n]*)`)
		slots = append(slots, CellctlShimTemplate[m[0]:m[1]])
		last = m[1]
	}
	return regexp.MustCompile(`\A` + pat.String() + regexp.QuoteMeta(CellctlShimTemplate[last:]) + `\z`), slots
}()

// shimFieldUnsafe are the bytes that let a field value change what a rendered shim runs: inside a
// double-quoted bash word `"`, `$`, backtick and backslash are live, and a line break or NUL
// starts a new command (the cell name sits in a comment line).
const shimFieldUnsafe = "\"$`\\\n\r\x00"

// ShimExecTarget reports whether script is a cellctl shim and, when it is, the one binary it runs.
// A shim is admitted ONLY when its bytes equal CellctlShimTemplate rendered for the field values
// it carries, every value is a plain literal, and the binary is an absolute path env(1) runs as a
// program. Anything else that carries the marker (a hand edit anywhere in the file, a hotfix
// shim's run-time "$bin", the bash oracle drifting from the Go template) returns isShim=true with
// an error: what runs is not knowable from the text, so the caller reports could-not-check.
func ShimExecTarget(script []byte) (target string, isShim bool, err error) {
	if !bytes.HasPrefix(script, []byte("#!")) || !bytes.Contains(script, []byte("\n"+CellctlShimMarker)) {
		return "", false, nil
	}
	m := shimShape.FindStringSubmatch(string(script))
	if m == nil {
		return "", true, errors.New("not byte-for-byte a generated cellctl shim")
	}
	f := map[string]string{}
	for i, slot := range shimSlots {
		v := m[i+1]
		if prev, seen := f[slot]; seen && prev != v {
			return "", true, errors.New("not byte-for-byte a generated cellctl shim: " + slot + " differs between its uses")
		}
		if v == "" || strings.ContainsAny(v, shimFieldUnsafe) {
			return "", true, errors.New("cellctl shim field " + slot + " is not a plain literal")
		}
		f[slot] = v
	}
	bin := f["__BIN__"]
	if !filepath.IsAbs(bin) || strings.Contains(bin, "=") {
		return "", true, errors.New("cellctl shim binary " + bin + " is not an absolute path env(1) runs as a program")
	}
	if RenderCellctlShim(f["__CELL__"], f["__CELL_HOME__"], f["__GH_WRAP__"], bin) != string(script) {
		return "", true, errors.New("not byte-for-byte a generated cellctl shim")
	}
	return bin, true, nil
}
