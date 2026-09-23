package main

// table.go — the default (no-flag) rendering: one row per queue item, ported from the
// oracle's render_table() (assay-inbox.sh:398-415) plus its display-hygiene clean() and
// title-truncation rule (assay-inbox.sh:369-394).

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

// cleanText strips every C0/DEL control byte from s — the oracle's `clean` jq def
// (`gsub("\\p{Cc}";"")`). Issue titles and label names are attacker-supplied on any repo a
// stranger can open an issue in, and a raw ESC (0x1b) would otherwise reach the operator's
// terminal intact. Display-only: ranking and matching run on the UNCLEANED strings.
func cleanText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// truncateTitle cuts a cleaned title to 57 runes + "..." when longer — the oracle's
// `if length > 57 then .[:57] + "..." else . end`.
func truncateTitle(s string) string {
	r := []rune(s)
	if len(r) > 57 {
		return string(r[:57]) + "..."
	}
	return s
}

// ageOf renders the oracle's render_table age column: whole days since createdAt (floor,
// via integer division of the epoch delta — assay-inbox.sh:402-408), or the raw createdAt
// string when it does not parse as RFC3339 (the oracle's fallback when neither GNU nor BSD
// `date` can parse it).
func ageOf(createdAt string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return createdAt
	}
	days := int(now.UTC().Sub(t.UTC()).Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

// renderTable prints one row per item, oldest-urgent-first (the order fetchQueue already
// produced) — printf "%s %-14s %-45s %-8s %-60s %-5s %s\n" in the oracle, marker "**" for
// rank 0 else two spaces.
func renderTable(w io.Writer, items []item, now time.Time) {
	for _, it := range items {
		marker := "  "
		if it.Rank == 0 {
			marker = "**"
		}
		labelNames := make([]string, 0, len(it.Labels))
		for _, l := range it.Labels {
			labelNames = append(labelNames, cleanText(l))
		}
		title := truncateTitle(cleanText(it.Title))
		fmt.Fprintf(w, "%s %-14s %-45s %-8s %-60s %-5s %s\n",
			marker,
			strings.Join(labelNames, ","),
			it.Repo,
			fmt.Sprintf("#%d", it.Number),
			title,
			ageOf(it.CreatedAt, now),
			cleanText(it.URL),
		)
	}
}
