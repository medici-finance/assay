// Package intentkeys is the TRUST ANCHOR for a signed intent on the `/v1`
// named actions (the authorization contract, gap 3, ruling 1): a verifying key is
// trusted ONLY when it is enrolled in a reviewed cell-config file that maps
// each login to its ed25519 public key(s). Changes to that file land by
// reviewed PR, like cells.yaml beside it. Nothing else — no platform lookup, no
// request field, no environment variable — can make a key trusted.
//
// # The file
//
//	version: 1
//	logins:
//	  <github-login>:
//	    - id: <key-id>             # [a-z0-9][a-z0-9._-]{0,63}, unique per login
//	      ed25519: <base64>        # the RAW 32-byte public key, std base64
//
// # Fail closed
//
// [LoadOrEmpty] is the only loader deskd calls. A missing file, an empty file,
// a file that does not parse, or a file that fails ANY rule below yields the
// EMPTY ring — no key is trusted, so every signed intent is refused — and a
// reason the caller logs. A file is accepted whole or not at all: one bad entry
// never leaves the other entries trusted, because "the reviewed file" is the
// unit a reviewer approved.
//
// There is deliberately no programmatic constructor: the ONLY way to build a
// non-empty [Ring] is from file bytes ([Parse]), so production code has no path
// to trust a key the reviewed file does not name.
package intentkeys

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Version is the only file format version this package reads.
const Version = 1

// DefaultPath is where the reviewed file lives beside cells.yaml.
const DefaultPath = "config/intent-keys.yaml"

var (
	// loginRE is GitHub's login grammar (alphanumerics and single hyphens, no
	// leading hyphen, at most 39 characters). It is also what keeps a login
	// safe to place in the answer-record marker.
	loginRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9]|-[a-z0-9]){0,38}$`)
	keyIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

// ErrEmpty is returned by [Parse] for a file with no content at all.
var ErrEmpty = errors.New("intent-keys file is empty")

// Key is one enrolled ed25519 public key.
type Key struct {
	// ID names the key inside its login's list (e.g. the device it lives on).
	ID string
	// Pub is the 32-byte ed25519 public key.
	Pub ed25519.PublicKey
}

// Fingerprint is the key's SHA256 fingerprint in the OpenSSH display form
// (`SHA256:<unpadded base64>`), recorded on the audit trail.
func (k Key) Fingerprint() string {
	sum := sha256.Sum256(k.Pub)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// Ring is an immutable set of enrolled keys, keyed by lowercased login. The
// zero value and a nil *Ring are both the EMPTY ring: nothing is trusted.
type Ring struct {
	byLogin map[string][]Key
}

// KeysFor returns the keys enrolled for login (case-insensitive), or nil. The
// returned slice is a copy, so a caller cannot widen the ring through it.
func (r *Ring) KeysFor(login string) []Key {
	if r == nil || r.byLogin == nil {
		return nil
	}
	keys := r.byLogin[strings.ToLower(strings.TrimSpace(login))]
	if len(keys) == 0 {
		return nil
	}
	out := make([]Key, len(keys))
	for i, k := range keys {
		out[i] = Key{ID: k.ID, Pub: append(ed25519.PublicKey(nil), k.Pub...)}
	}
	return out
}

// Len reports how many keys are enrolled across all logins.
func (r *Ring) Len() int {
	if r == nil {
		return 0
	}
	n := 0
	for _, ks := range r.byLogin {
		n += len(ks)
	}
	return n
}

// Logins lists the enrolled logins, sorted (for the startup banner).
func (r *Ring) Logins() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.byLogin))
	for l := range r.byLogin {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// fileShape is the on-disk YAML shape. KnownFields(true) at decode makes any
// key not declared here a parse error.
type fileShape struct {
	Version int                   `yaml:"version"`
	Logins  map[string][]keyShape `yaml:"logins"`
}

type keyShape struct {
	ID      string `yaml:"id"`
	Ed25519 string `yaml:"ed25519"`
}

// LoadOrEmpty reads the reviewed file at path. On ANY failure — missing,
// unreadable, empty, unparsable, invalid — it returns the empty ring and a
// non-empty reason; the caller logs the reason and every signed intent is then
// refused. It never returns a partially trusted ring.
func LoadOrEmpty(path string) (*Ring, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Ring{}, fmt.Sprintf("%s does not exist — no key is enrolled", path)
		}
		return &Ring{}, fmt.Sprintf("%s is unreadable (%v) — no key is trusted", path, err)
	}
	r, err := Parse(data)
	if err != nil {
		return &Ring{}, fmt.Sprintf("%s refused whole (%v) — no key is trusted", path, err)
	}
	if r.Len() == 0 {
		return r, fmt.Sprintf("%s enrols no key", path)
	}
	return r, ""
}

// Parse strictly decodes and validates the file content. Any error means the
// whole file is refused.
func Parse(data []byte) (*Ring, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, ErrEmpty
	}
	var f fileShape
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	// A second YAML document is not part of the reviewed shape.
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, errors.New("parse: more than one YAML document")
	}
	if f.Version != Version {
		return nil, fmt.Errorf("version %d is not %d", f.Version, Version)
	}
	ring := &Ring{byLogin: map[string][]Key{}}
	owner := map[string]string{} // raw key bytes -> login that enrolled it
	for rawLogin, keys := range f.Logins {
		login := strings.ToLower(rawLogin)
		if login != rawLogin {
			return nil, fmt.Errorf("login %q must be written lowercase", rawLogin)
		}
		if !loginRE.MatchString(login) {
			return nil, fmt.Errorf("login %q is not a GitHub login", rawLogin)
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("login %q lists no key — remove the entry instead", login)
		}
		ids := map[string]bool{}
		for i, ks := range keys {
			if !keyIDRE.MatchString(ks.ID) {
				return nil, fmt.Errorf("login %q key %d: id %q must match %s", login, i, ks.ID, keyIDRE)
			}
			if ids[ks.ID] {
				return nil, fmt.Errorf("login %q: key id %q appears twice", login, ks.ID)
			}
			ids[ks.ID] = true
			pub, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(ks.Ed25519))
			if err != nil {
				return nil, fmt.Errorf("login %q key %q: ed25519 is not base64: %v", login, ks.ID, err)
			}
			if len(pub) != ed25519.PublicKeySize {
				return nil, fmt.Errorf("login %q key %q: ed25519 key is %d bytes, want %d", login, ks.ID, len(pub), ed25519.PublicKeySize)
			}
			if prev, dup := owner[string(pub)]; dup {
				// One key under two logins would let one private key sign as
				// either person: the file names a key's owner, singly.
				return nil, fmt.Errorf("the same public key is enrolled under %q and %q", prev, login)
			}
			owner[string(pub)] = login
			ring.byLogin[login] = append(ring.byLogin[login], Key{ID: ks.ID, Pub: ed25519.PublicKey(pub)})
		}
	}
	return ring, nil
}
