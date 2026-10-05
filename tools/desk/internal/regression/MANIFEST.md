# Regression floor

Resolved behavior inherited by desktools-v2. Existing behavior tests are reused; the
`TestReg` entry points make two existing shell-fixture suites part of the desk suite.
Every fixture stays offline: local git repositories, local HTTP servers, or fake CLI
executables. No live forge or cluster is involved.

| Issue | Behavior pinned | Owning package | Test | Fixing commit |
|---|---|---|---|---|
| #643 | GitLab session commit identity is distinct from the role account | tools/desk/internal/deskkit | TestCommitIdentityGitLabSessionEmail | 0276ce0a5 |
| #656 | Windows add uses the portable worktree prefix | tools/desk/cmd/deskwt | TestAddOnWindowsCreatesUnderPortablePrefix | b31be9266 |
| #687 | GitLab filing reaches the resolved backend instead of the interim refusal | tools/desk/cmd/deskfile | TestDeskfileFilesOnGitLabThroughBackend | b7a82c025 |
| #697 | CE project-approvals 404 degrades the head pin rather than refusing reviews | tools/desk/internal/deskkit | TestForgeGitlabTierErrors | 3040653c4 |
| #708 | A scriptless adopter dispatches through the native claim binary | tools/desk/cmd/deskdispatch | TestDispatchFallsBackToTheGoClaimBinaryWhenNoScriptIsPresent | bf168ab2f |
| #727 | A linked worktree resolves its actual origin instead of a SaaS default | tools/desk/cmd/deskclaim-ref | TestReg727WorktreeOrigin | 63303b19c |
| #757 | Dispatch accepts the Windows home emitted by deskwt | tools/desk/cmd/deskdispatch | TestDispatchAcceptsDeskwtWindowsWorktreeHome | 03cb769a4 |
| #772 | GitLab ready-flip auth never attempts a GitHub App mint | tools/desk/cmd/deskflip | TestAppTokenGitLabRepoSkipsGitHubMint | 53d6cbe7e |
| #773 | GitLab reviewer assignments fetch the merge-request ref | tools/desk/cmd/deskdispatch | TestReviewKitHeadFetchIsGitLabShapedOnAGitLabRepo | f4693f1df |
| #786 | Fleet provisioning keeps credentials off argv and records transport failures | tools/desk/internal/regression | TestReg786FleetHardening | 643637114 |
| #999 | Truncated shallow history is could-not-check for consumed fragments | statusgen/ | TestConsumedFragmentIndexShallowCloneIsCouldNotCheck | dc410608c |
| #1007 | Human-stamp pathspecs normalize both separator styles | statusgen/ | TestRelPath_HandlesBothSeparatorStyles | 98960cedb |
| #1033 | GitLab open issues are read instead of a stubbed refusal | tools/desk/internal/deskkit | TestForgeGitlabGolden | 7a7cac919 |
| #1034 | Concurrent rotation retains a live role credential | tools/desk/cmd/desktoken | TestGitLabConcurrentMintsForOneRoleDoNotRevokeTheLivePAT | e428134c6 |
| #1056 | Review board recognizes the rostered GitLab username | tools/desk/cmd/deskboard | TestBoardAcceptsGitLabReviewerUsername | dac811073 |
| #1067 | Unpinned review SHA degrades one row rather than the whole board | tools/desk/cmd/deskboard | TestUnpinnedReviewSHADegradesRowNotSweep | e1742b9f8 |
| #1086 | Issue labels target the issue rather than a same-numbered merge request | tools/desk/internal/deskkit | TestLabelTargetRoutes | 980ad8cf0 |
| #1145 | An isolated shim authenticates its nested forge CLI without losing HOME isolation | tools/desk/internal/regression | TestReg1145ShimCredential | f84dde307 |
| #1146 | A decision-script child receives the dispatching role credential | tools/desk/cmd/deskdispatch | TestDecisionChildReceivesTheMintedRoleTokenInItsEnvironment | 8467637e4 |
| #1223 | Scan reads use the native forge with no working forge CLI | statusgen/ | TestScanIssueReadUsesNativeForgeNotGH | f19569abc |
| #1203 | GitLab claim dispatch uses its role PAT instead of a GitHub App mint | tools/desk/cmd/deskdispatch | TestGitLabWorkerDispatchClaimUsesGitLabPATNotTheAppMinter | ccaa5d566 |
| #1411 | GitLab checks fall back to the pipeline at the requested SHA | tools/desk/internal/deskkit | TestGitLabPipelineByShaFallbackReachesGate | e67e20f2f |
| #1415 | Draft change creation retries the transient missing-source-branch response | tools/desk/internal/deskkit | TestGitLabCreateDraftChangeRetriesTransientMissingBranch | 401d545a1 |
| #1418 | An unbootstrapped shell produces could-not-run rather than a false test failure | statusgen/ | TestRunWitnesses_WSLLauncher_BootstrapIsCouldNotRun | 5cc0c0d1b |
| #1490 | Verifier dispatch stamps its worktree role identity rather than shared identity | tools/desk/cmd/deskdispatch | TestVerifierDispatchStampsWorktreeIdentityNotTheSharedCheckouts | 8e838df89 |
| #1864 | Command sources obtain the API host from its declared home | tools/desk/cmd/deskfleet | TestNoAPIHostLiteral | 45d4f34a5 |

Existing names are retained verbatim, including historical long names. The 31-character
limit applies to newly introduced `TestReg<N>` names. For golden-corpus tests the relevant
cases are `TestForgeGitlabTierErrors/ce_404_on_project_approvals_degrades_not_refuses`
and `TestForgeGitlabGolden/list_open_issues`; the full top-level tests also cover neighboring
fail-closed cases. #687 uses the final filing implementation, not the interim named refusal.
#1145 checks the preserved shell oracle together with its later credential-isolation
correction; the Go-native port is owned by the separate cellctl migration.

## Dropped

| Issue | Reason |
|---|---|
| #322 | Cross-compile break: held by desktools-v2/13's T1 cross-compile gate, not a behavioral test. |
| #719 | Documentation-only close; no code fix. |
