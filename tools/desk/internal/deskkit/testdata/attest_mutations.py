#!/usr/bin/env python3
"""Run in an owned checkout; each guard removal must cause an assertion failure."""
from pathlib import Path
import subprocess

pkg = Path(__file__).resolve().parents[1]
module = pkg.parents[1]
ATTEST = "internal/deskkit/verifierattestation.go"
SOURCE = "internal/deskkit/verifiersource.go"
EVIDENCE = "cmd/deskevidence/deskevidence.go"
KIT = "./internal/deskkit"
# statusgen is its own module: its entries name it as their fifth field.
SG = module.parents[1] / "statusgen"
VERIFYRUN = "verifyrun.go"
ADMISSION = "verifieradmission.go"
CLOSURE = "TestAttestSourceClosure"
TARGET = "TestVerifierEvidenceTargetBinding"
INDEX = "TestAttestStreamIndex"
# (name, [(file, before, after), ...], package, test regex[, module dir])
mutations = [
    ("typed-issue-author", [(ATTEST, 'strings.HasPrefix(title, VerifierAttestationTitle) && verifierAuthority(author)', 'strings.HasPrefix(title, VerifierAttestationTitle)')], KIT, "TestAttestationExcludedDuringOpenWindowBothForges"),
    ("qualified-source", [(ATTEST, '"rev-parse", "refs/remotes/origin/main"', '"rev-parse", "origin/main"')], KIT, "TestAttestRemoteRef"),
    ("actor-separation", [(ATTEST, "!SameActor(desk, verifier)", 'verifier != ""')], KIT, "TestAttestActorSeparation"),
    ("detached-head", [(ATTEST, 'head != b.Source || branch != "HEAD"', 'head != b.Source || (false && branch != "HEAD")')], KIT, CLOSURE + "/branch"),
    ("source-commit", [(ATTEST, 'head != b.Source || branch != "HEAD"', '(false && head != b.Source) || branch != "HEAD"')], KIT, CLOSURE + "/commit"),
    ("tracked-content", [(SOURCE, "if verifierBlobID(newHash, data) == object {", "if true || verifierBlobID(newHash, data) == object {")], KIT, CLOSURE + "/(tracked|staged|assume-unchanged|skip-worktree|clean-filter)$"),
    ("all-additional-files", [(SOURCE, 'return Refused("unattested worktree file: " + rel)', "return nil")], KIT, CLOSURE + "/(untracked|ignored|second-site)$"),
    ("missing-files", [(SOURCE, "if len(missing) > 0 {", "if false && len(missing) > 0 {")], KIT, CLOSURE + "/deleted$"),
    ("replace-refs", [(SOURCE, 'if refs != "" {', 'if false && refs != "" {')], KIT, CLOSURE + "/replace-ref$"),
    ("no-replace-objects", [(ATTEST, '"--no-replace-objects", "-C", root', '"-C", root'), (ATTEST, '"GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1"', '"GIT_ATTR_NOSYSTEM=1"')], KIT, "TestVerifierTreeIgnoresReplacements"),
    ("filter-driver", [(SOURCE, 'len(f) < 3 || (f[2] != "unspecified" && f[2] != "unset")', "len(f) < 0")], KIT, CLOSURE + "/smudge-filter$"),
    ("info-attributes", [(SOURCE, "if _, err := os.Lstat(attrs); err == nil {", "if _, err := os.Lstat(attrs); false && err == nil {")], KIT, CLOSURE + "/info-attributes-(crlf|encoding)$"),
    ("pinned-autocrlf", [(SOURCE, '"-c", "core.autocrlf=" + c.autocrlf, ', "")], KIT, CLOSURE + "/local-autocrlf$"),
    ("global-attributes", [(SOURCE, ', "-c", "core.attributesFile=" + os.DevNull}', "}")], KIT, CLOSURE + "/attributes-file$"),
    ("attribute-source", [(SOURCE, '"--attr-source=" + c.source, ', "")], KIT, CLOSURE + "/attr-tree$"),
    ("pinned-eol", [(SOURCE, '"-c", "core.eol=" + c.eol, ', "")], KIT, CLOSURE + "/local-eol$"),
    ("check-attr-pinned", [(SOURCE, 'verifierGit(home, checkout.args("check-attr", "-z", "filter", "--", rel)...)', 'verifierGit(home, "check-attr", "-z", "filter", "--", rel)')], KIT, CLOSURE + "/attr-tree-filter$"),
    ("index-binding", [(SOURCE, "if err := verifierIndexMatches(home, tree); err != nil {", "if err := error(nil); err != nil {")], KIT, CLOSURE + "/index-(removed|swapped|added|attributes|redirect)$"),
    ("env-strip", [(ATTEST, 'if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {', "if true {")], KIT, CLOSURE + "/index-redirect$"),
    ("env-strip-each", [(ATTEST, 'if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {', "if true {")], KIT, "TestVerifierEnvStrip"),
    ("work-tree-called", [(ATTEST, "if err := verifierOwnWorkTree(home); err != nil {", "if err := error(nil); err != nil {")], KIT, CLOSURE + "/(core-worktree|worktree-config-worktree|core-worktree-link|core-bare)$"),
    ("work-tree-resolved", [(SOURCE, "if err != nil || resolved != home {", "if false && (err != nil || resolved != home) {")], KIT, CLOSURE + "/(core-worktree|worktree-config-worktree)$"),
    ("work-tree-present", [(SOURCE, 'if err != nil {\n\t\treturn Refused("verifier home has no git work tree', 'if err != nil {\n\t\treturn nil\n\t\treturn Refused("verifier home has no git work tree')], KIT, CLOSURE + "/core-bare$"),
    ("work-tree-config", [(SOURCE, '"config", "--get-all", "core.worktree"', '"config", "--get-all", "fixture.unset"')], KIT, CLOSURE + "/core-worktree-link$"),
    ("work-tree-config-scope", [(SOURCE, '"config", "--get-all", "core.worktree"', '"config", "--local", "--get-all", "core.worktree"')], KIT, CLOSURE + "/worktree-config-link$"),
    ("work-tree-config-includes", [(SOURCE, '"config", "--get-all", "core.worktree"', '"config", "--no-includes", "--get-all", "core.worktree"')], KIT, CLOSURE + "/include-worktree-link$"),
    ("work-tree-config-unread", [(SOURCE, 'return Unverifiable("cannot inspect verifier work tree config", err)', "return nil")], KIT, "TestWorkTreeConfigUnread"),
    ("work-tree-resolved-view", [(SOURCE, "if err != nil || resolved != home {", "if false && (err != nil || resolved != home) {")], KIT, "TestAttestSubdirHome"),
    ("render-needs-source", [(SOURCE, 'return nil, Unverifiable("cannot render attested source file "+rel, errors.New("no attested attribute source"))', "return nil, nil")], KIT, "TestRenderNeedsAttrSource"),
    ("old-git-class", [(SOURCE, '"version"); verifierLacksAttrSource(probe) {', '"version"); probe != nil {')], KIT, "TestOldGitProbeClass"),
    ("old-git-option", [(SOURCE, 'bytes.Contains(exit.Stderr, []byte("unknown option: --attr-source"))', "true")], KIT, "TestOldGitProbeClass/other-usage"),
    ("row-reads-called", [(ATTEST, "if err := verifierRowGitReads(home); err != nil {", "if err := error(nil); err != nil {")], KIT, CLOSURE + "/(grep|log|status)-config$"),
    ("row-reads-exit", [(SOURCE, "case errors.As(err, &exit) && exit.ExitCode() == 1:", "case errors.As(err, &exit):")], KIT, CLOSURE + "/(grep|log|status)-config$"),
    ("row-reads-unrunnable", [(SOURCE, 'return Unverifiable("cannot probe verifier row git reads: git "+read[0], err)', "return nil")], KIT, "TestRowReadProbeUnrunnable"),
    ("row-read-grep", [(SOURCE, '\t{"grep", "-q", "-e", "assay-admission-probe", "--", verifierProbePath},\n', "")], KIT, CLOSURE + "/grep-config$"),
    ("row-read-log", [(SOURCE, '\t{"log", "-1", "--format=%H", "--", verifierProbePath},\n', "")], KIT, CLOSURE + "/log-config$"),
    ("row-read-status", [(SOURCE, '\t{"status", "--porcelain", "--", verifierProbePath},\n', "")], KIT, CLOSURE + "/status-config$"),
    ("home-git-spelling", [(SOURCE, "\treturn given, resolved, nil\n}", "\treturn given, given, nil\n}")], KIT, "TestAttestCaseVariantRoot"),
    ("home-same-dir", [(SOURCE, "if err != nil || !verifierSameDir(resolved, given) {", "if err != nil || resolved != given {")], KIT, "TestAttestCaseVariantRoot"),
    ("same-dir-identity", [(SOURCE, "return err == nil && os.SameFile(ai, bi)", "return err == nil && ai == bi")], KIT, "TestVerifierSameDir"),
    ("row-env-source", [(VERIFYRUN, "cmd.Env = plan.rowEnv()", "cmd.Env = os.Environ()")], ".", "TestAdmittedRows|TestRowEnvSingleSource", SG),
    ("row-env-admitted", [(VERIFYRUN, "\t\tplan.env = admittedRowEnv(os.Environ())\n", "")], ".", "TestAdmittedRows(StripGitEnv|PinGrep)/admitted", SG),
    ("row-env-strip", [(ADMISSION, 'case strings.HasPrefix(strings.ToUpper(key), "GIT_") && !admittedRowNarrowing(kv, home):', 'case false && admittedRowNarrowing(kv, home):')], ".", "TestAdmittedRowsStripGitEnv/admitted|TestAdmittedRowEnvShape", SG),
    ("row-env-narrowing", [(ADMISSION, 'case strings.HasPrefix(strings.ToUpper(key), "GIT_") && !admittedRowNarrowing(kv, home):', 'case strings.HasPrefix(strings.ToUpper(key), "GIT_") && (home == "" || home != ""):')], ".", "TestAdmittedRowsKeepNarrowing|TestAdmittedEnvNarrowOnly", SG),
    ("row-env-narrow-global", [(ADMISSION, 'return value == os.DevNull || (filepath.IsAbs(home) && filepath.Clean(value) == filepath.Join(home, ".gitconfig"))', 'return value != "" && filepath.IsAbs(home)')], ".", "TestAdmittedEnvNarrowOnly", SG),
    ("row-env-narrow-bool", [(ADMISSION, 'return slices.Contains([]string{"1", "true", "yes", "on"}, strings.ToLower(value))', "return value != \"\"")], ".", "TestAdmittedEnvNarrowOnly", SG),
    ("row-env-shell-startup", [(ADMISSION, "case admittedRowShellStartup(key):", "case false && admittedRowShellStartup(key):")], ".", "TestAdmittedRowsShellEnv/.*/admitted|TestAdmittedRowEnvShape", SG),
    ("row-env-bash-env", [(ADMISSION, '[]string{"BASH_ENV", "ENV",', '[]string{"ENV",')], ".", "TestAdmittedRowsShellEnv/bash-env/admitted", SG),
    ("row-env-bash-func", [(ADMISSION, 'return strings.HasPrefix(key, "BASH_FUNC_") || ', "return ")], ".", "TestAdmittedRowsShellEnv/bash-func/admitted", SG),
    ("row-env-grep-pin", [(ADMISSION, '\t\t"GIT_CONFIG_COUNT=2",\n\t\t"GIT_CONFIG_KEY_0=grep.patternType", "GIT_CONFIG_VALUE_0=default",\n\t\t"GIT_CONFIG_KEY_1=grep.extendedRegexp", "GIT_CONFIG_VALUE_1=false")', ")")], ".", "TestAdmittedRowsPinGrep/admitted", SG),
    ("system-attributes", [(ATTEST, '"GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1"', '"GIT_NO_REPLACE_OBJECTS=1"')], KIT, "TestVerifierEnvIgnoresSystemAttributes"),
    ("checkout-from-binding", [(ATTEST, "checkout, err := parseVerifierCheckout(b.Checkout)", "checkout, err := readVerifierCheckout(home)")], KIT, "TestVerifierCheckoutConversionPinned"),
    ("index-at-execution", [(ATTEST, "if phase == verifierEvidence {\n\t\tallowed[index] = true", "if true {\n\t\tallowed[index] = true")], KIT, INDEX),
    ("index-edit-check", [(ATTEST, "if phase == verifierEvidence {\n\t\t_, nn, _", "if false {\n\t\t_, nn, _")], KIT, INDEX),
    ("index-own-row-only", [(SOURCE, "return err == nil && bytes.Equal(rebased, data)", "return rebased != nil || err != nil || true")], KIT, INDEX),
    ("evidence-target", [(ATTEST, 'return Refused("Evidence target " + target + " is not bound to the attested brief " + r.Binding.Brief)', "return nil")], KIT, TARGET),
    ("outcome-key", [(ATTEST, "recStream == stream && recNN == nn", "recStream == stream && (recNN == nn || nn != recNN)")], KIT, TARGET),
    ("index-rows", [(ATTEST, "if strings.TrimSpace(row) != nn {", "if false {")], KIT, TARGET),
    ("target-in-admission", [(ATTEST, "if err := r.VerifierReceipt.CheckEvidenceTarget(target); err != nil {", "if err := error(nil); err != nil {")], KIT, TARGET),
    ("landing-target", [(EVIDENCE, "admitVerifierEvidence(*root, repoSlug, targetRepoPath, ac)", "admitVerifierEvidence(*root, repoSlug, evidenceRepoPath, ac)")], "./cmd/deskevidence", "TestVerifierEvidenceTargetBoundToAttestedBrief/brief-path"),
    ("fragment-outside-root", [(EVIDENCE, '*briefPath == "" || pathWithin(*root, evidenceRepoPath)', '*briefPath == ""')], "./cmd/deskevidence", "TestVerifierEvidenceLandingFormAdmitted/fragment-inside-home-refuses"),
    ("absolute-fragment-form", [(EVIDENCE, '*briefPath == "" || pathWithin(*root, evidenceRepoPath)', 'true')], "./cmd/deskevidence", "TestVerifierEvidenceLandingFormAdmitted/documented-form-lands"),
    ("landing-home-direction", [(ATTEST, 'if _, serr := os.Stat(record); perr != nil || serr != nil {', 'if _, serr := os.Stat(record); false && (perr != nil || serr != nil) {')], "./cmd/deskevidence", "TestVerifierEvidenceLandingFormAdmitted/desk-checkout"),
    ("commit-binding", [(EVIDENCE, '"Evidence: verification row for " + targetRepoPath + ac.attestationTrailer() + commitSuffix', '"Evidence: verification row for " + targetRepoPath + commitSuffix')], "./cmd/deskevidence", "TestVerifierEvidenceTargetBoundToAttestedBrief/bound-direct-commit"),
]
for name, edits, package, tests, *where in mutations:
    cwd = where[0] if where else module
    originals = {}
    try:
        for rel, before, after in edits:
            path = cwd / rel
            text = path.read_text()
            originals.setdefault(path, text)
            if text.count(before) != 1:
                raise SystemExit(f"{name}: mutation anchor absent/ambiguous in {rel}")
            path.write_text(text.replace(before, after))
        run = subprocess.run(["go", "test", package, "-run", tests, "-count=1", "-timeout", "120s"], cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if run.returncode == 0 or "--- FAIL:" not in run.stdout or "build failed" in run.stdout:
            raise SystemExit(f"{name}: no assertion failure\n{run.stdout}")
        print(f"{name}: killed")
        for line in run.stdout.splitlines():
            if "--- FAIL:" in line or "want refusal" in line or "admitted" in line or "landed" in line or "lost" in line:
                print(line)
    finally:
        for path, text in originals.items():
            path.write_text(text)
