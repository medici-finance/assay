// Command opmetrics is the LOCAL operator-layer collector.
//
// It measures the human at the top of the loop — how much of his message traffic is
// plumbing rather than decision — and emits ONE aggregates-only day-file:
//
//	<root>/docs/reports/daily/<date>/opmetrics.json
//
// WHERE IT RUNS. On the OPERATOR's machine, not in CI. Its inputs (session
// transcripts, roster beacons, dispatch claims) exist only there; the "run by the
// daily harvest" reading is wrong as written. The day-file rides into
// the repo alongside the harvest, not via it.
//
// PRIVACY, WHICH IS THE POINT OF THIS TOOL'S DESIGN. opmetrics reads a person's own
// session transcripts. Everything it emits is a count, a ratio, a percentile, or a
// fixed status code; nothing it emits can reconstruct what was said. See emit.go for
// the three independent enforcements. It NEVER writes to, moves, or deletes anything
// under the transcript tree — the only file it writes is the day-file, under --root.
//
// THREE STATES. A collector that cannot read an input says could-not-check with a
// reason CODE. It never reports zero. Zero relays and a blind collector are different
// facts and the day-file distinguishes them.
//
// USAGE
//
//	opmetrics --root <repo> --date YYYY-MM-DD [--gh-fixture F] [--trend 7]
//	          [--transcripts DIR] [--desk-tools DIR] [--now RFC3339] [--stdout]
//
// Exit codes follow the deskkit contract: 0 ok · 3 disabled (kill switch) ·
// 5 refused (bad invocation) · 6 unverifiable (could not write the day-file).
package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `opmetrics — local operator-layer collector (relay ratio, intervention rate,
decision latency, session hygiene) → an AGGREGATES-ONLY day-file.

USAGE:
  opmetrics --root <repo> --date YYYY-MM-DD [flags]
  opmetrics --version

FLAGS:
  --root DIR          repo root the day-file is written under (required unless --stdout)
  --date YYYY-MM-DD   the day to report (default: today, local time)
  --transcripts DIR   session transcripts (default: ~/.claude/projects)
  --desk-tools DIR    desk-tools state holding roster/ and claims/
                      (default: ~/.config/assay — see hygiene.go's PATH NOTE)
  --gh-fixture FILE   gh JSON export for merged PRs and decision latency. Produce it with:
                        gh pr list --repo O/R --state merged --limit 200 \
                          --search "merged:<date>" --json number,createdAt,mergedAt,labels
                      Omitted → the gh-derived blocks report could-not-check, never 0.
  --gh-json FILE      alias for --gh-fixture
  --trend N           also compare against the N prior day-files under --root
  --now RFC3339       the instant "stale" is measured from (default: real now).
                      Exists so zombie detection is deterministic under test.
  --tz LOCATION       IANA location the day boundary is measured in (default: local).
                      Pin it (e.g. --tz UTC) whenever the answer must be machine-independent.
  --stdout            print the JSON instead of writing the day-file
  --version           print the build stamp

Exit: 0 ok · 3 disabled · 5 refused (bad invocation) · 6 unverifiable (write failed).

This tool READS a human's private session transcripts and emits only aggregates.
It never writes anything under the transcript tree.`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "opmetrics sourceSHA=%s builtAt=%s classifier=%s schema=%s releaseTag=%s\n",
			sha, built, ClassifierVersion, SchemaVersion, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(stderr, usage)
		return deskkit.ExitOK
	}

	fs := flag.NewFlagSet("opmetrics", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		root        = fs.String("root", "", "repo root to write the day-file under")
		date        = fs.String("date", "", "day to report, YYYY-MM-DD")
		transcripts = fs.String("transcripts", "", "session transcripts dir")
		deskTools   = fs.String("desk-tools", "", "desk-tools state dir holding roster/ and claims/")
		ghFixture   = fs.String("gh-fixture", "", "gh JSON export of merged PRs")
		ghJSON      = fs.String("gh-json", "", "alias for --gh-fixture")
		trend       = fs.Int("trend", 0, "compare against N prior day-files")
		nowFlag     = fs.String("now", "", "RFC3339 instant staleness is measured from")
		tz          = fs.String("tz", "", "IANA location the day boundary is measured in (default: local)")
		toStdout    = fs.Bool("stdout", false, "print the JSON instead of writing the day-file")
	)
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}

	// The kill switch is the first gated action, exactly as every other desk tool.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(stderr)

	// The day boundary is a LOCAL-time question — the operator's day is his day, not
	// UTC's — but "local" makes the same command answer differently on two machines,
	// which is not a property a metric may have. --tz pins it. Tests and the brief's
	// Verify rows pass --tz UTC so the fixture timestamps mean one thing everywhere.
	loc := time.Local
	if *tz != "" {
		l, lerr := time.LoadLocation(*tz)
		if lerr != nil {
			fmt.Fprintln(stderr, "refused: --tz is not a known IANA location")
			return deskkit.ExitRefused
		}
		loc = l
	}

	now := time.Now().In(loc)
	if *nowFlag != "" {
		t, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			fmt.Fprintln(stderr, "refused: --now is not RFC3339")
			return deskkit.ExitRefused
		}
		now = t.In(loc)
	}
	if *date == "" {
		*date = now.Format("2006-01-02")
	}
	day, err := time.ParseInLocation("2006-01-02", *date, loc)
	if err != nil {
		fmt.Fprintln(stderr, "refused: --date is not YYYY-MM-DD")
		return deskkit.ExitRefused
	}
	if *root == "" && !*toStdout {
		fmt.Fprintln(stderr, "refused: --root is required (or pass --stdout)")
		return deskkit.ExitRefused
	}
	if *ghFixture == "" {
		*ghFixture = *ghJSON
	}
	if *trend < 0 {
		fmt.Fprintln(stderr, "refused: --trend must be >= 0")
		return deskkit.ExitRefused
	}

	tdir, err := resolveTranscripts(*transcripts)
	if err != nil {
		// A resolution failure is not fatal: the operator block reports
		// could-not-check and the rest of the day-file still lands.
		fmt.Fprintln(stderr, "warning: "+err.Error())
	}
	sdir, err := resolveDeskTools(*deskTools)
	if err != nil {
		fmt.Fprintln(stderr, "warning: "+err.Error())
	}

	rep := Build(Inputs{
		Date:           *date,
		Day:            day,
		Now:            now,
		TranscriptsDir: tdir,
		DeskToolsDir:   sdir,
		GHJSON:         *ghFixture,
		Root:           *root,
		Trend:          *trend,
	}, stderr)

	if *toStdout {
		raw, merr := marshalReport(rep)
		if merr != nil {
			fmt.Fprintln(stderr, merr.Error())
			return deskkit.ExitUnverifiable
		}
		stdout.Write(raw)
		return deskkit.ExitOK
	}

	path, _, werr := WriteReport(*root, *date, rep)
	if werr != nil {
		fmt.Fprintln(stderr, "unverifiable: "+werr.Error())
		return deskkit.ExitUnverifiable
	}
	fmt.Fprintln(stdout, path)
	return deskkit.ExitOK
}

// resolveTranscripts defaults to ~/.claude/projects. Returning "" (with an error) is
// how a missing HOME becomes could-not-check instead of a panic or a wrong path.
func resolveTranscripts(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve HOME for the default transcripts dir: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// resolveDeskTools defaults to deskkit.StateDir() — ~/.config/assay — which is where
// roster beacons and dispatch claims ACTUALLY live on this tree. See hygiene.go.
func resolveDeskTools(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	dir, err := deskkit.StateDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve the desk-tools state dir: %w", err)
	}
	return dir, nil
}

// Inputs is everything Build needs. Every path is explicit so no code path in Build
// can reach for the real home directory.
type Inputs struct {
	Date           string
	Day            time.Time
	Now            time.Time
	TranscriptsDir string
	DeskToolsDir   string
	GHJSON         string
	Root           string
	Trend          int
}

// Build assembles the report. It never returns an error: every input failure becomes a
// could-not-check block with a reason CODE, and the human-readable detail — which is
// where paths would leak — goes to stderr, which is not committed anywhere.
func Build(in Inputs, stderr io.Writer) Report {
	rep := Report{
		Schema:            SchemaVersion,
		Date:              in.Date,
		GeneratedAt:       in.Now.UTC().Format(time.RFC3339),
		ClassifierVersion: ClassifierVersion,
		Unmeasured:        []string{UnmeasuredPromptBlockage, UnmeasuredThinkTime},
	}

	// ---- operator messages / relay ratio ----
	var labels []Label
	var operatorTotal *int
	if in.TranscriptsDir == "" {
		rep.Operator = OperatorBlock{Status: StatusCouldNotCheck, Reason: ReasonTranscriptsUnread}
	} else if tr, err := ReadOperatorMessages(in.TranscriptsDir, in.Day); err != nil {
		fmt.Fprintln(stderr, "could-not-check operator block: "+err.Error())
		rep.Operator = OperatorBlock{Status: StatusCouldNotCheck, Reason: ReasonTranscriptsUnread}
	} else if tr.Files == 0 {
		// Zero FILES is not a quiet day, it is a mis-pointed flag. Say so.
		fmt.Fprintln(stderr, "could-not-check operator block: no .jsonl transcripts under "+in.TranscriptsDir)
		rep.Operator = OperatorBlock{Status: StatusCouldNotCheck, Reason: ReasonNoTranscriptFiles}
	} else {
		c := NewClassifier()
		var fam RelayFamilies
		relay, subst, empty := 0, 0, 0
		for _, m := range tr.Messages {
			l := c.Classify(m.Session, m.Text)
			labels = append(labels, l)
			switch l.Class {
			case ClassRelay:
				relay++
				switch l.Family {
				case familySync:
					fam.Sync++
				case familyState:
					fam.StateEcho++
				case familyPoke:
					fam.Poke++
				case familyLookup:
					fam.Lookup++
				case familyDuplicate:
					fam.Duplicate++
				}
			case ClassSubstantive:
				subst++
			case ClassEmpty:
				empty++
			}
		}
		total := len(tr.Messages)
		classified := relay + subst
		blk := OperatorBlock{
			Status:              StatusOK,
			MessagesTotal:       iptr(total),
			MessagesClassified:  iptr(classified),
			RelayMessages:       iptr(relay),
			SubstantiveMessages: iptr(subst),
			EmptyMessages:       iptr(empty),
			RelayFamilies:       fam,
			TranscriptFiles:     tr.Files,
			UnparseableLines:    tr.Unparseable,
		}
		if classified > 0 {
			r := round4(float64(relay) / float64(classified))
			blk.RelayRatio = fptr(r)
			rep.RelayRatio = fptr(r)
		} else {
			// Files were read but the operator said nothing all day. That is a real
			// observation, not a blind spot — but a ratio over zero messages does
			// not exist, so it stays null with its own reason.
			blk.Reason = ReasonNoOperatorMessages
		}
		operatorTotal = iptr(total)
		rep.Operator = blk

		rep.SessionHygiene.SessionsObserved = iptr(len(tr.Spans))
		rep.SessionHygiene.SessionsOver24h = iptr(countLongSessions(tr.Spans))
	}

	// ---- correction recurrence ----
	if rep.Operator.Status != StatusOK {
		rep.CorrectionRecurrence = CorrectionBlock{Status: StatusCouldNotCheck, Reason: rep.Operator.Reason}
	} else {
		corrective := 0
		for _, l := range labels {
			if l.Corrective {
				corrective++
			}
		}
		rep.CorrectionRecurrence = CorrectionBlock{
			Status:              StatusOK,
			CorrectiveMessages:  iptr(corrective),
			RecurrenceCandidate: iptr(correctionCandidates(labels)),
		}
	}

	// ---- gh-derived: intervention rate + decision latency ----
	var interventionRate *float64
	if in.GHJSON == "" {
		rep.Intervention = InterventionBlock{Status: StatusCouldNotCheck, Reason: ReasonGHNotSupplied, OperatorMessages: operatorTotal}
		rep.DecisionLatency = LatencyBlock{Status: StatusCouldNotCheck, Reason: ReasonGHNotSupplied}
	} else if lr, err := ReadGH(in.GHJSON, in.Day); err != nil {
		fmt.Fprintln(stderr, "could-not-check gh blocks: "+err.Error())
		rep.Intervention = InterventionBlock{Status: StatusCouldNotCheck, Reason: ReasonGHUnreadable, OperatorMessages: operatorTotal}
		rep.DecisionLatency = LatencyBlock{Status: StatusCouldNotCheck, Reason: ReasonGHUnreadable}
	} else {
		ib := InterventionBlock{OperatorMessages: operatorTotal, MergedPRs: iptr(lr.MergedOnDate)}
		switch {
		case operatorTotal == nil:
			// The numerator is missing, so the rate is not computable even though
			// the denominator read fine.
			ib.Status, ib.Reason = StatusCouldNotCheck, rep.Operator.Reason
		case lr.MergedOnDate == 0:
			// Division by zero is not "infinite intervention"; it is no denominator.
			ib.Status, ib.Reason = StatusCouldNotCheck, ReasonNoMergedPRs
		default:
			ib.Status = StatusOK
			r := round4(float64(*operatorTotal) / float64(lr.MergedOnDate))
			ib.MessagesPerMergedPR = fptr(r)
			interventionRate = fptr(r)
		}
		rep.Intervention = ib

		lb := LatencyBlock{
			Samples: iptr(lr.Samples),
			Basis: LatencyBasis{
				ReadyFlip:       lr.BasisReady,
				DecisionRequest: lr.BasisDecisionReq,
				CreatedAt:       lr.BasisCreated,
			},
		}
		if lr.Samples == 0 {
			lb.Status, lb.Reason = StatusCouldNotCheck, ReasonNoLatencySamples
		} else {
			lb.Status = StatusOK
			lb.P50Minutes = iptr(lr.P50Minutes)
			lb.P90Minutes = iptr(lr.P90Minutes)
		}
		rep.DecisionLatency = lb
	}

	// ---- session hygiene: beacons + claims ----
	hb := rep.SessionHygiene
	hb.Status = StatusOK
	if in.DeskToolsDir == "" {
		hb.Status, hb.Reason = StatusCouldNotCheck, ReasonRosterUnreadable
	} else {
		if rr, err := ReadBeacons(filepath.Join(in.DeskToolsDir, "roster"), in.Now); err != nil {
			fmt.Fprintln(stderr, "could-not-check roster beacons: "+err.Error())
			hb.Status, hb.Reason = StatusCouldNotCheck, ReasonRosterUnreadable
		} else {
			hb.RosterBeacons = iptr(rr.Beacons)
			hb.ZombieAgents = iptr(rr.Zombies)
			hb.UnparseableFiles += rr.Unparseable
		}
		if cr, err := ReadClaims(filepath.Join(in.DeskToolsDir, "claims"), in.Day); err != nil {
			fmt.Fprintln(stderr, "could-not-check dispatch claims: "+err.Error())
			hb.Status, hb.Reason = StatusCouldNotCheck, ReasonClaimsUnreadable
		} else {
			hb.ClaimsFiled = iptr(cr.FiledOnDate)
			hb.ClaimsOpen = iptr(cr.Open)
			hb.SessionsActive = iptr(cr.Owners)
			hb.UnparseableFiles += cr.Unparseable
		}
	}
	rep.SessionHygiene = hb

	// ---- trend ----
	if in.Trend > 0 {
		root := in.Root
		if root == "" {
			root = "."
		}
		tb := BuildTrend(root, in.Date, in.Trend, rep.RelayRatio, interventionRate)
		rep.Trend = &tb
	}

	return rep
}

// round4 keeps ratios to four decimals. Emitting a full float64 would make two runs
// over the same day differ in the 15th digit and turn a diff into noise.
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
