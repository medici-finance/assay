package target

// dup.go carries a duplicated block — the planted duplication suspect. A
// dupl-class clone detector flags the two near-identical funcs; the scripted
// verdict judges it a FALSE POSITIVE (the duplication is intentional), so it
// exercises the suppressed section.

func handleA(x int) int {
	y := x + 1
	y = y * 2
	return y
}

func handleB(x int) int {
	y := x + 1
	y = y * 2
	return y
}
