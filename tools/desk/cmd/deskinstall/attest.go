package main

// Attestation verification — the second, independent check on every release
// asset, run after the sha256 pin check and before anything is placed.
//
// The sha256 pin proves the downloaded bytes are the bytes the reviewed
// manifest names. It cannot tell whether the bytes the manifest names were
// built by this repository's release workflow: a re-pin to a locally built or
// substituted binary passes it. The attestation check closes that gap. It
// verifies, in process and with the standard library only, the Sigstore bundle
// GitHub artifact attestations publish (actions/attest-build-provenance):
//
//  1. the bundle is a Sigstore bundle (v0.1–v0.3) carrying a DSSE envelope and a
//     leaf certificate — a bare public key is refused;
//  2. the leaf certificate chains to a Fulcio CA in the pinned trusted root, at
//     the time the transparency log recorded the entry;
//  3. one transparency-log entry from a log in the pinned trusted root carries a
//     signed entry timestamp (SET) that verifies, its integrated time lies inside
//     the leaf certificate's validity window, and its body names THIS envelope's
//     signature, certificate and payload digest;
//  4. the DSSE signature verifies under the leaf certificate's key;
//  5. the certificate's OIDC issuer is GitHub Actions and its SAN is this repo's
//     release workflow at the pinned tag (a tag-push run) or main (a
//     workflow_dispatch run);
//  6. the statement is an in-toto v1 statement whose predicate is SLSA provenance
//     v1 and one of whose subjects carries the asset's sha256.
//
// Any failure is a refusal. There is no flag that skips this check, and no
// warn-and-continue path: an asset without a verifiable bundle is not placed.
//
// Not checked, by design of this minimal verifier: the inclusion proof and
// checkpoint (the SET is the log's signed promise of inclusion and carries the
// time the chain check uses), the certificate's embedded SCT, and Rekor v2
// entries (which carry no SET) — a bundle whose only log entry is a Rekor v2
// entry is REFUSED, never accepted on weaker evidence.

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	_ "embed"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// bundleAssetExt is appended to an asset's release-download URL to locate its
	// attestation bundle: statusgen-windows-amd64.exe -> statusgen-windows-amd64.exe.sigstore.json.
	bundleAssetExt = ".sigstore.json"

	// The signer identity an asset's attestation must carry. Compiled in, never
	// read from the manifest: a manifest an attacker can edit must not be able to
	// name the identity that vouches for the bytes it pins.
	attestSignerRepo     = "medici-finance/assay"
	attestSignerWorkflow = ".github/workflows/release.yml"
	attestIssuer         = "https://token.actions.githubusercontent.com"

	slsaProvenanceV1  = "https://slsa.dev/provenance/v1"
	inTotoStatementV1 = "https://in-toto.io/Statement/v1"
	inTotoPayloadType = "application/vnd.in-toto+json"

	// maxBundleBytes bounds what a single bundle download may be; a real one is
	// a few KiB to a few tens of KiB. The production fetch reads the bundle
	// through readBounded (install.go), so a larger response is refused without
	// being buffered; verifyAttestation re-checks the bound on whatever bytes an
	// injected Fetcher hands it.
	maxBundleBytes = 1 << 20
)

// Fulcio certificate extensions carrying the OIDC issuer: .1.8 is the
// DER-encoded UTF8String form, .1.1 the deprecated raw-string form.
var (
	oidIssuerV2 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}
	oidIssuerV1 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
)

// embeddedTrustedRoot is the public-good Sigstore trusted root, byte-identical
// to the TUF target `trusted_root.json` (sha256
// 6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66) served by
// the public-good TUF repository. Re-pinning it is a reviewed diff, like the
// release pins themselves.
//
//go:embed sigstore_trusted_root.json
var embeddedTrustedRoot []byte

// signerPolicy is the certificate identity an attestation must carry.
type signerPolicy struct {
	repo     string   // owner/name
	workflow string   // repo-relative workflow path
	refs     []string // accepted git refs the workflow ran at
}

// releasePolicy is the policy for an asset pinned at tag: this repo's release
// workflow, run either by the tag push or by workflow_dispatch from main.
//
// INVARIANT — accepting refs/heads/main is safe only while release.yml's `on:`
// triggers stay EXACTLY a tag push plus workflow_dispatch. This verifier does
// not read the Fulcio Build Trigger extension (OID 1.3.6.1.4.1.57264.1.20), so
// it cannot tell a workflow_dispatch run from main apart from any other run of
// release.yml at the default branch. If release.yml ever gains a trigger that
// runs at refs/heads/main (schedule, workflow_run, pull_request_target, a push
// to main), that run's attestations would pass this policy for EVERY pinned
// tag. Adding such a trigger must come with pinning the Build Trigger
// extension here (to push or workflow_dispatch) or dropping refs/heads/main.
func releasePolicy(tag string) signerPolicy {
	return signerPolicy{
		repo:     attestSignerRepo,
		workflow: attestSignerWorkflow,
		refs:     []string{"refs/tags/" + tag, "refs/heads/main"},
	}
}

func (p signerPolicy) allows(san string) bool {
	prefix := "https://github.com/" + p.repo + "/" + p.workflow + "@"
	if !strings.HasPrefix(san, prefix) {
		return false
	}
	ref := strings.TrimPrefix(san, prefix)
	for _, r := range p.refs {
		if ref == r {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Trusted root (trusted_root.json, protobuf-JSON)
// ---------------------------------------------------------------------------

type validFor struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func (v validFor) contains(t time.Time) bool {
	if v.Start.IsZero() || t.Before(v.Start) {
		return false
	}
	return v.End.IsZero() || !t.After(v.End)
}

type trustedRootDoc struct {
	Tlogs []struct {
		PublicKey struct {
			RawBytes   []byte   `json:"rawBytes"`
			KeyDetails string   `json:"keyDetails"`
			ValidFor   validFor `json:"validFor"`
		} `json:"publicKey"`
		LogID struct {
			KeyID []byte `json:"keyId"`
		} `json:"logId"`
	} `json:"tlogs"`
	CertificateAuthorities []struct {
		CertChain struct {
			Certificates []struct {
				RawBytes []byte `json:"rawBytes"`
			} `json:"certificates"`
		} `json:"certChain"`
		ValidFor validFor `json:"validFor"`
	} `json:"certificateAuthorities"`
}

type tlogKey struct {
	pub      *ecdsa.PublicKey
	validFor validFor
}

type certAuthority struct {
	roots, intermediates *x509.CertPool
	validFor             validFor
}

// trustedMaterial is the parsed trust root: the transparency logs whose SETs
// are accepted (keyed by hex log ID) and the CAs a leaf may chain to.
type trustedMaterial struct {
	tlogs map[string]tlogKey
	cas   []certAuthority
}

// parseTrustedRoot parses a trusted_root.json document. Only ECDSA log keys
// can sign a SET, so other log key types (Rekor v2's Ed25519 shard key) are
// skipped; a root that yields no usable log or no CA is refused.
func parseTrustedRoot(b []byte) (*trustedMaterial, error) {
	var doc trustedRootDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("trusted root: %w", err)
	}
	tm := &trustedMaterial{tlogs: map[string]tlogKey{}}
	for _, l := range doc.Tlogs {
		if !strings.HasPrefix(l.PublicKey.KeyDetails, "PKIX_ECDSA_") {
			continue
		}
		pub, err := x509.ParsePKIXPublicKey(l.PublicKey.RawBytes)
		if err != nil {
			return nil, fmt.Errorf("trusted root: tlog key: %w", err)
		}
		ek, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("trusted root: tlog key is %T, keyDetails says ECDSA", pub)
		}
		id := sha256.Sum256(l.PublicKey.RawBytes)
		if !bytes.Equal(id[:], l.LogID.KeyID) {
			return nil, errors.New("trusted root: tlog logId is not the sha256 of its key")
		}
		tm.tlogs[hex.EncodeToString(id[:])] = tlogKey{pub: ek, validFor: l.PublicKey.ValidFor}
	}
	for _, ca := range doc.CertificateAuthorities {
		certs := ca.CertChain.Certificates
		if len(certs) == 0 {
			continue
		}
		c := certAuthority{roots: x509.NewCertPool(), intermediates: x509.NewCertPool(), validFor: ca.ValidFor}
		for i, raw := range certs {
			cert, err := x509.ParseCertificate(raw.RawBytes)
			if err != nil {
				return nil, fmt.Errorf("trusted root: CA certificate: %w", err)
			}
			if i == len(certs)-1 {
				c.roots.AddCert(cert)
			} else {
				c.intermediates.AddCert(cert)
			}
		}
		tm.cas = append(tm.cas, c)
	}
	if len(tm.tlogs) == 0 || len(tm.cas) == 0 {
		return nil, errors.New("trusted root: no usable transparency log or certificate authority")
	}
	return tm, nil
}

// ---------------------------------------------------------------------------
// Bundle (Sigstore bundle, protobuf-JSON)
// ---------------------------------------------------------------------------

// jsonInt64 accepts an int64 as either a JSON number or a JSON string, which is
// how protobuf-JSON renders int64 fields.
type jsonInt64 int64

func (j *jsonInt64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*j = jsonInt64(v)
	return nil
}

type bundleJSON struct {
	MediaType            string `json:"mediaType"`
	VerificationMaterial struct {
		Certificate *struct {
			RawBytes []byte `json:"rawBytes"`
		} `json:"certificate"`
		X509CertificateChain *struct {
			Certificates []struct {
				RawBytes []byte `json:"rawBytes"`
			} `json:"certificates"`
		} `json:"x509CertificateChain"`
		TlogEntries []struct {
			LogIndex jsonInt64 `json:"logIndex"`
			LogID    struct {
				KeyID []byte `json:"keyId"`
			} `json:"logId"`
			KindVersion struct {
				Kind    string `json:"kind"`
				Version string `json:"version"`
			} `json:"kindVersion"`
			IntegratedTime   jsonInt64 `json:"integratedTime"`
			InclusionPromise *struct {
				SignedEntryTimestamp []byte `json:"signedEntryTimestamp"`
			} `json:"inclusionPromise"`
			CanonicalizedBody []byte `json:"canonicalizedBody"`
		} `json:"tlogEntries"`
	} `json:"verificationMaterial"`
	DSSEEnvelope *struct {
		Payload     []byte `json:"payload"`
		PayloadType string `json:"payloadType"`
		Signatures  []struct {
			Sig []byte `json:"sig"`
		} `json:"signatures"`
	} `json:"dsseEnvelope"`
}

var bundleMediaTypes = map[string]bool{
	"application/vnd.dev.sigstore.bundle+json;version=0.1": true,
	"application/vnd.dev.sigstore.bundle+json;version=0.2": true,
	"application/vnd.dev.sigstore.bundle+json;version=0.3": true,
	"application/vnd.dev.sigstore.bundle.v0.3+json":        true,
}

// rekorSETPayload is the document a Rekor v1 log signs as the SET. Field order
// is the RFC 8785 canonical order (keys sorted), and none of the values carry
// a character json.Marshal would escape, so json.Marshal yields the canonical
// bytes.
type rekorSETPayload struct {
	Body           string `json:"body"`
	IntegratedTime int64  `json:"integratedTime"`
	LogID          string `json:"logID"`
	LogIndex       int64  `json:"logIndex"`
}

// rekorDSSEBody is the canonicalized body of a dsse/0.0.1 Rekor entry.
type rekorDSSEBody struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		PayloadHash struct {
			Algorithm string `json:"algorithm"`
			Value     string `json:"value"`
		} `json:"payloadHash"`
		Signatures []struct {
			Signature string `json:"signature"`
			Verifier  string `json:"verifier"`
		} `json:"signatures"`
	} `json:"spec"`
}

type inTotoStatement struct {
	Type          string `json:"_type"`
	PredicateType string `json:"predicateType"`
	Subject       []struct {
		Name   string            `json:"name"`
		Digest map[string]string `json:"digest"`
	} `json:"subject"`
}

// dssePAE is the DSSE v1 pre-authentication encoding the envelope signature
// covers.
func dssePAE(payloadType string, payload []byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "DSSEv1 %d %s %d ", len(payloadType), payloadType, len(payload))
	b.Write(payload)
	return b.Bytes()
}

// verifyAttestation verifies an attestation bundle for an artifact whose
// sha256 is digestHex, against the signer policy and the trusted material.
// A nil return means every check in this file's header held.
func verifyAttestation(raw []byte, digestHex string, pol signerPolicy, tm *trustedMaterial) error {
	if len(raw) > maxBundleBytes {
		return fmt.Errorf("attestation bundle is %d bytes, over the %d-byte bound", len(raw), maxBundleBytes)
	}
	var b bundleJSON
	if err := json.Unmarshal(raw, &b); err != nil {
		return fmt.Errorf("attestation bundle does not parse: %w", err)
	}
	if !bundleMediaTypes[b.MediaType] {
		return fmt.Errorf("attestation bundle media type %q is not a Sigstore bundle", b.MediaType)
	}

	// 1. Leaf certificate and DSSE envelope.
	var leafDER []byte
	switch vm := b.VerificationMaterial; {
	case vm.Certificate != nil:
		leafDER = vm.Certificate.RawBytes
	case vm.X509CertificateChain != nil && len(vm.X509CertificateChain.Certificates) > 0:
		leafDER = vm.X509CertificateChain.Certificates[0].RawBytes
	default:
		return errors.New("attestation bundle carries no signing certificate (a bare public key is not accepted)")
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		return fmt.Errorf("attestation signing certificate does not parse: %w", err)
	}
	env := b.DSSEEnvelope
	if env == nil {
		return errors.New("attestation bundle carries no DSSE envelope")
	}
	if env.PayloadType != inTotoPayloadType {
		return fmt.Errorf("attestation payload type %q is not %s", env.PayloadType, inTotoPayloadType)
	}
	if len(env.Signatures) != 1 {
		return fmt.Errorf("attestation envelope carries %d signatures, want exactly 1", len(env.Signatures))
	}
	sig := env.Signatures[0].Sig

	// 3. Transparency-log entry: a verified SET supplies the signing time.
	signedAt, err := verifyTlog(b, leafDER, sig, env.Payload, tm)
	if err != nil {
		return err
	}
	if signedAt.Before(leaf.NotBefore) || signedAt.After(leaf.NotAfter) {
		return fmt.Errorf("attestation was logged at %s, outside the signing certificate's validity window %s–%s",
			signedAt.UTC().Format(time.RFC3339), leaf.NotBefore.UTC().Format(time.RFC3339), leaf.NotAfter.UTC().Format(time.RFC3339))
	}

	// 2. Certificate chain to a trusted CA, evaluated at the logged time.
	if err := verifyChain(leaf, signedAt, tm); err != nil {
		return err
	}

	// 4. DSSE signature under the leaf key.
	if err := verifySig(leaf.PublicKey, dssePAE(env.PayloadType, env.Payload), sig); err != nil {
		return fmt.Errorf("attestation envelope signature does not verify: %w", err)
	}

	// 5. Signer identity.
	if err := checkIdentity(leaf, pol); err != nil {
		return err
	}

	// 6. Statement: SLSA provenance naming this artifact.
	var st inTotoStatement
	if err := json.Unmarshal(env.Payload, &st); err != nil {
		return fmt.Errorf("attestation statement does not parse: %w", err)
	}
	if st.Type != inTotoStatementV1 {
		return fmt.Errorf("attestation statement _type %q is not %s", st.Type, inTotoStatementV1)
	}
	if st.PredicateType != slsaProvenanceV1 {
		return fmt.Errorf("attestation predicateType %q is not %s", st.PredicateType, slsaProvenanceV1)
	}
	want := strings.ToLower(digestHex)
	for _, s := range st.Subject {
		if strings.ToLower(s.Digest["sha256"]) == want {
			return nil
		}
	}
	return fmt.Errorf("attestation has no subject with sha256 %s — these bytes are not the bytes the release workflow attested", want)
}

// verifyTlog finds a log entry from a trusted log whose SET verifies and whose
// body is bound to this bundle, and returns its integrated time.
func verifyTlog(b bundleJSON, leafDER, sig, payload []byte, tm *trustedMaterial) (time.Time, error) {
	entries := b.VerificationMaterial.TlogEntries
	if len(entries) == 0 {
		return time.Time{}, errors.New("attestation bundle carries no transparency-log entry")
	}
	if len(entries) > 8 {
		return time.Time{}, fmt.Errorf("attestation bundle carries %d transparency-log entries, over the bound of 8", len(entries))
	}
	var reasons []string
	for _, e := range entries {
		id := hex.EncodeToString(e.LogID.KeyID)
		key, ok := tm.tlogs[id]
		if !ok {
			reasons = append(reasons, "entry from a log not in the trusted root")
			continue
		}
		if e.InclusionPromise == nil || len(e.InclusionPromise.SignedEntryTimestamp) == 0 {
			reasons = append(reasons, "entry carries no signed entry timestamp")
			continue
		}
		at := time.Unix(int64(e.IntegratedTime), 0)
		if !key.validFor.contains(at) {
			reasons = append(reasons, "log key not valid at the entry's integrated time")
			continue
		}
		setDoc, err := json.Marshal(rekorSETPayload{
			Body:           base64.StdEncoding.EncodeToString(e.CanonicalizedBody),
			IntegratedTime: int64(e.IntegratedTime),
			LogID:          id,
			LogIndex:       int64(e.LogIndex),
		})
		if err != nil {
			return time.Time{}, err
		}
		h := sha256.Sum256(setDoc)
		if !ecdsa.VerifyASN1(key.pub, h[:], e.InclusionPromise.SignedEntryTimestamp) {
			reasons = append(reasons, "signed entry timestamp does not verify")
			continue
		}
		if err := checkLogBody(e.KindVersion.Kind, e.KindVersion.Version, e.CanonicalizedBody, leafDER, sig, payload); err != nil {
			reasons = append(reasons, err.Error())
			continue
		}
		return at, nil
	}
	return time.Time{}, fmt.Errorf("attestation has no verifiable transparency-log entry: %s", strings.Join(reasons, "; "))
}

// checkLogBody binds a dsse/0.0.1 log entry to this bundle: the entry must
// record this envelope's signature, this signing certificate, and this
// payload's sha256. Other entry kinds are refused.
func checkLogBody(kind, version string, body, leafDER, sig, payload []byte) error {
	if kind != "dsse" || version != "0.0.1" {
		return fmt.Errorf("log entry kind %s/%s is not supported (want dsse/0.0.1)", kind, version)
	}
	var rb rekorDSSEBody
	if err := json.Unmarshal(body, &rb); err != nil {
		return fmt.Errorf("log entry body does not parse: %w", err)
	}
	if rb.Kind != kind || rb.APIVersion != version {
		return errors.New("log entry body kind/version disagrees with the entry's kindVersion")
	}
	ph := sha256.Sum256(payload)
	if rb.Spec.PayloadHash.Algorithm != "sha256" || !strings.EqualFold(rb.Spec.PayloadHash.Value, hex.EncodeToString(ph[:])) {
		return errors.New("log entry payload hash does not match the envelope payload")
	}
	if len(rb.Spec.Signatures) != 1 {
		return fmt.Errorf("log entry records %d signatures, want exactly 1", len(rb.Spec.Signatures))
	}
	ls, err := base64.StdEncoding.DecodeString(rb.Spec.Signatures[0].Signature)
	if err != nil || !bytes.Equal(ls, sig) {
		return errors.New("log entry signature does not match the envelope signature")
	}
	pemBytes, err := base64.StdEncoding.DecodeString(rb.Spec.Signatures[0].Verifier)
	if err != nil {
		return errors.New("log entry verifier does not decode")
	}
	blk, _ := pem.Decode(pemBytes)
	if blk == nil || !bytes.Equal(blk.Bytes, leafDER) {
		return errors.New("log entry verifier is not the bundle's signing certificate")
	}
	return nil
}

func verifyChain(leaf *x509.Certificate, at time.Time, tm *trustedMaterial) error {
	var last error
	for _, ca := range tm.cas {
		if !ca.validFor.contains(at) {
			continue
		}
		_, err := leaf.Verify(x509.VerifyOptions{
			Roots:         ca.roots,
			Intermediates: ca.intermediates,
			CurrentTime:   at,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		})
		if err == nil {
			return nil
		}
		last = err
	}
	if last == nil {
		return errors.New("attestation signing certificate: no trusted CA was valid at the logged time")
	}
	return fmt.Errorf("attestation signing certificate does not chain to a trusted CA: %w", last)
}

func verifySig(pub crypto.PublicKey, msg, sig []byte) error {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		var digest []byte
		switch k.Curve {
		case elliptic.P256():
			h := sha256.Sum256(msg)
			digest = h[:]
		case elliptic.P384():
			h := sha512.Sum384(msg)
			digest = h[:]
		default:
			return fmt.Errorf("unsupported ECDSA curve %s", k.Curve.Params().Name)
		}
		if !ecdsa.VerifyASN1(k, digest, sig) {
			return errors.New("ECDSA signature invalid")
		}
		return nil
	case ed25519.PublicKey:
		if !ed25519.Verify(k, msg, sig) {
			return errors.New("Ed25519 signature invalid")
		}
		return nil
	default:
		return fmt.Errorf("unsupported signing key type %T", pub)
	}
}

func checkIdentity(leaf *x509.Certificate, pol signerPolicy) error {
	issuer := ""
	for _, ext := range leaf.Extensions {
		switch {
		case ext.Id.Equal(oidIssuerV2):
			var s string
			if rest, err := asn1.UnmarshalWithParams(ext.Value, &s, "utf8"); err != nil || len(rest) != 0 {
				return errors.New("attestation certificate issuer extension is malformed")
			}
			issuer = s
		case ext.Id.Equal(oidIssuerV1) && issuer == "":
			issuer = string(ext.Value)
		}
	}
	if issuer != attestIssuer {
		return fmt.Errorf("attestation certificate OIDC issuer %q is not %s", issuer, attestIssuer)
	}
	if len(leaf.URIs) != 1 {
		return fmt.Errorf("attestation certificate carries %d URI SANs, want exactly 1", len(leaf.URIs))
	}
	san := leaf.URIs[0].String()
	if !pol.allows(san) {
		return fmt.Errorf("attestation signer %q is not %s's %s at %s",
			san, pol.repo, pol.workflow, strings.Join(pol.refs, " or "))
	}
	return nil
}
