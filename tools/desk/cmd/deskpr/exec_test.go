package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestGithubCustodyMintRefusesWithoutMintedToken is the forge-op equivalent of the retired
// gh()-guard test: since the write-verbs-C migration deskpr reaches the forge through the
// resolver, and the GitHub custody minter it installs (github.go) hands over the token deskpr
// ALREADY minted (ghToken). With no minted token the minter must REFUSE — never fall back to an
// ambient forge identity — so a code path that reaches the forge before minting fails closed
// instead of writing as whatever credential happens to be active. The `--as-app=false` ambient
// fallback that used to make this a two-sided guard is retired outright.
func TestGithubCustodyMintRefusesWithoutMintedToken(t *testing.T) {
	old := ghToken
	ghToken = ""
	t.Cleanup(func() { ghToken = old })

	_, _, err := githubCustodyMint("worker", deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"})
	if err == nil {
		t.Fatal("githubCustodyMint returned no error with no minted token — it must refuse rather than fall " +
			"back to an ambient forge identity/keyring")
	}
	if !strings.Contains(err.Error(), "minted") {
		t.Fatalf("custody refusal = %q, want it to name the missing minted token", err.Error())
	}
}

// TestGithubCustodyMintHandsMintedToken proves the other side: once a token is minted, the
// custody step hands exactly it (and the test-only base override) to the resolver.
func TestGithubCustodyMintHandsMintedToken(t *testing.T) {
	oldTok, oldBase := ghToken, forgeAPIBase
	ghToken = "worker-token-xyz"
	forgeAPIBase = "https://forge.example"
	t.Cleanup(func() { ghToken, forgeAPIBase = oldTok, oldBase })

	tok, base, err := githubCustodyMint("worker", deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"})
	if err != nil {
		t.Fatalf("custody mint refused a minted token: %v", err)
	}
	if tok != "worker-token-xyz" {
		t.Errorf("custody token = %q, want the minted value", tok)
	}
	if base != "https://forge.example" {
		t.Errorf("custody base = %q, want the test override read at call time", base)
	}
}
