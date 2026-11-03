package cellcache

import (
	"context"
	"time"
)

// Maintain lets an existing between-pass supervisor trigger explicit cache
// maintenance. It grants no launch authority and uses Check's existing locks,
// inventory and final revalidation. Fresh accounting needs no recursive work.
func Maintain(ctx context.Context, p Policy) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	if r, why := cheapClean(p, false, admitEnv{free: diskFree, now: time.Now}); why == "" {
		return r, nil
	}
	// A just-finished build can have a filesystem timestamp inside the
	// admission clock margin. Settle that existing margin BEFORE measuring;
	// stamping a newer report without a new measurement would be unsafe.
	settled := time.NewTimer(AdmitMtimeSlack)
	defer settled.Stop()
	select {
	case <-ctx.Done():
		return Report{}, ctx.Err()
	case <-settled.C:
	}
	return Check(p, false)
}
