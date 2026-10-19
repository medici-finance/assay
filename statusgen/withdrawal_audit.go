package main

// withdrawal-audit is an offline rollout report over a caller-supplied forge
// snapshot. It never changes a branch, claim, PR or lifecycle cell.
import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type withdrawalAuditPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
	Files       []struct {
		Path string `json:"path"`
	} `json:"files"`
}
type withdrawalAuditItem struct {
	Basis      []string `json:"basis"`
	PR         int      `json:"pr"`
	Head       string   `json:"head"`
	Brief      string   `json:"brief"`
	Successors []string `json:"successors"`
	Treatment  string   `json:"treatment"`
}
type withdrawalAuditReport struct {
	Schema      string                `json:"schema"`
	Coverage    string                `json:"coverage"`
	SnapshotPRs int                   `json:"snapshot_prs"`
	Affected    []withdrawalAuditItem `json:"affected"`
}

var withdrawalAuditHeadRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

func auditWithdrawals(streams []*Stream, prs []withdrawalAuditPR) withdrawalAuditReport {
	report := withdrawalAuditReport{Schema: "withdrawal-audit-v1", Coverage: "snapshot-only; re-read heads and claims before acting; unlisted PRs and indirect consumers are could-not-check", SnapshotPRs: len(prs), Affected: []withdrawalAuditItem{}}
	res := newWithdrawalResolver(streams)
	branchPatterns := map[string]*regexp.Regexp{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			if isWithdrawn(b.Status) {
				branchPatterns[s.Name+"/"+b.Num] = regexp.MustCompile(`(^|[/_-])` + regexp.QuoteMeta(s.Name) + `[/_-]` + regexp.QuoteMeta(b.Num) + `($|[/_-])`)
			}
		}
	}
	for _, pr := range prs {
		for _, s := range streams {
			for _, b := range s.Briefs {
				if !isWithdrawn(b.Status) {
					continue
				}
				id := s.Name + "/" + b.Num
				// Path match covers both implementation and evidence PRs. A README
				// match is intentionally broad: it may alter any lifecycle row in it.
				basis := []string{}
				matched := branchPatterns[id].MatchString(pr.HeadRefName)
				if matched {
					basis = append(basis, "branch identity")
				}
				for _, f := range pr.Files {
					p := filepath.ToSlash(filepath.Clean(f.Path))
					prefix := "docs/streams/" + s.Name + "/"
					if !strings.HasPrefix(p, prefix) {
						continue
					}
					tail := strings.TrimPrefix(p, prefix)
					if tail == "README.md" || filepath.Base(tail) == "brief-"+b.Num+".md" || strings.HasPrefix(filepath.Base(tail), "brief-"+b.Num+"-") || strings.HasPrefix(tail, "evidence/"+b.Num+".") || strings.HasPrefix(tail, "evidence/"+b.Num+"-") {
						matched = true
						if tail == "README.md" {
							basis = append(basis, "stream README (conservative overlap)")
						} else {
							basis = append(basis, "brief/evidence path")
						}
					}
				}
				if matched {
					report.Affected = append(report.Affected, withdrawalAuditItem{Basis: basis, PR: pr.Number, Head: pr.HeadRefOid, Brief: id, Successors: append([]string{}, res.successorsOf(s.Root, s.Name, b.Num, &b)...), Treatment: "reconcile surviving work, evidence and active claim; rebase/retarget or propose disposition with independent confirmation; never auto-close"})
				}
			}
		}
	}
	sort.Slice(report.Affected, func(i, j int) bool {
		a, b := report.Affected[i], report.Affected[j]
		if a.PR != b.PR {
			return a.PR < b.PR
		}
		return a.Brief < b.Brief
	})
	return report
}

func runWithdrawalAudit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("withdrawal-audit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "candidate board root (offline)")
	snapshot := fs.String("prs", "", "JSON snapshot: gh pr list --state open --limit <sufficient> --json number,headRefName,headRefOid,files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *snapshot == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "withdrawal-audit requires --prs FILE and optional --root DIR")
		return 2
	}
	raw, err := os.ReadFile(*snapshot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var prs []withdrawalAuditPR
	if err := json.Unmarshal(raw, &prs); err != nil || prs == nil {
		fmt.Fprintln(stderr, "withdrawal-audit: snapshot must be a JSON array of PR records")
		return 2
	}
	for _, pr := range prs {
		if pr.Number < 1 || !withdrawalAuditHeadRe.MatchString(pr.HeadRefOid) || pr.Files == nil {
			fmt.Fprintln(stderr, "withdrawal-audit: each PR needs a positive number, immutable headRefOid and files array")
			return 2
		}
	}
	streams, _, err := loadStreams(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(auditWithdrawals(streams, prs)); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}
