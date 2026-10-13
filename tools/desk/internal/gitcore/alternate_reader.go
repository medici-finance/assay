package gitcore

import (
	"errors"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

// alternateReader keeps each borrowed store's pack indexes for the lifetime of
// an Open. go-git v5.19.2 constructs a new ObjectStorage on every alternate
// lookup, so even cached objects cause every pack index to be decoded again.
// Local existence checks avoid that fallback; the persistent readers below use
// the same DotGit.Alternates resolver and filesystem boundary as before.
type alternateReader struct {
	objects  *filesystem.ObjectStorage
	dir      *dotgit.DotGit
	cache    ObjectCache
	loaded   bool
	children []*alternateReader
}

func (r *alternateReader) lookup(h plumbing.Hash, seen map[string]bool) (plumbing.EncodedObject, error) {
	root := r.dir.Fs().Root()
	if seen[root] {
		return nil, plumbing.ErrObjectNotFound
	}
	seen[root] = true
	err := r.objects.HasEncodedObject(h)
	if err == nil {
		// AnyObject prevents a type mismatch from invoking go-git's alternate
		// fallback. The caller checks the requested type after the lookup.
		return r.objects.EncodedObject(plumbing.AnyObject, h)
	}
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, err
	}
	if !r.loaded {
		r.loaded = true
		// An absent/unresolvable alternates file means no usable children,
		// matching go-git's fallback. Open's diagnosis remains available to
		// callers that report an incomplete object store.
		if dirs, err := r.dir.Alternates(); err == nil {
			for _, dir := range dirs {
				r.children = append(r.children, &alternateReader{
					objects: filesystem.NewObjectStorage(dir, r.cache),
					dir:     dir, cache: r.cache,
				})
			}
		}
	}
	for _, child := range r.children {
		obj, err := child.lookup(h, seen)
		if err == nil {
			return obj, nil
		}
		if !errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, err
		}
	}
	return nil, plumbing.ErrObjectNotFound
}

// All writes, refs, config and transport custody still go to the original
// storer. Only object reads are routed through persistent alternate readers.
type alternateStorer struct {
	storage.Storer
	reader *alternateReader
}

func (s alternateStorer) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	obj, err := s.reader.lookup(h, make(map[string]bool))
	if err != nil {
		return nil, err
	}
	if t != plumbing.AnyObject && obj.Type() != t {
		return nil, plumbing.ErrObjectNotFound
	}
	return obj, nil
}
