package deskkit

// A single process-local discovery slot; never a persistent index or a grant.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
)

const manifestRevision = "activation-discovery/1"
const manifestLimit = 1 << 20

// Test observer includes work before performance wrappers. No runtime collector.
var activationWork = func(event string) {}

// One membership policy, shared by reference enumeration and independent validation.
// Neither HEAD, ignore rules nor a remembered path list proves complete membership.
func walkManifestTree(root, event string, visit func(string, bool) error) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("root %s is not a directory", root)
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			activationWork(event)
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if path != root && hasGitEntry(path) {
				if err := visit(path, true); err != nil {
					return err
				}
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "component.yaml" {
			return visit(path, false)
		}
		return nil
	})
}

type manifestSource struct {
	hash  hash.Hash
	size  int
	valid bool
}

func newManifestSource() *manifestSource {
	s := &manifestSource{hash: sha256.New(), valid: true}
	s.add("revision", manifestRevision, nil)
	return s
}
func (s *manifestSource) add(kind, path string, data []byte) {
	fmt.Fprintf(s.hash, "%d:%s%d:%s%d:", len(kind), kind, len(path), path, len(data))
	_, _ = s.hash.Write(data)
	s.size += len(path) + len(data)
	if s.size > manifestLimit {
		s.valid = false
	}
}
func (s *manifestSource) digest() string { return string(s.hash.Sum(nil)) }
func validateManifests(root string) (*manifestSource, error) {
	s := newManifestSource()
	err := walkManifestTree(root, "validation-directory", func(path string, nested bool) error {
		if nested {
			s.add("nested", relSlash(root, path), nil)
			return nil
		}
		activationWork("validation-read")
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, manifestLimit+1))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) > manifestLimit {
			return fmt.Errorf("manifest exceeds reuse bound")
		}
		s.add("manifest", relSlash(root, path), data)
		return nil
	})
	return s, err
}

// Wire form preserves the semantic type's unexported root-relative path. Only
// discovery data is stored; configuration and computed activation never are.
type manifestWire struct {
	Path     string
	Manifest componentManifest
}
type manifestPayload struct {
	Revision  string
	Manifests []manifestWire
	Skipped   []string
}
type manifestSnapshot struct {
	root     string
	identity os.FileInfo
	source   string
	payload  []byte
	checksum [32]byte
}

var activationIndex struct {
	sync.Mutex
	slot *manifestSnapshot
}

func activationManifests(root string) ([]componentManifest, []string, error) {
	activationIndex.Lock()
	defer activationIndex.Unlock()
	info, err := os.Stat(root)
	if err != nil {
		return DiscoverManifests(root)
	}
	if cached := activationIndex.slot; cached != nil && cached.root == root && os.SameFile(info, cached.identity) {
		source, err := validateManifests(root)
		if err == nil && source.valid && source.digest() == cached.source && len(cached.payload) <= manifestLimit && sha256.Sum256(cached.payload) == cached.checksum {
			data, decodeOK := decodeManifestPayload(cached.payload)
			if decodeOK {
				var manifests []componentManifest
				for _, m := range data.Manifests {
					manifests = append(manifests, m.Manifest)
				}
				activationWork("reuse")
				return manifests, data.Skipped, nil
			}
		}
	}
	activationIndex.slot = nil
	source := newManifestSource()
	manifests, skipped, err := discoverManifests(root, source)
	if err != nil || !source.valid {
		return manifests, skipped, err
	}
	// Reference discovery returns its own observation unchanged. Retain only a
	// candidate captured from those exact parsed bytes. The ONE independent full
	// validation above must succeed immediately before any later reused return.
	// Read failures mark source invalid in the reference reader, so a partial
	// unreadable diagnostic can never be retained and survive a later deletion.
	data := manifestPayload{Revision: manifestRevision, Skipped: skipped}
	for _, m := range manifests {
		data.Manifests = append(data.Manifests, manifestWire{m.path, m})
	}
	payload, encodeErr := json.Marshal(data)
	if encodeErr != nil || len(payload) > manifestLimit {
		return manifests, skipped, err
	}
	decoded, decodeOK := decodeManifestPayload(payload)
	// JSON replaces invalid UTF-8 in every string, including YAML !!binary keys
	// and filesystem names. Admit only an exactly lossless round trip over the
	// entire parsed value, not a whitelist of fields that could miss a new one.
	if decodeOK && reflect.DeepEqual(data, decoded) {
		activationIndex.slot = &manifestSnapshot{root, info, source.digest(), payload, sha256.Sum256(payload)}
	}
	return manifests, skipped, err
}

func decodeManifestPayload(payload []byte) (manifestPayload, bool) {
	var data manifestPayload
	if json.Unmarshal(payload, &data) != nil || data.Revision != manifestRevision {
		return data, false
	}
	for i := range data.Manifests {
		data.Manifests[i].Manifest.path = data.Manifests[i].Path
	}
	return data, true
}
