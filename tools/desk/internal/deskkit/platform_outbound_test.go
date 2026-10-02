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

// This checks the GitLab.com no-reply shape and a reserved documentation host.
// Generic self-managed host recognition is an unresolved production scope item on issue 1836.
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

// TestOutboundWindowsMachinePaths is the brief-10 Windows row (desktools-v2/12): the
// absolute-machine-path class recognises a drive-letter path under the Users root and a UNC
// path, each confirmed absolute by IsAbsFor("windows", …), and a public target refuses the
// write before any request leaves. A private target is not scanned for self-containment, and
// the bare roots — what documentation of the check has to spell — stay tolerated.
func TestOutboundWindowsMachinePaths(t *testing.T) {
	obRoster(t)
	bs := `\`
	refused := []struct{ name, path string }{
		{"drive_backslash", `C:` + bs + `Users` + bs + `example` + bs + `src` + bs + `notes.md`},
		{"drive_forward_slash", "D:/" + "Users/example/AppData/Local/Temp/x"},
		{"drive_lowercase", `c:` + bs + `users` + bs + `example`},
		{"unc", bs + bs + `fileserver` + bs + `share` + bs + `team` + bs + `doc.md`},
		{"unc_host_share_only", bs + bs + `fileserver` + bs + `share`},
	}
	for _, c := range refused {
		t.Run("public/"+c.name+"_refused", func(t *testing.T) {
			if !IsAbsFor("windows", c.path) {
				t.Fatalf("fixture %q is not a Windows absolute path", c.path)
			}
			s := newGLServer(t)
			f := OutboundChecked(s.forge(), "worker")
			_, err := f.PostCommentTyped(obRepo(obPublic), 7, TargetChange, "see "+c.path+" for the log")
			if err == nil || !IsRefused(err) || len(s.requests) != 0 {
				t.Fatalf("public write carrying %q: err=%v requests=%v", c.path, err, s.requests)
			}
			if !strings.Contains(err.Error(), "absolute machine path") {
				t.Fatalf("refusal does not name the class: %v", err)
			}
		})
		t.Run("private/"+c.name+"_passes", func(t *testing.T) {
			s := newGLServer(t)
			f := OutboundChecked(s.forge(), "worker")
			if _, err := f.PostCommentTyped(obRepo(obPrivate), 7, TargetChange, "see "+c.path+" for the log"); err != nil || len(s.requests) != 1 {
				t.Fatalf("private write refused: %v requests=%v", err, s.requests)
			}
		})
	}
	// Documentation of the check names the roots, never a machine: these carry no user,
	// host or share and must not refuse (the #380 lesson the POSIX roots already follow).
	for name, body := range map[string]string{
		"bare_drive_root": "the scan covers `C:" + bs + "Users" + bs + "` and its forward-slash form",
		"placeholder_unc": "and UNC paths (`" + bs + bs + "<host>" + bs + "<share>`)",
		"drive_relative":  "a drive-relative `C:rel" + bs + "x` is not absolute",
		"regex_escape":    "the pattern `" + bs + bs + "d" + bs + bs + "s` is a regex",
		"non_users_drive": "the installer lands in `C:" + bs + "Program Files" + bs + "Tool`",
		"url_not_unc":     "see https://example.com/a/b",
	} {
		t.Run("public/"+name+"_passes", func(t *testing.T) {
			s := newGLServer(t)
			f := OutboundChecked(s.forge(), "worker")
			if _, err := f.PostCommentTyped(obRepo(obPublic), 7, TargetChange, body); err != nil || len(s.requests) != 1 {
				t.Fatalf("documentation body refused: %v requests=%v", err, s.requests)
			}
		})
	}
}
