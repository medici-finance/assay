package evaluation

import (
	"bytes"
	"strings"
	"testing"
)

// witness_test.go — the witness-v2 reader's library tests (statusgen/36 Verify
// rows 1–4, 7, 8, 17–20, 22). Every case builds one in-memory repository with
// witness_fake_test.go's fixture and judges it through EvaluateCoverage, the
// public entry point, so each refusal is shown where a flip reads it.

// v2case is one fixture variant and the verdict it must reach.
type v2case struct {
	name string
	mut  func(fx *v2fx)
	gap  string   // "" means a release
	has  []string // substrings the refusal reason names
}

func runV2Cases(t *testing.T, cases []v2case) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newV2fx(t)
			if tc.mut != nil {
				tc.mut(fx)
			}
			c := fx.claim()
			if tc.gap == "" {
				wantRelease(t, tc.name, c)
				return
			}
			wantGap(t, tc.name, c, tc.gap, tc.has...)
		})
	}
}

func setFile(p, content string) func(map[string]fakeEntry, map[string][]string) {
	return func(tree map[string]fakeEntry, _ map[string][]string) {
		tree[p] = fakeEntry{"100644", content}
	}
}

func delFile(p string) func(map[string]fakeEntry, map[string][]string) {
	return func(tree map[string]fakeEntry, _ map[string][]string) { delete(tree, p) }
}

// regression: #2026
func TestWitnessV2DigestRelease(t *testing.T) {
	t.Run("squash-orphaned witness commit releases with its informational revision", func(t *testing.T) {
		fx := newV2fx(t)
		c := fx.claim()
		wantRelease(t, "default", c)
		if _, ok := fx.repo.commits[fxRunCommit]; ok {
			t.Fatal("fixture: the run commit must be absent from the object store")
		}
		if c.Revision != fxRunCommit {
			t.Errorf("the released claim's revision is %q, want the record's informational.commit %q", c.Revision, fxRunCommit)
		}
		if c.Revision == "[w2]" {
			t.Error("the claim's revision is the cell's [w2] marker")
		}
	})
	for _, commit := range []string{"0123abc", "zz" + strings.Repeat("0", 38), ""} {
		fx := newV2fx(t)
		fx.rec = func(r *WitnessRecord) { r.Informational.Commit = commit }
		c := fx.claim()
		wantRelease(t, "informational commit "+commit, c)
		if c.Revision != "" {
			t.Errorf("informational commit %q: revision %q, want empty", commit, c.Revision)
		}
	}
	makefile := func(fx *v2fx) {
		fx.files["Makefile"] = fakeEntry{"100644", "all:\n"}
		fx.deps = &DependencyManifest{Complete: true, Deps: []WorkInputDep{{Kind: DepBuild, Path: "Makefile"}}}
	}
	env := func(rec, cur string) func(*v2fx) {
		return func(fx *v2fx) {
			fx.deps = &DependencyManifest{Complete: true, Deps: []WorkInputDep{{Kind: DepEnvironment, Name: "GOFLAGS", Recorded: rec, Current: cur}}}
		}
	}
	wholeTree := func(fx *v2fx) { fx.declared, fx.found = nil, false }
	runV2Cases(t, []v2case{
		{name: "github squash landing", mut: func(fx *v2fx) { fx.form = "github-squash" }},
		{name: "gitlab merge landing", mut: func(fx *v2fx) { fx.form = "gitlab-merge" }},
		{name: "unrelated later change", mut: func(fx *v2fx) { fx.atR = setFile("README.md", "changed\n") }},
		{name: "build dependency unchanged", mut: makefile},
		{name: "build dependency changed", mut: func(fx *v2fx) { makefile(fx); fx.atR = setFile("Makefile", "all: x\n") }, gap: GapInputChanged, has: []string{"Makefile"}},
		{name: "environment dependency unchanged", mut: env("a", "a")},
		{name: "environment dependency changed", mut: env("a", "b"), gap: GapEnvironmentChanged, has: []string{"GOFLAGS"}},
		{name: "environment dependency unknown", mut: env("a", ""), gap: GapUnestablished},
		{name: "whole-tree unchanged", mut: wholeTree},
		{name: "whole-tree bookkeeping-only change", mut: func(fx *v2fx) { wholeTree(fx); fx.atR = setFile("STATUS.md", "regen\n") }},
		{name: "whole-tree changed", mut: func(fx *v2fx) { wholeTree(fx); fx.atR = setFile("README.md", "changed\n") }, gap: GapInputChanged, has: []string{"whole tree"}},
	})
}

// regression: #2026
func TestWitnessV2ByteChange(t *testing.T) {
	runV2Cases(t, []v2case{
		{name: "unaltered"},
		{name: "one byte of a declared input", mut: func(fx *v2fx) {
			fx.atR = setFile("tools/x/pkg/a.go", "package pkg\n\nfunc A() int { return 2 }\n")
		}, gap: GapInputChanged, has: []string{"tools/x/pkg/a.go"}},
		{name: "one byte of another declared input", mut: func(fx *v2fx) { fx.atR = setFile("tools/x/pkg/a_test.go", "package pkg \n") }, gap: GapInputChanged, has: []string{"tools/x/pkg/a_test.go"}},
		{name: "a file outside the inputs", mut: func(fx *v2fx) { fx.atR = setFile("other/main.go", "package main\n") }},
	})
	// The refusal is wrong-revision, the result a flip reads as stale.
	fx := newV2fx(t)
	fx.atR = setFile("tools/x/pkg/a.go", "package pkg\n// edited\n")
	if c := fx.claim(); c.Result != ResultWrongRevision {
		t.Errorf("a changed input reports %s, want %s", c.Result, ResultWrongRevision)
	}
}

// regression: #2026
func TestWitnessV2MissingDigest(t *testing.T) {
	otherID := strings.Repeat("ab", 32)
	runV2Cases(t, []v2case{
		{name: "unaltered"},
		{name: "the cell names a record that does not exist", mut: func(fx *v2fx) {
			fx.cell = func(c string) string { return c[:strings.Index(c, "rec:")] + "rec:" + otherID }
		}, gap: GapRecordMissing, has: []string{otherID}},
		{name: "the cell names no record", mut: func(fx *v2fx) {
			fx.cell = func(c string) string { return strings.TrimSpace(c[:strings.Index(c, "rec:")]) }
		}, gap: GapRecordMissing},
		{name: "an abbreviated record id", mut: func(fx *v2fx) {
			fx.cell = func(c string) string { return c[:strings.Index(c, "rec:")+4+12] }
		}, gap: GapRecordMismatch},
		{name: "a malformed input digest", mut: func(fx *v2fx) {
			fx.raw = func(b []byte) []byte {
				oid := fakeBlobID(fx.files["tools/x/pkg/a.go"].content)
				return bytes.Replace(b, []byte(`"`+oid+`"`), []byte(`"`+oid[:39]+`"`), 1)
			}
		}, gap: GapRecordMismatch, has: []string{"tools/x/pkg/a.go"}},
		{name: "an input with no digest", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) {
				delete(r.Inputs, "tools/x/pkg/a.go")
				delete(r.Attributes, "tools/x/pkg/a.go")
			}
		}, gap: GapInputUndeclaredAtRecord, has: []string{"tools/x/pkg/a.go"}},
		{name: "a record whose id does not match the cell", mut: func(fx *v2fx) {
			fx.feature = func(tree map[string]fakeEntry) {
				tree[WitnessRecordPath(fxBriefID, otherID)] = tree[fx.recordPath]
			}
			fx.cell = func(c string) string { return c[:strings.Index(c, "rec:")] + "rec:" + otherID }
		}, gap: GapRecordMismatch, has: []string{otherID}},
		{name: "an unknown field", mut: func(fx *v2fx) {
			fx.raw = func(b []byte) []byte { return bytes.Replace(b, []byte(`{`), []byte(`{"extra":1,`), 1) }
		}, gap: GapRecordMismatch, has: []string{"extra"}},
		{name: "a duplicate key", mut: func(fx *v2fx) {
			fx.raw = func(b []byte) []byte { return bytes.Replace(b, []byte(`{`), []byte(`{"row":"1",`), 1) }
		}, gap: GapRecordMismatch},
		{name: "not JSON", mut: func(fx *v2fx) { fx.raw = func(b []byte) []byte { return b[:len(b)/2] } }, gap: GapRecordMismatch},
		{name: "a pending change request", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.CR = "pending" }
			fx.cell = func(c string) string { return strings.Replace(c, "cr:12", "cr:pending", 1) }
		}, gap: GapCRUnbound, has: []string{"pending"}},
		{name: "no change request", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.CR = "" }
			fx.cell = func(c string) string { return strings.Replace(c, "cr:12 ", "", 1) }
		}, gap: GapCRUnbound},
		{name: "a malformed cell", mut: func(fx *v2fx) {
			fx.cell = func(c string) string { return strings.Replace(c, "cr:12", "cr:12 extra", 1) }
		}, gap: GapRecordMismatch},
	})
}

// regression: #2026
func TestWitnessSchemaUnknown(t *testing.T) {
	for _, marker := range []string{"[w3]", "[w]", "[w2x]", "[W2]", "[]"} {
		fx := newV2fx(t)
		fx.cell = func(c string) string { return strings.Replace(c, "[w2]", marker, 1) }
		c := fx.claim()
		wantGap(t, marker, c, GapSchemaUnknown, marker)
		if c.Revision != "" {
			t.Errorf("%s: an unknown schema reports revision %q", marker, c.Revision)
		}
	}
	// A cell with no Runner text is read from its tree token, with the same
	// three-way split.
	for marker, gap := range map[string]string{"[w3]": GapSchemaUnknown, "[w2]": GapRecordMissing} {
		fx := newV2fx(t)
		fx.wit = func(w *Witness) { w.Runner, w.Tree = "", marker }
		wantGap(t, "tree token "+marker, fx.claim(), gap)
	}
	// A v1 cell is never read as v2 or as unknown: no witness- gap.
	fx := newV2fx(t)
	fx.wit = func(w *Witness) { w.Runner, w.Tree = "verifier-app @ 0123456789ab", "0123456789ab" }
	if c := fx.claim(); WitnessGap(c.Reason) != "" {
		t.Errorf("a v1 cell reached the v2 reader: %s", c.Reason)
	}
	for _, cell := range []string{"x @ [w3] cr:1 rec:a", "x @[w]", "x @ [w2x] (note)"} {
		if got := ParseRunnerCell(cell); got.Schema != WitnessSchemaUnknown {
			t.Errorf("ParseRunnerCell(%q).Schema = %d, want unknown", cell, got.Schema)
		}
	}
	for _, cell := range []string{"x @ 0123abc", "x @ 0123abc+dirty", "x"} {
		if got := ParseRunnerCell(cell); got.Schema != WitnessSchemaV1 {
			t.Errorf("ParseRunnerCell(%q).Schema = %d, want v1", cell, got.Schema)
		}
	}
}

// regression: #2026
func TestWitnessImplicitInputs(t *testing.T) {
	bump := func(old, new string) func(*v2fx) {
		return func(fx *v2fx) {
			fx.atR = func(tree map[string]fakeEntry, _ map[string][]string) {
				tree[WitnessPinFile] = fakeEntry{"100644", strings.Replace(fxPins, old, new, 1)}
			}
		}
	}
	runV2Cases(t, []v2case{
		{name: "unaltered"},
		{name: "go.sum on the input's module path", mut: func(fx *v2fx) { fx.atR = setFile("tools/x/go.sum", "example.test/dep v1.0.1 h1:cccc\n") }, gap: GapInputChanged, has: []string{"tools/x/go.sum"}},
		{name: "go.mod on the input's module path", mut: func(fx *v2fx) { fx.atR = setFile("tools/x/go.mod", "module example.test/x\n\ngo 1.26\n") }, gap: GapInputChanged, has: []string{"tools/x/go.mod"}},
		{name: "go.work at the root", mut: func(fx *v2fx) { fx.atR = setFile("go.work", "go 1.26\n") }, gap: GapInputChanged, has: []string{"go.work"}},
		{name: "the harness digest", mut: bump("sha256:1111", "sha256:2222"), gap: GapInputChanged, has: []string{WitnessPinFile}},
		{name: "the statusgen tag", mut: bump("statusgen/v1.0.59", "statusgen/v1.0.60"), gap: GapInputChanged, has: []string{WitnessPinFile}},
		{name: "an eol attribute on an input", mut: func(fx *v2fx) {
			fx.atR = func(_ map[string]fakeEntry, attrs map[string][]string) {
				attrs["tools/x/pkg/a.go"] = []string{"text: set", "eol: crlf"}
			}
		}, gap: GapInputChanged, has: []string{"tools/x/pkg/a.go", "attributes"}},
		{name: "a recorded tool the pins disagree with", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.Tools["statusgen"] = "statusgen/v1.0.58" }
		}, gap: GapToolChanged, has: []string{"statusgen"}},
		{name: "a new requirements file on the input's path", mut: func(fx *v2fx) { fx.atR = setFile("tools/x/requirements-dev.txt", "x\n") }, gap: GapInputUndeclaredAtRecord, has: []string{"tools/x/requirements-dev.txt"}},
		{name: "go.sum in an unrelated module", mut: func(fx *v2fx) { fx.atR = setFile("other/go.sum", "example.test/odep v1.0.1 h1:dddd\n") }},
		{name: "go.mod in an unrelated module", mut: func(fx *v2fx) { fx.atR = setFile("other/go.mod", "module example.test/other\n\ngo 1.26\n") }},
	})
}

// regression: #2026
func TestWitnessEnvRerunOwed(t *testing.T) {
	host := func(fx *v2fx) {
		fx.rec = func(r *WitnessRecord) { r.RunnerClass, r.Network = "host", "host" }
	}
	hostCmd := func(cmd, shell string) func(*v2fx) {
		return func(fx *v2fx) {
			fx.row.Command, fx.row.Shell = cmd, shell
			host(fx)
		}
	}
	backfill := func(fx *v2fx) {
		fx.rec = func(r *WitnessRecord) {
			r.Origin, r.SourceCR, r.SourceCell = "backfill", "7", "verifier-app @ 0123456789ab"
			r.RunnerClass, r.Network = "host", "unknown"
		}
	}
	policy := func(p WitnessPolicy, then func(*v2fx)) func(*v2fx) {
		return func(fx *v2fx) {
			fx.policy = p
			if then != nil {
				then(fx)
			}
		}
	}
	envRow := func(fx *v2fx) { fx.row.Obligations = []string{ObligationEnv} }
	wholeTree := func(fx *v2fx) { fx.declared, fx.found = nil, false }
	network := hostCmd("cd tools/x && go test ./pkg/ && curl -s http://localhost:1/", "")
	cases := []v2case{
		// Option 1, the ruled default.
		{name: "option 1: container run", mut: nil},
		{name: "option 1: +env row", mut: envRow, gap: GapRerunOwed, has: []string{"+env"}},
		{name: "option 1: +env container row", mut: func(fx *v2fx) { envRow(fx) }, gap: GapRerunOwed},
		{name: "option 1: host run, readable, no network tool", mut: host},
		{name: "option 1: host run naming a network tool", mut: network, gap: GapRerunOwed, has: []string{"network tool"}},
		{name: "option 1: host run naming git fetch", mut: hostCmd("cd tools/x && git fetch && go test ./pkg/", ""), gap: GapRerunOwed, has: []string{"network tool"}},
		{name: "option 1: host run, unreadable command (bash)", mut: hostCmd("cd tools/x && go test ./pkg/", "bash"), gap: GapRerunOwed, has: []string{"cannot read"}},
		{name: "option 1: host run, unreadable command (open quote)", mut: hostCmd("cd tools/x && go test './pkg/", ""), gap: GapRerunOwed, has: []string{"cannot read"}},
		{name: "option 1: back-filled run, readable", mut: backfill},
		{name: "option 1: a recorded tool with no pin", mut: func(fx *v2fx) {
			fx.atR = setFile(WitnessPinFile, "statusgen:\n  tag: statusgen/v1.0.59\nharness:\n  digest: sha256:1111111111111111111111111111111111111111111111111111111111111111\n")
			fx.rec = func(r *WitnessRecord) {}
		}, gap: GapInputChanged}, // the pin file is itself an implicit input
		{name: "option 1: the record names no desk-tools", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { delete(r.Tools, "desk-tools") }
		}, gap: GapRerunOwed, has: []string{"desk-tools"}},
		{name: "option 1: tools unknown", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.ToolsSource = "none" }
		}, gap: GapRerunOwed, has: []string{"tools"}},
		// Option 2.
		{name: "option 2: container run", mut: policy(WitnessPolicyContainerOnly, nil)},
		{name: "option 2: host run", mut: policy(WitnessPolicyContainerOnly, host), gap: GapRerunOwed, has: []string{"container"}},
		{name: "option 2: back-filled run", mut: policy(WitnessPolicyContainerOnly, backfill), gap: GapRerunOwed},
		{name: "option 2: +env row", mut: policy(WitnessPolicyContainerOnly, envRow), gap: GapRerunOwed},
		// Option 3.
		{name: "option 3: host run naming a network tool", mut: policy(WitnessPolicyUnmarked, network)},
		{name: "option 3: +env row", mut: policy(WitnessPolicyUnmarked, envRow), gap: GapRerunOwed},
		// Option 4.
		{name: "option 4: declared-scope container record", mut: policy(WitnessPolicyWholeTreeOnly, nil), gap: GapRerunOwed, has: []string{"declared-scope"}},
		{name: "option 4: whole-tree container record", mut: policy(WitnessPolicyWholeTreeOnly, wholeTree)},
		{name: "option 4: whole-tree host record", mut: policy(WitnessPolicyWholeTreeOnly, func(fx *v2fx) { wholeTree(fx); host(fx) }), gap: GapRerunOwed},
	}
	runV2Cases(t, cases)

	// A tool recorded with no pin at R, with the pin file unchanged: the pin
	// file at run time already lacked it.
	fx := newV2fx(t)
	fx.files[WitnessPinFile] = fakeEntry{"100644", "statusgen:\n  tag: statusgen/v1.0.59\nharness:\n  digest: sha256:1111111111111111111111111111111111111111111111111111111111111111\n"}
	wantGap(t, "a recorded tool with no pin at R", fx.claim(), GapRerunOwed, "desk-tools", "no pin")
}

// regression: #2026
func TestWitnessV2Replay(t *testing.T) {
	runV2Cases(t, []v2case{
		{name: "the unaltered citation"},
		{name: "a record for another row of this brief", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.Row = "2" }
		}, gap: GapRecordReplayed, has: []string{"row #2"}},
		{name: "a record for another brief with the same command and inputs", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.Brief = "s/02" }
		}, gap: GapRecordReplayed, has: []string{"s/02"}},
		{name: "a different Result", mut: func(fx *v2fx) { fx.wit = func(w *Witness) { w.Result = "PASS (rerun)" } }, gap: GapRecordReplayed},
		{name: "a different output digest", mut: func(fx *v2fx) { fx.wit = func(w *Witness) { w.Output = strings.Repeat("3", 64) } }, gap: GapRecordReplayed},
		{name: "a different Date", mut: func(fx *v2fx) { fx.wit = func(w *Witness) { w.Date = "2026-10-26" } }, gap: GapRecordReplayed},
		{name: "a different identity", mut: func(fx *v2fx) {
			fx.cell = func(c string) string { return strings.Replace(c, "verifier-app @", "someone-else @", 1) }
		}, gap: GapRunnerMismatch, has: []string{"someone-else"}},
	})
	// The same record cited from a second Verify row of the brief.
	fx := newV2fx(t)
	fx.extraRows = []VerifyRow{{ID: "2", Command: fx.row.Command, Expect: fx.row.Expect, ClassCell: fx.row.ClassCell}}
	cs := fx.eval()
	if len(cs) != 2 {
		t.Fatalf("want 2 claims, got %d", len(cs))
	}
	wantRelease(t, "row 1", cs[0])
	wantGap(t, "the record cited from row 2", cs[1], GapRecordReplayed, "row #2")
}

// regression: #2026
func TestWitnessV2AcceptanceChanged(t *testing.T) {
	failRec := func(fx *v2fx) {
		fx.rec = func(r *WitnessRecord) { r.Result, r.Exit = "FAIL", "1" }
	}
	acc := strings.Repeat("c", 40)
	runV2Cases(t, []v2case{
		{name: "unaltered"},
		{name: "Expect loosened after the run", mut: func(fx *v2fx) { fx.afterRow = func(r *VerifyRow) { r.Expect = "exit 0" } }, gap: GapExpectChanged},
		{name: "Class changed after the run", mut: func(fx *v2fx) { fx.afterRow = func(r *VerifyRow) { r.ClassCell = "check" } }, gap: GapClassChanged},
		{name: "Command changed after the run (consistent record)", mut: func(fx *v2fx) {
			fx.afterRow = func(r *VerifyRow) { r.Command = "cd tools/x && go test -run X ./pkg/" }
		}, gap: GapCommandChanged},
		{name: "inconsistent record: matches the Verify row, not its own Evidence Command cell", mut: func(fx *v2fx) {
			fx.wit = func(w *Witness) { w.Command = "cd tools/x && go test -run Y ./pkg/" }
		}, gap: GapCommandCellMismatch},
		{name: "a recorded failure", mut: failRec, gap: GapOutcomeFail},
		{name: "a recorded could-not-run", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.Result, r.Exit = "COULD-NOT-RUN", "127" }
			fx.wit = func(w *Witness) { w.State = StateCouldNotRun }
		}, gap: GapOutcomeCouldNotRun},
		{name: "an unparseable Result", mut: func(fx *v2fx) { fx.wit = func(w *Witness) { w.State = "" } }, gap: GapOutcomeUnparseable},
		{name: "a pass on a prose-led row", mut: func(fx *v2fx) {
			fx.afterRow = func(r *VerifyRow) {
				r.ProseLed, r.ProseLedNote = "mention", "the Command cell is a mention, not a command"
			}
		}, gap: GapProseLed, has: []string{"mention"}},
		{name: "a failure whose Command changed after the run", mut: func(fx *v2fx) {
			failRec(fx)
			fx.afterRow = func(r *VerifyRow) { r.Command = "cd tools/x && go test -run X ./pkg/" }
		}, gap: GapCommandChanged},
		{name: "a failure whose record is missing", mut: func(fx *v2fx) {
			failRec(fx)
			fx.cell = func(c string) string { return c[:strings.Index(c, "rec:")] + "rec:" + strings.Repeat("ab", 32) }
		}, gap: GapRecordMissing},
		{name: "acceptance revision with the same Expect", mut: func(fx *v2fx) {
			fx.acceptance = acc
			fx.repo.expectAt[acc] = fx.row.Expect
		}},
		{name: "acceptance revision with a different Expect", mut: func(fx *v2fx) {
			fx.acceptance = acc
			fx.repo.expectAt[acc] = "exit 0; PASS; no skips"
		}, gap: GapExpectChanged, has: []string{acc}},
		{name: "acceptance revision unreadable", mut: func(fx *v2fx) { fx.acceptance = acc }, gap: GapUnestablished},
		{name: "acceptance revision abbreviated", mut: func(fx *v2fx) { fx.acceptance = acc[:12] }, gap: GapUnestablished},
	})
	fx := newV2fx(t)
	failRec(fx)
	if c := fx.claim(); c.Result != ResultFail {
		t.Errorf("a recorded failure reports %s, want %s", c.Result, ResultFail)
	}
}

// regression: #2026
func TestWitnessV2CRVerified(t *testing.T) {
	landedAs := func(fx *v2fx) fakeForge { return fakeForge{landed: map[string]string{"12": fx.landing}} }
	withForge := func(f func(fx *v2fx) fakeForge) func(*v2fx) {
		return func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.forge = f(fx) }
		}
	}
	runV2Cases(t, []v2case{
		{name: "github merge commit"},
		{name: "github squash commit", mut: func(fx *v2fx) { fx.form = "github-squash" }},
		{name: "gitlab merge commit", mut: func(fx *v2fx) { fx.form = "gitlab-merge" }},
		{name: "the cell's cr differs from the record's", mut: func(fx *v2fx) {
			fx.cell = func(c string) string { return strings.Replace(c, "cr:12", "cr:13", 1) }
		}, gap: GapCRMismatch, has: []string{"13", "12"}},
		{name: "not a change-request id", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.CR = "x12" }
			fx.cell = func(c string) string { return strings.Replace(c, "cr:12", "cr:x12", 1) }
		}, gap: GapCRUnbound},
		{name: "the landing names another change request", mut: func(fx *v2fx) {
			fx.landingMsg = "Merge pull request #13 from example-org/feat\n"
		}, gap: GapCRUnverified, has: []string{"13"}},
		{name: "the landing does not change the citing brief", mut: func(fx *v2fx) { fx.briefInBase = true }, gap: GapCRUnverified, has: []string{fxBriefRel}},
		{name: "a rebase landing, offline", mut: func(fx *v2fx) { fx.form = "none" }, gap: GapCRUnverified, has: []string{"--forge"}},
		{name: "a rebase landing the forge confirms", mut: func(fx *v2fx) { fx.form = "none"; withForge(landedAs)(fx) }},
		{name: "a record not yet on R's first-parent history", mut: func(fx *v2fx) { fx.evalAtFeature = true }, gap: GapCRUnverified},
		{name: "a single-parent commit with a merge-commit subject", mut: func(fx *v2fx) { fx.landingPars = 1 }, gap: GapCRUnverified, has: []string{"parent"}},
		{name: "a two-parent commit with only a squash suffix", mut: func(fx *v2fx) { fx.form, fx.landingPars = "github-squash", 2 }, gap: GapCRUnverified, has: []string{"parent"}},
		{name: "a merge whose first parent already holds the record", mut: func(fx *v2fx) {
			// The merge's first parent is the feature commit, which added the
			// record: the merge adds nothing, so the feature commit is the
			// adding commit and it lands in no recognised form.
			fx.refs = func(fx *v2fx) {
				c := fx.repo.commits[fx.landing]
				c.parents = []string{fx.featureID, fx.base}
			}
		}, gap: GapCRUnverified},
		{name: "a merge whose first parent already holds the record, and so does the second", mut: func(fx *v2fx) {
			// The adding commit keeps a merge subject and two parents, and both
			// parents hold the record: only the first-parent test refuses it.
			fx.refs = func(fx *v2fx) {
				fx.repo.objectParents = map[string][]string{fx.landing: {fx.featureID, fx.featureID}}
			}
		}, gap: GapCRUnverified, has: []string{"first parent"}},
		{name: "the default branch's first-parent history cannot be read", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.noFirstParents = true }
		}, gap: GapUnestablished, has: []string{"first-parent history could not be read"}},
		{name: "a merge whose second parent lacks the record", mut: func(fx *v2fx) { fx.evilMerge = true }, gap: GapCRUnverified, has: []string{"second parent"}},
		{name: "a merge-style subject on a feature branch, not the default branch", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.refs["refs/remotes/origin/HEAD"] = fx.base }
		}, gap: GapCRUnverified, has: []string{"default branch"}},
		{name: "the forge: another merge request's trailer on a fast-forward", mut: func(fx *v2fx) {
			fx.form = "none"
			fx.landingMsg = "Merge branch 'feat' into 'main'\n\nSee merge request example-org/example-repo!13\n"
			fx.landingPars = 2
			withForge(landedAs)(fx)
		}, gap: GapCRUnverified, has: []string{"13"}},
		{name: "the forge: not merged", mut: withForge(func(*v2fx) fakeForge { return fakeForge{landed: map[string]string{}} }), gap: GapCRUnverified},
		{name: "the forge: landed as another commit", mut: withForge(func(fx *v2fx) fakeForge { return fakeForge{landed: map[string]string{"12": fx.base}} }), gap: GapCRUnverified},
		{name: "the forge: cannot answer", mut: withForge(func(*v2fx) fakeForge { return fakeForge{unknown: true} }), gap: GapUnestablished},
		{name: "trunk with no origin/HEAD, offline", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) {
				delete(fx.repo.refs, "refs/remotes/origin/HEAD")
				fx.repo.refs["refs/remotes/origin/trunk"] = fx.landing
			}
		}, gap: GapCRUnverified, has: []string{"git remote set-head origin --auto"}},
		{name: "trunk after set-head", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) {
				fx.repo.refs["refs/remotes/origin/trunk"] = fx.landing
				fx.repo.refs["refs/remotes/origin/HEAD"] = fx.landing
			}
		}},
		{name: "trunk named by the forge", mut: func(fx *v2fx) {
			fx.form = "github-squash"
			fx.refs = func(fx *v2fx) {
				delete(fx.repo.refs, "refs/remotes/origin/HEAD")
				fx.repo.refs["refs/remotes/origin/trunk"] = fx.landing
				fx.forge = fakeForge{branch: "trunk", landed: map[string]string{"12": fx.landing}}
			}
		}},
		{name: "a shallow clone whose boundary is the landing commit", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.shallow[fx.landing] = true }
		}, gap: GapUnestablished, has: []string{"shallow"}},
		{name: "a landing parent missing from the object store", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { delete(fx.repo.commits, fx.featureID) }
		}, gap: GapUnestablished, has: []string{"shallow"}},
		{name: "a replace ref on the landing commit", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.replaced[fx.landing] = true }
		}, gap: GapUnestablished, has: []string{"replace ref"}},
		{name: "a replace ref on a landing parent", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.replaced[fx.featureID] = true }
		}, gap: GapUnestablished, has: []string{"replace ref"}},
		{name: "no first-parent addition", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.additions = func([]WitnessAddition) []WitnessAddition { return nil } }
		}, gap: GapCRUnverified, has: []string{"no first-parent commit"}},
	})
}

// regression: #2026
func TestWitnessV2DirtyOutsideInputs(t *testing.T) {
	dirtyContent := "package pkg\n\nfunc A() int { return 3 }\n"
	dirtyInput := func(fx *v2fx) {
		fx.rec = func(r *WitnessRecord) {
			r.Inputs["tools/x/pkg/a.go"] = fakeBlobID(dirtyContent)
			r.DirtyPaths = []string{"tools/x/pkg/a.go"}
			r.Informational.Dirty = true
		}
	}
	runV2Cases(t, []v2case{
		{name: "unaltered"},
		{name: "a dirty path outside the required set", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.DirtyPaths = []string{"README.md"} }
		}, gap: GapDirtyOutsideInputs, has: []string{"README.md"}},
		{name: "a dirty implicit input", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.DirtyPaths = []string{"tools/x/go.sum"} }
		}},
		{name: "a dirty input that landed byte-identical", mut: func(fx *v2fx) {
			dirtyInput(fx)
			fx.atR = setFile("tools/x/pkg/a.go", dirtyContent)
		}},
		{name: "a dirty input that landed differently", mut: func(fx *v2fx) {
			dirtyInput(fx)
			fx.atR = setFile("tools/x/pkg/a.go", dirtyContent+"// more\n")
		}, gap: GapInputChanged, has: []string{"tools/x/pkg/a.go"}},
		{name: "a dirty input that never landed", mut: dirtyInput, gap: GapInputChanged, has: []string{"tools/x/pkg/a.go"}},
	})
}

// regression: #2026
func TestWitnessV2InputSetEqual(t *testing.T) {
	declareFile := func(fx *v2fx) {
		fx.files["tools/x/data.txt"] = fakeEntry{"100644", "data\n"}
		fx.declared = []string{"tools/x/data.txt", "tools/x/pkg/"}
	}
	runV2Cases(t, []v2case{
		{name: "unaltered"},
		{name: "a file deleted under a declared directory", mut: func(fx *v2fx) { fx.atR = delFile("tools/x/pkg/a_test.go") }, gap: GapInputSetChanged, has: []string{"tools/x/pkg/a_test.go"}},
		{name: "a removed go.sum", mut: func(fx *v2fx) { fx.atR = delFile("tools/x/go.sum") }, gap: GapInputSetChanged, has: []string{"tools/x/go.sum"}},
		{name: "a removed go.work", mut: func(fx *v2fx) { fx.atR = delFile("go.work") }, gap: GapInputSetChanged, has: []string{"go.work"}},
		{name: "a file added under a declared directory", mut: func(fx *v2fx) { fx.atR = setFile("tools/x/pkg/b.go", "package pkg\n") }, gap: GapInputUndeclaredAtRecord, has: []string{"tools/x/pkg/b.go"}},
		{name: "a recorded key no longer required", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) {
				r.Inputs["tools/x/pkg/gone.go"] = fakeBlobID("x")
				r.Attributes["tools/x/pkg/gone.go"] = AttributeDigest(nil)
			}
		}, gap: GapInputSetChanged, has: []string{"tools/x/pkg/gone.go"}},
		{name: "an implicit key no longer required", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.Implicit["other/go.sum"] = fakeBlobID("x") }
		}, gap: GapInputSetChanged, has: []string{"other/go.sum"}},
		{name: "an attribute entry missing for a recorded input", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { delete(r.Attributes, "tools/x/pkg/a.go") }
		}, gap: GapInputUndeclaredAtRecord, has: []string{"tools/x/pkg/a.go"}},
		{name: "an attribute entry for a path that is no input", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.Attributes["tools/x/pkg/zzz.go"] = AttributeDigest(nil) }
		}, gap: GapInputSetChanged, has: []string{"tools/x/pkg/zzz.go"}},
		{name: "an input recorded absent", mut: func(fx *v2fx) {
			fx.rec = func(r *WitnessRecord) { r.Inputs["tools/x/pkg/a.go"] = WitnessAbsent }
		}, gap: GapInputAbsent, has: []string{"tools/x/pkg/a.go"}},
		{name: "a declared file missing at R", mut: func(fx *v2fx) {
			declareFile(fx)
			fx.atR = delFile("tools/x/data.txt")
		}, gap: GapInputAbsent, has: []string{"tools/x/data.txt"}},
		{name: "a declared file present", mut: declareFile},
		{name: "a declared input that is a symlink whose target changed", mut: func(fx *v2fx) {
			fx.files["tools/x/pkg/link.go"] = fakeEntry{"120000", "a.go"}
			fx.atR = func(tree map[string]fakeEntry, _ map[string][]string) {
				tree["tools/x/pkg/link.go"] = fakeEntry{"120000", "a_test.go"}
			}
		}, gap: GapInputSymlink, has: []string{"tools/x/pkg/link.go"}},
		{name: "a whole-tree record carrying an inputs key", mut: func(fx *v2fx) {
			fx.declared, fx.found = nil, false
			fx.rec = func(r *WitnessRecord) {
				r.Inputs["tools/x/pkg/a.go"] = fakeBlobID(fx.files["tools/x/pkg/a.go"].content)
			}
		}, gap: GapInputSetChanged, has: []string{"tools/x/pkg/a.go"}},
		{name: "a declared-scope record judged after the declaration was removed", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.declared, fx.found = nil, false }
		}, gap: GapInputSetChanged, has: []string{"scope"}},
		{name: "a declaration widened after the run", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.declared = []string{"tools/x/"} }
		}, gap: GapInputUndeclaredAtRecord, has: []string{"tools/x/go.mod"}},
		{name: "a Command naming a path outside the declaration", mut: func(fx *v2fx) {
			fx.row.Command = "cd tools/x && go test ./pkg/ && cat ../../README.md"
		}, gap: GapInputUndeclared, has: []string{"README.md"}},
		{name: "a submodule input", mut: func(fx *v2fx) {
			fx.atR = func(tree map[string]fakeEntry, _ map[string][]string) {
				tree["tools/x/pkg/a.go"] = fakeEntry{"160000", tree["tools/x/pkg/a.go"].content}
			}
		}, gap: GapUnestablished, has: []string{"submodule"}},
	})
}

// TestWitnessV2Unreadable covers the target and port reads that precede the
// record: each is could-not-check, never a release.
// regression: #2026
func TestWitnessV2Unreadable(t *testing.T) {
	runV2Cases(t, []v2case{
		{name: "a dirty item revision", mut: func(fx *v2fx) { fx.refs = func(fx *v2fx) { fx.revision = fx.R + "+dirty" } }, gap: GapUnestablished, has: []string{"uncommitted"}},
		{name: "an unknown item revision", mut: func(fx *v2fx) { fx.revision = strings.Repeat("d", 40) }, gap: GapUnestablished},
		{name: "a repository without the witness port", mut: func(fx *v2fx) { fx.plainRepo = true }, gap: GapUnestablished},
	})
}

// regression: #2026
func TestWitnessRecordCanonical(t *testing.T) {
	fx := newV2fx(t)
	rec := fx.defaultRecord(fx.row)
	canonical, id, err := EncodeWitnessRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 64 || sha256Hex(canonical) != id {
		t.Fatalf("the id %q is not the sha256 of the canonical bytes", id)
	}
	// Key order and insignificant whitespace do not change the id.
	spaced := bytes.ReplaceAll(canonical, []byte(`,"`), []byte(",\n  \""))
	if _, id2, err := ParseWitnessRecord(spaced); err != nil || id2 != id {
		t.Errorf("a reformatted record: id %q err %v, want %q", id2, err, id)
	}
	got, id3, err := ParseWitnessRecord(canonical)
	if err != nil || id3 != id || got.TreeDigest != rec.TreeDigest || got.Inputs["tools/x/pkg/a.go"] != rec.Inputs["tools/x/pkg/a.go"] {
		t.Errorf("round trip: id %q err %v", id3, err)
	}
	for name, raw := range map[string]string{
		"invalid UTF-8":     strings.Replace(string(canonical), `"brief":"s/01"`, "\"brief\":\"s/\xff1\"", 1),
		"schema 3":          strings.Replace(string(canonical), `"schema":2`, `"schema":3`, 1),
		"a float schema":    strings.Replace(string(canonical), `"schema":2`, `"schema":2.0`, 1),
		"a backslash path":  strings.Replace(string(canonical), `"tools/x/pkg/a.go"`, `"tools\\x/pkg/a.go"`, 1),
		"an unknown origin": strings.Replace(string(canonical), `"origin":"run"`, `"origin":"copy"`, 1),
		"a trailing value":  string(canonical) + "{}",
	} {
		if _, _, err := ParseWitnessRecord([]byte(raw)); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
	// The line separators are escaped, so the canonical bytes are one line.
	rec.Informational.Commit = "a" + string(rune(0x2028)) + "b"
	if b, _, err := EncodeWitnessRecord(rec); err != nil || bytes.Contains(b, []byte(string(rune(0x2028)))) || !bytes.Contains(b, []byte("\\u2028")) {
		t.Errorf("U+2028 is not escaped: %q (%v)", b, err)
	}
}
