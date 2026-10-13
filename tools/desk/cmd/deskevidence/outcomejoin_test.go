package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	jRegister  = "example-org/tracker"
	jSHA       = "0123456789abcdef0123456789abcdef01234567"
	jMerge     = "89abcdef0123456789abcdef0123456789abcdef"
	jDispatch  = "assay--x--1@20261006T141502Z.3fa9c01b7d2e"
	jHiddenA   = "example-org/hidden-lane"
	jOtherRepo = "example-org/other"
)

func sha256hex(s string) string {
	d := sha256.Sum256([]byte(s))
	return hex.EncodeToString(d[:])
}

// jrec renders the base verify-wake-v1 verify-fail record the join-key tests start from, with the
// overrides applied (a nil override deletes the key).
func jrec(t testing.TB, over map[string]any) string {
	t.Helper()
	m := map[string]any{
		"ts": "2026-10-06T14:20:00Z", "brief": "example-stream/14", "outcome": "verify-fail", "sha": jSHA,
		"wake_schema": deskkit.SchemaWakeV1, "receipt_id": "vw1-0123456789abcdef", "blocker_kind": "implementation",
		"wake_predicate": "explicit-recheck-with-reason", "recheck_reason": "fixture",
		"prs":           []any{map[string]any{"repo": jRegister, "number": 7, "merge_sha": jMerge}},
		"verifier_tier": "strong", "started": "2026-10-06T14:00:00Z", "finished": "2026-10-06T14:10:00Z",
		"rows_passed": 0, "rows_total": 1,
	}
	for k, v := range over {
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// jlegacy renders a legacy-shape verified record (no wake_schema) with the overrides applied.
func jlegacy(t testing.TB, over map[string]any) string {
	t.Helper()
	m := map[string]any{"ts": "2026-10-06T14:20:00Z", "brief": "example-stream/14", "outcome": "verified", "sha": jSHA}
	for k, v := range over {
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func jpr(repo string, n int, sha string) map[string]any {
	return map[string]any{"repo": repo, "number": n, "merge_sha": sha}
}

func prURL(repo string, n int) string {
	return fmt.Sprintf("https://forge.example/%s/pull/%d", repo, n)
}

// joinFake is setupFake plus one merged PR (the register's #7), served strictly.
func joinFake(t *testing.T) (*fakeForge, *int) {
	t.Helper()
	f, _ := setupFake(t)
	f.prs = map[string]*deskkit.PullRequest{
		jRegister + "#7": {URL: prURL(jRegister, 7), Merged: true, MergeCommitSHA: jMerge, BaseRef: "main", MergedAt: "2026-10-06T10:00:00Z"},
	}
	admissions := new(int)
	inner := verifierEvidenceAdmissionFn
	verifierEvidenceAdmissionFn = func(a, b, c string, fg deskkit.Forge) (deskkit.VerifierReceipt, error) {
		*admissions++
		return inner(a, b, c, fg)
	}
	return f, admissions
}

// jland runs the writer on one record line and returns the exit code and stderr.
func jland(t *testing.T, line string) (int, string) {
	t.Helper()
	return jlandAt(t, jRegister, line)
}

// jlandAt is jland against another register.
func jlandAt(t *testing.T, register, line string) (int, string) {
	t.Helper()
	errBuf := stderr.(interface{ String() string })
	before := len(errBuf.String())
	code := run([]string{register, "main", "--outcome-record", writeRepoFile(t, "record.json", line+"\n")})
	return code, errBuf.String()[before:]
}

func wantRefused(t *testing.T, name string, code int, msg string, names ...string) {
	t.Helper()
	if code != deskkit.ExitRefused {
		t.Errorf("%s: exit = %d, want %d (stderr %q)", name, code, deskkit.ExitRefused, msg)
		return
	}
	for _, n := range names {
		if !strings.Contains(msg, n) {
			t.Errorf("%s: refusal %q does not name %q", name, msg, n)
		}
	}
}

func wantNothingWritten(t *testing.T, name string, f *fakeForge) {
	t.Helper()
	if len(f.writes) != 0 || len(f.changes) != 0 {
		t.Errorf("%s: a refused record changed the forge: writes=%d changes=%d", name, len(f.writes), len(f.changes))
	}
}

func landed(t *testing.T, f *fakeForge) string {
	t.Helper()
	if len(f.writes) != 1 {
		t.Fatalf("writes = %d, want exactly one", len(f.writes))
	}
	return string(f.writes[0].Content)
}

// regression: a verify-fail record landed with no receipt, and the wake keys were optional.
func TestOutcomeRecord_FailWithoutReceipt(t *testing.T) {
	t.Run("legacy_verify_fail_names_wake_schema", func(t *testing.T) {
		f, errBuf := setupFake(t)
		line := jlegacy(t, map[string]any{"outcome": "verify-fail"})
		code := run([]string{jRegister, "main", "--outcome-record", writeRepoFile(t, "r.json", line)})
		wantRefused(t, "legacy verify-fail", code, errBuf.String(), "wake_schema")
		wantNothingWritten(t, "legacy verify-fail", f)
	})
	t.Run("missing_blocker_kind_names_blocker_kind", func(t *testing.T) {
		f, _ := joinFake(t)
		code, msg := jland(t, jrec(t, map[string]any{"blocker_kind": nil}))
		wantRefused(t, "no blocker_kind", code, msg, "blocker_kind")
		wantNothingWritten(t, "no blocker_kind", f)
	})
	t.Run("missing_receipt_id_names_receipt_id", func(t *testing.T) {
		f, _ := joinFake(t)
		code, msg := jland(t, jrec(t, map[string]any{"receipt_id": nil}))
		wantRefused(t, "no receipt_id", code, msg, "receipt_id")
		wantNothingWritten(t, "no receipt_id", f)
	})
}

// regression: a verify-outcome record named no delivered change, and a stated one was never read back from the forge.
func TestOutcomeRecord_PRsDereference(t *testing.T) {
	t.Run("not_found_refused", func(t *testing.T) {
		f, _ := joinFake(t)
		f.prs = map[string]*deskkit.PullRequest{}
		code, msg := jland(t, jrec(t, nil))
		wantRefused(t, "missing PR", code, msg, "exists", "prs[0]")
		wantNothingWritten(t, "missing PR", f)
	})
	t.Run("unmerged_refused", func(t *testing.T) {
		f, _ := joinFake(t)
		f.prs[jRegister+"#7"].Merged, f.prs[jRegister+"#7"].MergedAt = false, ""
		code, msg := jland(t, jrec(t, nil))
		wantRefused(t, "unmerged PR", code, msg, "merged")
		wantNothingWritten(t, "unmerged PR", f)
	})
	t.Run("merge_sha_mismatch_refused", func(t *testing.T) {
		f, _ := joinFake(t)
		other := strings.Repeat("b", 40)
		code, msg := jland(t, jrec(t, map[string]any{"prs": []any{jpr(jRegister, 7, other)}}))
		wantRefused(t, "wrong merge sha", code, msg, "merge commit")
		wantNothingWritten(t, "wrong merge sha", f)
	})
	t.Run("same_repo_not_ancestor_refused", func(t *testing.T) {
		f, _ := joinFake(t)
		f.compareStatus = map[string]string{jRegister + ":" + jMerge + "..." + jSHA: "diverged"}
		code, msg := jland(t, jrec(t, nil))
		wantRefused(t, "not an ancestor", code, msg, "ancestor")
		wantNothingWritten(t, "not an ancestor", f)
		if got := strings.Join(f.joinReads, "|"); !strings.Contains(got, "CompareRefs "+jRegister+" "+jMerge+"..."+jSHA) {
			t.Errorf("same-repo ancestry was not read against the record's sha: %s", got)
		}
	})
	t.Run("cross_repo_checked_against_default_tip", func(t *testing.T) {
		f, _ := joinFake(t)
		f.prs[jOtherRepo+"#9"] = &deskkit.PullRequest{URL: prURL(jOtherRepo, 9), Merged: true, MergeCommitSHA: jMerge, BaseRef: "trunk"}
		f.defaultBranches = map[string]string{jOtherRepo: "trunk"}
		code, msg := jland(t, jrec(t, map[string]any{"prs": []any{jpr(jOtherRepo, 9, jMerge)}}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		got := strings.Join(f.joinReads, "|")
		if !strings.Contains(got, "CompareRefs "+jOtherRepo+" "+jMerge+"...trunk") || strings.Contains(got, "..."+jSHA) {
			t.Errorf("cross-repo ancestry must be checked against the other repo's default tip, not the record sha: %s", got)
		}
		f.compareStatus = map[string]string{jOtherRepo + ":" + jMerge + "...trunk": "behind"}
		f.writes, f.joinReads = nil, nil
		f.files = nil
		code, msg = jland(t, jrec(t, map[string]any{"prs": []any{jpr(jOtherRepo, 9, jMerge)}, "ts": "2026-10-06T14:21:00Z"}))
		wantRefused(t, "behind default tip", code, msg, "ancestor")
	})
	t.Run("forge_error_exit_6", func(t *testing.T) {
		f, _ := joinFake(t)
		f.prErr = errors.New("503 unavailable")
		code, msg := jland(t, jrec(t, nil))
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit = %d, want %d (stderr %q)", code, deskkit.ExitUnverifiable, msg)
		}
		wantNothingWritten(t, "forge error", f)
	})
	t.Run("empty_merge_commit_exit_6", func(t *testing.T) {
		f, _ := joinFake(t)
		f.prs[jRegister+"#7"].MergeCommitSHA = ""
		code, msg := jland(t, jrec(t, nil))
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit = %d, want %d (stderr %q)", code, deskkit.ExitUnverifiable, msg)
		}
		wantNothingWritten(t, "empty merge commit", f)
	})
	t.Run("gitlab_uses_base_ref_and_merged_at", func(t *testing.T) {
		f, _ := joinFake(t)
		forgeKindFn = func(string, string) (deskkit.ForgeKind, error) { return deskkit.ForgeGitLab, nil }
		f.compareErr = deskkit.Unverifiable("could-not-check: compare is deferred on this forge", nil)
		code, msg := jland(t, jrec(t, nil))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		if strings.Contains(strings.Join(f.joinReads, "|"), "CompareRefs") {
			t.Errorf("a GitLab-kind landing must not call CompareRefs: %v", f.joinReads)
		}
	})
	t.Run("gitlab_wrong_base_refused", func(t *testing.T) {
		f, _ := joinFake(t)
		forgeKindFn = func(string, string) (deskkit.ForgeKind, error) { return deskkit.ForgeGitLab, nil }
		f.prs[jRegister+"#7"].BaseRef = "release"
		code, msg := jland(t, jrec(t, nil))
		wantRefused(t, "wrong base", code, msg, "default branch")
		f.prs[jRegister+"#7"].BaseRef = "main"
		f.prs[jRegister+"#7"].MergedAt = "2026-10-06T15:00:00Z" // after the record's ts
		code, msg = jland(t, jrec(t, map[string]any{"ts": "2026-10-06T14:21:00Z"}))
		wantRefused(t, "merged after ts", code, msg, "after the record's ts")
		wantNothingWritten(t, "gitlab refusals", f)
	})
	t.Run("gitlab_compare_error_not_downgraded", func(t *testing.T) {
		f, _ := joinFake(t)
		f.compareErr = deskkit.Unverifiable("could-not-check: compare failed", nil)
		code, msg := jland(t, jrec(t, nil))
		if code != deskkit.ExitUnverifiable {
			t.Errorf("a GitHub-kind CompareRefs error must stay exit %d, got %d (stderr %q)", deskkit.ExitUnverifiable, code, msg)
		}
		wantNothingWritten(t, "compare error", f)
	})
	t.Run("legacy_verified_with_prs_dereferenced", func(t *testing.T) {
		f, _ := joinFake(t)
		f.prs = map[string]*deskkit.PullRequest{}
		code, msg := jland(t, jlegacy(t, map[string]any{"prs": []any{jpr(jRegister, 7, jMerge)}}))
		wantRefused(t, "legacy verified, missing PR", code, msg, "exists")
		wantNothingWritten(t, "legacy verified", f)
	})
	t.Run("input_withheld_entry_refused", func(t *testing.T) {
		for name, line := range map[string]string{
			"wake-v1": jrec(t, map[string]any{"prs": []any{map[string]any{"withheld": strings.Repeat("a", 64)}}}),
			"legacy":  jlegacy(t, map[string]any{"prs": []any{map[string]any{"withheld": strings.Repeat("a", 64)}}}),
		} {
			f, _ := joinFake(t)
			code, msg := jland(t, line)
			wantRefused(t, name, code, msg, "withheld")
			if len(f.joinReads) != 0 {
				t.Errorf("%s: forge reads for a refused input: %v", name, f.joinReads)
			}
			wantNothingWritten(t, name, f)
		}
	})
	t.Run("malformed_entry_no_read", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			entry map[string]any
			field string
		}{
			{"repo not owner/name", jpr("notowner", 7, jMerge), "repo"},
			{"number 0", jpr(jRegister, 0, jMerge), "number"},
			{"number -1", jpr(jRegister, -1, jMerge), "number"},
			{"short merge_sha", jpr(jRegister, 7, "abc123"), "merge_sha"},
			{"uppercase merge_sha", jpr(jRegister, 7, strings.ToUpper(jMerge)), "merge_sha"},
		} {
			for _, vis := range []string{"private", "public"} {
				f, adm := joinFake(t)
				f.visibility = vis
				code, msg := jland(t, jrec(t, map[string]any{"prs": []any{c.entry}}))
				wantRefused(t, c.name+"/"+vis, code, msg, c.field, "prs[0]")
				if len(f.joinReads) != 0 || *adm != 0 {
					t.Errorf("%s/%s: reads=%v admissions=%d, want none", c.name, vis, f.joinReads, *adm)
				}
				wantNothingWritten(t, c.name, f)
			}
		}
	})
	t.Run("not_admitted_no_read", func(t *testing.T) {
		f, _ := joinFake(t)
		verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) {
			return deskkit.VerifierReceipt{}, deskkit.Refused("refused: no attested run for this brief")
		}
		code, msg := jland(t, jrec(t, nil))
		wantRefused(t, "not admitted", code, msg, "attested")
		if len(f.joinReads) != 0 || len(f.reads) != 0 {
			t.Errorf("an unadmitted landing read the forge: joinReads=%v reads=%v", f.joinReads, f.reads)
		}
		wantNothingWritten(t, "not admitted", f)
	})
	t.Run("case_variant_key_no_read", func(t *testing.T) {
		line := jrec(t, nil)
		bad := map[string]string{
			"brief":        strings.Replace(line, "{", `{"Brief":"example-stream/15",`, 1),
			"ts":           strings.Replace(line, "{", `{"ts":"2099-01-01T00:00:00Z",`, 1),
			"prs":          strings.Replace(line, "{", `{"PRS":[],`, 1),
			"dispatch_ref": strings.Replace(strings.Replace(line, "{", `{"dispatch_ref":"`+jDispatch+`",`, 1), "{", `{"Dispatch_Ref":"`+jDispatch+`",`, 1),
		}
		legacy := jlegacy(t, nil)
		bad["legacy brief"] = strings.Replace(legacy, "{", `{"Brief":"example-stream/15",`, 1)
		bad["legacy ts"] = strings.Replace(legacy, "{", `{"ts":"2099-01-01T00:00:00Z",`, 1)
		for name, rec := range bad {
			for _, vis := range []string{"private", "public"} {
				f, adm := joinFake(t)
				f.visibility = vis
				code, msg := jland(t, rec)
				key := strings.Fields(strings.NewReplacer("legacy ", "").Replace(name))[0]
				wantRefused(t, name+"/"+vis, code, msg, key)
				if len(f.joinReads) != 0 || *adm != 0 || len(f.hits) != 0 {
					t.Errorf("%s/%s: reads=%v admissions=%d hits=%v, want none", name, vis, f.joinReads, *adm, f.hits)
				}
				wantNothingWritten(t, name, f)
			}
		}
	})
	t.Run("rewritten_target_refused", func(t *testing.T) {
		old := redactOutcomeFn
		t.Cleanup(func() { redactOutcomeFn = old })
		for _, c := range []struct{ name, key, val string }{
			{"other brief", "brief", "example-stream/15"},
			{"other ts", "ts", "2026-10-06T14:30:00Z"},
		} {
			f, _ := joinFake(t)
			redactOutcomeFn = func(raw []byte, _ deskkit.JoinInput, _ joinFacts) ([]byte, error) {
				m := map[string]json.RawMessage{}
				if err := json.Unmarshal(raw, &m); err != nil {
					return nil, err
				}
				m[c.key], _ = json.Marshal(c.val)
				return json.Marshal(m)
			}
			code, msg := jland(t, jrec(t, nil))
			wantRefused(t, c.name, code, msg, "rewritten")
			wantNothingWritten(t, c.name, f)
		}
	})
	t.Run("valid_accepted", func(t *testing.T) {
		f, _ := joinFake(t)
		code, msg := jland(t, jrec(t, nil))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		if !strings.Contains(landed(t, f), jMerge) {
			t.Errorf("the dereferenced PR is not in the landed record")
		}
	})
}

// regression: a verified record under verify-wake-v1 could land without naming what it delivered.
func TestOutcomeRecord_VerifiedRules(t *testing.T) {
	t.Run("legacy_verified_no_new_requirement", func(t *testing.T) {
		f, _ := joinFake(t)
		code, msg := jland(t, jlegacy(t, nil))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		if len(f.joinReads) != 0 {
			t.Errorf("a legacy verified record with no join keys read the forge: %v", f.joinReads)
		}
	})
	t.Run("wake_v1_verified_without_prs_refused", func(t *testing.T) {
		f, _ := joinFake(t)
		code, msg := jland(t, jrec(t, map[string]any{"outcome": "verified", "prs": nil}))
		wantRefused(t, "verified, no prs", code, msg, "prs")
		wantNothingWritten(t, "verified, no prs", f)
	})
	t.Run("wake_v1_verified_complete_accepted", func(t *testing.T) {
		f, _ := joinFake(t)
		code, msg := jland(t, jrec(t, map[string]any{"outcome": "verified", "rows_passed": 1, "rows_total": 1}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		landed(t, f)
	})
}

// regression: a record landed in a non-private register named private repositories and the clear dispatch_ref.
func TestOutcomeRecord_NonPrivateRedaction(t *testing.T) {
	hiddenPR := func(f *fakeForge, repo string, n int) {
		f.prs[repo+"#"+fmt.Sprint(n)] = &deskkit.PullRequest{URL: prURL(strings.ToLower(repo), n), Merged: true, MergeCommitSHA: jMerge, BaseRef: "main", MergedAt: "2026-10-06T10:00:00Z"}
		f.defaultBranches = map[string]string{strings.ToLower(repo): "main"}
	}
	hiddenRec := func(t *testing.T, repo string, over map[string]any) string {
		o := map[string]any{"prs": []any{jpr(repo, 9, jMerge)}}
		for k, v := range over {
			o[k] = v
		}
		return jrec(t, o)
	}
	wantWithheld := func(t *testing.T, name, bytes, repo string) {
		t.Helper()
		digest := sha256hex(strings.ToLower(repo) + "#9@" + jMerge)
		if !strings.Contains(bytes, `"withheld":"`+digest+`"`) {
			t.Errorf("%s: landed bytes carry no withheld digest %s: %s", name, digest, bytes)
		}
		if strings.Contains(strings.ToLower(bytes), strings.ToLower(repo)) {
			t.Errorf("%s: landed bytes name %s in clear: %s", name, repo, bytes)
		}
	}

	t.Run("private_pr_on_public_register_withheld", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		f.visibilityByRepo = map[string]string{jHiddenA: "private"}
		hiddenPR(f, jHiddenA, 9)
		code, msg := jland(t, hiddenRec(t, jHiddenA, nil))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		wantWithheld(t, "public register", landed(t, f), jHiddenA)
	})
	t.Run("internal_register_redacts", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "internal"
		hiddenPR(f, jHiddenA, 9)
		code, msg := jland(t, hiddenRec(t, jHiddenA, map[string]any{"dispatch_ref": jDispatch}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		b := landed(t, f)
		wantWithheld(t, "internal register", b, jHiddenA)
		if strings.Contains(b, "dispatch_ref") {
			t.Errorf("an internal register kept dispatch_ref: %s", b)
		}
	})
	t.Run("unrecognised_visibility_spelling_redacts", func(t *testing.T) {
		for _, spelling := range []string{"hidden", "Confidential", "", "public-ish", "publicc"} {
			f, _ := joinFake(t)
			f.visibility = "public"
			f.visibilityByRepo = map[string]string{jHiddenA: spelling}
			hiddenPR(f, jHiddenA, 9)
			code, msg := jland(t, hiddenRec(t, jHiddenA, nil))
			if code != deskkit.ExitOK {
				t.Fatalf("%q: exit = %d (stderr %q)", spelling, code, msg)
			}
			wantWithheld(t, "visibility "+spelling, landed(t, f), jHiddenA)
		}
		// An unrecognised REGISTER spelling is not private either: the dispatch_ref is dropped.
		f, _ := joinFake(t)
		f.visibility = "Privatee"
		code, msg := jland(t, jrec(t, map[string]any{"dispatch_ref": jDispatch}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		if b := landed(t, f); strings.Contains(b, "dispatch_ref") {
			t.Errorf("an unrecognised register spelling kept a dispatch_ref form: %s", b)
		}
	})
	t.Run("padded_uppercase_private_register_keeps_clear", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "  PRIVATE \n"
		hiddenPR(f, jHiddenA, 9)
		code, msg := jland(t, hiddenRec(t, jHiddenA, nil))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		if b := landed(t, f); !strings.Contains(b, jHiddenA) || strings.Contains(b, "withheld") {
			t.Errorf("a private register must keep prs in clear: %s", b)
		}
	})
	t.Run("entry_repo_internal_withheld", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		f.visibilityByRepo = map[string]string{jHiddenA: "internal"}
		hiddenPR(f, jHiddenA, 9)
		code, msg := jland(t, hiddenRec(t, jHiddenA, nil))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		wantWithheld(t, "internal entry", landed(t, f), jHiddenA)
	})
	t.Run("register_visibility_read_error_exit_6", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visErrByRepo = map[string]error{jRegister: errors.New("503 unavailable")}
		code, msg := jland(t, jrec(t, nil))
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit = %d, want %d (stderr %q)", code, deskkit.ExitUnverifiable, msg)
		}
		wantNothingWritten(t, "register read error", f)
	})
	t.Run("entry_visibility_read_error_exit_6", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		f.visErrByRepo = map[string]error{jHiddenA: errors.New("503 unavailable")}
		hiddenPR(f, jHiddenA, 9)
		code, msg := jland(t, hiddenRec(t, jHiddenA, nil))
		if code != deskkit.ExitUnverifiable {
			t.Errorf("exit = %d, want %d (stderr %q)", code, deskkit.ExitUnverifiable, msg)
		}
		wantNothingWritten(t, "entry read error", f)
	})
	t.Run("dispatch_ref_dropped_and_not_hashed", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		code, msg := jland(t, jrec(t, map[string]any{"dispatch_ref": jDispatch}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		b := landed(t, f)
		if strings.Contains(b, jDispatch) || strings.Contains(b, sha256hex(jDispatch)) || strings.Contains(b, "dispatch_ref_sha256") {
			t.Errorf("a non-private register must neither keep nor hash the dispatch_ref: %s", b)
		}
	})
	t.Run("private_keeps_clear_prs", func(t *testing.T) {
		f, _ := joinFake(t)
		hiddenPR(f, jHiddenA, 9)
		f.prs[jRegister+"#7"] = &deskkit.PullRequest{URL: prURL(jRegister, 7), Merged: true, MergeCommitSHA: jMerge, BaseRef: "main"}
		code, msg := jland(t, jrec(t, map[string]any{"prs": []any{jpr(jRegister, 7, jMerge), jpr(jHiddenA, 9, jMerge)}}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		b := landed(t, f)
		if !strings.Contains(b, jHiddenA) || !strings.Contains(b, jRegister) || strings.Contains(b, "withheld") {
			t.Errorf("a private register keeps every prs entry in clear: %s", b)
		}
		for _, r := range f.joinReads {
			if strings.HasPrefix(r, "RepoVisibility "+jHiddenA) {
				t.Errorf("a private register needs no per-entry visibility read: %v", f.joinReads)
			}
		}
	})
	t.Run("private_dispatch_digest_only", func(t *testing.T) {
		for name, line := range map[string]string{
			"wake-v1": jrec(t, map[string]any{"dispatch_ref": jDispatch}),
			"legacy":  jlegacy(t, map[string]any{"dispatch_ref": jDispatch}),
		} {
			f, _ := joinFake(t)
			code, msg := jland(t, line)
			if code != deskkit.ExitOK {
				t.Fatalf("%s: exit = %d (stderr %q)", name, code, msg)
			}
			b := landed(t, f)
			var m map[string]any
			if err := json.Unmarshal([]byte(b), &m); err != nil {
				t.Fatal(err)
			}
			if m["dispatch_ref_sha256"] != sha256hex(jDispatch) {
				t.Errorf("%s: dispatch_ref_sha256 = %v, want the sha256 of the full ref", name, m["dispatch_ref_sha256"])
			}
			if _, has := m["dispatch_ref"]; has || strings.Contains(b, jDispatch) {
				t.Errorf("%s: the clear dispatch_ref was written: %s", name, b)
			}
		}
	})
	t.Run("withheld_uses_canonical_repo", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		f.visibilityByRepo = map[string]string{jHiddenA: "private"}
		// The record spells the repo in mixed case; the forge's URL spells it canonically.
		f.prs["Example-Org/Hidden-Lane#9"] = &deskkit.PullRequest{URL: prURL(jHiddenA, 9), Merged: true, MergeCommitSHA: jMerge, BaseRef: "main"}
		f.defaultBranches = map[string]string{jHiddenA: "main"}
		code, msg := jland(t, jrec(t, map[string]any{"prs": []any{jpr("Example-Org/Hidden-Lane", 9, jMerge)}}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		wantWithheld(t, "canonical spelling", landed(t, f), jHiddenA)
	})
	t.Run("written_bytes_contain_no_clear_private_repo", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		f.visibilityByRepo = map[string]string{jHiddenA: "private", "example-org/second-lane": "private"}
		hiddenPR(f, jHiddenA, 9)
		f.prs["example-org/second-lane#3"] = &deskkit.PullRequest{URL: prURL("example-org/second-lane", 3), Merged: true, MergeCommitSHA: jMerge, BaseRef: "main"}
		f.defaultBranches["example-org/second-lane"] = "main"
		code, msg := jland(t, jrec(t, map[string]any{
			"prs":          []any{jpr(jHiddenA, 9, jMerge), jpr("example-org/second-lane", 3, jMerge)},
			"dispatch_ref": jDispatch,
		}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		for _, w := range f.writes {
			all := w.File + string(w.Content) + w.Message
			for _, name := range []string{"hidden-lane", "second-lane", jDispatch} {
				if strings.Contains(all, name) {
					t.Errorf("the write (path, bytes, message) names %q in clear: %s", name, all)
				}
			}
		}
	})
	t.Run("land_as_change_bytes_redacted", func(t *testing.T) {
		f, _ := joinFake(t)
		forgeKindFn = func(string, string) (deskkit.ForgeKind, error) { return deskkit.ForgeGitLab, nil }
		f.visibility = "public"
		f.visibilityByRepo = map[string]string{jHiddenA: "private"}
		f.defaultBranch = "main" // the closed default branch: the record lands as a draft change
		hiddenPR(f, jHiddenA, 9)
		code, msg := jland(t, jrec(t, map[string]any{"prs": []any{jpr(jHiddenA, 9, jMerge)}, "dispatch_ref": jDispatch}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		if len(f.changes) != 1 || len(f.writes) != 1 {
			t.Fatalf("changes=%d writes=%d, want one draft change carrying one write", len(f.changes), len(f.writes))
		}
		all := f.writes[0].File + string(f.writes[0].Content) + f.writes[0].Message + f.changes[0].Title + f.changes[0].Body
		if strings.Contains(all, "hidden-lane") || strings.Contains(all, jDispatch) {
			t.Errorf("the land-as-change bytes name a private repo or the dispatch_ref: %s", all)
		}
		if !strings.Contains(string(f.writes[0].Content), "withheld") {
			t.Errorf("the land-as-change record is not the redacted one: %s", f.writes[0].Content)
		}
	})
	t.Run("redacted_before_outbound_scan", func(t *testing.T) {
		// The register must be CONFIGURED public (the scan keys on the roster, not the live read).
		const pubRegister = "example-org/proposals"
		t.Setenv(deskkit.EnvWithheldIdentifiers, "hidden-lane")
		deskkit.ReloadConfig()
		t.Cleanup(deskkit.ReloadConfig)
		f, _ := joinFake(t)
		f.visibility = "public"
		f.visibilityByRepo = map[string]string{jHiddenA: "private"}
		hiddenPR(f, jHiddenA, 9)
		line := hiddenRec(t, jHiddenA, nil)
		code, msg := jlandAt(t, pubRegister, line)
		if code != deskkit.ExitOK {
			t.Fatalf("a record naming a withheld repo must be rewritten, not refused: exit = %d (stderr %q)", code, msg)
		}
		landed(t, f)
		// Control: with the redaction switched off the very same record IS refused by the scan, so
		// the pass above is the redaction's doing and the token map is live.
		old := redactOutcomeFn
		redactOutcomeFn = func(raw []byte, _ deskkit.JoinInput, _ joinFacts) ([]byte, error) { return raw, nil }
		t.Cleanup(func() { redactOutcomeFn = old })
		g, _ := joinFake(t)
		g.visibility = "public"
		g.visibilityByRepo = map[string]string{jHiddenA: "private"}
		hiddenPR(g, jHiddenA, 9)
		if code, _ := jlandAt(t, pubRegister, line); code == deskkit.ExitOK || len(g.writes) != 0 {
			t.Errorf("control: the unredacted record landed (exit %d); the token map is not live", code)
		}
	})
	t.Run("legacy_verified_non_public_pr_withheld", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		f.visibilityByRepo = map[string]string{jHiddenA: "private"}
		hiddenPR(f, jHiddenA, 9)
		code, msg := jland(t, jlegacy(t, map[string]any{"prs": []any{jpr(jHiddenA, 9, jMerge)}}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		wantWithheld(t, "legacy verified", landed(t, f), jHiddenA)
	})
	t.Run("legacy_verified_dispatch_ref_dropped", func(t *testing.T) {
		f, _ := joinFake(t)
		f.visibility = "public"
		code, msg := jland(t, jlegacy(t, map[string]any{"dispatch_ref": jDispatch}))
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d (stderr %q)", code, msg)
		}
		if b := landed(t, f); strings.Contains(b, "dispatch_ref") || strings.Contains(b, sha256hex(jDispatch)) {
			t.Errorf("a non-private register kept a dispatch_ref form: %s", b)
		}
	})
}

// regression: the run metadata (tier, window, rows, dispatch_ref) was never shape-checked on a landing.
func TestOutcomeRecord_RunMetadata(t *testing.T) {
	refuse := func(t *testing.T, name string, line, field string) {
		t.Helper()
		f, adm := joinFake(t)
		code, msg := jland(t, line)
		wantRefused(t, name, code, msg, field)
		if len(f.joinReads) != 0 {
			t.Errorf("%s: forge reads for a malformed record: %v", name, f.joinReads)
		}
		_ = adm
		wantNothingWritten(t, name, f)
	}
	t.Run("missing_verifier_tier_refused", func(t *testing.T) {
		refuse(t, "no tier", jrec(t, map[string]any{"verifier_tier": nil}), "verifier_tier")
	})
	t.Run("vendor_model_name_refused", func(t *testing.T) {
		refuse(t, "model name", jrec(t, map[string]any{"verifier_tier": "gpt-6-astra"}), "verifier_tier")
	})
	t.Run("tier_disagreeing_with_admitted_binding_refused", func(t *testing.T) {
		f, _ := joinFake(t)
		inner := verifierEvidenceAdmissionFn
		verifierEvidenceAdmissionFn = func(a, b, target string, fg deskkit.Forge) (deskkit.VerifierReceipt, error) {
			r, err := inner(a, b, target, fg)
			r.Binding.Tier = "any"
			return r, err
		}
		code, msg := jland(t, jrec(t, map[string]any{"verifier_tier": "strong"}))
		wantRefused(t, "tier disagrees", code, msg, "verifier_tier")
		if len(f.joinReads) != 0 {
			t.Errorf("the tier cross-check must precede every record-driven read: %v", f.joinReads)
		}
		wantNothingWritten(t, "tier disagrees", f)
	})
	t.Run("started_after_finished_refused", func(t *testing.T) {
		refuse(t, "started>finished", jrec(t, map[string]any{"started": "2026-10-06T14:11:00Z"}), "started")
		refuse(t, "finished>ts", jrec(t, map[string]any{"finished": "2026-10-06T14:30:00Z"}), "finished")
	})
	t.Run("row_results_count_mismatch_refused", func(t *testing.T) {
		rr := []any{map[string]any{"row": 1, "result": "fail"}, map[string]any{"row": 2, "result": "pass"}}
		refuse(t, "count", jrec(t, map[string]any{"row_results": rr, "rows_total": 1}), "row_results")
		refuse(t, "pass count", jrec(t, map[string]any{"row_results": rr, "rows_total": 2, "rows_passed": 0}), "row_results")
	})
	t.Run("unknown_result_refused", func(t *testing.T) {
		rr := []any{map[string]any{"row": 1, "result": "green"}}
		refuse(t, "result", jrec(t, map[string]any{"row_results": rr}), "result")
	})
	t.Run("dispatch_ref_bad_shape_refused", func(t *testing.T) {
		tail := "@20261006T141502Z.3fa9c01b7d2e"
		bad := []string{
			"assay--x--1", // no nonce
			"assay--x--1@20261006T141502Z.3fa9c01b7d2",      // 11-char nonce
			"assay--x--1@20261006T141502Z.3FA9C01B7D2E",     // uppercase nonce
			"assay--x--1@2026-10-06T14:15:02Z.3fa9c01b7d2e", // extended-form timestamp
			"assay@x--1" + tail,                             // '@' in the prefix
			"assay x--1" + tail,                             // space in the prefix
			"assay\x01x--1" + tail,                          // control byte in the prefix
			strings.Repeat("a", 227) + tail,                 // 227-byte prefix
		}
		for _, ref := range bad {
			for _, vis := range []string{"private", "public"} {
				for _, mk := range []func(testing.TB, map[string]any) string{jrec, jlegacy} {
					f, adm := joinFake(t)
					f.visibility = vis
					code, msg := jland(t, mk(t, map[string]any{"dispatch_ref": ref}))
					wantRefused(t, fmt.Sprintf("%q/%s", ref, vis), code, msg, "dispatch_ref")
					if len(f.joinReads) != 0 || *adm != 0 {
						t.Errorf("%q: reads=%v admissions=%d, want none", ref, f.joinReads, *adm)
					}
					wantNothingWritten(t, ref, f)
				}
			}
		}
		f, _ := joinFake(t)
		code, msg := jland(t, jrec(t, map[string]any{"dispatch_ref": strings.Repeat("a", 226) + tail}))
		if code != deskkit.ExitOK {
			t.Errorf("a 226-byte prefix must be accepted: exit = %d (stderr %q)", code, msg)
		}
		_ = f
	})
	t.Run("input_dispatch_digest_refused", func(t *testing.T) {
		refuse(t, "digest", jrec(t, map[string]any{"dispatch_ref_sha256": strings.Repeat("a", 64)}), "dispatch_ref_sha256")
		refuse(t, "legacy digest", jlegacy(t, map[string]any{"dispatch_ref_sha256": strings.Repeat("a", 64)}), "dispatch_ref_sha256")
	})
	t.Run("valid_with_and_without_optionals", func(t *testing.T) {
		f, _ := joinFake(t)
		code, msg := jland(t, jrec(t, nil))
		if code != deskkit.ExitOK {
			t.Fatalf("without optionals: exit = %d (stderr %q)", code, msg)
		}
		g, _ := joinFake(t)
		rr := []any{map[string]any{"row": 1, "result": "fail"}}
		code, msg = jland(t, jrec(t, map[string]any{"row_results": rr, "dispatch_ref": jDispatch, "ts": "2026-10-06T14:21:00Z"}))
		if code != deskkit.ExitOK {
			t.Fatalf("with optionals: exit = %d (stderr %q)", code, msg)
		}
		_, _ = f, g
		landed(t, g)
	})
}
