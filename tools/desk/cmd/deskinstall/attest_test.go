package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// An in-process stand-in for the public-good Sigstore instance: a Fulcio-shaped
// CA, a Rekor-shaped log key, and a bundle builder that produces the same v0.3
// bundle shape actions/attest-build-provenance publishes (DSSE envelope over an
// in-toto statement, leaf certificate, one dsse/0.0.1 tlog entry with a signed
// entry timestamp). Nothing here touches the network.
// ---------------------------------------------------------------------------

type testSigstore struct {
	caKey    *ecdsa.PrivateKey
	caCert   *x509.Certificate
	caDER    []byte
	rekorKey *ecdsa.PrivateKey
	rekorDER []byte
}

func newTestSigstore(t *testing.T) *testSigstore {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"test.invalid"}, CommonName: "test-fulcio"},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	rekorKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rekorDER, err := x509.MarshalPKIXPublicKey(&rekorKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return &testSigstore{caKey: caKey, caCert: caCert, caDER: caDER, rekorKey: rekorKey, rekorDER: rekorDER}
}

// trustedRoot renders this instance in the trusted_root.json shape the
// installer embeds for the real public-good instance, so tests exercise the
// SAME parser production uses.
func (s *testSigstore) trustedRoot(t *testing.T) []byte {
	t.Helper()
	logID := sha256.Sum256(s.rekorDER)
	start := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	doc := map[string]any{
		"mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
		"tlogs": []any{map[string]any{
			"baseUrl":       "https://rekor.test.invalid",
			"hashAlgorithm": "SHA2_256",
			"publicKey": map[string]any{
				"rawBytes":   base64.StdEncoding.EncodeToString(s.rekorDER),
				"keyDetails": "PKIX_ECDSA_P256_SHA_256",
				"validFor":   map[string]any{"start": start},
			},
			"logId": map[string]any{"keyId": base64.StdEncoding.EncodeToString(logID[:])},
		}},
		"certificateAuthorities": []any{map[string]any{
			"uri": "https://fulcio.test.invalid",
			"certChain": map[string]any{"certificates": []any{
				map[string]any{"rawBytes": base64.StdEncoding.EncodeToString(s.caDER)},
			}},
			"validFor": map[string]any{"start": start},
		}},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bundleOpts drives one generated bundle. Zero values give a valid bundle
// for the release workflow at the given tag.
type bundleOpts struct {
	san           string             // default: the release workflow at refs/tags/<tag>
	issuer        string             // default: the GitHub Actions OIDC issuer
	predicateType string             // default: SLSA provenance v1
	signer        *ecdsa.PrivateKey  // CA key that signs the leaf; default s.caKey
	signerCert    *x509.Certificate  // CA cert for signer; default s.caCert
	timeShift     time.Duration      // shifts integratedTime relative to the leaf's validity
	subjects      map[string]string  // name -> sha256 hex
	mutate        func(b *bundleDoc) // last-step mutation of the bundle document
}

// bundleDoc is the v0.3 bundle shape, built as plain maps so a test can
// corrupt any field.
type bundleDoc = map[string]any

func releaseSAN(tag string) string {
	return "https://github.com/" + attestSignerRepo + "/" + attestSignerWorkflow + "@refs/tags/" + tag
}

func (s *testSigstore) bundle(t *testing.T, tag string, o bundleOpts) []byte {
	t.Helper()
	if o.san == "" {
		o.san = releaseSAN(tag)
	}
	if o.issuer == "" {
		o.issuer = attestIssuer
	}
	if o.predicateType == "" {
		o.predicateType = slsaProvenanceV1
	}
	if o.signer == nil {
		o.signer, o.signerCert = s.caKey, s.caCert
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuerExt, err := asn1.MarshalWithParams(o.issuer, "utf8")
	if err != nil {
		t.Fatal(err)
	}
	uri, err := url.Parse(o.san)
	if err != nil {
		t.Fatal(err)
	}
	notBefore := time.Now().Add(-1 * time.Minute).Truncate(time.Second)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		NotBefore:    notBefore,
		NotAfter:     notBefore.Add(10 * time.Minute),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs:         []*url.URL{uri},
		ExtraExtensions: []pkix.Extension{
			{Id: oidIssuerV2, Value: issuerExt},
		},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, o.signerCert, &leafKey.PublicKey, o.signer)
	if err != nil {
		t.Fatal(err)
	}

	subjects := []any{}
	for name, d := range o.subjects {
		subjects = append(subjects, map[string]any{"name": name, "digest": map[string]any{"sha256": d}})
	}
	statement := map[string]any{
		"_type":         inTotoStatementV1,
		"subject":       subjects,
		"predicateType": o.predicateType,
		"predicate":     map[string]any{"buildDefinition": map[string]any{"buildType": "https://actions.github.io/buildtypes/workflow/v1"}},
	}
	payload, err := json.Marshal(statement)
	if err != nil {
		t.Fatal(err)
	}
	pae := dssePAE(inTotoPayloadType, payload)
	h := sha256.Sum256(pae)
	sig, err := ecdsa.SignASN1(rand.Reader, leafKey, h[:])
	if err != nil {
		t.Fatal(err)
	}

	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	payloadSum := sha256.Sum256(payload)
	body := map[string]any{
		"apiVersion": "0.0.1",
		"kind":       "dsse",
		"spec": map[string]any{
			"envelopeHash": map[string]any{"algorithm": "sha256", "value": strings.Repeat("0", 64)},
			"payloadHash":  map[string]any{"algorithm": "sha256", "value": hex.EncodeToString(payloadSum[:])},
			"signatures": []any{map[string]any{
				"signature": base64.StdEncoding.EncodeToString(sig),
				"verifier":  base64.StdEncoding.EncodeToString(leafPEM),
			}},
		},
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	integrated := notBefore.Add(30*time.Second + o.timeShift).Unix()
	logID := sha256.Sum256(s.rekorDER)
	const logIndex = 4242
	set := rekorSETPayload{
		Body:           base64.StdEncoding.EncodeToString(bodyJSON),
		IntegratedTime: integrated,
		LogID:          hex.EncodeToString(logID[:]),
		LogIndex:       logIndex,
	}
	setJSON, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	setHash := sha256.Sum256(setJSON)
	setSig, err := ecdsa.SignASN1(rand.Reader, s.rekorKey, setHash[:])
	if err != nil {
		t.Fatal(err)
	}

	doc := bundleDoc{
		"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json",
		"verificationMaterial": map[string]any{
			"certificate": map[string]any{"rawBytes": base64.StdEncoding.EncodeToString(leafDER)},
			"tlogEntries": []any{map[string]any{
				"logIndex":          fmt.Sprint(logIndex),
				"logId":             map[string]any{"keyId": base64.StdEncoding.EncodeToString(logID[:])},
				"kindVersion":       map[string]any{"kind": "dsse", "version": "0.0.1"},
				"integratedTime":    fmt.Sprint(integrated),
				"inclusionPromise":  map[string]any{"signedEntryTimestamp": base64.StdEncoding.EncodeToString(setSig)},
				"canonicalizedBody": base64.StdEncoding.EncodeToString(bodyJSON),
			}},
		},
		"dsseEnvelope": map[string]any{
			"payload":     base64.StdEncoding.EncodeToString(payload),
			"payloadType": inTotoPayloadType,
			"signatures":  []any{map[string]any{"sig": base64.StdEncoding.EncodeToString(sig)}},
		},
	}
	if o.mutate != nil {
		o.mutate(&doc)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ---------------------------------------------------------------------------
// Install-level fixture: the pinned assets, their attestation bundles, and a
// fetcher serving both — the bundle at <asset-url>.sigstore.json.
// ---------------------------------------------------------------------------

type attestFixture struct {
	tag      string
	ss       *testSigstore
	trust    []byte
	sg, dt   []byte // statusgen / desk-tools asset bytes
	sgURL    string
	dtURL    string
	served   map[string][]byte
	manifest string
}

func newAttestFixture(t *testing.T) *attestFixture {
	t.Helper()
	const tag = "v0.26.0"
	f := &attestFixture{tag: tag, ss: newTestSigstore(t)}
	f.trust = f.ss.trustedRoot(t)
	f.sg = []byte("STATUSGEN-WINDOWS-AMD64-EXE-fixture-bytes\x00\x01\x02")
	f.dt = tarGz(t, "deskboard.exe", []byte("deskboard.exe fixture bytes"))
	base := "https://github.com/medici-finance/assay/releases/download/" + tag + "/"
	f.sgURL = base + "statusgen-windows-amd64.exe"
	f.dtURL = base + "desk-tools-windows-amd64.tar.gz"
	f.served = map[string][]byte{
		f.sgURL:                  f.sg,
		f.dtURL:                  f.dt,
		f.sgURL + bundleAssetExt: f.ss.bundle(t, tag, bundleOpts{subjects: map[string]string{"statusgen-windows-amd64.exe": sum(f.sg)}}),
		f.dtURL + bundleAssetExt: f.ss.bundle(t, tag, bundleOpts{subjects: map[string]string{"desk-tools-windows-amd64.tar.gz": sum(f.dt)}}),
	}
	f.writeManifest(t, sum(f.sg), sum(f.dt))
	return f
}

func (f *attestFixture) writeManifest(t *testing.T, sgSum, dtSum string) {
	t.Helper()
	m := fmt.Sprintf(`schema: paired-versions-v1
plugin: "0.5.1"
statusgen:
  release_home: medici-finance/assay
  tag: %[1]s
  platforms:
    windows-amd64: statusgen-windows-amd64.exe %[1]s %[2]s
desk-tools:
  release_home: medici-finance/assay
  tag: %[1]s
  platforms:
    windows-amd64: desk-tools-windows-amd64.tar.gz %[1]s %[3]s
`, f.tag, sgSum, dtSum)
	if f.manifest == "" {
		f.manifest = filepath.Join(t.TempDir(), "paired-versions.yaml")
	}
	if err := os.WriteFile(f.manifest, []byte(m), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *attestFixture) fetch(url string) ([]byte, error) {
	if b, ok := f.served[url]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("GET %s: status 404 Not Found", url)
}

func (f *attestFixture) install(t *testing.T, dest string) error {
	t.Helper()
	return Install(Options{
		ManifestPath: f.manifest,
		DestDir:      dest,
		Platform:     "windows-amd64",
		Fetch:        f.fetch,
		TrustedRoot:  f.trust,
	})
}

func assertNothingPlaced(t *testing.T, dest string) {
	t.Helper()
	entries, err := os.ReadDir(dest)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("REFUSED install still placed files: %v", names)
	}
}

// TestRefusesBadAttestation — brief sdlc/20 Verify row 1. A tampered bundle
// is refused before placement; a valid one places; perturbing one byte of the
// binary after attestation is refused even when the sha256 pin is re-pinned to
// the perturbed bytes (the attestation layer catches what a bad re-pin lets
// through, so the two checks fail for different reasons).
func TestRefusesBadAttestation(t *testing.T) {
	t.Run("valid bundle places", func(t *testing.T) {
		f := newAttestFixture(t)
		dest := t.TempDir()
		if err := f.install(t, dest); err != nil {
			t.Fatalf("valid attestation refused: %v", err)
		}
		for _, n := range []string{"statusgen-windows-amd64.exe", "deskboard.exe"} {
			if _, err := os.Stat(filepath.Join(dest, n)); err != nil {
				t.Errorf("%s not placed: %v", n, err)
			}
		}
	})

	t.Run("tampered bundle refuses before placement", func(t *testing.T) {
		tampers := map[string]func(b *bundleDoc){
			"dsse signature byte flipped": func(b *bundleDoc) {
				env := (*b)["dsseEnvelope"].(map[string]any)
				sigs := env["signatures"].([]any)
				s0 := sigs[0].(map[string]any)
				raw, _ := base64.StdEncoding.DecodeString(s0["sig"].(string))
				raw[len(raw)-1] ^= 0x01
				s0["sig"] = base64.StdEncoding.EncodeToString(raw)
			},
			"statement payload rewritten": func(b *bundleDoc) {
				env := (*b)["dsseEnvelope"].(map[string]any)
				raw, _ := base64.StdEncoding.DecodeString(env["payload"].(string))
				raw = bytes.Replace(raw, []byte("buildtypes"), []byte("buildtypeZ"), 1)
				env["payload"] = base64.StdEncoding.EncodeToString(raw)
			},
			"signed entry timestamp flipped": func(b *bundleDoc) {
				vm := (*b)["verificationMaterial"].(map[string]any)
				e := vm["tlogEntries"].([]any)[0].(map[string]any)
				ip := e["inclusionPromise"].(map[string]any)
				raw, _ := base64.StdEncoding.DecodeString(ip["signedEntryTimestamp"].(string))
				raw[len(raw)-1] ^= 0x01
				ip["signedEntryTimestamp"] = base64.StdEncoding.EncodeToString(raw)
			},
			"tlog entries removed": func(b *bundleDoc) {
				vm := (*b)["verificationMaterial"].(map[string]any)
				vm["tlogEntries"] = []any{}
			},
		}
		for name, tamper := range tampers {
			t.Run(name, func(t *testing.T) {
				f := newAttestFixture(t)
				f.served[f.sgURL+bundleAssetExt] = f.ss.bundle(t, f.tag, bundleOpts{
					subjects: map[string]string{"statusgen-windows-amd64.exe": sum(f.sg)},
					mutate:   tamper,
				})
				dest := filepath.Join(t.TempDir(), "bin")
				err := f.install(t, dest)
				if err == nil {
					t.Fatal("SECURITY: tampered attestation bundle was accepted — install must REFUSE")
				}
				if !strings.Contains(err.Error(), "attestation") {
					t.Errorf("refusal did not name the attestation check: %v", err)
				}
				assertNothingPlaced(t, dest)
			})
		}
	})

	t.Run("binary perturbed after attestation refuses", func(t *testing.T) {
		f := newAttestFixture(t)
		perturbed := append([]byte(nil), f.sg...)
		perturbed[3] ^= 0x01
		f.served[f.sgURL] = perturbed
		// Re-pin the sha256 to the perturbed bytes: the sha256 layer now passes,
		// so ONLY the attestation layer stands between these bytes and placement.
		f.writeManifest(t, sum(perturbed), sum(f.dt))
		dest := filepath.Join(t.TempDir(), "bin")
		err := f.install(t, dest)
		if err == nil {
			t.Fatal("SECURITY: bytes the attestation does not cover were placed — install must REFUSE")
		}
		if strings.Contains(err.Error(), "sha256 mismatch") {
			t.Fatalf("refused by the sha256 layer, not the attestation layer: %v", err)
		}
		if !strings.Contains(err.Error(), "no subject") {
			t.Errorf("refusal did not name the uncovered digest: %v", err)
		}
		assertNothingPlaced(t, dest)
	})

	t.Run("missing bundle refuses", func(t *testing.T) {
		f := newAttestFixture(t)
		delete(f.served, f.dtURL+bundleAssetExt)
		dest := filepath.Join(t.TempDir(), "bin")
		if err := f.install(t, dest); err == nil {
			t.Fatal("SECURITY: an asset with no attestation bundle was placed — install must REFUSE")
		}
		assertNothingPlaced(t, dest)
	})

	t.Run("other signer workflow refuses", func(t *testing.T) {
		f := newAttestFixture(t)
		f.served[f.sgURL+bundleAssetExt] = f.ss.bundle(t, f.tag, bundleOpts{
			san:      "https://github.com/" + attestSignerRepo + "/.github/workflows/other.yml@refs/tags/" + f.tag,
			subjects: map[string]string{"statusgen-windows-amd64.exe": sum(f.sg)},
		})
		dest := filepath.Join(t.TempDir(), "bin")
		err := f.install(t, dest)
		if err == nil {
			t.Fatal("SECURITY: an attestation from a non-release workflow was accepted")
		}
		if !strings.Contains(err.Error(), "signer") {
			t.Errorf("refusal did not name the signer identity: %v", err)
		}
		assertNothingPlaced(t, dest)
	})
}

// TestNoAttestFlagAbsent — brief sdlc/20 Verify row 2. There is no opt-out:
// --no-attest (and its obvious respellings) is an unknown argument, refused
// before anything is fetched, and the usage text states the check is
// mandatory.
func TestNoAttestFlagAbsent(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "paired-versions.yaml")
	if err := os.WriteFile(manifest, []byte("schema: paired-versions-v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--no-attest", "--no-attest=true", "--skip-attestation", "--insecure-skip-attest"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--manifest", manifest, "--dest", t.TempDir(), flag}, &stdout, &stderr)
		if code != 5 {
			t.Errorf("%s: exit %d, want 5 (refused)", flag, code)
		}
		if !strings.Contains(stderr.String(), fmt.Sprintf("unknown argument %q", flag)) {
			t.Errorf("%s: not refused as an unknown argument; stderr=%q", flag, stderr.String())
		}
	}
	if !strings.Contains(usage, "attestation") || !strings.Contains(usage, "no flag skips it") {
		t.Errorf("usage must state the attestation check is mandatory with no opt-out; got:\n%s", usage)
	}
}

// TestRealProvenanceBundle runs the verifier, with the EMBEDDED public-good
// trusted root, over a real GitHub artifact attestation (a SLSA provenance
// bundle GitHub serves for a public project's release asset). It proves the
// production path parses and verifies the bundle shape GitHub actually
// publishes, not only the shape this package's own test builder emits.
func TestRealProvenanceBundle(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "gh-cli-provenance.sigstore.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm, err := parseTrustedRoot(embeddedTrustedRoot)
	if err != nil {
		t.Fatalf("embedded trusted root does not parse: %v", err)
	}
	// The bundle's first subject: gh_2.102.0_linux_386.deb.
	const digest = "759b31942b78c05434a5d958fdacfc66aaf81446e283b4a8b1444104cf9de850"
	pol := signerPolicy{
		repo:     "cli/cli",
		workflow: ".github/workflows/deployment.yml",
		refs:     []string{"refs/heads/trunk"},
	}
	if err := verifyAttestation(raw, digest, pol, tm); err != nil {
		t.Fatalf("real provenance bundle refused: %v", err)
	}
	// Same bundle, this repo's release policy: a different signer — refuse.
	if err := verifyAttestation(raw, digest, releasePolicy("v1.0.0"), tm); err == nil {
		t.Fatal("SECURITY: another project's attestation satisfied this repo's release policy")
	}
	// Same bundle, a digest it does not cover — refuse.
	if err := verifyAttestation(raw, strings.Repeat("ab", 32), pol, tm); err == nil {
		t.Fatal("SECURITY: a digest the attestation does not cover was accepted")
	}
}

// TestAttestRefusalCases pins each remaining refusal branch of the verifier
// against a generated bundle, one perturbation at a time.
func TestAttestRefusalCases(t *testing.T) {
	ss := newTestSigstore(t)
	tm, err := parseTrustedRoot(ss.trustedRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	const tag = "v1.2.3"
	art := []byte("artifact")
	subj := map[string]string{"a": sum(art)}
	rogue := newTestSigstore(t)

	cases := map[string]struct {
		o    bundleOpts
		want string
	}{
		"untrusted CA":             {bundleOpts{signer: rogue.caKey, signerCert: rogue.caCert}, "certificate"},
		"wrong OIDC issuer":        {bundleOpts{issuer: "https://issuer.test.invalid"}, "issuer"},
		"other repository":         {bundleOpts{san: "https://github.com/example/fork/" + attestSignerWorkflow + "@refs/tags/" + tag}, "signer"},
		"other tag":                {bundleOpts{san: releaseSAN("v9.9.9")}, "signer"},
		"not SLSA provenance":      {bundleOpts{predicateType: "https://example.test/predicate/v1"}, "predicateType"},
		"logged after cert expiry": {bundleOpts{timeShift: time.Hour}, "validity"},
		"payload hash mismatch in log body": {bundleOpts{mutate: func(b *bundleDoc) {
			vm := (*b)["verificationMaterial"].(map[string]any)
			e := vm["tlogEntries"].([]any)[0].(map[string]any)
			body, _ := base64.StdEncoding.DecodeString(e["canonicalizedBody"].(string))
			body = bytes.Replace(body, []byte(`"payloadHash":{"algorithm":"sha256","value":"`), []byte(`"payloadHash":{"algorithm":"sha256","value":"f`), 1)
			e["canonicalizedBody"] = base64.StdEncoding.EncodeToString(body)
		}}, "attestation"},
		"unknown bundle media type": {bundleOpts{mutate: func(b *bundleDoc) {
			(*b)["mediaType"] = "application/json"
		}}, "media type"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c.o.subjects = subj
			b := ss.bundle(t, tag, c.o)
			err := verifyAttestation(b, sum(art), releasePolicy(tag), tm)
			if err == nil {
				t.Fatalf("SECURITY: %s was accepted", name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("refusal %q does not name %q", err, c.want)
			}
		})
	}

	// The unperturbed bundle verifies, so each refusal above is caused by its
	// own perturbation and not by a broken baseline.
	if err := verifyAttestation(ss.bundle(t, tag, bundleOpts{subjects: subj}), sum(art), releasePolicy(tag), tm); err != nil {
		t.Fatalf("baseline bundle refused: %v", err)
	}
	// A workflow_dispatch run from main is the release workflow's other trigger.
	mainSAN := "https://github.com/" + attestSignerRepo + "/" + attestSignerWorkflow + "@refs/heads/main"
	if err := verifyAttestation(ss.bundle(t, tag, bundleOpts{san: mainSAN, subjects: subj}), sum(art), releasePolicy(tag), tm); err != nil {
		t.Fatalf("dispatch-from-main bundle refused: %v", err)
	}
}
