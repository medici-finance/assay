package main

// packHeader is the honest framing printed at the top of every generated .md.
// It carries the same non-claim register as docs/evidence-bundle.md: it says
// what the pack IS (a record of demonstrations performed at one release), what
// it is NOT (an audit opinion, a conformance statement, or coverage of any
// control outside the declared set), and the two things it deliberately does
// not carry — a re-run interval, and any rule for what happens to work already
// verified by a control later found blind. This wording lives in source, not in
// the hand of whoever writes the release notes, so it cannot be quietly dropped.
const packHeader = "" +
	"> **What this is.** A record of the mutation demonstrations the release ran for the\n" +
	"> declared refusal gates below: for each gate, an injected instance of the error it\n" +
	"> claims to stop, and whether the test suite caught it — the demonstration\n" +
	"> `docs/mistake-proofing.md` §3 D1 requires, captured as at this release rather than\n" +
	"> left to expire in a job log.\n" +
	">\n" +
	"> **What this is not.** It is **not an audit opinion** and it does **not** state that\n" +
	"> anything conforms to any standard. It is an *input to* a review, not a compliance\n" +
	"> artifact in itself. It does **not** claim coverage of any control outside the\n" +
	"> declared set — see the scope boundary and the drift section below for what it does\n" +
	"> not cover. It does **not** assess whether a gate's mutations are the RIGHT mutations,\n" +
	"> only whether the ones declared reddened their suite.\n" +
	">\n" +
	"> **Two things it deliberately does not carry.** (1) No re-run interval: a\n" +
	"> demonstration recorded here is a demonstration *as at this release*, not a standing\n" +
	"> guarantee that the gate still fires today. (2) No rule for prior results: it does\n" +
	"> **not** state what happens to work already verified by a gate that a later release\n" +
	"> finds blind. Both are out of scope for a record; both belong to the process that\n" +
	"> reads it.\n"

// packScope is the D6 scope boundary — a maintained, visible statement of what
// the pack does NOT check, kept beside the checks it describes.
const packScope = "" +
	"This pack covers **only** the desk-tools refusal gates the release mutates (the\n" +
	"declared set below). It does **not** cover the lint rule set, the workflow-level\n" +
	"checks, brief-authored rule-16 demonstrations, or any other control in the system.\n" +
	"A mutation spec present in `tools/desk` but absent from the declared set is listed\n" +
	"under *Declared-set drift* below, so the boundary is visible rather than assumed.\n"
