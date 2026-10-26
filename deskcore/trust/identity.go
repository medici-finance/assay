// Package trust evaluates supplied roster and content facts without effects.
package trust

import (
	"strconv"
	"strings"
	"time"
)

// Profile records whether the source can supply numeric IDs.
type Profile uint8

const (
	LoginOnly  Profile = iota // legacy login-only observation; an ID is not asserted
	ObservedID                // ID-bearing observations must match; zero denotes a legacy list
	Content                   // content requires human IDs; legacy bot renderings may omit IDs
	NumericID                 // this source promises an ID for every identity
)

func IsBot(login string) bool {
	return strings.HasSuffix(login, "[bot]") || strings.HasPrefix(login, "app/")
}
func Admit(configured, trusted bool, pinned int64, observed int64, bot bool, profile Profile) bool {
	if profile > NumericID {
		return false
	}
	if !configured || !trusted {
		return false
	}
	// A profile can permit an absent ID; it cannot override an asserted ID.
	if observed != 0 {
		return pinned > 0 && observed == pinned
	}
	if profile == LoginOnly {
		return true
	}
	switch profile {
	case ObservedID:
		return true
	case Content:
		return bot
	default:
		return false
	}
}

// ExpectedID resolves decorated GitHub identities and supplied GitLab bare identities.
func ExpectedID(login string, humans, bots, bareBots map[string]int64) (int64, bool) {
	l := strings.ToLower(login)
	if id, ok := humans[l]; ok {
		return id, true
	}
	if id, ok := bareBots[l]; ok {
		return id, true
	}
	switch {
	case strings.HasSuffix(l, "[bot]"):
		l = strings.TrimSuffix(l, "[bot]")
	case strings.HasPrefix(l, "app/"):
		l = strings.TrimPrefix(l, "app/")
	default:
		return 0, false
	}
	id, ok := bots[l]
	return id, ok
}

type Event struct {
	Authority, Trusted bool
	Created, Edited    time.Time
}

func Blessed(bodyEdited time.Time, events []Event) bool {
	var at time.Time
	for _, e := range events {
		if e.Authority && e.Created.After(at) {
			at = e.Created
		}
	}
	if at.IsZero() || !bodyEdited.Before(at) {
		return false
	}
	for _, e := range events {
		if e.Trusted {
			continue
		}
		last := e.Created
		if e.Edited.After(last) {
			last = e.Edited
		}
		if !last.Before(at) {
			return false
		}
	}
	return true
}

func ParseDotenv(s string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		if v := strings.TrimSpace(val); v != "" {
			m[key] = v
		}
	}
	return m
}

func SplitList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func SplitIdentity(entry string) (login string, id int64, ok bool) {
	l, rest, found := strings.Cut(entry, ":")
	l = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "@")))
	if l == "" {
		return "", 0, false
	}
	if !found || strings.TrimSpace(rest) == "" {
		return l, 0, true
	}
	n, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return l, n, true
}

func LooksLikeBot(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	return strings.HasSuffix(l, "[bot]") ||
		strings.HasPrefix(l, "app/") ||
		strings.HasSuffix(l, "-app") ||
		strings.HasSuffix(l, "-bot")
}
