package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/packet"
)

const performanceSuffix = ".perf-context.json"

// writePerformanceHandoff reuses the dispatch packet cache and owner-only writer.
// Attribution is optional: failures are silent gaps, never failed dispatches or
// metadata in diagnostic output. A dry run never calls this boundary.
func writePerformanceHandoff(c perfrec.Context) string {
	if c.Profile() == "" {
		return ""
	}
	cache, err := packetCacheDir()
	if err != nil || !filepath.IsAbs(cache) {
		return ""
	}
	dir := filepath.Join(cache, "assay", "packets")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ""
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return ""
	}
	id, err := perfrec.NewID()
	if err != nil {
		return ""
	}
	dest := filepath.Join(dir, id+performanceSuffix)
	if _, err := packet.AssignmentLine(dest); err != nil {
		return ""
	}
	now := packetNow().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	data, err := json.Marshal(struct {
		Environment      []string `json:"environment"`
		DeskRole         string   `json:"deskRoleOverride,omitempty"`
		ExpiresBeforeUTC string   `json:"expiresBeforeUtc"`
	}{perfrec.ContextEnv(nil, c, false), c.Data().DeskRole, today.AddDate(0, 0, 7).Format(time.RFC3339)})
	if err != nil {
		return ""
	}
	if err := packet.Write(dest, packet.Packet{Text: string(data) + "\n"}); err != nil {
		return ""
	}
	cutoff := today.AddDate(0, 0, -6)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), performanceSuffix) {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return dest
}
