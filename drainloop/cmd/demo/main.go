// Command demo drains a handful of fake items with the stand-in adapters, so you can watch the
// engine claim, dispatch, await, land, release, and idle — one item in flight at a time — with
// nothing from any real stack attached. Run it with:
//
//	go run ./cmd/demo
//
// If you keep this module nested inside a larger tree, prefix with GOWORK=off so it builds on
// its own module.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/drainloop"
)

func main() {
	items := []drainloop.Item{
		{ID: "item-1", Payload: map[string]string{"kind": "build"}},
		{ID: "item-2", Payload: map[string]string{"kind": "verify"}},
		{ID: "item-3", Payload: map[string]string{"kind": "deploy"}},
		{ID: "item-4", Payload: map[string]string{"kind": "check"}},
		{ID: "item-5", Payload: map[string]string{"kind": "report"}},
	}
	queue := drainloop.NewMemoryQueue("demo", items)

	claimDir, err := os.MkdirTemp("", "drainloop-claims-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create claim dir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(claimDir)
	claimer, err := drainloop.NewFileClaim(claimDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create claimer:", err)
		os.Exit(1)
	}

	err = drainloop.Run(drainloop.Config{
		Loop:         queue,
		Claimer:      claimer,
		PoolSize:     2, // pool ceiling; the engine awaits each dispatch, so one item is in flight at a time
		StopWhenIdle: true,
		Log:          func(line string) { fmt.Println(line) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "drain error:", err)
		os.Exit(1)
	}

	fmt.Println("---")
	for _, it := range items {
		if r, ok := queue.Landed(it.ID); ok {
			out := ""
			if len(r.Rows) > 0 {
				out = r.Rows[0].Output
			}
			fmt.Printf("%s landed verdict=%s: %s\n", it.ID, r.Verdict, out)
		}
	}
}
