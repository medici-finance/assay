package deskkit

import "github.com/medici-finance/assay/statusgen/briefdoc"

// BriefFilesTags reads the repository aliases used to route the brief.
func BriefFilesTags(text string) []string { return briefdoc.BriefFilesTags(text) }
