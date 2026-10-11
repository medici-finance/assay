package loopengine

// FrontmatterList reads one top-level frontmatter key of a brief as a list of strings. It is
// the exported face of the reader the write-scope derivation already uses, so a second
// caller reads a brief's lists with the same grammar instead of a copy of it: the block form
// (`key:` then `- item` lines) and the inline form (`key: [a, b]`).
//
// ok is false when the key is absent — or the content has no frontmatter block at all — so
// "not stated" stays distinguishable from "stated, and empty".
func FrontmatterList(content, key string) (items []string, ok bool) {
	return frontmatterList(frontmatterOf(content), key)
}
