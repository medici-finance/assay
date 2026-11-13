package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

func foreignSignedGrant(t *testing.T, f *authorityFixture) SignedGrant {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	message := append([]byte("forge-local-grant-v1\n"), f.grant.Payload...)
	foreign := SignedGrant{Payload: f.grant.Payload, Signature: ed25519.Sign(key, message)}
	if ed25519.Verify(f.verifierConfig.Bootstrap.PublicKey, message, foreign.Signature) {
		t.Fatal("fixture grant unexpectedly validates under the trusted key")
	}
	return foreign
}

func TestForgeLocalAuthorityProducerGrantSignature(t *testing.T) {
	f := newAuthorityFixture(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	probe := CurrentProbe{
		Schema: Schema, Instance: f.verifierConfig.Bootstrap.Instance,
		Challenge: bytes.Repeat([]byte{1}, 32), Grant: f.grant,
	}
	// Probe the producer directly so its signature check stands on its own.
	if _, err := f.client.probe(ctx, probe); err != nil {
		t.Fatalf("genuine grant did not receive a current proof: %v", err)
	}
	probe.Grant = foreignSignedGrant(t, f)
	proof, err := f.client.probe(ctx, probe)
	if !errors.Is(err, ErrHeld) || len(proof.Payload) != 0 || len(proof.Signature) != 0 {
		t.Fatalf("producer did not hold the foreign grant signature: %v", err)
	}
}

func TestForgeLocalAuthorityVerifierGrantSignature(t *testing.T) {
	f := newAuthorityFixture(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	scope := f.config.Launches[0].Scope.Scope

	// A test peer signs a valid proof over the submitted digest. The producer
	// builds the rest of the proof from the genuine registered grant.
	reply := func(probe CurrentProbe) (SignedProof, error) {
		digest, err := grantDigest(probe.Grant, f.config.MaxFrameBytes)
		if err != nil {
			return SignedProof{}, err
		}
		probe.Grant = f.grant
		signed, err := f.controller.current(ctx, probe)
		if err != nil {
			return SignedProof{}, err
		}
		var proof CurrentProof
		if err := canonicalDecode(signed.Payload, f.config.MaxFrameBytes, &proof); err != nil {
			return SignedProof{}, err
		}
		proof.GrantDigest = digest
		payload, err := encode(proof, f.config.MaxFrameBytes)
		if err != nil {
			return SignedProof{}, err
		}
		sig, err := f.controller.signer.Sign(append([]byte("forge-local-current-v1\n"), payload...))
		return SignedProof{Payload: payload, Signature: sig}, err
	}

	t.Run("genuine-grant", func(t *testing.T) {
		verifier := currentFixtureProxy(t, f, reply)
		read, err := verifier.Authorize(ctx, f.grant, scope)
		if err != nil || read.Actor() != f.config.Launches[0].Scope.Actor {
			t.Fatalf("fixture proof did not release the genuine grant: %v", err)
		}
	})
	t.Run("foreign-signature", func(t *testing.T) {
		verifier := currentFixtureProxy(t, f, reply)
		read, err := verifier.Authorize(ctx, foreignSignedGrant(t, f), scope)
		assertHeld(t, read, err)
	})
}
