package main

import (
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Desk helpers run before the harness has applied its command environment. Give
// them the same cell roster, credential home, role and session at that boundary.
func (c *Cell) deskHelperEnv(parent []string, role, session string) []string {
	values := map[string]string{
		"HOME": c.Home, "USERPROFILE": c.Home, "ASSAY_CONFIG_HOME": c.Config,
		"DESK_LOOP": role, "DESK_SESSION": session, "DESK_ROOTS": c.Env.Get("CELL_ROOTS"),
	}
	env := make([]string, 0, len(parent)+len(values))
	for _, pair := range parent {
		key, _, _ := strings.Cut(pair, "=")
		if _, replaced := values[key]; !replaced {
			env = append(env, pair)
		}
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

// deskd resolves credentials in-process. Its CLI is single threaded; scope the
// process home and working directory to the loaded cell and invalidate the
// roster cache on both sides. Restore even when a refusal unwinds through die.
func (c *Cell) enterDeskCustody() func() {
	previous := map[string]*string{}
	wd, err := os.Getwd()
	if err != nil {
		die("deskd: cannot save working directory: %v", err)
	}
	restore := func() {
		for key, value := range previous {
			if value == nil {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, *value)
			}
		}
		_ = os.Chdir(wd)
		deskkit.ReloadConfig()
	}
	for _, pair := range c.deskHelperEnv(nil, "the-desk", c.Name+"-deskd") {
		key, value, _ := strings.Cut(pair, "=")
		if old, ok := os.LookupEnv(key); ok {
			previous[key] = &old
		} else {
			previous[key] = nil
		}
		if err := os.Setenv(key, value); err != nil {
			restore()
			die("deskd: cannot select cell custody: %v", err)
		}
	}
	if err := os.Chdir(c.Repo); err != nil {
		restore()
		die("deskd: cannot enter cell repository: %v", err)
	}
	deskkit.ReloadConfig()
	return restore
}
