// Command deskscanuntrusted is the CLI over the deterministic inbound pre-scanner
// (deskkit.UntrustScan) — Layer A of the untrusted-read sandbox. It reads untrusted
// CONTENT bytes from a file, a directory, or stdin, runs the three deterministic detector
// families (exfil, injection, codeexec) over them with NO model and NO network, and emits
// a three-state verdict plus a neutralised rendering that strips/escapes every invisible
// codepoint and fences the body as inert data.
//
// It never executes the bytes and never makes a network call. The code leg shells out to a
// vendored Semgrep rule set with metrics and the version check disabled; a missing Semgrep
// makes the code leg could-not-check, never clean.
//
// A DIRECTORY input has two modes. A directory that holds a `corpus.yaml` positive-control
// manifest (the untrusted-read corpus format, brief 02) is loaded through that generator,
// so the inert samples are ASSEMBLED from their codepoint table at run time rather than
// stored decoded on disk — the scanner's positive controls without re-authoring a payload.
// Any other directory is scanned file-by-file over its regular files' raw bytes.
//
// VERDICT EXIT CODES (the brief's contract, three-state and fail-closed):
//
//	0  checked-clean       — no family flagged and none abstained
//	3  checked-failed      — at least one family flagged
//	4  could-not-check     — no flag, but a family could not render a verdict (fail closed)
//
// This tool performs no outward write and advances no desk flow, so it carries no kill
// switch: its exit status IS the scan verdict.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit/untrustcorpus"
)

const usage = `deskscanuntrusted — deterministic inbound exfil + injection pre-scanner (Layer A).

USAGE:
  deskscanuntrusted --input <file|dir|->  [--format json|human] [--neutralized-out <path>]
  deskscanuntrusted --version

--input             a file (scan its bytes), a directory (a corpus.yaml manifest dir is
                    loaded through the brief-02 generator; otherwise each regular file is
                    scanned), or - for stdin.
--format            json (default) | human.
--neutralized-out   write the neutralised rendering (invisibles escaped, body fenced as
                    inert data) to this path. For a multi-document directory the renderings
                    are concatenated.

Exit: 0 checked-clean · 3 checked-failed · 4 could-not-check (fail closed).`

// Verdict exit codes — the brief's three-state contract, distinct from the shared desk-tool
// exit-code table because here the code IS the scan verdict.
const (
	exitClean  = 0
	exitFailed = 3
	exitCNC    = 4
	exitUsage  = 2
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	var (
		input    string
		format   = "json"
		neuOut   string
		showVers bool
	)
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--version" || a == "-version":
			showVers = true
			i++
		case a == "-h" || a == "--help" || a == "help":
			fmt.Fprintln(os.Stderr, usage)
			return deskkit.ExitOK
		case a == "--input":
			if i+1 >= len(args) {
				return usageErr("--input needs a value")
			}
			input = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--input="):
			input = strings.TrimPrefix(a, "--input=")
			i++
		case a == "--format":
			if i+1 >= len(args) {
				return usageErr("--format needs a value")
			}
			format = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
			i++
		case a == "--neutralized-out":
			if i+1 >= len(args) {
				return usageErr("--neutralized-out needs a value")
			}
			neuOut = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--neutralized-out="):
			neuOut = strings.TrimPrefix(a, "--neutralized-out=")
			i++
		default:
			return usageErr(fmt.Sprintf("unknown argument %q", a))
		}
	}

	if showVers {
		sha, built := deskkit.Version()
		fmt.Printf("deskscanuntrusted sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if format != "json" && format != "human" {
		return usageErr(fmt.Sprintf("--format must be json or human, got %q", format))
	}
	if input == "" {
		return usageErr("--input is required")
	}

	verdict, err := scanInput(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deskscanuntrusted: %v\n", err)
		// A read that could not happen is could-not-check, never a silent clean.
		return exitCNC
	}

	if neuOut != "" {
		if err := os.WriteFile(neuOut, []byte(verdict.Neutralized), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "deskscanuntrusted: write neutralized-out: %v\n", err)
			return exitCNC
		}
	}

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(verdict); err != nil {
			fmt.Fprintf(os.Stderr, "deskscanuntrusted: encode: %v\n", err)
			return exitCNC
		}
	case "human":
		printHuman(os.Stdout, verdict)
	}

	switch verdict.Overall {
	case deskkit.UntrustStateFailed:
		return exitFailed
	case deskkit.UntrustStateCNC:
		return exitCNC
	default:
		return exitClean
	}
}

// scanInput resolves --input to one or more documents, scans each, and aggregates the
// verdicts into one.
func scanInput(input string) (deskkit.UntrustVerdict, error) {
	if input == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return deskkit.UntrustVerdict{}, fmt.Errorf("read stdin: %w", err)
		}
		return deskkit.UntrustScan(b, deskkit.UntrustScanOptions{}), nil
	}

	fi, err := os.Stat(input)
	if err != nil {
		return deskkit.UntrustVerdict{}, fmt.Errorf("stat %q: %w", input, err)
	}
	if !fi.IsDir() {
		b, err := os.ReadFile(input)
		if err != nil {
			return deskkit.UntrustVerdict{}, fmt.Errorf("read %q: %w", input, err)
		}
		return deskkit.UntrustScan(b, deskkit.UntrustScanOptions{}), nil
	}

	// Directory: a corpus.yaml manifest is loaded through the brief-02 generator; any
	// other directory is scanned file-by-file.
	docs, err := documentsFromDir(input)
	if err != nil {
		return deskkit.UntrustVerdict{}, err
	}
	if len(docs) == 0 {
		return deskkit.UntrustVerdict{}, fmt.Errorf("no documents to scan under %q", input)
	}
	verdicts := make([]deskkit.UntrustVerdict, 0, len(docs))
	for _, d := range docs {
		verdicts = append(verdicts, deskkit.UntrustScan(d, deskkit.UntrustScanOptions{}))
	}
	return aggregate(verdicts), nil
}

// documentsFromDir returns the byte documents a directory contributes: the assembled inert
// samples of a corpus.yaml manifest, or each regular file's raw bytes.
func documentsFromDir(dir string) ([][]byte, error) {
	if _, err := os.Stat(filepath.Join(dir, untrustcorpus.CorpusFile)); err == nil {
		samples, lerr := untrustcorpus.Load(dir)
		if lerr != nil {
			return nil, fmt.Errorf("load corpus manifest %q: %w", dir, lerr)
		}
		docs := make([][]byte, 0, len(samples))
		for _, s := range samples {
			docs = append(docs, s.Bytes)
		}
		return docs, nil
	}

	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}
	var docs [][]byte
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			return nil, fmt.Errorf("read %q: %w", e.Name(), rerr)
		}
		docs = append(docs, b)
	}
	return docs, nil
}

// aggregate folds many per-document verdicts into one: a family is failed if any document
// failed it, else could-not-check if any could not check it, else clean; markers are
// unioned; the neutralised renderings are concatenated. The roll-up preserves the fail-
// closed ordering (failed > could-not-check > clean).
func aggregate(vs []deskkit.UntrustVerdict) deskkit.UntrustVerdict {
	fams := []string{deskkit.FamilyExfil, deskkit.FamilyInjection, deskkit.FamilyCodeExec}
	out := deskkit.UntrustVerdict{Families: map[string]deskkit.UntrustFamilyResult{}}
	markerSets := map[string]map[string]bool{}
	states := map[string]string{}
	for _, f := range fams {
		markerSets[f] = map[string]bool{}
		states[f] = deskkit.UntrustStateClean
	}

	var neu []string
	for _, v := range vs {
		neu = append(neu, v.Neutralized)
		for _, f := range fams {
			r := v.Families[f]
			states[f] = mergeState(states[f], r.State)
			for _, m := range r.Markers {
				markerSets[f][m] = true
			}
		}
	}

	for _, f := range fams {
		markers := make([]string, 0, len(markerSets[f]))
		for m := range markerSets[f] {
			markers = append(markers, m)
		}
		sort.Strings(markers)
		out.Families[f] = deskkit.UntrustFamilyResult{State: states[f], Markers: markers}
	}
	out.Overall = rollUp(out.Families)
	out.Neutralized = strings.Join(neu, "\n")
	return out
}

// mergeState combines two family states with the fail-closed precedence
// failed > could-not-check > clean.
func mergeState(a, b string) string {
	if a == deskkit.UntrustStateFailed || b == deskkit.UntrustStateFailed {
		return deskkit.UntrustStateFailed
	}
	if a == deskkit.UntrustStateCNC || b == deskkit.UntrustStateCNC {
		return deskkit.UntrustStateCNC
	}
	return deskkit.UntrustStateClean
}

// rollUp mirrors the engine's overall computation for an aggregated verdict.
func rollUp(families map[string]deskkit.UntrustFamilyResult) string {
	anyCNC := false
	for _, f := range families {
		switch f.State {
		case deskkit.UntrustStateFailed:
			return deskkit.UntrustStateFailed
		case deskkit.UntrustStateCNC:
			anyCNC = true
		}
	}
	if anyCNC {
		return deskkit.UntrustStateCNC
	}
	return deskkit.UntrustStateClean
}

func printHuman(w io.Writer, v deskkit.UntrustVerdict) {
	_, _ = fmt.Fprintf(w, "overall: %s\n", v.Overall)
	for _, f := range []string{deskkit.FamilyExfil, deskkit.FamilyInjection, deskkit.FamilyCodeExec} {
		r := v.Families[f]
		markers := "-"
		if len(r.Markers) > 0 {
			markers = strings.Join(r.Markers, ", ")
		}
		_, _ = fmt.Fprintf(w, "  %-10s %-16s %s\n", f, r.State, markers)
	}
}

func usageErr(msg string) int {
	fmt.Fprintf(os.Stderr, "deskscanuntrusted: %s\n\n%s\n", msg, usage)
	return exitUsage
}
