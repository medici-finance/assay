package store

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"math/big"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
)

// Private storage codec v1 uses Go's bounded-data gob encoding, not JSONB: source
// strings may contain NUL or non-UTF8 bytes; time.Time preserves nanosecond instants.
// The explicit bits restore gob's otherwise collapsed nil/known-empty collections.
// This is not a public wire protocol. Semantic field changes require codec review.
type mirrorFacts struct {
	Record  mirror.Observation
	Present [4]bool
}
type cappedBytes struct {
	bytes.Buffer
	limit int
}

func (b *cappedBytes) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, mirror.ErrInvalid
	}
	return b.Buffer.Write(p)
}
func encodeFact(r mirror.Observation, limit int) ([]byte, error) {
	if !factFits(r, limit) {
		return nil, mirror.ErrInvalid
	}
	wire := mirrorFacts{Record: r, Present: [4]bool{r.Labels != nil, r.RequestedReviewers != nil, r.Reviews.GitHub != nil, r.Reviews.GitLab != nil}}
	wire.Record.ID = mirror.RecordID{}
	wire.Record.InInventory = false
	out := &cappedBytes{limit: limit}
	out.WriteByte(1)
	if err := gob.NewEncoder(out).Encode(wire); err != nil {
		return nil, mirror.ErrInvalid
	}
	return out.Bytes(), nil
}
func decodeFact(payload []byte, limit int) (mirror.Observation, error) {
	if len(payload) < 2 || len(payload) > limit || payload[0] != 1 {
		return mirror.Observation{}, mirror.ErrInvalid
	}
	var wire mirrorFacts
	reader := bytes.NewReader(payload[1:])
	decoder := gob.NewDecoder(reader)
	if err := decoder.Decode(&wire); err != nil {
		return mirror.Observation{}, mirror.ErrInvalid
	}
	// Gob allows consecutive values; storage v1 permits exactly one.
	var trailing mirrorFacts
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) || reader.Len() != 0 {
		return mirror.Observation{}, mirror.ErrInvalid
	}
	r := wire.Record
	if r.ID != (mirror.RecordID{}) || r.InInventory {
		return mirror.Observation{}, mirror.ErrInvalid
	}
	if wire.Present[0] && r.Labels == nil {
		r.Labels = []string{}
	}
	if wire.Present[1] && r.RequestedReviewers == nil {
		r.RequestedReviewers = []mirror.Actor{}
	}
	if wire.Present[2] && r.Reviews.GitHub == nil {
		r.Reviews.GitHub = []mirror.GitHubReview{}
	}
	if wire.Present[3] && r.Reviews.GitLab == nil {
		r.Reviews.GitLab = []mirror.GitLabApproval{}
	}
	if !wire.Present[0] && r.Labels != nil || !wire.Present[1] && r.RequestedReviewers != nil || !wire.Present[2] && r.Reviews.GitHub != nil || !wire.Present[3] && r.Reviews.GitLab != nil {
		return mirror.Observation{}, mirror.ErrInvalid
	}
	if !factFits(r, limit) {
		return mirror.Observation{}, mirror.ErrInvalid
	}
	return r, nil
}
func factFits(r mirror.Observation, limit int) bool {
	left := limit
	add := func(s string) bool {
		if len(s) > left {
			return false
		}
		left -= len(s)
		return true
	}
	actor := func(a mirror.Actor) bool { return add(a.ID) && add(a.Login) && add(a.Type) }
	if limit <= 0 || !add(r.State) || !add(r.ClosureReason) || !add(r.HeadRevision) || !add(r.Title) || !add(r.Body) || !actor(r.Author) {
		return false
	}
	// Charge even empty entries before traversing them, bounding both work and size.
	for _, n := range []int{len(r.Labels), len(r.RequestedReviewers), len(r.Reviews.GitHub), len(r.Reviews.GitLab)} {
		if n > left/16 {
			return false
		}
		left -= n * 16
	}
	for _, s := range r.Labels {
		if !add(s) {
			return false
		}
	}
	for _, a := range r.RequestedReviewers {
		if !actor(a) {
			return false
		}
	}
	for _, v := range r.Reviews.GitHub {
		if !add(v.ID) || !actor(v.Author) || !add(v.State) || !add(v.Commit) {
			return false
		}
	}
	for _, v := range r.Reviews.GitLab {
		if !actor(v.User) {
			return false
		}
	}
	return true
}

var billion = big.NewInt(1_000_000_000)

// SQL numeric(30,0) compares exact Unix nanoseconds, without int64 UnixNano's
// 1677–2262 range or PostgreSQL timestamptz's microsecond truncation. NULL is unknown.
func stamp(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	n := new(big.Int).Mul(big.NewInt(t.Unix()), billion)
	n.Add(n, big.NewInt(int64(t.Nanosecond())))
	return n.String()
}
func parseStamp(raw *string) (time.Time, error) {
	if raw == nil {
		return time.Time{}, nil
	}
	n, ok := new(big.Int).SetString(*raw, 10)
	if !ok {
		return time.Time{}, errors.New("invalid timestamp")
	}
	seconds, remainder := new(big.Int), new(big.Int)
	seconds.QuoRem(n, billion, remainder)
	if remainder.Sign() < 0 {
		seconds.Sub(seconds, big.NewInt(1))
		remainder.Add(remainder, billion)
	}
	if !seconds.IsInt64() {
		return time.Time{}, errors.New("timestamp outside Go range")
	}
	return time.Unix(seconds.Int64(), remainder.Int64()).UTC(), nil
}
