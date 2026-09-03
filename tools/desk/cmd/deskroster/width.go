package main

// width.go — the CLI half of the per-loop agent-pool WIDTH: the knob the coordinator moves
// when one desk is the bottleneck, and the value that desk's loop re-reads every tick.
//
// The STORE and the single reader live in internal/deskkit (widthstore.go) because
// `deskboard throughput` needs the same answer as its depth/slot denominator, and a second
// implementation of "is this entry still fresh?" is how the ratio and the pool it describes
// drift apart. This file is the two verbs over that store, and nothing else.

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// setWidth is the write half behind `deskroster set --role <loop> --width N`.
//
// The bound is applied BEFORE anything is stored, so a refused width leaves the store
// untouched: an operator who sees exit 5 knows the width already in force survived, and does
// not have to go and check.
//
// A width set here leaves any RESERVE already stored on this loop's entry in force (the field
// this write does not touch is simply carried forward) — narrowing the width alone does not
// silently clear a reservation the CheckReserve bound would now refuse; the reservation is
// re-validated in cmdSet's caller, which STOPS the width write rather than accepting one that
// would leave a fresh-set width and a now-invalid reservation on the same entry.
func setWidth(rawLoop string, n int, setBy string, now time.Time) (canonical string, err error) {
	canonical, known := deskkit.CanonicalLoopName(rawLoop)
	if !known {
		return "", deskkit.Refused(fmt.Sprintf(
			"refused: --role %q is not a loop this roster recognises, so there is no pool to widen. "+
				"Known loop names: %s", rawLoop, strings.Join(deskkit.KnownLoopNames(), ", ")))
	}
	if err := deskkit.CheckWidth(canonical, n); err != nil {
		return "", err
	}
	existing, fresh, lerr := deskkit.LoadWidth(canonical, now)
	if lerr != nil {
		return "", lerr
	}
	var reserve map[string]int
	if fresh {
		reserve = existing.Reserve
	}
	if len(reserve) > 0 {
		if rerr := deskkit.CheckReserve(canonical, reserve, n); rerr != nil {
			return "", deskkit.Refused(fmt.Sprintf(
				"refused: narrowing %s to %d would leave its already-set reservation (%s) swallowing "+
					"the new width. %s", canonical, n, deskkit.FormatReserve(reserve), rerr.Error()))
		}
	}
	return canonical, deskkit.SaveWidth(&deskkit.WidthEntry{
		Loop:    canonical,
		Width:   n,
		Reserve: reserve,
		SetBy:   setBy,
		Updated: now.UTC().Format(time.RFC3339),
	})
}

// parseReserve parses `--reserve resume=2,rework=1` into a class->count map. It validates only
// SHAPE here (an integer per class); CheckReserve is the semantic gate (known classes,
// non-negative, sum below width) and is applied by the caller, which knows the width to check
// against.
func parseReserve(raw string) (map[string]int, error) {
	out := map[string]int{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, deskkit.Refused(fmt.Sprintf(
				"refused: --reserve %q is not class=N (comma-separated pairs, e.g. resume=2,rework=1)", pair))
		}
		class := strings.TrimSpace(kv[0])
		n, cerr := strconv.Atoi(strings.TrimSpace(kv[1]))
		if cerr != nil {
			return nil, deskkit.Refused(fmt.Sprintf(
				"refused: --reserve %s=%q is not an integer", class, strings.TrimSpace(kv[1])))
		}
		out[class] = n
	}
	return out, nil
}

// setReserve is the write half behind `deskroster width --role <loop> --reserve
// resume=N,rework=M`. It validates against the loop's CURRENT effective width (whatever
// ResolvedWidth reads right now — the shipped default or a width already set) and stores the
// reservation on that SAME width entry, so the two share one TTL from this point on: a bare
// `--reserve` with no `--width` in the same invocation does not leave the width entry
// unresolved, it captures the width already in force.
func setReserve(rawLoop string, reserve map[string]int, setBy string, now time.Time) (canonical string, width int, err error) {
	canonical, known := deskkit.CanonicalLoopName(rawLoop)
	if !known {
		return "", 0, deskkit.Refused(fmt.Sprintf(
			"refused: --role %q is not a loop this roster recognises, so there is no pool to reserve "+
				"against. Known loop names: %s", rawLoop, strings.Join(deskkit.KnownLoopNames(), ", ")))
	}
	width, _, werr := deskkit.ResolvedWidth(canonical)
	if werr != nil {
		return "", 0, werr
	}
	if err := deskkit.CheckReserve(canonical, reserve, width); err != nil {
		return "", 0, err
	}
	return canonical, width, deskkit.SaveWidth(&deskkit.WidthEntry{
		Loop:    canonical,
		Width:   width,
		Reserve: reserve,
		SetBy:   setBy,
		Updated: now.UTC().Format(time.RFC3339),
	})
}

// cmdWidth is the read/reserve verb: `deskroster width --role <loop>` is a PURE READ of the
// effective width and reservation; `deskroster width --role <loop> --reserve
// resume=N,rework=M` is the ONE write this otherwise-pure-read verb makes, SETTING the
// reservation (example-stream/05). It lives here rather than under `set --width` because a
// reservation only means anything beside the width it floors a share of, and this is where
// that width is already resolved and printed; `set --width` remains the width-only knob.
//
// The plain read prints `width=<n> reserve=<classes> (source=default|set, expires=...)` on
// stdout — a compound line, not the bare integer this verb used to print alone, because a
// reader now needs both numbers to know whether a resume/rework item is actually protected.
// --verbose adds the ceiling and the width's own source on stderr, as before.
func cmdWidth(args []string) error {
	fs := flag.NewFlagSet("width", flag.ContinueOnError)
	role := fs.String("role", "", "Loop whose pool width to read (e.g. worker-desk, pr-review-desk)")
	verbose := fs.Bool("verbose", false, "Also print where the width came from and the ceiling that bounds it")
	reserve := fs.String("reserve", "", "SET this loop's per-class concurrency reservation, e.g. resume=2,rework=1 "+
		"— a floor of slots held for resume/rework items so fresh dispatch cannot crowd them out even under a "+
		"full pool; never idles a slot when no reserved-class item is waiting; refused when the sum would "+
		"consume the whole width")
	session := fs.String("session", "", "Session name for --reserve's attribution (env: $DESK_SESSION or $CLAUDE_SESSION_ID)")
	// `width --help` prints the flag surface (including --reserve) and exits 0, discoverable
	// from the subcommand itself rather than only the top-level usage — the same shape
	// `fanoutloop plan --help` uses. Checked BEFORE Parse: flag.ErrHelp from Parse itself would
	// otherwise round-trip through the same Refused (exit 5) a bad flag gets, and a help
	// request is not a refusal.
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			fs.SetOutput(os.Stdout)
			fs.Usage()
			return nil
		}
	}
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("width: %v", err))
	}
	if strings.TrimSpace(*role) == "" {
		return deskkit.Refused("width requires --role: a pool width is a property of ONE loop, and " +
			"guessing which one is how a loop reads someone else's number. Loops with a declared width: " +
			strings.Join(deskkit.WidthLoops(), ", "))
	}
	canonical, known := deskkit.CanonicalLoopName(*role)
	if !known {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %q is not a loop this roster recognises. Known loop names: %s",
			*role, strings.Join(deskkit.KnownLoopNames(), ", ")))
	}

	if strings.TrimSpace(*reserve) != "" {
		parsed, perr := parseReserve(*reserve)
		if perr != nil {
			return perr
		}
		setBy, serr := resolveSession(*session)
		if serr != nil {
			return serr
		}
		_, width, werr := setReserve(canonical, parsed, setBy, time.Now())
		if werr != nil {
			return werr
		}
		fmt.Printf("deskroster: %s set %s reservation to %s (width %d; in force for %s)\n",
			setBy, canonical, deskkit.FormatReserve(parsed), width, deskkit.WidthTTL)
		return nil
	}

	width, source, err := deskkit.ResolvedWidth(canonical)
	if err != nil {
		return err
	}
	entry, fresh, lerr := deskkit.LoadWidth(canonical, time.Now())
	if lerr != nil {
		return lerr
	}
	reserveMap, reserveSourceLabel, expiresLabel := reserveDisplay(canonical, entry, fresh)

	fmt.Printf("width=%d reserve=%s (source=%s, expires=%s)\n",
		width, deskkit.FormatReserve(reserveMap), reserveSourceLabel, expiresLabel)
	if *verbose {
		max, why, merr := deskkit.MaxWidth(canonical)
		if merr != nil {
			return merr
		}
		def, _ := deskkit.DefaultWidth(canonical)
		fmt.Fprintf(os.Stderr, "loop=%s width=%d source=%q default=%d max=%d bound-by=%q ttl=%s\n",
			canonical, width, source, def, max, why, deskkit.WidthTTL)
	}
	return nil
}

// reserveDisplay derives the plain read's reserve map and its two labels — "default"/"set", and
// an expiry timestamp or "n/a" — from the raw stored entry. A stored entry with no Reserve set
// (an entry saved before this feature, or a plain `--width` set with no `--reserve`) reads as
// the shipped default, exactly as ResolvedWidth itself falls back when nothing was stored.
func reserveDisplay(loop string, entry *deskkit.WidthEntry, fresh bool) (reserve map[string]int, sourceLabel, expiresLabel string) {
	if fresh && entry.Reserve != nil {
		updated, perr := time.Parse(time.RFC3339, entry.Updated)
		if perr != nil {
			return entry.Reserve, "set", "unparseable"
		}
		return entry.Reserve, "set", updated.Add(deskkit.WidthTTL).UTC().Format(time.RFC3339)
	}
	def, _ := deskkit.DefaultReserve(loop)
	return def, "default", "n/a"
}
