package gamma

func Gamma(xs []int) int {
	_ = xs
	t := 0
	for _, x := range xs {
		if x > 0 {
			t += x
		}
	}
	return t
}
