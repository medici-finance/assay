package deskkit

import "testing"

func TestNativeReadBackendCustodyNegative(t *testing.T) {
	plantAmbientCredentials(t)
	repo := ForgeRepo{Owner: "example", Name: "repo"}
	for _, backend := range []string{"github", "gitlab"} {
		t.Run(backend, func(t *testing.T) {
			gh, gl := newGoldenServer(t), newGLServer(t)
			var f Forge
			if backend == "github" {
				g := gh.forge()
				g.Token = ""
				f = g
			} else {
				g := gl.forge()
				g.Token = ""
				f = g
			}
			_, err := f.GetPullRequest(repo, 7)
			if err == nil || ExitCodeOf(err) != ExitUnverifiable {
				t.Fatalf("unminted read admitted or misclassified: %v", err)
			}
			if len(gh.requests)+len(gl.requests) != 0 {
				t.Fatal("unminted read reached wire")
			}
		})
	}
}
