package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/deskcore/issueflow"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var handoffRoot = os.Getwd

const handoffRecordsFile = "intake-handoffs-v1.jsonl"

func readHandoffRecords(path string) ([]issueflow.IntakeHandoff, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var records []issueflow.IntakeHandoff
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 65536), 1024*1024)
	for sc.Scan() {
		h, err := issueflow.DecodeIntakeHandoff(sc.Bytes())
		if err != nil {
			return nil, err
		}
		records = append(records, h)
	}
	return records, sc.Err()
}

func appendHandoffRecord(h issueflow.IntakeHandoff) error {
	return withAuditLock(func(dir string) error {
		path := filepath.Join(dir, handoffRecordsFile)
		records, err := readHandoffRecords(path)
		if err != nil {
			return deskkit.Unverifiable("cannot read intake handoff history", err)
		}
		revision := 0
		for _, prev := range records {
			if !strings.EqualFold(prev.Item, h.Item) || prev.Opened != h.Opened {
				continue
			}
			if prev.Revision == h.Revision {
				if reflect.DeepEqual(prev, h) {
					return nil
				}
				return deskkit.Refused("intake handoff revision already contains another judgment")
			}
			if prev.Revision > revision {
				revision = prev.Revision
			}
		}
		if h.Revision != revision+1 {
			return deskkit.Refused(fmt.Sprintf("intake handoff revision must be %d", revision+1))
		}
		raw, err := json.Marshal(h)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return deskkit.Unverifiable("cannot open intake handoff history", err)
		}
		if _, err := f.Write(append(raw, '\n')); err != nil {
			f.Close()
			return deskkit.Unverifiable("cannot append intake handoff history", err)
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return deskkit.Unverifiable("cannot sync intake handoff history", err)
		}
		return f.Close()
	})
}

// A companion can refine a mechanical exit without rewriting intake-exit-v1.
// Retry still writes the companion after a prior exit succeeded: the v1 no-op
// must never suppress recovery from a failed second write.
func landHandoff(rec IntakeExitRecord, h issueflow.IntakeHandoff) error {
	if err := rec.Validate(); err != nil {
		return err
	}
	if h.Item != rec.Item || h.Opened != rec.Opened || h.DecidingRole != rec.TriagerRole {
		return deskkit.Refused("handoff must bind this event and the acting role")
	}
	if h.Disposition == issueflow.NeedsScoping && rec.TriagerTier != tierStrong {
		return deskkit.Refused("authoring handoff requires a strong deciding session")
	}
	facts := issueflow.IntakeFacts{Item: rec.Item, Opened: rec.Opened, Complete: true}
	if h.Verification != nil {
		// This validates the supplied observation's consistency only. The record
		// cannot prove live source freshness or confer dispatch/acceptance authority.
		facts.Verification = issueflow.HandoffFacts{Source: h.Verification.Source, Complete: true, Now: landNow()}
	}
	assessment := issueflow.AssessIntakeHandoff(h, facts)
	if assessment.State != "observed" && assessment.State != "external-wait" {
		return deskkit.Refused("intake disposition: " + assessment.Reason)
	}
	if h.Disposition != issueflow.External {
		if err := checkHandoffSuccessor(h, rec.Repo); err != nil {
			return err
		}
	}
	exists := false
	if err := withAuditLock(func(dir string) error {
		records, err := readExitRecords(filepath.Join(dir, intakeExitsFile))
		if err != nil {
			return err
		}
		for _, prev := range records {
			if sameEvent(prev, rec) {
				exists = true
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !exists {
		if _, err := appendExitRecord(rec, func() error {
			return auditExit("scanloop", ExitRecord{ItemID: rec.Item, Exit: rec.Exit, Lane: LaneRouting, Artifact: rec.Artifact})
		}); err != nil {
			return err
		}
	}
	return appendHandoffRecord(h)
}

// Resolve links through the existing forge adapter or the declared brief files.
// A syntactically valid reference alone must not satisfy successor-owed work.
func checkHandoffSuccessor(h issueflow.IntakeHandoff, sourceRepo string) error {
	if repo, rawNumber, ok := strings.Cut(h.Successor, "#"); ok {
		if repo == "" {
			repo = sourceRepo
		}
		number, err := strconv.Atoi(rawNumber)
		if err != nil {
			return deskkit.Refused("invalid successor issue")
		}
		f, fr, err := forgeFor(repo)
		if err != nil {
			return err
		}
		issue, err := f.GetIssue(fr, number)
		if err != nil {
			return deskkit.Unverifiable("cannot read handoff successor", err)
		}
		if issue.State != "open" {
			return deskkit.Refused("successor is not open; retain successor-owed work")
		}
		admission := admitOne(Inbound{Repo: repo, Number: number}, forgeTrustProbe(nil))
		if !admission.Admitted() {
			return deskkit.Unverifiable("successor trust is not admitted", nil)
		}
		return nil
	}
	stream, num, ok := strings.Cut(h.Successor, "/")
	if !ok {
		return deskkit.Refused("successor has no typed owner")
	}
	root, err := handoffRoot()
	if err != nil {
		return err
	}
	dir, err := os.OpenRoot(filepath.Join(root, "docs", "streams", stream))
	if err != nil {
		return deskkit.Unverifiable("cannot read successor brief owner", err)
	}
	defer dir.Close()
	listing, err := dir.Open(".")
	if err != nil {
		return err
	}
	defer listing.Close()
	names, err := listing.Readdirnames(-1)
	if err != nil {
		return err
	}
	var matches []string
	for _, name := range names {
		if (name == "brief-"+num+".md" || strings.HasPrefix(name, "brief-"+num+"-")) && strings.HasSuffix(name, ".md") {
			matches = append(matches, name)
		}
	}
	if len(matches) != 1 {
		return deskkit.Refused("successor brief is missing or ambiguous")
	}
	raw, err := dir.ReadFile(matches[0])
	if err != nil {
		return deskkit.Unverifiable("cannot read successor brief", err)
	}
	fields := deskkit.ParseBriefAdmission(string(raw))
	if !fields.Frontmatter {
		return deskkit.Refused("successor lacks brief frontmatter")
	}
	// Both supported qualified IDs and legacy stream/NN identify the same file.
	if fields.ID != h.Successor && !strings.HasSuffix(fields.ID, ":"+stream+":"+num) && !strings.HasSuffix(fields.ID, ":"+stream+":b-"+num) {
		return deskkit.Refused("successor file declares another brief")
	}
	if h.Disposition == issueflow.NeedsScoping && fields.ExecTier != "strong" {
		return deskkit.Refused("successor authoring brief does not declare strong execution")
	}
	return nil
}
