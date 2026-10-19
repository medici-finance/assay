package main

// briefschemaversions.go — which brief schemas this statusgen supports, and the
// loud refusal for a schema newer than this binary.
//
// The normative table (schema version → first statusgen release that supports
// it) lives in spec/brief-v1.md §"Schema versions". briefSchemaFirstRelease is
// that table as this binary knows it; `statusgen --version --json` prints it so
// a release's supported set is machine-readable. A row whose release is ""
// ships in the release that first carries this source (the release cut fills
// the spec table in).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// briefSchemaFirstRelease maps every recognized brief schema to the first
// statusgen release that reads it. Keep it in lockstep with
// recognizedBriefSchemas (TestBriefSchemaFirstReleaseCoversRecognized).
var briefSchemaFirstRelease = map[string]string{
	briefSchemaCurrent: "v0.9.0",  // brief-v1: the earliest published release
	briefSchemaV2:      "v1.0.0",  // brief-v2: the derived-board release boundary (versiongate.go)
	briefSchemaV3:      "v1.0.50", // brief-v3: the withdrawn-status release
}

// isBriefV2Family reports whether a schema carries every brief-v2 rule: brief-v2
// itself and brief-v3 (brief-v2 plus the withdrawn status keys).
func isBriefV2Family(schema string) bool {
	return schema == briefSchemaV2 || schema == briefSchemaV3
}

// versionJSON is the `statusgen --version --json` document.
type versionJSON struct {
	Version      string            `json:"version"`
	BriefSchemas []string          `json:"briefSchemas"`
	FirstRelease map[string]string `json:"briefSchemaFirstRelease"`
}

// printVersionJSON writes the machine-readable version + supported-schema set.
// A "" first release is reported as this build's own version.
func printVersionJSON() {
	v := versionJSON{Version: statusgenVersion, FirstRelease: map[string]string{}}
	for s := range recognizedBriefSchemas {
		v.BriefSchemas = append(v.BriefSchemas, s)
		rel := briefSchemaFirstRelease[s]
		if rel == "" {
			rel = statusgenVersion
		}
		v.FirstRelease[s] = rel
	}
	sort.Strings(v.BriefSchemas)
	out, _ := json.Marshal(v)
	fmt.Println(string(out))
}

// pinnedStatusgenFor walks up from a brief path to the nearest .assay-versions
// and returns the statusgen tag it pins, or "" when none is found.
func pinnedStatusgenFor(path string) string {
	dir := filepath.Dir(path)
	for i := 0; i < 64; i++ {
		raw, err := os.ReadFile(filepath.Join(dir, ".assay-versions"))
		if err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				f := strings.Fields(line)
				if len(f) >= 2 && f[0] == "statusgen" {
					return f[1]
				}
			}
			return ""
		}
		// Stop at this checkout's boundary, never borrow a parent's pin.
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			break
		}
		if (filepath.Base(dir) == "streams" || filepath.Base(dir) == "archive") && filepath.Base(filepath.Dir(dir)) == "docs" {
			// A source export may have no .git: its docs/streams or docs/archive anchor is
			// enough to bound the search to the owning root.
			root := filepath.Dir(filepath.Dir(dir))
			raw, err := os.ReadFile(filepath.Join(root, ".assay-versions"))
			if err == nil {
				for _, line := range strings.Split(string(raw), "\n") {
					f := strings.Fields(line)
					if len(f) >= 2 && f[0] == "statusgen" {
						return f[1]
					}
				}
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// unknownBriefSchemaMessage is the refusal for a brief-v* schema this binary
// does not recognize (fail CLOSED, #271 — never the no-schema exemption). It
// names the schema the tree needs, the schemas this build supports, where the
// supporting release is listed, and the statusgen the repo pins.
func unknownBriefSchemaMessage(path, schema string) string {
	pin := pinnedStatusgenFor(path)
	pinPart := "this repo pins no statusgen in .assay-versions"
	if pin != "" {
		pinPart = "this repo pins statusgen " + pin + " in .assay-versions"
	}
	return fmt.Sprintf("%s: unsupported brief schema — needs schema %s, but this statusgen (%s) supports only %s; %s is supported from a later statusgen release (spec/brief-v1.md §Schema versions lists the first release per schema); %s. Upgrade statusgen (assay:upgrade-assay) — schema evolution fails closed and is never exempted (#271)",
		path, schema, statusgenVersion, recognizedBriefSchemaList(), schema, pinPart)
}
