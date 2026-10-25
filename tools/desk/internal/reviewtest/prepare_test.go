package reviewtest_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

func prepareAcceptance(t *testing.T, name string, packages ...string) {
	t.Helper()
	pkg := "./cmd/deskdispatch"
	if len(packages) > 0 {
		pkg = packages[0]
	}
	cmd := exec.Command("go", "test", "-v", "-count=1", "-run", "^"+name+"$", pkg)
	cmd.Dir = moduleRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	b, e := goquiet.Cmd(cmd).CombinedOutput()
	if e != nil || !strings.Contains(string(b), "--- PASS: "+name+" ") {
		t.Fatalf("production preparation acceptance: %v\n%s", e, b)
	}
	t.Log(string(b))
}
func TestReviewPrepareFlow(t *testing.T) {
	prepareAcceptance(t, "TestReviewPrepareAdaptersFlow")
	prepareAcceptance(t, "TestReviewPrepareDispatchWiring")
	prepareAcceptance(t, "TestReviewPrepareKeys")
	prepareAcceptance(t, "TestReviewPrepareDiffCoverage")
	prepareAcceptance(t, "TestFileAtBoundedPreflight", "./internal/gitcore")
	prepareAcceptance(t, "TestDiffBoundedPreflight", "./internal/gitcore")
	prepareAcceptance(t, "TestPreparationExactObjects", "./internal/gitcore")
	prepareAcceptance(t, "TestGitLabPreparationBase", "./internal/deskkit")
	if os.Getenv("ASSAY_REVIEW_CONTRACT_MUTANT") != "1" {
		t.Run("mutation", func(t *testing.T) {
			contractMutation(t, "cmd/deskdispatch/packet_review.go", "reviewPacketDiffInputCap = 4 << 20", "reviewPacketDiffInputCap = reviewPacketDiffCap", "^TestReviewPrepareDiffCoverage$", "small edit in large file lost its local diff", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/packet_review.go", "diff, derr := fg.ChangeDiff(fr, pr)", "diff, derr := fg.ChangeDiff(fr, pr); if p,ok:=fg.(*preparedReview);ok && p.localDiffSkipped!=\"\" {out.Omit(\"diff\",packet.SizeUnknown,p.localDiffSkipped);return out,nil}", "^TestReviewPrepareDiffCoverage$", "local resource fallback dropped collected patches", "./cmd/deskdispatch")
			contractMutation(t, "internal/deskkit/forge_gitlab.go", "BaseSHA:           mr.DiffRefs.BaseSha,", "BaseSHA: \"\",", "^TestGitLabPreparationBase$", "GitLab exact base was discarded", "./internal/deskkit")

			contractMutation(t, "internal/gitcore/filebounded.go", "return r.repo.CommitObject(plumbing.NewHash(id))", "resolved,e:=r.Resolve(id);if e!=nil{return nil,e};return r.repo.CommitObject(resolved)", "^TestPreparationExactObjects$", "missing exact file object accepted a reference substitute", "./internal/gitcore")

			contractMutation(t, "internal/gitcore/filebounded.go", "entry, e := tree.FindEntry(path)", "_, earlyErr := tree.File(path); if earlyErr != nil {return \"\",false,earlyErr}; entry, e := tree.FindEntry(path)", "^TestFileAtBoundedPreflight$", "over-cap loose blob inflated before refusal", "./internal/gitcore")
			contractMutation(t, "internal/gitcore/filebounded.go", "if entry.Mode != filemode.Regular && entry.Mode != filemode.Executable {", "if false && entry.Mode != filemode.Regular && entry.Mode != filemode.Executable {", "^TestFileAtBoundedPreflight$", "non-regular blob loaded before mode refusal", "./internal/gitcore")
			contractMutation(t, "internal/gitcore/filebounded.go", "if n < 0 || n > budget-total {", "if false && (n < 0 || n > budget-total) {", "^TestDiffBoundedPreflight$", "diff inflated blobs before budget refusal", "./internal/gitcore")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "if p.filesRead {", "if false && p.filesRead {", "^TestReviewPrepareAdaptersFlow$", "shared changed files were fetched twice", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "p.local.FileAtBounded(in.Ref, in.File, packet.DefaultPerItemCap)", `p.local.FileAtBounded(func() string { h, _ := p.local.Resolve("HEAD"); return h.String() }(), in.File, packet.DefaultPerItemCap)`, "^TestReviewPrepareAdaptersFlow$", "packet did not use assigned-head bytes", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "if v, ok := p.content[key]; ok {", "if v, ok := p.content[key]; false && ok {", "^TestReviewPrepareKeys$", "immutable provider content read repeated", "./cmd/deskdispatch")

		})
	}
}
func TestReviewPrepareInvalidation(t *testing.T) {
	prepareAcceptance(t, "TestReviewPrepareAdaptersInvalidation")
	prepareAcceptance(t, "TestReviewPrepareObservation")
	prepareAcceptance(t, "TestReviewPrepareMissingBase")
	prepareAcceptance(t, "TestReviewPrepareBudgets")
	prepareAcceptance(t, "TestReviewPrepareDispatchAbort")
	prepareAcceptance(t, "TestReviewPrepareVolatileMetadata")
	prepareAcceptance(t, "TestReviewPrepareUnknownIdentityFallback")
	prepareAcceptance(t, "TestReviewPrepareLocalOmission")
	prepareAcceptance(t, "TestForgePreparationCancellation", "./internal/deskkit")
	if os.Getenv("ASSAY_REVIEW_CONTRACT_MUTANT") != "1" {
		t.Run("mutation", func(t *testing.T) {
			contractMutation(t, "cmd/deskdispatch/packet_review.go", "in.plan.reviewContext.startPacket()", "_ = in.plan.reviewContext", "^TestReviewPrepareBudgets$", "claim/workspace latency consumed packet budget", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "return a != nil && b != nil && a.HeadSHA == b.HeadSHA && a.BaseSHA == b.BaseSHA && a.BaseRef == b.BaseRef && a.Body == b.Body && a.State == b.State && hasDecisionLabel(a.Labels) == hasDecisionLabel(b.Labels)", "return reflect.DeepEqual(a,b)", "^TestReviewPrepareVolatileMetadata$", "volatile metadata spuriously invalidates the prepared source", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "return nil, deskkit.Unverifiable(\"observation identity could not be checked\", err)\n\t\t}\n\t\treturn nil, nil", "return nil, deskkit.Unverifiable(\"observation identity could not be checked\", err)\n\t\t}\n\t\treturn nil, err", "^TestReviewPrepareUnknownIdentityFallback$", "unknown wrapper did not retain unprepared compatibility path", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "p.omissions[key] = \"local exact blob exceeds the packet per-item byte cap\"", "p.omissions[key] = \"\"", "^TestReviewPrepareLocalOmission$", "local cap omission attributed to provider", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/dispatch.go", "if plan.reviewContext != nil && !plan.reviewContext.checked {", "if false && plan.reviewContext != nil && !plan.reviewContext.checked {", "^TestReviewPrepareDispatchAbort$", "dispatch did not abort on body", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "fresh, cancel := boundedPreparation(p.source)", "fresh, cancel := p.preparationForge, func(){}", "^TestReviewPrepareBudgets$", "packet expiry poisoned the fresh terminal check", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "!samePreparedSubject(pr, p.change)", "pr.HeadSHA != p.change.HeadSHA", "^TestReviewPrepareAdaptersInvalidation$", "mutable preparation accepted changed body", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "if p.reviewsRead {\n\t\tv, e := fresh.ReviewsAtHead", "if false && p.reviewsRead {\n\t\tv, e := fresh.ReviewsAtHead", "^TestReviewPrepareAdaptersInvalidation$", "mutable preparation accepted changed reviews", "./cmd/deskdispatch")
			contractMutation(t, "cmd/deskdispatch/reviewcontext.go", "if p.checks != nil {", "if false && p.checks != nil {", "^TestReviewPrepareAdaptersInvalidation$", "mutable preparation accepted changed checks", "./cmd/deskdispatch")
		})
	}
}
