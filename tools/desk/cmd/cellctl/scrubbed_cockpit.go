package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func scrubbedDisplayEnv(env ComposedEnv, cockpit string) ComposedEnv {
	if cockpit == "herdr" {
		env.Pairs = envSet(env.Pairs, envAssayCockpit, "tmux")
		env.Sorted = append([]string(nil), env.Pairs...)
		sort.Strings(env.Sorted)
	}
	return env
}

// Auto preserves the historical private tmux surface for scrubbed cells. Herdr is
// opt-in and displays the SAME private session; it never owns a second harness.
func (c *Cell) scrubbedCockpit(flag string) cockpitResolution {
	want, src := c.cockpitWant(flag)
	if want == "auto" {
		if !onPath("tmux") {
			return cockpitResolution{Err: "scrubbed auto requires tmux on PATH"}
		}
		return cockpitResolution{Cockpit: "tmux", Why: "scrubbed auto: private socket"}
	}
	if want != "tmux" && want != "herdr" {
		return cockpitResolution{Err: "scrubbed cockpit must be tmux or herdr"}
	}
	return c.resolveCockpit(want, src)
}

func (c *Cell) scrubbedHerdrAttach(socket, session string) error {
	if !helpHas("create", "herdr", "tab") || !helpHas("run", "herdr", "pane") {
		return fmt.Errorf("Herdr requires tab create and pane run")
	}
	w, err := c.ensureHerdrWorkspace()
	if err != nil {
		return err
	}
	// Serialize surface creation as well as workspace custody. The private tmux lock
	// has already excluded another harness owner for this cell.
	lock, err := c.herdrLease()
	if err != nil {
		return err
	}
	live, err := verifyHerdrWorkspace(w)
	if err == nil && !live {
		err = fmt.Errorf("cell Herdr workspace disappeared")
	}
	if err == nil {
		tmux, e := exec.LookPath("tmux")
		if e != nil {
			err = e
		} else {
			sh := paneShellFor(runtime.GOOS, "herdr")
			command := sh.invoke(sh.quote(tmux)) + " -S " + sh.quote(socket) + " attach -t " + sh.quote(session)
			if sh == shellPOSIX {
				command = "exec " + command
			}
			err = c.herdrWindow(c.Name+"-cell", command, w.ID)
		}
	}
	deskkit.UnlockFile(lock)
	lock.Close()
	if err != nil {
		return err
	}
	if err := c.dropHerdrDefault(w); err != nil {
		return err
	}
	fmt.Printf("[cell] %s displayed in cell-owned Herdr workspace (private tmux session retained)\n", c.Name)
	return nil
}
