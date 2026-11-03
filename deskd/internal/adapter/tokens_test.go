package adapter_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// recordingAuthBase records the Authorization header of every request that
// reaches it — the observable proof that a client built from a REFRESHED
// token set is the one a fetch actually used (the service contract Verify row 5).
// Every response is a valid, empty issue-list page, so [adapter.Client] never
// falls back to a "list probe did not complete" could-not-check that would
// mask the property under test.
type recordingAuthBase struct {
	authHeaders []string
}

func (r *recordingAuthBase) RoundTrip(req *http.Request) (*http.Response, error) {
	r.authHeaders = append(r.authHeaders, req.Header.Get("Authorization"))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

// TestTokensDir proves the service contract reload contract:
//   - a token rotated between cycles is observed on the NEXT cycle, with no
//     restart (the seam the service contract's sidecar writes into);
//   - a missing token file for a configured org flips that org's poll to
//     could-not-check, naming the org and the path;
//   - none of that ever echoes the token VALUE, missing or present.
func TestTokensDir(t *testing.T) {
	repo := adapter.Repo{Owner: "example-org", Name: "console"}
	ctx := context.Background()

	t.Run("rotation observed between cycles", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "EXAMPLE_ORG")
		if err := os.WriteFile(path, []byte("tok-double-cycle-one\n"), 0o600); err != nil {
			t.Fatalf("seed token file: %v", err)
		}
		base := &recordingAuthBase{}
		router := adapter.NewReloadingRouter("https://api.github.com", adapter.FileTokenSource{Dir: dir}, []string{repo.Owner}, base)

		router.RefreshCycle(ctx)
		router.FetchEscalations(ctx, repo)
		afterCycleOne := len(base.authHeaders)
		if afterCycleOne == 0 {
			t.Fatalf("cycle 1 made no request through the router's client")
		}
		for _, h := range base.authHeaders[:afterCycleOne] {
			if h != "Bearer tok-double-cycle-one" {
				t.Fatalf("cycle 1 request carried Authorization %q, want the FIRST double", h)
			}
		}

		// Rotate the file — a sidecar (the service contract) rewriting it in
		// place — and refresh again, with NO deskd restart.
		if err := os.WriteFile(path, []byte("tok-double-cycle-two\n"), 0o600); err != nil {
			t.Fatalf("rotate token file: %v", err)
		}
		router.RefreshCycle(ctx)
		router.FetchEscalations(ctx, repo)
		if len(base.authHeaders) == afterCycleOne {
			t.Fatalf("cycle 2 made no request through the router's client")
		}
		for _, h := range base.authHeaders[afterCycleOne:] {
			if h != "Bearer tok-double-cycle-two" {
				t.Fatalf("cycle 2 request carried Authorization %q, want the SECOND double — rotation between cycles must be observed with no restart", h)
			}
		}
	})

	t.Run("missing file flips the org to could-not-check naming the org and the path", func(t *testing.T) {
		dir := t.TempDir() // no token file for the owner at all
		router := adapter.NewReloadingRouter("https://api.github.com", adapter.FileTokenSource{Dir: dir}, []string{repo.Owner}, &recordingAuthBase{})
		router.RefreshCycle(ctx)

		fr := router.FetchEscalations(ctx, repo)
		if fr.Health.Assessment.Confidence == domain.CheckedClean {
			t.Fatalf("a missing token file must yield could-not-check, got checked-clean")
		}
		reason := fr.Health.Assessment.Reason
		if !strings.Contains(reason, repo.Owner) {
			t.Fatalf("reason does not name the owner %q: %s", repo.Owner, reason)
		}
		wantPath := filepath.Join(dir, "EXAMPLE_ORG")
		if !strings.Contains(reason, wantPath) {
			t.Fatalf("reason does not name the token-file path %q: %s", wantPath, reason)
		}
	})

	t.Run("value never appears in the reason string", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "EXAMPLE_ORG")
		secret := "tok-double-should-never-leak"
		if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
			t.Fatalf("seed token file: %v", err)
		}
		router := adapter.NewReloadingRouter("https://api.github.com", adapter.FileTokenSource{Dir: dir}, []string{repo.Owner}, &recordingAuthBase{})
		router.RefreshCycle(ctx) // resolves the secret once — proves it CAN read it

		// Empty the file out (as if the sidecar rotated it away) and refresh
		// again: the reason built for THIS cycle's missing token must still
		// never echo the value that used to be there.
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("empty token file: %v", err)
		}
		router.RefreshCycle(ctx)
		fr := router.FetchEscalations(ctx, repo)
		if strings.Contains(fr.Health.Assessment.Reason, secret) {
			t.Fatalf("the could-not-check reason echoed the token value: %s", fr.Health.Assessment.Reason)
		}
	})
}
