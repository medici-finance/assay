package decoy

// prefix keeps a tab inside the literal: only leading tabs count.
func prefix(s string) string {
	return "p	" + s
}
