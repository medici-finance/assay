package deskkit

import (
	"os"
	"regexp"
	"strings"
)

// signoff.go — ONE reader for "has a human signed this ruling?".
//
// A rulings register records process authority no agent holds. Every gate that
// consumes one is asking the same question and must therefore parse it the same
// way; a second parser is a second answer, and the two disagree silently. This
// file is that one parser. Callers map the three states onto their own exit
// codes, but they do not re-decide what the states mean.
//
// # Why three states and not two
//
// "Signed" and "unsigned" are both POSITIVE determinations about what a human
// did. "I could not read the register" is a statement about the reader, and
// collapsing it into either of the other two manufactures a fact. Both
// collapses are dangerous, in opposite directions:
//
//   - could-not-read reported as SIGNED grants authority nobody gave;
//   - could-not-read reported as UNSIGNED accuses a human of not deciding, and
//     sends the operator to go get a signature that already exists.
//
// The second is the one that actually happened. A line-oriented reader that
// looked only at the `**Sign-off:**` line itself reported every ruling in a
// fully signed register as UNSIGNED, because the register's house style puts
// the artifact URL on the line BELOW the prose. The tool went inert, and its
// message told operators the human had not ruled. Hence SignOffBlock: the unit
// of parsing is the block, not the line.
//
// # What is deliberately NOT decided here
//
// Whether the URL resolves, who authored the artifact, and whether the text
// says the words a particular lane needs. Those are network-and-identity
// questions and they belong to the caller's fetch-and-verify step. This reader
// answers exactly one question — what does the FILE claim — and a Granted
// result is the beginning of a check, never the end of one.

// SignOffState is the three-state result of reading a ruling's sign-off.
type SignOffState int

const (
	// SignOffCouldNotRead means the register could not be read in the shape
	// this reader understands. It is NOT a finding about the human. Nothing may
	// be authorized on it, and nothing may be reported as unsigned because of
	// it.
	SignOffCouldNotRead SignOffState = iota

	// SignOffUnsigned means the register was read, the ruling's sign-off block
	// was found, and it carries no artifact. This is a positive determination:
	// the human has not granted this.
	SignOffUnsigned

	// SignOffGranted means the block names exactly one artifact URL. The URL is
	// a CLAIM to be fetched and its author verified — never authorization on
	// its own.
	SignOffGranted
)

func (s SignOffState) String() string {
	switch s {
	case SignOffGranted:
		return "granted"
	case SignOffUnsigned:
		return "unsigned"
	default:
		return "could-not-read"
	}
}

// SignOff is what the register claims about one ruling.
type SignOff struct {
	// RulingID is the id that was looked up, e.g. "R-1".
	RulingID string
	// State is the three-state determination.
	State SignOffState
	// URL is set only when State is SignOffGranted.
	URL string
	// Detail says why, in words a caller can put in front of an operator. It is
	// always populated for the two non-granted states.
	Detail string
	// Block is the raw sign-off text as read, joined with spaces. Empty when the
	// block was never found. Callers may quote it; they must not re-parse it.
	Block string
}

var (
	// signOffKeyRe matches the sign-off key and captures the rest of its line.
	signOffKeyRe = regexp.MustCompile(`(?i)^\s*\*\*Sign-?off:?\*\*\s*(.*)$`)
	// boldKeyRe matches any bold key line, which ENDS a sign-off block. Without
	// this a following "**Amends.** ..." paragraph would be swallowed into the
	// block and any URL it quotes would be read as the authorization.
	boldKeyRe = regexp.MustCompile(`^\s*\*\*[^*]+\*\*`)
	// signOffURLRe finds artifact URLs. Trailing punctuation is excluded so a
	// URL ending a sentence does not capture the full stop.
	signOffURLRe = regexp.MustCompile(`https://[^\s)\]>,"']+`)
)

// ReadSignOff answers what the rulings register at path claims about one ruling.
//
// An unreadable file is could-not-read and nothing else. It is the state most
// likely to be reached by accident — a caller invoked from the wrong working
// directory sees exactly this — and it is precisely the state that must not be
// allowed to masquerade as a finding about the human.
func ReadSignOff(path, rulingID string) SignOff {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SignOff{
			RulingID: rulingID,
			State:    SignOffCouldNotRead,
			Detail: "the rulings register at " + StripControl(path) + " could not be read (" +
				StripControl(err.Error()) + "). Nothing is established either way: an unreadable " +
				"register is not an unsigned one",
		}
	}
	return ReadSignOffFromText(string(raw), rulingID)
}

// ReadSignOffFromText answers what a rulings register's text claims about one
// ruling. It never returns an error: the "I could not tell" outcome is a state,
// because a caller that gets an error is free to ignore it and a caller that
// gets SignOffCouldNotRead still has to decide what to do about it.
//
// The block runs from the sign-off key to the first blank line, heading, or new
// bold key. Everything in between is one authorization statement, however the
// author chose to wrap it.
func ReadSignOffFromText(text, rulingID string) SignOff {
	out := SignOff{RulingID: rulingID}
	id := strings.TrimSpace(rulingID)
	if id == "" {
		out.Detail = "no ruling id was named, so there is nothing to look up"
		return out
	}

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	// Collect EVERY sign-off block inside the named section. Two of them is an
	// ambiguous register, not a reason to take the first: silently picking one
	// of two candidate authorizations is how a stale sign-off outlives the
	// ruling it signed.
	var blocks [][]string
	inSection := false
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if strings.HasPrefix(ln, "#") {
			// Section boundaries. Enter on the named ruling's heading and LEAVE
			// on the next heading of any level: without the leave arm a later
			// ruling's signed block reads as this one's, which turns "one ruling
			// was signed" into "all of them were".
			inSection = isRulingHeading(ln, id)
			continue
		}
		if !inSection {
			continue
		}
		m := signOffKeyRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		block := []string{strings.TrimSpace(m[1])}
		for j := i + 1; j < len(lines); j++ {
			nxt := lines[j]
			if strings.TrimSpace(nxt) == "" || strings.HasPrefix(nxt, "#") || boldKeyRe.MatchString(nxt) {
				break
			}
			block = append(block, strings.TrimSpace(nxt))
			i = j
		}
		blocks = append(blocks, block)
	}

	switch len(blocks) {
	case 0:
		out.Detail = "the register has no `## " + id + "` section carrying a `**Sign-off:**` line, " +
			"so it is not in the shape this reader understands. That is a fact about the FILE, " +
			"not about whether a human ruled — nothing may be authorized on it and nothing may be " +
			"reported as unsigned because of it"
		return out
	case 1:
	default:
		out.Detail = "the `## " + id + "` section carries " + itoa(len(blocks)) + " separate " +
			"`**Sign-off:**` lines. Which one authorizes is not decidable from the file, and " +
			"taking the first would silently pick one of several candidate authorizations"
		return out
	}

	block := strings.TrimSpace(strings.Join(blocks[0], " "))
	out.Block = block

	urls := signOffURLRe.FindAllString(block, -1)
	switch len(urls) {
	case 1:
		out.State = SignOffGranted
		out.URL = urls[0]
		return out
	case 0:
		// The lesson, encoded. Prose that ends by promising an artifact — "…
		// approved:" — and then supplies none is a TRUNCATED read, not a
		// decision not to sign. Reporting it as unsigned would accuse a human of
		// not deciding on the strength of the reader's own blindness.
		if endsPromisingAnArtifact(block) {
			out.Detail = "the `" + id + "` sign-off block reads " + shortQuote(block) +
				" — it ends promising an artifact and none follows. That is a block this reader " +
				"could not finish, not a human who declined to sign"
			return out
		}
		out.State = SignOffUnsigned
		out.Detail = "`" + id + "` carries a sign-off line with no artifact URL, so the ruling is " +
			"UNSIGNED and grants nothing. This is the gate working, not an input to fix"
		return out
	default:
		out.Detail = "the `" + id + "` sign-off block names " + itoa(len(urls)) + " URLs, so which " +
			"artifact authorizes is not decidable from the file"
		return out
	}
}

// isRulingHeading reports whether a heading line opens the named ruling's
// section. It matches the id as a whole token so "R-1" never opens "R-12".
func isRulingHeading(line, id string) bool {
	h := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if !strings.HasPrefix(h, id) {
		return false
	}
	rest := h[len(id):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// endsPromisingAnArtifact reports whether a URL-less block reads as cut off
// rather than as a deliberate blank.
func endsPromisingAnArtifact(block string) bool {
	b := strings.TrimSpace(block)
	if b == "" {
		return false
	}
	for _, suffix := range []string{":", "—", "–", "-", "…"} {
		if strings.HasSuffix(b, suffix) {
			return true
		}
	}
	// A block that names the scheme but no host is a URL this reader failed to
	// finish reading, whatever produced it.
	return strings.Contains(b, "https:") || strings.Contains(b, "http:")
}

// shortQuote renders a block for an operator message without letting an
// arbitrarily long or control-laden register control the output.
func shortQuote(s string) string {
	s = StripControl(strings.TrimSpace(s))
	const max = 120
	if len(s) > max {
		s = s[:max] + "…"
	}
	return "\"" + s + "\""
}

// itoa avoids pulling strconv in for two call sites of small positive ints.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
