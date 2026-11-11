package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellrunner"
)

func (c *Cell) cachePolicy() (*cellcache.Policy, error) {
	if c.Env == nil {
		return nil, nil
	}
	return cellcache.Resolve(c.Dir, c.Env.Get)
}
func (c *Cell) cacheEnv(env []string) []string {
	p, err := c.cachePolicy()
	if err != nil {
		die("Go cache policy: %v", err)
	}
	// An ambient policy is not an opt-in for a different cell.
	env = envUnset(env, cellcache.EnvPolicy)
	if p == nil {
		return env
	}
	ambient := ""
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == "GOFLAGS" {
			ambient = v
		}
	}
	for _, kv := range p.WithAmbientGoFlags(ambient).Env() {
		k, v, _ := strings.Cut(kv, "=")
		env = envSet(env, k, v)
	}
	return env
}
func (c *Cell) printCachePolicy() {
	p, err := c.cachePolicy()
	if err != nil {
		die("Go cache policy: %v", err)
	}
	if p != nil {
		b, _ := json.Marshal(p)
		fmt.Printf("[go-cache] %s (logical bytes; checked before each managed launch and dispatch)\n", b)
	}
}

// cacheAdmission performs bounded preparation. Missing or dirty accounting
// requires explicit maintenance; launch never falls through into a cache walk.
func (c *Cell) cacheAdmission() error {
	p, err := c.cachePolicy()
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	r, err := cellcache.Prepare(*p)
	b, _ := json.Marshal(r)
	fmt.Fprintf(os.Stderr, "storage-admission %s\n", b)
	return err
}

// upPrepareLaunch is prepareLaunch for `cellctl up`. When bounded admission
// defers on a refusal one clean can clear (the recorded size or free-space
// margin, or an aged or overtaken measurement), it runs that clean once and
// prepares again; a second deferral refuses exactly as before. Every other
// refusal (ownership, domain, corrupt or dirty accounting) is returned
// untouched and stays with the operator. Desk and scratch launches keep
// calling prepareLaunch, which never walks the cache.
func (c *Cell) upPrepareLaunch() (bool, error) {
	fresh, err := c.prepareLaunch()
	var d *cellcache.Deferred
	if err == nil || !errors.As(err, &d) || !d.Report.Reclaimable() {
		return fresh, err
	}
	if err := c.upCacheClean(d.Report.Reason); err != nil {
		return false, err
	}
	return c.prepareLaunch()
}

// upCacheClean is the one clean `up` may run: the same Check, not dry, that
// `cellctl cache <cell> clean` runs, under the same lock and entry budget. It
// prints one line naming why it ran and the logical bytes before and after.
func (c *Cell) upCacheClean(why string) error {
	p, err := c.cachePolicy()
	if err != nil || p == nil {
		return err
	}
	// A just-finished build can stamp a top-level directory inside the
	// admission clock margin; settle it before measuring, as maintenance does.
	time.Sleep(cellcache.AdmitMtimeSlack)
	r, err := cellcache.Check(*p, false)
	why, _, _ = strings.Cut(why, "; run cellctl cache")
	fmt.Fprintf(os.Stderr, "up cache clean: %s; logical bytes before %s, after %s, reclaimed %d; outcome %s\n",
		why, logicalBytes(r.Before), logicalBytes(r.After), r.Reclaimed, r.Outcome)
	return err
}

func logicalBytes(m cellcache.Measurement) string {
	if m.LogicalBytes == nil {
		return "unknown"
	}
	return strconv.FormatInt(*m.LogicalBytes, 10)
}

// cadenceCacheMaintenance belongs to the existing supervisor between completed
// passes. Interactive launches, startup and Finish never call this lifecycle.
func (c *Cell) cadenceCacheMaintenance(ctx context.Context) error {
	p, err := c.cachePolicy()
	if err != nil || p == nil {
		return err
	}
	r, err := cellcache.Maintain(ctx, *p)
	if err != nil && r.Outcome == "" {
		r.Policy, r.Outcome, r.Reason = *p, "could-not-check", "cache maintenance did not complete"
	}
	b, _ := json.Marshal(r)
	fmt.Fprintf(os.Stderr, "cadence-cache-maintenance %s\n", b)
	return err
}

func cmdCache(cell string, args []string) {
	c := loadCell(cell)
	p, err := c.cachePolicy()
	if len(args) == 1 && args[0] == "wipe" {
		p, err = c.maintenanceCachePolicy()
	}
	if err != nil {
		die("Go cache policy: %v", err)
	}
	if p == nil {
		die("cache requires CELL_GO_CACHE=on in cell.env, or in the cells root's %s (%s)", cellDefaultsFile, c.goCacheSupply())
	}
	if len(args) == 2 && args[0] == "recover" && args[1] == "--confirm-stopped" {
		if err = cellcache.Recover(*p, true); err != nil {
			die("cache recover: %v", err)
		}
		return
	}
	if len(args) != 1 || (args[0] != "status" && args[0] != "clean" && args[0] != "wipe") {
		die("cache <cell> status|clean|wipe|recover --confirm-stopped (status is dry-run)")
	}
	if args[0] == "wipe" {
		if _, err := os.Lstat(p.Root); os.IsNotExist(err) {
			fmt.Println("[go-cache] no managed cache to wipe")
			return
		} else if err != nil {
			die("cache wipe: %v", err)
		}
	}
	var report cellcache.Report
	if args[0] == "wipe" {
		report, err = cellcache.Wipe(*p, c.Env.Get("DRY_RUN") == "1")
	} else {
		report, err = cellcache.Check(*p, args[0] == "status")
	}
	if e := json.NewEncoder(os.Stdout).Encode(report); e != nil {
		die("cache report: %v", e)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitWith(6)
	}
}

// Detached tmux panes must retain a supervisor for the cache-consumer lifetime;
// the parent cellctl returning from new-session cannot release that custody.
func cmdCacheRun(args []string) {
	if len(args) == 0 {
		die("cache-run requires an executable")
	}
	runCachedForeground(args, os.Environ(), "")
}
func runCachedForeground(args, env []string, dir string) {
	ctx, stop := cellrunner.NotifyContext(context.Background())
	defer stop()
	code, uncertain, err := cellrunner.RunInteractive(ctx, args, env, dir, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if uncertain || code < 0 || (err != nil && code == 0) {
		code = 6
	}
	exitWith(code)
}

// Maintenance resolves retained cache custody even after launch opt-in is off.
func (c *Cell) maintenanceCachePolicy() (*cellcache.Policy, error) {
	return cellcache.Resolve(c.Dir, func(k string) string {
		switch k {
		case "CELL_GO_CACHE":
			return "on"
		case "CELL_GO_CACHE_ROOT":
			return c.Env.Get("CELL_GO_CACHE_ROOT")
		case "CELL_GO_CACHE_BYTES":
			return c.Env.Get("CELL_GO_CACHE_BYTES")
		case "CELL_GO_CACHE_MIN_FREE":
			return c.Env.Get("CELL_GO_CACHE_MIN_FREE")
		case "CELL_GO_CACHE_BUILD_MAX_AGE":
			return c.Env.Get("CELL_GO_CACHE_BUILD_MAX_AGE")
		case "CELL_GO_CACHE_LOW_WATER_PERCENT":
			return c.Env.Get("CELL_GO_CACHE_LOW_WATER_PERCENT")
		case "CELL_GO_CACHE_TRIM_MIN_AGE":
			return c.Env.Get("CELL_GO_CACHE_TRIM_MIN_AGE")
		case "CELL_GO_TRIMPATH":
			return c.Env.Get("CELL_GO_TRIMPATH")
		}
		return ""
	})
}
