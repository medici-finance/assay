package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
)

func (c *Cell) cachePolicy() (*cellcache.Policy, error) {
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
	for _, kv := range p.Env() {
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

func (c *Cell) cacheAdmission() error {
	p, err := c.cachePolicy()
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	r, err := cellcache.Check(*p, false)
	b, _ := json.Marshal(r)
	fmt.Fprintf(os.Stderr, "storage-admission %s\n", b)
	return err
}
func cmdCache(cell string, args []string, confirm bool) {
	c := loadCell(cell)
	p, err := c.cachePolicy()
	if err != nil {
		die("Go cache policy: %v", err)
	}
	if p == nil {
		die("cache requires CELL_GO_CACHE=on in cell.env")
	}
	if len(args) == 1 && args[0] == "recover" && confirm {
		if err = cellcache.Recover(*p, true); err != nil {
			die("cache recover: %v", err)
		}
		return
	}
	if confirm || len(args) != 1 || (args[0] != "status" && args[0] != "clean") {
		die("cache <cell> status|clean|recover --confirm-stopped (status is dry-run)")
	}
	report, err := cellcache.Check(*p, args[0] == "status")
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
	ctx, stop := cellprocess.NotifyContext(context.Background())
	defer stop()
	code, uncertain, err := cellprocess.RunInteractive(ctx, args, env, dir, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if uncertain || code < 0 || (err != nil && code == 0) {
		code = 6
	}
	exitWith(code)
}
