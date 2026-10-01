package deskkit

import (
	"strings"
	"testing"
)

func TestOutboundGitLabTypedNotes(t *testing.T) {
	obRoster(t)
	for _, slug := range []string{obPublic, obInternal} {
		t.Run(slug, func(t *testing.T) {
			s := newGLServer(t)
			s.project = map[string]any{"visibility": "internal"}
			f := OutboundChecked(s.forge(), "worker")
			_, err := f.PostCommentTyped(obRepo(slug), 7, TargetChange, "example note about "+obWithheld)
			if slug == obPublic {
				if err == nil || !IsRefused(err) || len(s.requests) != 0 {
					t.Fatalf("refusal leaked a write: %v requests=%v", err, s.requests)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if len(s.requests) != 1 || !strings.Contains(s.requests[0].Path, "/merge_requests/7/notes") {
					t.Fatalf("typed internal note resolved another object: %v", s.requests)
				}
			}
		})
	}
}

func TestGitLabPushRewriteRefusesSSH(t *testing.T) {
	cases := []struct{ name, base, ssh string }{
		{"self_managed", "https://gitlab.example/", "git@gitlab.example:"},
		{"non22_port", "https://gitlab.example/", "ssh://git@gitlab.example:2222/"},
		{"nested_subgroup", "https://gitlab.example/", "git@gitlab.example:"},
		{"oauth2_user", "https://oauth2@gitlab.example/", "git@gitlab.example:"},
	}
	for _, mode := range []string{"insteadOf", "pushInsteadOf"} {
		for _, c := range cases {
			t.Run(mode+"/"+c.name, func(t *testing.T) {
				t.Setenv("DESK_LOOP", "worker-desk")
				dir := rewriteRepo(t)
				suffix := "group/sub/project.git"
				rewriteGit(t, "-C", dir, "config", "remote.origin.url", c.base+suffix)
				rewriteGit(t, "-C", dir, "config", "url."+c.ssh+"."+mode, c.base)
				resolved := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin"))
				if resolved != c.ssh+suffix {
					t.Fatalf("git oracle=%q", resolved)
				}
				err := CheckPushTransport(realGitGateInput(dir))
				if err == nil || ExitCodeOf(err) != ExitRefused {
					t.Fatalf("GitLab SSH rewrite admitted: %v", err)
				}
			})
		}
	}
}

func TestOutboundGitLabNoReply(t *testing.T) {
	obRoster(t)
	for _, host := range []string{"gitlab.com", "gitlab.example"} {
		t.Run(host, func(t *testing.T) {
			s := newGLServer(t)
			f := OutboundChecked(s.forge(), "worker")
			body := "example attribution 123-example" + "@" + "users.noreply." + host
			_, err := f.PostCommentTyped(obRepo(obPublic), 7, TargetChange, body)
			if err != nil || len(s.requests) != 1 {
				t.Fatalf("non-personal no-reply note refused: %v requests=%v", err, s.requests)
			}
		})
	}
}
