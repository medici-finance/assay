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
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// setWidth is the write half behind `deskroster set --role <loop> --width N`.
//
// The bound is applied BEFORE anything is stored, so a refused width leaves the store
// untouched: an operator who sees exit 5 knows the width already in force survived, and does
// not have to go and check.
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
	return canonical, deskkit.SaveWidth(&deskkit.WidthEntry{
		Loop:    canonical,
		Width:   n,
		SetBy:   setBy,
		Updated: now.UTC().Format(time.RFC3339),
	})
}

// cmdWidth is the read half: `deskroster width --role <loop>`. It is a PURE READ — it claims
// nothing, stores nothing, and never creates the width directory.
//
// It prints the effective width on stdout (one integer, so a shell can consume it) and, with
// --verbose, where that number came from on stderr. The source is worth printing because "5"
// means three different things depending on whether it is the shipped default, a width a
// desk set, or a larger stored value a ceiling clamped down.
func cmdWidth(args []string) error {
	fs := flag.NewFlagSet("width", flag.ContinueOnError)
	role := fs.String("role", "", "Loop whose pool width to read (e.g. worker-desk, pr-review-desk)")
	verbose := fs.Bool("verbose", false, "Also print where the width came from and the ceiling that bounds it")
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

	width, source, err := deskkit.ResolvedWidth(canonical)
	if err != nil {
		return err
	}

	fmt.Println(width)
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
