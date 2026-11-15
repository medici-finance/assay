package main

// routeflags.go — refuse a flag the selected landing shape does not honour (#4554).
//
// cmdEvidence parses ONE flag set for every landing shape, then routes to one of them. A flag
// parsed for every shape but read by only some of them is accepted and silently ignored on the
// rest: --dry-run was read only by the --evidence-file route, so `--outcome-record … --dry-run`
// exited 0 and committed the record for real. The same shape held for --append-only,
// --allow-shrink and --row on the --outcome-record route.
//
// Every route now declares the flags it honours, and a flag set on the command line that the
// selected route does not declare is refused before mint or any forge call. A new flag is
// therefore refused on every route until one declares it, and a new route must list what it
// honours: neither can again accept a flag and ignore it.

import (
	"flag"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	routeEvidenceFile  = "--evidence-file"
	routeOutcomeRecord = "--outcome-record"
)

// routeHonours lists, per landing route, every flag name the route reads. The outbound
// override is applied before routing, so both routes honour it.
var routeHonours = map[string][]string{
	routeEvidenceFile: {
		"evidence-file", "brief-path", "root", "append-only", "allow-shrink", "dry-run", "row",
		deskkit.ScanOverrideFlag,
	},
	routeOutcomeRecord: {
		"outcome-record", "outcome-only", "root", "dry-run",
		deskkit.ScanOverrideFlag,
	},
}

// refuseUnhonouredFlags refuses when fs carries a flag set on the command line that route does
// not honour. An unknown route honours nothing, so it refuses any flag at all.
func refuseUnhonouredFlags(fs *flag.FlagSet, route string) error {
	honoured := make(map[string]bool, len(routeHonours[route]))
	for _, name := range routeHonours[route] {
		honoured[name] = true
	}
	var ignored []string
	fs.Visit(func(f *flag.Flag) {
		if !honoured[f.Name] {
			ignored = append(ignored, "--"+f.Name)
		}
	})
	if len(ignored) == 0 {
		return nil
	}
	sort.Strings(ignored)
	return deskkit.Refused("refused: " + strings.Join(ignored, ", ") + " has no effect on the " + route +
		" landing shape — refusing rather than accept a flag this route would silently ignore")
}
