package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// authority.go — the two fetch-and-verify gates. Nothing in this file trusts a flag,
// a login string handed to it, or a file the caller can write. Each gate ends in a
// GitHub read whose AUTHOR is checked against the roster's pinned blessing authority.

// isBotOrAppLogin reports whether a rendered forge login belongs to a bot/App rather than a
// human — the seam's stand-in for the REST `type != "User"` check. GitHub renders an App/Bot
// author as `<slug>[bot]` (and, in some surfaces, `app/<slug>`); both are excluded.
func isBotOrAppLogin(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	return l == "" || strings.HasSuffix(l, "[bot]") || strings.HasPrefix(l, "app/")
}

// defaultRulingsPath is where R-1 lives in this repo. It is a path, not a URL: the
// FILE states the claim ("R-1 was signed, here is the artifact"), and the artifact is
// what is verified. Overriding the path only changes which URL gets fetched.
const defaultRulingsPath = "docs/streams/issue-flow/rulings.md"

// rulingID is the ruling that grants the close lanes. deskclose implements ONE ruling
// and names it in every close comment it writes, so a reader of a closed issue can go
// read the authority rather than take the tool's word for it.
const rulingID = "R-1"

// supersedeMarker is what R-1's sign-off must say to unlock the duplicate lane.
//
// R-1's own conflict disclosure sets this bar: the duplicate close is granted by the
// two-role duplicate procedure to a strong-tier worker plus a reviewer, and the
// close-grant ruling states explicitly that duplicates are NOT covered by the desk's
// close grant. R-1 therefore says, in terms, that deskclose "must refuse duplicate
// closes", and that moving the lane to the desk "is an EXPLICIT supersession of the
// two-role ruling ... and should be stated as such in the sign-off, not inferred from
// acceptance of this ruling".
//
// So the unlock is not "R-1 is signed". It is "R-1's sign-off comment says the words".
// Accepting R-1 as written must never widen the desk's authority past what the human
// said, and silence is not consent — the widening has to be visible in the text being
// signed.
const supersedeMarker = "supersedes the two-role duplicate procedure"

// grant is the verified outcome of the ruling gate: the human artifact that authorizes
// this run, already fetched and already checked.
type grant struct {
	// SignOffURL is the artifact URL recorded on R-1's Sign-off line. Every close
	// comment cites it.
	SignOffURL string
	// AuthorLogin is the verified author of that artifact.
	AuthorLogin string
	// DuplicateLaneGranted is true only when the sign-off text explicitly supersedes
	// the two-role duplicate procedure.
	DuplicateLaneGranted bool
}

// commentURLRe parses a GitHub issue/PR comment permalink into (owner, repo, kind,
// item number, comment id). Anchorless URLs do not match: a link to a whole THREAD is
// not an authorization, because a thread's content is written by whoever shows up.
var commentURLRe = regexp.MustCompile(
	`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/(issues|pull)/(\d+)#issuecomment-(\d+)$`)

// readRulingSignOff extracts R-1's sign-off URL from the rulings file.
//
// Three states, and they are different answers:
//   - the file or the section cannot be read      → Unverifiable (6), could-not-check
//   - the section is there and the block is EMPTY → Refused (5), the ruling is unsigned
//   - a URL is present                            → returned, to be FETCHED next
//
// An unsigned ruling is a positive determination ("the human has not granted this"),
// which is why it is a refusal rather than could-not-check. An unreadable file is the
// opposite epistemic state and must never be reported as either signed or unsigned.
//
// The determination itself is deskkit.ReadSignOff, and deliberately not a second copy
// of it here. The reason is a measured failure, not tidiness: the reader this function
// used to carry looked only at the `**Sign-off:**` LINE, while the register's house
// style puts the prose on that line and the artifact URL on the line below. Against the
// live register — five rulings, all five signed — it returned "UNSIGNED, the desk holds
// no authority" for every one of them. A false UNSIGNED is worse than a silent one: it
// is a positive claim, in the tool's own voice, that a human did not decide something
// they did decide. deskclose keeps its policy (which ruling, which exit code, what the
// operator is told); the shape of the register is read in exactly one place.
func readRulingSignOff(path string) (string, error) {
	so := deskkit.ReadSignOff(path, rulingID)
	switch so.State {
	case deskkit.SignOffGranted:
		return so.URL, nil
	case deskkit.SignOffUnsigned:
		return "", deskkit.Refused(
			"refused: " + so.Detail + " (" + deskkit.StripControl(path) + ") — the delegated close " +
				"lanes are UNSIGNED, so the desk holds no authority to close anything. " +
				"deskclose is inert until a human records an acceptance artifact on that line. " +
				"This is not a bug to route around: it is the gate working.")
	default:
		return "", deskkit.Unverifiable(
			"could-not-check: "+so.Detail+" — deskclose cannot establish that any close lane was "+
				"granted, so it closes nothing. This is NOT a finding that the ruling is unsigned "+
				"(pass --rulings <path> if your checkout root differs)", nil)
	}
}

// ghComment is the subset of a comment the authorization check consumes. It is populated from
// Forge.ListComments (which carries the author's login AND — since the write-verbs-C extension —
// numeric id, the pair the blessing-authority pin requires).
type ghComment struct {
	ID      int64  // the comment's numeric (database) id
	HTMLURL string
	Body    string
	User    struct {
		Login string
		ID    int64
	}
}

// fetchComment retrieves the comment a permalink names, through the resolved forge.
//
// The permalink gives the OWNER/REPO, the ITEM number and the comment id. deskclose lists the
// comments on THAT item (ListComments) and finds the one whose database id matches — so the
// permalink's item and the comment's actual thread are the SAME by construction (the old REST
// path fetched the comment by id and then cross-checked its issue_url; listing on the named item
// makes the cross-check inherent: a comment id that is not on the named item is refused).
//
// Fail-closed in every direction: an unparseable URL is refused, and a fetch that does not come
// back is Unverifiable (exit 6) with ZERO closes performed. COULD-NOT-CHECK IS NOT AUTHORIZATION.
func fetchComment(url string) (ghComment, error) {
	m := commentURLRe.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return ghComment{}, deskkit.Refused(
			"refused: " + deskkit.StripControl(url) + " is not a GitHub comment permalink " +
				"(want https://github.com/<owner>/<repo>/issues|pull/<N>#issuecomment-<id>). " +
				"A link to a thread is not an authorization: a thread is written by whoever shows up.")
	}
	owner, repo, itemStr, cidStr := m[1], m[2], m[4], m[5]
	itemN, ierr := strconv.Atoi(itemStr)
	if ierr != nil || itemN <= 0 {
		return ghComment{}, deskkit.Refused("refused: " + deskkit.StripControl(url) + " names an invalid item number")
	}
	cid, cerr := strconv.ParseInt(cidStr, 10, 64)
	if cerr != nil || cid <= 0 {
		return ghComment{}, deskkit.Refused("refused: " + deskkit.StripControl(url) + " names an invalid comment id")
	}
	fg, fr, ferr := forgeForFn(owner + "/" + repo)
	if ferr != nil {
		return ghComment{}, ferr
	}
	comments, err := fg.ListComments(fr, itemN)
	if err != nil {
		return ghComment{}, deskkit.Unverifiable(
			"could-not-check: the authorizing comment at "+deskkit.StripControl(url)+
				" could not be fetched — deskclose refuses. An unreadable authorization is not an "+
				"authorization; zero items were closed", err)
	}
	for _, c := range comments {
		if c.DatabaseID == cid {
			out := ghComment{ID: c.DatabaseID, HTMLURL: c.URL, Body: c.Body}
			out.User.Login = c.Author.Login
			out.User.ID = c.Author.ID
			if out.HTMLURL == "" {
				out.HTMLURL = deskkit.StripControl(url) // GitLab notes have no permalink; keep the caller's.
			}
			return out, nil
		}
	}
	return ghComment{}, deskkit.Refused(
		"refused: the authorization permalink names comment " + deskkit.StripControl(cidStr) +
			" on " + deskkit.StripControl(owner+"/"+repo+"#"+itemStr) +
			", but no such comment is on that item — the link and the artifact disagree, or the comment was deleted")
}

// verifyHumanAuthor is the load-bearing identity check, and it is deliberately made of
// two independent conditions rather than one.
//
// A shared automation account reports "type": "User" exactly as a person does, so the
// type check alone would admit it. And a login is a claim about a name, so the login
// check alone would admit a recycled or squatted one. The pair required together:
//
//	type == "User"                          — an App/Bot artifact is never authorization
//	IsBlessAuthorityIDStrict(login, id)     — the ONE roster-pinned human, login AND id
//
// IsBlessAuthorityIDStrict is the strict form on purpose: it fails closed on a missing
// numeric id. Login and id are parsed from the SAME payload here, so an absent id means
// the read was wrong — never "this surface has no ids".
//
// Note what is NOT here: no allowlist of automation accounts, and no "trusted author"
// fallback. deskkit.TrustedAuthor is a much wider set (it contains every desk App and
// the shared automation identity), and using it here would let the desk authorize its
// own batch. The blessing authority is a single human account, and the roster loader
// refuses to accept an App as one.
func verifyHumanAuthor(c ghComment, what string) error {
	who := deskkit.StripControl(c.User.Login)
	// App/Bot exclusion. The REST `type == "User"` field is not on the forge seam; a bot
	// identity is represented instead by its RENDERED login — `<slug>[bot]` / `app/<slug>` on
	// GitHub, which ListComments emits (the same rendering RoleAppLogin and the trust set use).
	// So a bot/App artifact is excluded by its login shape here. This is the SECOND layer; the
	// id-pin below is the primary control, and on a forge whose bot rendering this shape check
	// does not catch, the id-pin alone still fails the batch closed — a bot's numeric id is
	// never the roster-pinned human's.
	if isBotOrAppLogin(c.User.Login) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s is authored by %s, a bot/App rendered login — an App or Bot artifact is never a "+
				"human authorization, whatever permissions it holds", what, who))
	}
	if !deskkit.IsBlessAuthorityIDStrict(c.User.Login, c.User.ID) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s is authored by %s (id %d), which is not the configured blessing authority. "+
				"Reporting type=User is not enough: the desk, the workers and the human share automation "+
				"identities that also report type=User, so login alone cannot gate a batch close. "+
				"The authorizing artifact must be authored by the single roster-pinned human account.",
			what, who, c.User.ID))
	}
	return nil
}

// authorize runs the ruling gate: read the claim, fetch the artifact, verify the
// author. It returns the grant every close comment cites.
func authorize(rulingsPath string) (grant, error) {
	url, err := readRulingSignOff(rulingsPath)
	if err != nil {
		return grant{}, err
	}
	c, err := fetchComment(url)
	if err != nil {
		return grant{}, err
	}
	if err := verifyHumanAuthor(c, rulingID+"'s sign-off artifact"); err != nil {
		return grant{}, err
	}
	return grant{
		SignOffURL:           c.HTMLURL,
		AuthorLogin:          c.User.Login,
		DuplicateLaneGranted: strings.Contains(strings.ToLower(c.Body), supersedeMarker),
	}, nil
}

// authorizeManifest runs the manifest gate on top of an already-verified ruling grant.
//
// The extra property over the ruling gate: the authorization binds to THIS EXACT ROW
// SET. The manifest declares its own sha256, deskclose recomputes it from the rows, and
// the authorizing comment's body must contain that digest. Consequences, all wanted:
//
//   - a row added, retargeted or re-moded after the human looked changes the digest, so
//     the old authorization stops matching and the run refuses;
//   - an unrelated older comment by the same human cannot be replayed as authorization
//     for a manifest they never saw.
//
// This is why there is no --force. The escape hatch for "the human has not approved
// this batch" is the human approving the batch.
func authorizeManifest(m *manifest) (ghComment, error) {
	c, err := fetchComment(m.AuthorizedBy)
	if err != nil {
		return ghComment{}, err
	}
	if err := verifyHumanAuthor(c, "the manifest authorization"); err != nil {
		return ghComment{}, err
	}
	computed := m.Digest()
	if m.DeclaredDigest != "" && !strings.EqualFold(m.DeclaredDigest, computed) {
		return ghComment{}, deskkit.Refused(fmt.Sprintf(
			"refused: manifest declares sha256 %s but its rows hash to %s — the file changed after it "+
				"declared its digest", deskkit.StripControl(m.DeclaredDigest), computed))
	}
	if !strings.Contains(strings.ToLower(c.Body), strings.ToLower(computed)) {
		return ghComment{}, deskkit.Refused(fmt.Sprintf(
			"refused: the authorizing comment does not carry this manifest's digest (sha256 %s), so it "+
				"authorizes some other row set — or none. Authorization binds to exact content: a row "+
				"added after the human looked must invalidate it. Ask for a comment containing:\n"+
				"    %s sha256:%s", computed, manifestApprovalPhrase, computed))
	}
	return c, nil
}

// manifestApprovalPhrase is the literal a human writes to approve a batch. It is a
// fixed phrase plus the digest so an approval is unambiguous to read and impossible to
// produce by accident.
const manifestApprovalPhrase = "deskclose-manifest approved"

// atoiPositive parses a positive item number, refusing everything else.
func atoiPositive(s, what string) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	if err != nil || n <= 0 {
		return 0, deskkit.Refused("refused: " + what + " must be a positive item number, got " +
			deskkit.StripControl(s))
	}
	return n, nil
}
