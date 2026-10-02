package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAmbientDecoyMatrix(t *testing.T) {
	for _, backend := range []string{"github", "gitlab"} {
		t.Run(backend, func(t *testing.T) {
			home := t.TempDir()
			for _, k := range []string{"HOME", "USERPROFILE", "APPDATA", "GH_CONFIG_DIR", "GLAB_CONFIG_DIR"} {
				d := filepath.Join(home, k)
				if err := os.MkdirAll(d, 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv(k, d)
				if err := os.WriteFile(filepath.Join(d, "hosts.yml"), []byte("oauth_token: decoy-config"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN", "CI_JOB_TOKEN"} {
				t.Setenv(k, "decoy-"+k)
			}
			bin := t.TempDir()
			marker := filepath.Join(bin, "called")
			for _, name := range []string{"gh", "glab"} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\necho executed > \""+marker+"\"\nexit 77\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				auth := r.Header.Get("Authorization")
				if backend == "gitlab" {
					auth = r.Header.Get("PRIVATE-TOKEN")
				}
				if !strings.Contains(auth, "synthetic-minted") || strings.Contains(auth, "decoy") {
					t.Errorf("request used unassigned credential")
				}
				w.Header().Set("Content-Type", "application/json")
				if backend == "github" {
					json.NewEncoder(w).Encode(map[string]any{"number": 7, "state": "open", "head": map[string]any{"sha": "example-head"}})
				} else {
					json.NewEncoder(w).Encode(map[string]any{"iid": 7, "state": "opened", "sha": "example-head", "title": "example", "source_branch": "example", "changes_count": "0"})
				}
			}))
			defer srv.Close()
			repo := ForgeRepo{Owner: "example", Name: "repo"}
			var f Forge
			if backend == "github" {
				f = &GitHubForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: srv.Client()}
			} else {
				f = &GitLabForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: srv.Client()}
			}
			if _, err := f.GetPullRequest(repo, 7); err != nil {
				t.Fatal(err)
			}
			if requests == 0 {
				t.Fatal("no credential observed on wire")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("forge subprocess executed")
			}
		})
	}
}
