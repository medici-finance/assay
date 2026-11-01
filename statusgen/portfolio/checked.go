package portfolio

import "math/big"

// Every integer sum, difference and product in this package goes through these helpers: the
// values come from callers, and a result outside the int64 range is reported, never wrapped.
// TestPortfolioCheckedMath holds the package's production files to that.

// addInt64 returns a+b and whether it fits in an int64.
func addInt64(a, b int64) (int64, bool) {
	r := new(big.Int).Add(big.NewInt(a), big.NewInt(b))
	return r.Int64(), r.IsInt64()
}

// subInt64 returns a-b and whether it fits in an int64.
func subInt64(a, b int64) (int64, bool) {
	r := new(big.Int).Sub(big.NewInt(a), big.NewInt(b))
	return r.Int64(), r.IsInt64()
}

// mulInt64 returns a*b and whether it fits in an int64.
func mulInt64(a, b int64) (int64, bool) {
	r := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	return r.Int64(), r.IsInt64()
}
