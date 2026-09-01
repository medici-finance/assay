package dorajoin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// source.go — the pluggable delivery-metrics source (spec §8). The delivery-
// metrics collector keeps collecting; this join reads THROUGH this seam,
// never hardcoding a specific collector. A house wiring dorajoin to a live
// collector supplies its own DeliveryMetricsSource — a configuration-time
// adapter swap, never new join logic (denominator.go, cfr.go and joinkeys.go
// never reference FileSource or any other concrete source).

// DeliveryRecord is one delivery-metrics event as the join needs it: the join
// key to resolve it against the quality side, whether it represents an
// incident (the incident-based CFR's numerator input), and the window label
// it falls in. Any other field a live collector carries is the collector's
// own concern — this join reads only what it joins on.
type DeliveryRecord struct {
	Key      JoinKey `json:"key"`
	Window   string  `json:"window"`
	Incident bool    `json:"incident,omitempty"`
}

// DeliveryMetricsSource is the pluggable seam: given nothing (the source
// already knows its own configuration — a file path, a collector endpoint, a
// query window), it returns the delivery records the join resolves against
// the quality side. An error means the source could not be read at all; a
// caller reports that as could-not-join for every affected key, never as an
// empty-and-therefore-clean result.
type DeliveryMetricsSource interface {
	DeliveryRecords() ([]DeliveryRecord, error)
}

// FileSource is the in-tree reference adapter (spec §8): a newline-delimited
// JSON file of DeliveryRecord values, one per line. It is registered as the
// default source (DefaultSource) — a target with no live collector wired
// still gets a working join off a static export; wiring a live collector
// later is a config-time swap to a different DeliveryMetricsSource, not a
// code change here.
type FileSource struct {
	Path string
}

// NewFileSource builds the file-based reference adapter reading path.
func NewFileSource(path string) FileSource { return FileSource{Path: path} }

// DeliveryRecords reads and parses the newline-delimited JSON file. A blank
// line is skipped; a malformed line fails the whole read with its line
// number, rather than silently dropping one record from the join.
func (f FileSource) DeliveryRecords() ([]DeliveryRecord, error) {
	file, err := os.Open(f.Path)
	if err != nil {
		return nil, fmt.Errorf("dorajoin: opening delivery-metrics file %q: %w", f.Path, err)
	}
	defer file.Close()

	var out []DeliveryRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Text()
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var rec DeliveryRecord
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			return nil, fmt.Errorf("dorajoin: parsing %q line %d: %w", f.Path, line, err)
		}
		out = append(out, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("dorajoin: reading %q: %w", f.Path, err)
	}
	return out, nil
}

// DefaultSource returns the file-based reference adapter as the default
// DeliveryMetricsSource for path. A house wiring a live collector constructs
// its own DeliveryMetricsSource implementation instead — never a code change
// in this package.
func DefaultSource(path string) DeliveryMetricsSource {
	return NewFileSource(path)
}

// ensure FileSource satisfies the interface at compile time.
var _ DeliveryMetricsSource = FileSource{}
