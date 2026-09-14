package deskkit

import "testing"

// TestListCommentsResuffixesBotAuthor pins #747: a GraphQL Bot actor
// reports the BARE slug as its login (`assay-worker-app`), NOT the `<slug>[bot]` REST
// rendering RoleAppLogin returns and SameActor folds against. ListComments must re-suffix a
// Bot author to that rendering; before the fix a worker's own workpad comment never matched
// its own identity, so `deskreply --workpad`'s candidate filter found nothing and appended a
// second comment on every call instead of editing in place.
//
// This is fail-first evidence: on the UNFIXED ListComments (which passed the GraphQL login
// through unchanged) got[0].Author.Login stays the bare "assay-worker-app", the SameActor
// round trip returns false, and both assertions below fail.
func TestListCommentsResuffixesBotAuthor(t *testing.T) {
	s := newGoldenServer(t)
	// The nodes are shaped as GitHub's GraphQL API really renders them: a Bot actor's `login`
	// is the bare slug with a `__typename` of "Bot"; a human's is their login with "User".
	s.graphql = map[string]any{"data": map[string]any{"repository": map[string]any{
		"pullRequest": map[string]any{"comments": map[string]any{"nodes": []map[string]any{
			{"id": "IC_1", "databaseId": 11, "body": "<!-- assay:workpad -->", "isMinimized": false,
				"createdAt": "2026-08-24T00:00:00Z", "url": "https://example/pull/7#issuecomment-11",
				"author": map[string]any{"login": "assay-worker-app", "__typename": "Bot"}},
			{"id": "IC_2", "databaseId": 12, "body": "a human note", "isMinimized": false,
				"createdAt": "2026-08-24T00:01:00Z", "url": "https://example/pull/7#issuecomment-12",
				"author": map[string]any{"login": "some-human", "__typename": "User"}},
		}}},
	}}}

	got, err := s.forge().ListComments(forgeTestRepo, 7)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 comments, got %d", len(got))
	}

	// The Bot author is re-suffixed to the REST rendering an identity comparison expects.
	if got[0].Author.Login != "assay-worker-app[bot]" {
		t.Errorf("bot author login: want %q, got %q", "assay-worker-app[bot]", got[0].Author.Login)
	}
	// And that rendering folds against RoleAppLogin's "<slug>[bot]" through SameActor, so the
	// worker finds its OWN comment — the exact round trip the appended-workpad bug broke.
	if !SameActor(got[0].Author.Login, "assay-worker-app[bot]") {
		t.Errorf("SameActor(%q, %q) = false; the workpad candidate filter would miss the worker's own comment",
			got[0].Author.Login, "assay-worker-app[bot]")
	}
	// A human author is left exactly as GitHub reported it — the App affix is never invented,
	// so a look-alike human comment can never be mistaken for the bot's own workpad.
	if got[1].Author.Login != "some-human" {
		t.Errorf("human author login: want %q, got %q", "some-human", got[1].Author.Login)
	}
}
