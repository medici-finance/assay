// Package fixture is the NEGATIVE control for `statusgen shardcheck`
// (methodology/43): two files in one package that share no package-level
// symbol, so a split placing them in different shards is checked-clean.
//
// It exists as the paired half of a positive control. The positive half is the
// live tree — `shardcheck --root . --shard engine=statusgen/emit.go --shard
// cli=statusgen/main.go` refuses, because main.go calls emit(), which is the
// collision that actually reddened main. Running the same check over this
// directory returns 0. A check that only ever refused would be a constant, not
// a detector, and the pair is what distinguishes the two.
//
// Under testdata/ so the Go tool never builds it.
package fixture

// Alpha is referenced by nothing in beta.go.
func Alpha() int { return 1 }
