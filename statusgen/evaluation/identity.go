package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// EvaluatorVersion names the evaluator generation. It changes whenever a
// verdict can change for the same observations; the policy digest below covers
// the constants that are data rather than code.
const EvaluatorVersion = "statusgen-evaluation/2"

// Identity says which evaluator produced a verdict: the same observations
// under the same Identity yield the same verdicts, byte for byte.
type Identity struct {
	Version      string `json:"version"`
	PolicyDigest string `json:"policy_digest"`
}

// CurrentIdentity returns this build's evaluator identity.
func CurrentIdentity() Identity {
	return Identity{Version: EvaluatorVersion, PolicyDigest: PolicyDigest()}
}

// policyText is the deterministic rendering of every policy constant a verdict
// depends on: the reference grammar, the revision-token floor and shape, the
// landing bound and the board-bookkeeping surface.
func policyText() string {
	return strings.Join([]string{
		"tree-sha-len=" + strconv.Itoa(TreeSHALen),
		"max-landing=" + strconv.Itoa(DefaultMaxLandingCandidates),
		"hex-revision=" + hexRevisionRe.String(),
		"ref-brief=" + RefInRepoBriefRe.String(),
		"ref-repo-brief=" + RefCrossRepoBriefRe.String(),
		"ref-cell-brief=" + RefCrossCellBriefRe.String(),
		"ref-issue=" + RefInRepoIssueRe.String(),
		"ref-repo-issue=" + RefCrossRepoIssueRe.String(),
		"bookkeeping=STATUS.md,docs/streams/",
		"verify-written=" + strings.Join(verifyWrittenPatterns(), ","),
		"flow-obligation=" + ObligationFlow,
		"join-rule=" + RuleJoinMissingFlow,
	}, "\n")
}

// PolicyDigest is the hex SHA-256 of the policy constants.
func PolicyDigest() string {
	sum := sha256.Sum256([]byte(policyText()))
	return hex.EncodeToString(sum[:])
}
