package deskkit

import (
	"encoding/json"
	"strings"

	"github.com/medici-finance/assay/perfrec"
)

// Only compiled read operations on the documented public provider schemas are
// admitted. Enterprise and custom schemas retain explicit unknown cost; there
// is no capability probe, fallback send, or change to business selections.
// One closed compiled inventory owns both admission and the operation-shape guard.
// It is never extended by provider data or runtime configuration.
var ghPointQueries = [...]string{
	ghOpenChangesQuery, ghListChangesQuery, ghReviewQueueQuery,
	ghCommentsQuery, ghChangeCommentsWalkQuery, ghIssueCommentsQuery,
	ghListIssuesQuery, ghIssueStateEventsQuery, IssueTrustQuery, PRTrustQuery, IssueEventsQuery,
	verifierIssueBatchQuery, verifierHistoryBatchQuery, verifierLabelsBatchQuery,
	issueCommentBatchQuery,
	pendingChangeBatchQuery, pendingFilesBatchQuery,
}

func ghPointQuery(query string) string {
	for _, admitted := range ghPointQueries {
		if query == admitted {
			end := strings.LastIndex(query, "}")
			return query[:end] + ` _assayPerf:rateLimit{cost}` + query[end:]
		}
	}
	return query
}

func ghPointInput(in any) (any, bool) {
	m, ok := in.(map[string]any)
	if !ok {
		return in, false
	}
	query, ok := m["query"].(string)
	if !ok {
		return in, false
	}
	next := ghPointQuery(query)
	if next == query {
		return in, false
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	cp["query"] = next
	return cp, true
}

func decodeRequestPoints(raw []byte, unit perfrec.PointUnit) perfrec.Points {
	p := perfrec.Points{Coverage: perfrec.CoverageUnknown, Unit: unit}
	var envelope struct {
		Data struct {
			Meta json.RawMessage `json:"_assayPerf"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		p.Coverage = perfrec.CoverageFailed
		return p
	}
	if len(envelope.Data.Meta) == 0 || string(envelope.Data.Meta) == "null" {
		return p
	}
	var metadata map[string]json.RawMessage
	if json.Unmarshal(envelope.Data.Meta, &metadata) != nil {
		p.Coverage = perfrec.CoverageFailed
		return p
	}
	key := "cost"
	if unit == perfrec.UnitGitLabComplexity {
		key = "score"
	}
	value := metadata[key]
	if len(value) == 0 || string(value) == "null" {
		return p
	}
	var n int64
	if json.Unmarshal(value, &n) != nil || n < 0 {
		p.Coverage = perfrec.CoverageFailed
		return p
	}
	p.Coverage, p.Value = perfrec.CoverageMeasured, &n
	return p
}

// The GitLab SDK decodes before closing the response. This per-operation
// decoder attaches only scalar metadata before that Close publishes the
// observation. The original decoder still owns business errors and fields.
type pointCapture struct{ body *observedBody }
type pointCaptureKey struct{}
type pointDecoder struct {
	target  any
	capture *pointCapture
}

func (d pointDecoder) UnmarshalJSON(raw []byte) error {
	err := json.Unmarshal(raw, d.target)
	if d.capture.body != nil {
		attachRequestPoints(d.capture.body, decodeRequestPoints(raw, perfrec.UnitGitLabComplexity))
	}
	return err
}
