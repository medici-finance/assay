package main

import (
	"regexp"
	"sort"
	"strings"
)

// maxDeliveryLinkHops bounds how many tracked links one resolution may pass
// through. It matches the Linux kernel's own limit; a chain longer than this,
// or a loop, is refused rather than guessed at.
const maxDeliveryLinkHops = 40

// nonPortableLinkTarget matches a target text that some extractor could read
// as absolute or as a different path: a NUL, a backslash, or a drive prefix.
var nonPortableLinkTarget = regexp.MustCompile(`\x00|\\|^[A-Za-z]:`)

// escapingDeliveryLink returns the first tracked link (in path order) whose
// target leaves the snapshot root, or "" when every link stays inside it.
//
// links maps each tracked link's repository path to its target text, exactly as
// the tree records it. Resolution runs over those records only, never over a
// file system, so nothing here reads through a link. A target is resolved
// against the link's own directory one component at a time. When a component
// names another tracked link, that link's target is substituted in place,
// which is how a chain or a path through a directory link is followed to its
// real end. The link escapes when:
//
//   - its target, or the target of any link the chain reaches, is absolute or
//     not portable (see nonPortableLinkTarget);
//   - a `..` would step above the snapshot root at any point of the walk;
//   - the walk needs more than maxDeliveryLinkHops substitutions (a loop).
//
// A component that names no tracked entry stays a plain path component, so a
// dangling link whose target is inside the root does not escape. A real file
// system could only fail on such a path, so this reading never calls a path
// inside the root when the extracted snapshot would resolve it outside.
func escapingDeliveryLink(links map[string]string) string {
	paths := make([]string, 0, len(links))
	for p := range links {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if deliveryLinkEscapes(links, p) {
			return p
		}
	}
	return ""
}

func deliveryLinkEscapes(links map[string]string, path string) bool {
	parts := strings.Split(path, "/")
	stack := append([]string(nil), parts[:len(parts)-1]...)
	target := links[path]
	if target == "" || strings.HasPrefix(target, "/") || nonPortableLinkTarget.MatchString(target) {
		return true
	}
	pending := strings.Split(target, "/")
	hops := 1
	for len(pending) > 0 {
		c := pending[0]
		pending = pending[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			if len(stack) == 0 {
				return true
			}
			stack = stack[:len(stack)-1]
			continue
		}
		stack = append(stack, c)
		next, isLink := links[strings.Join(stack, "/")]
		if !isLink {
			continue
		}
		hops++
		if hops > maxDeliveryLinkHops {
			return true
		}
		if next == "" || strings.HasPrefix(next, "/") || nonPortableLinkTarget.MatchString(next) {
			return true
		}
		stack = stack[:len(stack)-1]
		pending = append(strings.Split(next, "/"), pending...)
	}
	return false
}
