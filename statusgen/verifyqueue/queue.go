// Package verifyqueue defines the canonical verification queue wire contract.
// Classification remains owned by statusgen's placeAwaiting; these types do not
// infer a bucket from brief state or grant execution or acceptance authority.
package verifyqueue

const Version = 1
const (
	Judgment     = "judgment-dispatch"
	Runner       = "runnable-offline"
	Continuation = "continuation"
	Human        = "human-action"
	Unreadable   = "unreadable"
	Paused       = "paused"
	Parked       = "parked"
)

// Row preserves the historical field ordering and additive v1 queue fields.
type Row struct {
	Brief             string `json:"brief"`
	Score             int    `json:"score"`
	BlockedCount      int    `json:"blockedCount"`
	Stream            string `json:"stream"`
	Status            string `json:"status"`
	Repo              string `json:"repo,omitempty"`
	QueueVersion      int    `json:"queueVersion"`
	QueueClass        string `json:"queueClass"`
	QueueBucket       string `json:"queueBucket,omitempty"`
	QueueOwner        string `json:"queueOwner,omitempty"`
	QueueNextAct      string `json:"queueNextAct,omitempty"`
	QueueGate         string `json:"queueGate,omitempty"`
	QueueRisk         bool   `json:"queueRisk"`
	QueueIrreversible bool   `json:"queueIrreversible"`
	QueueValid        bool   `json:"queueValid"`
	QueueWhy          string `json:"queueWhy,omitempty"`
}

// Supported rejects unknown vocabulary as well as unknown versions. A future
// producer's class cannot be counted as healthy coverage by an older consumer.
func Supported(version int, class string) bool {
	if version != Version {
		return false
	}
	switch class {
	case Judgment, Runner, Continuation, Human, Unreadable, Paused, Parked:
		return true
	}
	return false
}

func JudgmentWork(status, class string) bool {
	return status == "implemented" && class == Judgment
}
