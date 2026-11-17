package deskkit

import (
	"strings"
	"testing"
)

// regression: #6735
func TestForgeHostIdentityASCII(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "project"}
	roster := goldenRoster()
	roster[EnvRepoForges] = repo.Slug() + "=gitlab"
	withRoster(t, roster)

	for _, host := range []string{"g\u0130thub.com", "gitlab.\u212A.invalid", "host.\xff"} {
		if got, err := NormalizeForgeHost(host); err == nil || got != "" {
			t.Errorf("NormalizeForgeHost(%q) = %q, %v; want refusal", host, got, err)
		}
		if IsWellKnownForgeHost(host) {
			t.Errorf("recognized non-ASCII host %q", host)
		}
		for _, origin := range []string{"https://" + host + "/example/project", "git@" + host + ":example/project"} {
			if _, err := HostOfRemote(origin); err == nil {
				t.Errorf("parsed a non-ASCII host from %q", origin)
			}
		}
		if _, _, err := ForgeKindFromSlugAndHost(repo.Slug(), host); err == nil {
			t.Errorf("roster-selected forge accepted instance %q", host)
		}
		if _, err := resolveForgeKindWithHost(ForgeRepo{Owner: "example-org", Name: "unconfigured"}, host); err == nil {
			t.Errorf("inferred a forge from %q", host)
		}
		for _, kind := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
			if _, err := gitHostForKind(kind, "https://"+host+"/api", repo); err == nil {
				t.Errorf("accepted custody instance %q for %s", host, kind)
			}
		}
	}
	for _, bad := range []string{"user@host.invalid", "host.invalid/extra", "host.invalid:443"} {
		if _, _, err := ForgeKindFromSlugAndHost(repo.Slug(), bad); err == nil {
			t.Errorf("accepted non-bare instance %q", bad)
		}
	}
	for _, raw := range []string{"https://fixture-user@host.invalid:bad/example/project", "git@fixture-user@host.invalid:example/project"} {
		if _, err := HostOfRemote(raw); err == nil || strings.Contains(err.Error(), "fixture-user") {
			t.Error("malformed authority must be refused without echoing its userinfo")
		}
	}
	if _, err := HostOfRemote("https://github.com\u00A0"); err == nil {
		t.Error("non-ASCII whitespace was removed from a hostname")
	}
	if !IsWellKnownForgeHost("GitHub.COM") {
		t.Error("mixed-case ASCII host not recognized")
	}
	kind, host, err := ForgeKindFromSlugAndHost(repo.Slug(), "XN--EXAMPLE.INVALID")
	if err != nil || kind != ForgeGitLab || host != "xn--example.invalid" {
		t.Errorf("roster-selected ASCII instance = %s, %q, %v", kind, host, err)
	}
	if got, err := gitHostForKind(ForgeGitHub, "", repo); err != nil || got != "github.com" {
		t.Errorf("default GitHub instance = %q, %v", got, err)
	}
}
