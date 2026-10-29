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
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
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
  --transcripts DIR   session transcripts (repeatable; default: every ~/.claude*/projects).
                      Sessions synced across profiles are counted once.
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

func main() {
	os.Exit(deskkit.PerfRun("opmetrics", nil, os.Args[1:], func(a []string) int {
		return run(a, os.Stdout, os.Stderr)
	}))
}

// guard is the kill switch, a variable so the contract tests can count it.
var guard = deskkit.Guard

// collectFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var collectFn = collect

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// versionLine is what --version and -version print.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("opmetrics sourceSHA=%s builtAt=%s classifier=%s schema=%s releaseTag=%s",
		sha, built, ClassifierVersion, SchemaVersion, deskkit.ReleaseTagOrDev())
}

// bindings are every setting opmetrics declares: flags and defaults only. It reads no
// configuration file or environment variable for them.
func bindings() []cli.Binding {
	return []cli.Binding{
		// Repeatable and never split: a comma is part of a path, as it always was.
		{Key: "transcripts", Kind: cli.StringArray, Flag: "transcripts", Usage: "session transcripts dir (repeatable; default: every ~/.claude*/projects)"},
		{Key: "root", Kind: cli.String, Flag: "root", Usage: "repo root to write the day-file under"},
		{Key: "date", Kind: cli.String, Flag: "date", Usage: "day to report, YYYY-MM-DD"},
		{Key: "desk-tools", Kind: cli.String, Flag: "desk-tools", Usage: "desk-tools state dir holding roster/ and claims/"},
		{Key: "gh-fixture", Kind: cli.String, Flag: "gh-fixture", Usage: "gh JSON export of merged PRs"},
		{Key: "gh-json", Kind: cli.String, Flag: "gh-json", Usage: "alias for --gh-fixture"},
		{Key: "trend", Kind: cli.Int, Flag: "trend", Default: 0, Usage: "compare against N prior day-files"},
		{Key: "now", Kind: cli.String, Flag: "now", Usage: "RFC3339 instant staleness is measured from"},
		{Key: "tz", Kind: cli.String, Flag: "tz", Usage: "IANA location the day boundary is measured in (default: local)"},
		{Key: "stdout", Kind: cli.Bool, Flag: "stdout", Default: false, Usage: "print the JSON instead of writing the day-file"},
	}
}

// options are the checked values the collector runs with.
type options struct {
	date        string
	day, now    time.Time
	root        string
	deskTools   string
	ghFixture   string
	transcripts []string
	trend       int
	toStdout    bool
}

func run(args []string, stdout, stderr io.Writer) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("opmetrics", "local operator-layer collector")
		root.Long = usage
		set := cli.Declare(root, bindings()...)
		// Words after the flags are ignored, as before. A leading `help` word is the adapter's
		// help command, so it never reaches this handler.
		root.Args = cobra.ArbitraryArgs
		root.RunE = func(_ *cobra.Command, _ []string) error {
			if code := resolveAndCollect(set, stdout, stderr); code != deskkit.ExitOK {
				return exitErr(code)
			}
			return nil
		}
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A parse error keeps the legacy refusal code (5), not the adapter's default.
		UsageExit:    deskkit.ExitRefused,
		GoFlagCompat: true,
	})
}

// resolveAndCollect resolves the settings, then runs the kill switch, the value checks and
// the collector, in that order: the kill switch is the first gated action, exactly as every
// other desk tool, and help never reaches it.
func resolveAndCollect(set *cli.Set, stdout, stderr io.Writer) int {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		fmt.Fprintln(stderr, "refused:", err)
		return deskkit.ExitRefused
	}
	if onResolve != nil {
		onResolve(v)
	}
	if err := guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(stderr)

	// The day boundary is a LOCAL-time question — the operator's day is his day, not
	// UTC's — but "local" makes the same command answer differently on two machines,
	// which is not a property a metric may have. --tz pins it. Tests and the brief's
	// Verify rows pass --tz UTC so the fixture timestamps mean one thing everywhere.
	loc := time.Local
	if tz := v.String("tz"); tz != "" {
		l, lerr := time.LoadLocation(tz)
		if lerr != nil {
			fmt.Fprintln(stderr, "refused: --tz is not a known IANA location")
			return deskkit.ExitRefused
		}
		loc = l
	}

	o := options{
		root:        v.String("root"),
		date:        v.String("date"),
		deskTools:   v.String("desk-tools"),
		ghFixture:   v.String("gh-fixture"),
		transcripts: v.Strings("transcripts"),
		trend:       v.Int("trend"),
		toStdout:    v.Bool("stdout"),
	}
	o.now = time.Now().In(loc)
	if nowFlag := v.String("now"); nowFlag != "" {
		t, err := time.Parse(time.RFC3339, nowFlag)
		if err != nil {
			fmt.Fprintln(stderr, "refused: --now is not RFC3339")
			return deskkit.ExitRefused
		}
		o.now = t.In(loc)
	}
	if o.date == "" {
		o.date = o.now.Format("2006-01-02")
	}
	if o.day, err = time.ParseInLocation("2006-01-02", o.date, loc); err != nil {
		fmt.Fprintln(stderr, "refused: --date is not YYYY-MM-DD")
		return deskkit.ExitRefused
	}
	if o.root == "" && !o.toStdout {
		fmt.Fprintln(stderr, "refused: --root is required (or pass --stdout)")
		return deskkit.ExitRefused
	}
	if o.ghFixture == "" {
		o.ghFixture = v.String("gh-json")
	}
	if o.trend < 0 {
		fmt.Fprintln(stderr, "refused: --trend must be >= 0")
		return deskkit.ExitRefused
	}
	return collectFn(o, stdout, stderr)
}

// collect reads the inputs and writes (or prints) the day-file.
func collect(o options, stdout, stderr io.Writer) int {
	tdirs, err := resolveTranscripts(o.transcripts)
	if err != nil {
		// A resolution failure is not fatal: the operator block reports
		// could-not-check and the rest of the day-file still lands.
		fmt.Fprintln(stderr, "warning: "+err.Error())
	}
	sdir, err := resolveDeskTools(o.deskTools)
	if err != nil {
		fmt.Fprintln(stderr, "warning: "+err.Error())
	}

	rep := Build(Inputs{
		Date:            o.date,
		Day:             o.day,
		Now:             o.now,
		TranscriptsDirs: tdirs,
		DeskToolsDir:    sdir,
		GHJSON:          o.ghFixture,
		Root:            o.root,
		Trend:           o.trend,
	}, stderr)

	if o.toStdout {
		raw, merr := marshalReport(rep)
		if merr != nil {
			fmt.Fprintln(stderr, merr.Error())
			return deskkit.ExitUnverifiable
		}
		stdout.Write(raw)
		return deskkit.ExitOK
	}

	path, _, werr := WriteReport(o.root, o.date, rep)
	if werr != nil {
		fmt.Fprintln(stderr, "unverifiable: "+werr.Error())
		return deskkit.ExitUnverifiable
	}
	fmt.Fprintln(stdout, path)
	return deskkit.ExitOK
}

// resolveTranscripts turns the --transcripts flag(s) into the list of roots to
// read. Explicit flags win verbatim and in order. With none, the default is
// EVERY ~/.claude*/projects that exists — the live operator runs several Claude
// profiles (~/.claude, ~/.claude_2, ~/.claude_3, …) and the day-file must cover
// them all rather than only the one profile the historic default named. Sessions
// synced across profiles are deduped downstream (ReadOperatorMessagesMulti).
//
// Returning an empty list (with an error) is how a missing HOME becomes
// could-not-check instead of a panic or a wrong path. An empty list from a
// resolvable HOME that simply has no ~/.claude*/projects yet is NOT an error —
// the operator block then reports could-not-check with no readable root.
func resolveTranscripts(flagVals []string) ([]string, error) {
	if len(flagVals) > 0 {
		return flagVals, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve HOME for the default transcripts dirs: %w", err)
	}
	matches, gerr := filepath.Glob(filepath.Join(home, ".claude*", "projects"))
	if gerr != nil {
		return nil, fmt.Errorf("expanding the default transcripts roots: %w", gerr)
	}
	sort.Strings(matches)
	var out []string
	for _, m := range matches {
		if info, serr := os.Stat(m); serr == nil && info.IsDir() {
			out = append(out, m)
		}
	}
	return out, nil
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
	Date string
	Day  time.Time
	Now  time.Time
	// TranscriptsDirs is every transcript root to read (repeatable --transcripts,
	// or the default expansion over each ~/.claude*/projects). Empty means no root
	// could be resolved at all — could-not-check, never zero.
	TranscriptsDirs []string
	DeskToolsDir    string
	GHJSON          string
	Root            string
	Trend           int
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
	tr, skippedRoots, _, trErr := ReadOperatorMessagesMulti(in.TranscriptsDirs, in.Day)
	// A root that does not exist is a NOTICE on stderr (never committed), not a
	// failure of the whole read — the operator runs several profiles and one may
	// be absent on a given machine.
	for _, s := range skippedRoots {
		fmt.Fprintln(stderr, "notice: transcripts root skipped (not a readable directory): "+s)
	}
	if len(in.TranscriptsDirs) == 0 || trErr != nil {
		// Zero candidate roots, or not one of them readable: blind, not zero.
		if trErr != nil {
			fmt.Fprintln(stderr, "could-not-check operator block: "+trErr.Error())
		}
		rep.Operator = OperatorBlock{Status: StatusCouldNotCheck, Reason: ReasonTranscriptsUnread}
	} else if tr.Files == 0 {
		// Roots were readable but held no .jsonl transcripts — a mis-pointed flag,
		// not a quiet day. Say so.
		fmt.Fprintln(stderr, "could-not-check operator block: no .jsonl transcripts under any configured root")
		rep.Operator = OperatorBlock{Status: StatusCouldNotCheck, Reason: ReasonNoTranscriptFiles}
	} else {
		c := NewClassifier()
		var fam RelayFamilies
		var attn AttentionFamilies
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
			// v2 attention axis — additive, over every non-empty turn. A ClassEmpty
			// message has Attention "" and is not counted, so the eight family counts
			// sum to messages_classified.
			switch l.Attention {
			case AttnRoute:
				attn.Route++
			case AttnStatus:
				attn.Status++
			case AttnToil:
				attn.Toil++
			case AttnCorrection:
				attn.Correction++
			case AttnDecision:
				attn.Decision++
			case AttnIdea:
				attn.Idea++
			case AttnAck:
				attn.Ack++
			case AttnOther:
				attn.Other++
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
			AttentionFamilies:   attn,
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
