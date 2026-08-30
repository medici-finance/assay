package deskkit

// widthstore.go — where a SET pool width is stored, and the single reader that answers
// "what width is this loop running at right now?".
//
// It lives in deskkit rather than in the roster command because two different binaries need
// the same answer and must not derive it twice: `deskroster width --role` reports it to an
// operator, `deskboard throughput` uses it as the denominator of the depth/slot ratio, and
// a loop resolves it every tick to size its pool. A second implementation of "is this entry
// still fresh?" is exactly how the depth/slot ratio and the pool it describes drift apart.
//
// WHY A ROLE-KEYED FILE AND NOT A SESSION BEACON FIELD. A beacon is keyed by SESSION, holds
// the session's own role as one scalar, and is persisted by a whole-file truncating write
// with no locking. Putting a width there breaks three ways: the coordinator setting another
// desk's width would relabel ITS OWN session; two sessions writing one beacon lose each
// other's updates; and a width has to be readable by a session that does not know the
// setter's id. Keying by CANONICAL LOOP NAME in a sibling directory has none of those
// problems and matches what a width actually describes — a window, not a session.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WidthEntry is one loop's stored pool width. SetBy is attribution, never authority: what
// admits a width is the bound in width.go, not who wrote the file.
type WidthEntry struct {
	Loop    string `json:"loop"`
	Width   int    `json:"width"`
	SetBy   string `json:"set_by,omitempty"`
	Updated string `json:"updated"`
}

// WidthDir is the beacon store's sibling: <StateDir>/roster/width.
func WidthDir() (string, error) {
	base, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "roster", "width"), nil
}

// LoadWidth reads the stored entry for a loop. The name is canonicalised through the SAME
// loop-name roster the stop flag uses, so a session presenting a retired name reads the
// entry the canonical name wrote — a rename must not reset a pool.
//
// Three states, and the caller must keep them apart:
//
//	(entry, true,  nil) — a FRESH width is in force
//	(nil,   false, nil) — nothing stored, or what was stored EXPIRED: use the default
//	(nil,   false, err) — could not read: the caller must NOT substitute a default
//
// A malformed file is the third state, not the second. Reporting corruption as absence
// would silently revert a width a desk is relying on, with every instrument reporting the
// default as deliberate.
func LoadWidth(loop string, now time.Time) (*WidthEntry, bool, error) {
	canonical, known := CanonicalLoopName(loop)
	if !known {
		return nil, false, Refused(fmt.Sprintf(
			"refused: %q is not a loop this roster recognises. Known loop names: %v", loop, KnownLoopNames()))
	}
	dir, err := WidthDir()
	if err != nil {
		return nil, false, Unverifiable("cannot resolve the width store path", err)
	}
	path := filepath.Join(dir, canonical+".json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, Unverifiable("cannot read "+path, err)
	}
	var e WidthEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, false, Unverifiable("malformed width entry at "+path+
			" — refusing rather than treating a corrupt file as 'no width set'", err)
	}
	upd, perr := time.Parse(time.RFC3339, e.Updated)
	if perr != nil {
		return nil, false, Unverifiable("unparseable `updated` in "+path, perr)
	}
	if now.Sub(upd) > WidthTTL {
		return nil, false, nil // expired: decay to the default, the safe direction
	}
	return &e, true, nil
}

// SaveWidth persists one loop's width. It does NOT apply the bound — CheckWidth is the gate,
// and keeping them separate is deliberate: every caller must pass the bound explicitly
// rather than inherit it from a store that might one day stop applying it.
func SaveWidth(e *WidthEntry) error {
	canonical, known := CanonicalLoopName(e.Loop)
	if !known {
		return Refused(fmt.Sprintf("refused: %q is not a loop this roster recognises", e.Loop))
	}
	dir, err := WidthDir()
	if err != nil {
		return Unverifiable("cannot resolve the width store path", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Unverifiable("cannot create "+dir, err)
	}
	out := *e
	out.Loop = canonical
	data, merr := json.MarshalIndent(&out, "", "  ")
	if merr != nil {
		return Unverifiable("cannot encode the width entry", merr)
	}
	path := filepath.Join(dir, canonical+".json")
	if werr := os.WriteFile(path, append(data, '\n'), 0o600); werr != nil {
		return Unverifiable("cannot write "+path, werr)
	}
	return nil
}

// ResolvedWidth is THE reader: the width `loop` is running at right now, and a one-line
// statement of where that number came from.
//
// It is the single place the three consumers agree — the operator-facing `deskroster width`,
// the depth/slot denominator in `deskboard throughput`, and a loop sizing its own pool. The
// ceiling is re-applied HERE, at read time, not only at write time: a width stored before a
// budget constant was lowered must not outlive the bound that admitted it.
//
// An unreadable store is an error, never a silent default: could-not-check about a pool's
// size must not be reported as "running at the documented width".
func ResolvedWidth(loop string) (width int, source string, err error) {
	return ResolvedWidthAt(loop, time.Now())
}

// ResolvedWidthAt is ResolvedWidth with an injectable clock (test seam).
func ResolvedWidthAt(loop string, now time.Time) (width int, source string, err error) {
	canonical, known := CanonicalLoopName(loop)
	if !known {
		return 0, "", Refused(fmt.Sprintf(
			"refused: %q is not a loop this roster recognises. Known loop names: %v", loop, KnownLoopNames()))
	}
	stored, fresh, lerr := LoadWidth(canonical, now)
	if lerr != nil {
		return 0, "", lerr
	}
	requested := 0
	source = "shipped default"
	if fresh {
		requested = stored.Width
		source = fmt.Sprintf("set by %s at %s", stored.SetBy, stored.Updated)
	}
	eff, eerr := EffectiveWidth(canonical, requested, 0, false)
	if eerr != nil {
		return 0, "", eerr
	}
	if fresh && eff != requested {
		source += fmt.Sprintf(" (stored %d, clamped to the current ceiling)", requested)
	}
	return eff, source, nil
}
