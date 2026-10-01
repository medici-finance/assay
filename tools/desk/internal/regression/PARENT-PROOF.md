# Parent counterfactual method

The brief's Evidence table records actual failing assertions on each fixing
commit's first parent. These were bounded `go test -run '^<test>$' -count=1
-timeout 25s ./<owning-package>` runs, with `GOPROXY=off` and fixture-only
transports. The matching top-level test was also run green on the implementation
base. The API and fixture adaptations below are part of the evidence, not hidden
production changes.

Historical module sources came from `git archive <fix>^ <module>`. Tests and
fixture data came from the fixing snapshot. The parent production files stayed
unchanged. Initial compiler errors from copying tests referring to newly added
APIs were rejected as counterfactual evidence. Only the following compatible
runs are recorded:

- #643: transplant the single session-email acceptance test onto the parent's
  existing preflight helpers; spell the new roster key literally. The parent
  rejects that key, leaving no loaded role binding, so the expected clean session
  preflight fails. This measures the full valid-configuration flow (recognition
  plus identity), not the narrower identity branch in isolation.
- #656 and #757: keep only the verb-level test and add the inert test-only `goos`
  selector missing on the parent. Parent production does not consult it. The
  unchanged old verb creates the nonportable target or rejects the drive-rooted
  home; the fixed verb consults the selector and passes.
- #687: transplant the filing-success assertion and retain the parent's existing
  fake CLI/helpers. Omit the new-backend bookkeeping assertion that cannot exist
  on the parent. The entry point still exits 5 with its interim GitLab refusal,
  instead of filing (exit 0). The manifest points to the final implementation,
  not to the earlier refusal-only close.
- #708: transplant the scriptless dispatch case; declare its test-only PATH
  resolver and native-binary name on the parent. Parent production ignores that
  resolver and refuses because the legacy script is missing.
- #727: the new real linked-worktree test runs unchanged on the parent and fixed
  tree. It creates a complete local repository with `extensions.worktreeConfig`,
  then removes executable fallback from PATH. It neither contacts its placeholder
  remote nor relies on an incomplete `.git` fixture.
- #697, #772, #773, #999, #1007, #1056, #1203, #1411, #1418, #1490 and #1864:
  original fixing-snapshot tests and their historical helpers compile unchanged.
  For #697 select the CE-404 subtest; for #1033 select the open-issues golden case.
- #1033: copy its new golden fixture too; supply the new page-ceiling constant as
  test-only fixture data. The original production stub returns could-not-check
  instead of the expected list. A missing golden file is not counted as a red.
- #1034: retain the original concurrent fixture and first test only. Remove the
  newly added rotate/no-rotate boolean from the parent call's argument list. It
  invokes the same rotate operation; overlapping requests produce a revoked-token
  HTTP 401. No lock or concurrency fix is copied into the parent.
- #1067: keep the verb-level unpinned-SHA cases and existing parent helpers; omit
  tests of new private helper names. The comparison fails with missing base/head
  and escapes the entire classifier instead of degrading the row.
- #1086: retain the fixing HTTP fixture and issue-route expectations, removing
  only the `LabelChange.Target` input field absent in the old API. The old issue
  caller had exactly that targetless input shape. Its GET and PUT address the
  same-numbered merge request, while the fixed typed input addresses the issue.
- #1146: omit the separate unit test of the new `stepDecision` parameter; retain
  the unchanged child-environment acceptance test. The actual fixture decision
  script runs and records an empty credential instead of the minted role token.
- #1223: the parent production default was the direct `ghIssueLister` call rather
  than the newly declared default variable. Bind the transplanted production-read
  assertion to that historical entry point. The fake CLI fails, proving the
  native-read fixture cannot rescue the parent's actual production path.
- #1415: keep the transient-400 fake and retry-to-success assertion. Use
  `s.forge()` instead of the new sleep-seam helper; no delay occurs on the parent
  because it immediately returns the first 400. Its API error remains the failure.
- #786 and #1145: run the existing shell fixture suites with `FLEET_IMPL` or
  `CELLCTL` pointing to the fixing parent's script. The old fleet script exposes
  the fixture sentinel on argv and aborts instead of recording transport failure.
  The old shim fails its nested CLI authentication. Both suites are green against
  the current implementations, including later credential-isolation corrections.

No compile error, fixture absence, skipped test, empty selection, or test PASS at
parent is recorded as regression evidence. The table excerpts omit generated
fixture paths; they retain the observed value or route that made the test fail.
