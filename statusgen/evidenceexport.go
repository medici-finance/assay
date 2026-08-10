package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// evidenceManifest is the generated manifest.json inside the export tarball.
// It describes every file in the bundle and the generation context.
//
// Omitted records every artifact the collector could NOT bundle, with a reason.
// It is part of the manifest rather than stderr-only on purpose: this bundle is
// handed to auditors, and an incomplete bundle that looks complete is the single
// worst failure mode here. A consumer reads `omitted` to learn what is missing.
type evidenceManifest struct {
	Generated string          `json:"generated"` // ISO 8601 timestamp
	Version   string          `json:"version"`   // statusgenVersion
	Files     []manifestEntry `json:"files"`
	Omitted   []omittedEntry  `json:"omitted"`
}

type manifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// omittedEntry records one artifact that was expected but not bundled.
type omittedEntry struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// collectBundledFiles gathers brief files, the stream READMEs that carry their
// lifecycle/Verified/Reviewed cells, and register entries whose dates fall within
// [from, to] (inclusive). The date key for briefs is the authored: frontmatter
// field; for register entries it is the date: field.
//
// Returns the sorted repo-relative paths, a map of path → content bytes, and the
// list of artifacts that were expected but could not be bundled. The collector
// FAILS CLOSED: a real I/O error aborts the export, and anything skipped is
// reported rather than silently dropped (the old behaviour swallowed every
// ReadDir/ReadFile/parse error, so a corrupt evidence file vanished without a
// trace — the opposite of what a compliance bundle needs).
func collectBundledFiles(root, from, to string) ([]string, map[string][]byte, []omittedEntry, error) {
	fromDate, err := parseDate(from)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid from date %q: %w", from, err)
	}
	toDate, err := parseDate(to)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid to date %q: %w", to, err)
	}

	bundle := map[string][]byte{}
	var omitted []omittedEntry

	// relPath is a helper: repo-relative path for the manifest and tar header.
	relPath := func(path string) (string, error) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", fmt.Errorf("rel %s: %w", path, err)
		}
		return filepath.ToSlash(rel), nil
	}

	// Collect brief files whose authored date is in range, and remember which
	// streams contributed so their README travels with them.
	streams, _, err := loadStreams(root)
	if err != nil {
		return nil, nil, nil, err
	}
	contributingStreams := map[string]bool{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			inRange, err := briefInRange(path, fromDate, toDate)
			if err != nil {
				return nil, nil, nil, err
			}
			if !inRange {
				continue
			}
			rel, err := relPath(path)
			if err != nil {
				return nil, nil, nil, err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, nil, err
			}
			bundle[rel] = raw
			contributingStreams[s.Dir] = true
		}
	}

	// Collect the stream README for every stream that contributed an in-range
	// brief. The lifecycle Status column and the Verified/Reviewed cells exist
	// ONLY in this table — without it the bundle cannot evidence who verified
	// what or what state anything reached, which is exactly what the SOC2
	// mapping's Testing / Segregation-of-duties / Approval rows point at.
	//
	// NOTE: the README is bundled WHOLE, not date-filtered. Its table covers
	// every brief in the stream, including ones outside [from, to]. That is a
	// deliberate tradeoff (the rows carry no per-row date to filter on) and is
	// disclosed in docs/evidence-bundle.md.
	for _, s := range streams {
		if !contributingStreams[s.Dir] {
			continue
		}
		readme := filepath.Join(s.Dir, "README.md")
		raw, err := os.ReadFile(readme)
		if err != nil {
			rel, relErr := relPath(readme)
			if relErr != nil {
				return nil, nil, nil, relErr
			}
			if os.IsNotExist(err) {
				omitted = append(omitted, omittedEntry{Path: rel, Reason: "stream README not present; lifecycle and Verified/Reviewed cells unavailable for this stream"})
				continue
			}
			return nil, nil, nil, err
		}
		rel, err := relPath(readme)
		if err != nil {
			return nil, nil, nil, err
		}
		bundle[rel] = raw
	}

	// Collect register entries whose date is in range by scanning the directory.
	// Filenames are date-slug.md, not ID-based, so we scan and parse frontmatter.
	registers := []struct {
		dir   string
		label string
		parse func([]byte) (string, error)
	}{
		{
			dir:   filepath.Join(root, "docs", "streams", "intake"),
			label: "intake",
			parse: func(raw []byte) (string, error) {
				e, err := parseIntakeFile(raw)
				if err != nil {
					return "", err
				}
				return e.Date, nil
			},
		},
		{
			dir:   filepath.Join(root, "docs", "streams", "findings"),
			label: "findings",
			parse: func(raw []byte) (string, error) {
				e, err := parseFindingFile(raw)
				if err != nil {
					return "", err
				}
				return e.Date, nil
			},
		},
	}

	for _, reg := range registers {
		entries, err := os.ReadDir(reg.dir)
		if err != nil {
			rel, relErr := relPath(reg.dir)
			if relErr != nil {
				return nil, nil, nil, relErr
			}
			if os.IsNotExist(err) {
				// A register directory that does not exist in this repo is a
				// legitimate state, but it must be VISIBLE — the doc promises
				// register entries, so their absence has to be recorded.
				omitted = append(omitted, omittedEntry{Path: rel, Reason: "no " + reg.label + " register directory in this repo; no " + reg.label + " entries bundled"})
				continue
			}
			// Any other error (permissions, I/O) is real and must not be
			// mistaken for "this register is empty".
			return nil, nil, nil, fmt.Errorf("read %s register dir %s: %w", reg.label, reg.dir, err)
		}
		for _, f := range entries {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			path := filepath.Join(reg.dir, f.Name())
			rel, err := relPath(path)
			if err != nil {
				return nil, nil, nil, err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("read %s entry %s: %w", reg.label, path, err)
			}
			dateStr, err := reg.parse(raw)
			if err != nil {
				omitted = append(omitted, omittedEntry{Path: rel, Reason: "unparseable " + reg.label + " entry: " + err.Error()})
				continue
			}
			d, err := parseDate(dateStr)
			if err != nil {
				omitted = append(omitted, omittedEntry{Path: rel, Reason: "unparseable date in " + reg.label + " entry: " + err.Error()})
				continue
			}
			if !inDateRange(d, fromDate, toDate) {
				continue
			}
			bundle[rel] = raw
		}
	}

	// Sort paths for deterministic order.
	paths := make([]string, 0, len(bundle))
	for p := range bundle {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	sort.Slice(omitted, func(i, j int) bool { return omitted[i].Path < omitted[j].Path })

	return paths, bundle, omitted, nil
}

// briefInRange parses the brief file's authored date from its frontmatter and
// reports whether it falls within [from, to] inclusive. A brief with no
// frontmatter or no authored date is out of range.
func briefInRange(path string, from, to time.Time) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSpace(first) != "---" {
		return false, nil // no frontmatter
	}
	fmRaw, _, err := splitFrontmatter(content)
	if err != nil {
		return false, nil // unparseable frontmatter → skip
	}

	var fm struct {
		Authored string `yaml:"authored"`
	}
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return false, nil
	}
	if fm.Authored == "" {
		return false, nil
	}
	// authored can be a bare date "2026-07-17" or a fuller string
	// "2026-07-17 by fable-gtm-fanout". Take the first whitespace-delimited
	// token as the date.
	dateStr := strings.Fields(fm.Authored)[0]
	d, err := parseDate(dateStr)
	if err != nil {
		return false, nil
	}
	return inDateRange(d, from, to), nil
}

// parseDate parses a YYYY-MM-DD date string. No other format is accepted.
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD, got %q", s)
	}
	return t, nil
}

// inDateRange reports whether d is within [from, to] inclusive.
func inDateRange(d, from, to time.Time) bool {
	return !d.Before(from) && !d.After(to)
}

// writeEvidenceBundle writes a gzip-compressed tarball to outPath containing the
// bundled files and a generated manifest.json. Files are added in sorted path
// order and EVERY tar header carries a fixed zero modtime, so two runs over the
// same inputs with the same `generated` value produce byte-identical output.
//
// The only value that varies between runs is manifest.Generated. Previously the
// generation time also leaked into the tar ModTime of every non-brief file,
// which made the documented byte-identical guarantee false for any two runs more
// than a second apart. Callers that need reproducible bytes pass an explicit
// timestamp (`-generated`); see runEvidenceExport.
func writeEvidenceBundle(outPath string, paths []string, bundle map[string][]byte, omitted []omittedEntry, generated time.Time) (err error) {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Close in order (tar footer → gzip flush → file) and REPORT failures.
	// Discarding these meant a truncated bundle on a full disk still returned
	// nil and printed "exported N files" over an unreadable tarball.
	defer func() {
		cerr := tw.Close()
		if gerr := gw.Close(); cerr == nil {
			cerr = gerr
		}
		if ferr := f.Close(); cerr == nil {
			cerr = ferr
		}
		if err == nil && cerr != nil {
			err = fmt.Errorf("finalize bundle %s: %w", outPath, cerr)
		}
	}()

	// Files/Omitted are non-nil so the manifest serialises `[]`, never `null`,
	// for an empty result set.
	manifest := evidenceManifest{
		Generated: generated.UTC().Format(time.RFC3339),
		Version:   statusgenVersion,
		Files:     []manifestEntry{},
		Omitted:   []omittedEntry{},
	}
	if omitted != nil {
		manifest.Omitted = omitted
	}

	writeFile := func(name string, content []byte) error {
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0o644,
			// Fixed zero modtime for every entry — see the doc comment.
			ModTime:  time.Time{},
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(content)
		return err
	}

	for _, rel := range paths {
		content := bundle[rel]
		sum := sha256.Sum256(content)
		manifest.Files = append(manifest.Files, manifestEntry{
			Path:   rel,
			SHA256: fmt.Sprintf("%x", sum),
		})
		if err := writeFile(rel, content); err != nil {
			return err
		}
	}

	// Write manifest.json last so it is always at the end of the tar stream
	// for consumers that stream-scan.
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeFile("manifest.json", manifestJSON)
}

// runEvidenceExport is the entry point for --export-evidence <from> <to>.
//
// generated is the manifest timestamp. A zero value means "use the clock" —
// callers that want byte-reproducible output pass an explicit time via
// `-generated`, per brief-05's "generation timestamp passed in" deliverable.
//
// Exit codes: 0 clean · 1 error · 3 exported but INCOMPLETE (something the
// bundle was expected to contain could not be collected). 3 is distinct because
// a silently-incomplete compliance bundle is worse than a failed one.
func runEvidenceExport(root, from, to, output string, generated time.Time) int {
	paths, bundle, omitted, err := collectBundledFiles(root, from, to)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	if generated.IsZero() {
		generated = nowFunc()
	}
	if err := writeEvidenceBundle(output, paths, bundle, omitted, generated); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Printf("exported %d files to %s (range %s to %s)\n", len(paths), output, from, to)

	rc := 0
	if len(omitted) > 0 {
		fmt.Fprintf(os.Stderr, "statusgen: WARNING — bundle is INCOMPLETE, %d expected artifact(s) omitted (also recorded in manifest.json \"omitted\"):\n", len(omitted))
		for _, o := range omitted {
			fmt.Fprintf(os.Stderr, "  - %s: %s\n", o.Path, o.Reason)
		}
		rc = 3
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "statusgen: WARNING — the bundle contains no evidence files; check the date range and --root")
		rc = 3
	}
	return rc
}
