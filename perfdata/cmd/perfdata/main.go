// Command perfdata records and reports local performance-event-v1 records. It has no
// network code: recording appends to the local store, reporting reads it.
//
//	perfdata record --dir STATE --policy POLICY.json [--profile tool|workflow] [--input FILE|-] [--json]
//	perfdata report --dir STATE [--policy POLICY.json] [--from DAY] [--to DAY] [--json]
//
// Recording is armed only by ASSAY_TELEMETRY=1 plus an explicit profile (--profile or
// ASSAY_PERFDATA_PROFILE). When it is off, record exits 0 and creates nothing.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/medici-finance/assay/perfdata"
	"github.com/medici-finance/assay/perfdata/local"
	"github.com/medici-finance/assay/perfdata/report"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv, time.Now))
}

const usage = "usage: perfdata record|report [flags] (see -h)"

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "record":
		return record(args[1:], stdin, stdout, stderr, getenv, now)
	case "report":
		return reportCmd(args[1:], stdout, stderr, now)
	}
	fmt.Fprintln(stderr, usage)
	return 2
}

var buildTokenRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)

func buildInfo() perfdata.Build {
	v := "dev"
	if bi, ok := debug.ReadBuildInfo(); ok && buildTokenRE.MatchString(bi.Main.Version) {
		v = bi.Main.Version
	}
	return perfdata.Build{Version: v, OS: runtime.GOOS, Arch: runtime.GOARCH}
}

func loadPolicy(path string) (perfdata.Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return perfdata.Policy{}, fmt.Errorf("cannot read policy file")
	}
	return perfdata.DecodePolicy(b)
}

// readDrafts reads one draft per line (blank lines skipped). Decoding errors are
// reported by line number only; the input text is never echoed.
func readDrafts(r io.Reader) ([]perfdata.Draft, error) {
	var out []perfdata.Draft
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), perfdata.MaxEventBytes*4)
	n := 0
	for sc.Scan() {
		n++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var d perfdata.Draft
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&d); err != nil || dec.More() {
			return nil, fmt.Errorf("input line %d: malformed or unknown field", n)
		}
		out = append(out, d)
	}
	if sc.Err() != nil {
		return nil, fmt.Errorf("input line %d: unreadable or over the size bound", n+1)
	}
	return out, nil
}

func record(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "state directory (the store is <dir>/"+local.DirName+")")
	profile := fs.String("profile", "", "collection profile: tool or workflow (else "+perfdata.EnvProfile+")")
	policyPath := fs.String("policy", "", "performance-policy-v1 file (required when collection is armed)")
	input := fs.String("input", "-", "draft events, one JSON object per line; - is stdin")
	asJSON := fs.Bool("json", false, "print a JSON result instead of the stored lines")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dir == "" {
		fmt.Fprintln(stderr, "perfdata record: --dir is required")
		return 2
	}
	c := perfdata.ResolveConsent(getenv, *profile)
	if !c.Armed {
		fmt.Fprintf(stderr, "perfdata record: collection off (%s); nothing recorded\n", c.Reason)
		if *asJSON {
			_ = json.NewEncoder(stdout).Encode(map[string]any{"recorded": 0, "consent": c})
		}
		return 0
	}
	if *policyPath == "" {
		fmt.Fprintln(stderr, "perfdata record: --policy is required when collection is armed")
		return 2
	}
	pol, err := loadPolicy(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "perfdata record: policy refused: %v\n", perfdata.AsRefusal(err))
		return 2
	}
	var in io.Reader = stdin
	if *input != "-" {
		f, err := os.Open(*input)
		if err != nil {
			fmt.Fprintln(stderr, "perfdata record: cannot open input file")
			return 2
		}
		defer f.Close()
		in = f
	}
	drafts, err := readDrafts(in)
	if err != nil {
		fmt.Fprintf(stderr, "perfdata record: %v\n", err)
		return 1
	}
	st, err := local.Open(*dir, c, pol, local.Options{Build: buildInfo(), Now: now})
	if err != nil {
		fmt.Fprintf(stderr, "perfdata record: cannot open store: %v\n", err)
		return 1
	}
	defer st.Close()
	res, err := st.Write(drafts, false)
	if len(res.Refused) > 0 {
		for _, r := range res.Refused {
			fmt.Fprintf(stderr, "perfdata record: draft %d refused: %v\n", r.Index+1, r.Refusal)
		}
		if *asJSON {
			_ = json.NewEncoder(stdout).Encode(map[string]any{"recorded": 0, "events": []any{}, "refused": res.Refused})
		}
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "perfdata record: write failed: %v\n", err)
		return 1
	}
	if *asJSON {
		if res.Lines == nil {
			res.Lines = []json.RawMessage{}
		}
		_ = json.NewEncoder(stdout).Encode(map[string]any{"recorded": len(res.Lines), "events": res.Lines, "refused": []any{}})
		return 0
	}
	for _, l := range res.Lines {
		stdout.Write(l)
		io.WriteString(stdout, "\n")
	}
	return 0
}

func reportCmd(args []string, stdout, stderr io.Writer, now func() time.Time) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "state directory to read")
	policyPath := fs.String("policy", "", "optional performance-policy-v1 file (stamps the snapshot's policy revision)")
	from := fs.String("from", "", "first UTC day (YYYY-MM-DD); default: earliest data")
	to := fs.String("to", "", "last UTC day (YYYY-MM-DD); default: latest data")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dir == "" {
		fmt.Fprintln(stderr, "perfdata report: --dir is required")
		return 2
	}
	for _, d := range []string{*from, *to} {
		if _, ok := perfdata.ParseTime(d + "T00:00:00.000Z"); d != "" && !ok {
			fmt.Fprintln(stderr, "perfdata report: --from/--to must be YYYY-MM-DD")
			return 2
		}
	}
	rev := ""
	if *policyPath != "" {
		pol, err := loadPolicy(*policyPath)
		if err != nil {
			fmt.Fprintf(stderr, "perfdata report: policy refused: %v\n", perfdata.AsRefusal(err))
			return 2
		}
		rev = pol.Revision
	}
	rr, err := local.Read(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "perfdata report: cannot read store: %v\n", err)
		return 1
	}
	rep := report.Build(report.Input{Events: rr.Events, Coverage: rr.Coverage, Rollups: rr.Rollups, Cutoff: now(),
		PolicyRevision: rev, Window: perfdata.Window{From: *from, To: *to}})
	rep.Collection.Duplicates += rr.Duplicates
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return 1
		}
		return 0
	}
	if err := report.Render(stdout, rep); err != nil {
		return 1
	}
	return 0
}
