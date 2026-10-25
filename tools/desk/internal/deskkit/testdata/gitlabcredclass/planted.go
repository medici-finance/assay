package planted

// Positive control for gitlabcredsites_test.go: a second instance of the class
// credential-forwarded-on-redirect — a GitLab client built with no redirect policy and a
// hand-built request carrying PRIVATE-TOKEN through a default client. Neither is in the
// reviewed register, so the guard must report both. This file is never compiled.

import (
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func plantedClient(tok string) (*gitlab.Client, error) {
	return gitlab.NewClient(tok)
}

func plantedHeader(tok string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://gitlab.example/api/v4/user", nil)
	req.Header.Set("Private-Token", tok)
	return http.DefaultClient.Do(req)
}
