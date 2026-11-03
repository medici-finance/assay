package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/custody"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/local"
	"github.com/medici-finance/assay/deskd/internal/mcp"
)

type boundaryCredential struct{}

func (boundaryCredential) Present() bool { return true }

func (boundaryCredential) Token() (string, error) { panic("disabled writer read a token") }
func (boundaryCredential) TokenActor() string     { return "operator" }

// regression: #3669
func TestHumanPolicyDisabledByDefault(t *testing.T) {
	if os.Getenv("DESKD_TEST_POLICY_DISABLED") != "1" {
		for _, mode := range []string{"absent", "empty"} {
			cmd := exec.Command(os.Args[0], "-test.run=^TestHumanPolicyDisabledByDefault$")
			cmd.Env = append(os.Environ(), "DESKD_TEST_POLICY_DISABLED=1", "DESKD_TEST_POLICY_MODE="+mode)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s: %v\n%s", mode, err, out)
			}
		}
		return
	}
	if os.Getenv("DESKD_TEST_POLICY_MODE") == "empty" {
		if err := humanid.Configure(""); err != nil {
			t.Fatal(err)
		}
	}
	if humanid.Enabled() || humanid.Login() != "" {
		t.Fatal("default policy enabled")
	}
	if (&schema.Cell{AnswerAs: "operator"}).AnswerAsLogin() != "" {
		t.Fatal("cell configuration bypassed disabled policy")
	}
	for _, login := range []string{"", "operator", "other-user"} {
		if humanid.Matches(login) {
			t.Fatalf("disabled policy matched %q", login)
		}
		if _, err := custody.FromToken("fixture-token", login); err == nil {
			t.Fatalf("custody accepted %q", login)
		}
		if _, err := answer.NewSessionStore(nil, func() bool { return true }).BeginAsPresent(login, true); err == nil {
			t.Fatalf("session constructor accepted %q", login)
		}
		if got := identity.NewDeriver(nil, nil).Capability(context.Background(), &identity.Session{Login: login}, "example-org", "repo", login); got.IsAnswerer() {
			t.Fatalf("capability elevated %q", login)
		}
	}
	w := answer.NewHumanWriter("http://invalid.example", boundaryCredential{})
	ref := answer.Ref{Repo: "example-org/repo", Number: 1}
	if err := w.Comment(context.Background(), ref, "test"); err != humanid.ErrDisabled {
		t.Fatalf("writer bypass: %v", err)
	}
	service := answer.NewService(w, nil, nil, nil)
	service.SetAnswerAs("operator")
	if err := service.SaveDraft(&answer.Session{Actor: "operator", ExpiresAt: time.Now().Add(time.Hour)}, answer.Draft{}); err != humanid.ErrDisabled {
		t.Fatalf("service bypass: %v", err)
	}
	// No DB is needed: refusal must precede the storage dereference.
	if err := new(index.DB).SetRead("fixture", "operator", time.Now()); err != humanid.ErrDisabled {
		t.Fatalf("local write bypass: %v", err)
	}
	report := mcp.AuthorityReport([]mcp.Tool{{Name: "read"}})
	if !report.Clean() {
		t.Fatalf("blank disabled identity misclassified a read: %+v", report)
	}
	if mcp.AuthorityReport([]mcp.Tool{{Name: "write", Mutates: true}}).Clean() {
		t.Fatal("blank author allowed a mutation")
	}
}

// regression: #3669
func TestHumanPolicyExplicitPrincipal(t *testing.T) {
	if humanid.Login() != "operator" || !humanid.Matches("operator") {
		t.Fatal("explicit startup policy lost")
	}
	for _, wrong := range []string{"", " ", "other-user"} {
		if humanid.Matches(wrong) {
			t.Fatalf("matched wrong principal %q", wrong)
		}
		if _, err := custody.FromToken("fixture-token", wrong); err == nil {
			t.Fatalf("custody accepted %q", wrong)
		}
	}
	for _, login := range []string{"operator", "OPERATOR", " operator "} {
		c, err := custody.FromToken("fixture-token", login)
		if err != nil || c.TokenActor() != "operator" {
			t.Fatalf("source-compatible canonical custody for %q: %v", login, err)
		}
	}
	if got := (&schema.Cell{}).AnswerAsLogin(); got != "operator" {
		t.Fatalf("default principal = %q", got)
	}
	if got := (&schema.Cell{AnswerAs: "cell-owner"}).AnswerAsLogin(); got != "cell-owner" {
		t.Fatalf("cell-specific authorization changed: %q", got)
	}
	if err := humanid.Configure("other-user"); err == nil || humanid.Login() != "operator" {
		t.Fatal("policy can be replaced after startup")
	}
}

// regression: #3669
func TestHumanPolicySelectedPrincipalFlows(t *testing.T) {
	const selected = "second-operator"
	if os.Getenv("DESKD_TEST_POLICY_LOGIN") != selected {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHumanPolicySelectedPrincipalFlows$")
		cmd.Env = append(os.Environ(), "DESKD_TEST_POLICY_LOGIN="+selected)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("selected principal flow: %v\n%s", err, out)
		}
		return
	}
	values := map[string]string{
		"owner": humanid.Login(), "schema": schema.DefaultAnswerAs(), "cell": (&schema.Cell{}).AnswerAsLogin(),
		"custody": custody.Actor(), "writer": answer.NewPerRequestHumanWriter("").Actor(),
		"MCP": mcp.HumanIdentity(), "index": index.DefaultActor(), "local": local.New(nil).Actor,
	}
	for consumer, got := range values {
		if got != selected {
			t.Errorf("%s principal = %q, want %q", consumer, got, selected)
		}
	}
	if _, err := custody.FromToken("fixture-token", "operator"); err == nil {
		t.Fatal("previous synthetic principal retained authority")
	}
	c, err := custody.FromToken("fixture-token", selected)
	if err != nil || c.TokenActor() != selected {
		t.Fatalf("selected custody = %v, %v", c, err)
	}
}
