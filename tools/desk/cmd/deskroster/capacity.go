package main

// capacity.go — `deskroster capacity`, the read-only landing-capacity view.
//
// It answers "how many more items of this loop's work will the enforced meters land?" by
// reading the gate the loop's landing write actually calls (deskkit.ReadLoopCapacity →
// deskkit.ReadMeters, which walks that gate's own plan). It copies no cap, charges nothing,
// reserves nothing and changes no width or limit. Every reading is dated and scoped; what the
// local ledger cannot see (the forge's own quota, other hosts) is printed as unknown.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// capacityNow is the clock the view reads; a test seam, never wired to a flag.
var capacityNow = time.Now

func cmdCapacity(args []string) error {
	return runCapacity(os.Stdout, args)
}

func runCapacity(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("capacity", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	role := fs.String("role", "", "Loop whose landing capacity to read (e.g. worker-desk, verify-desk)")
	repos := fs.String("repo", "", "Comma-separated OWNER/NAME repos to read; each repo's meters are read separately")
	inFlight := fs.Int("in-flight", -1, "Items already dispatched whose landing writes are not yet charged "+
		"(default: assume every slot of the resolved width holds one)")
	depth := fs.Int("depth", -1, "Actionable model-dispatch items waiting (bounds the advice; omit for no advice)")
	held := fs.Int("held", 0, "Items held on a human decision or human-only action; reported, never counted as lanes")
	continuation := fs.Int("continuation", 0, "Continuation items (resume/rework) waiting; they use the reserved slots, not new dispatch lanes")
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			fs.SetOutput(w)
			fs.Usage()
			return nil
		}
	}
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("capacity: %v", err))
	}
	canonical, known := deskkit.CanonicalLoopName(*role)
	if strings.TrimSpace(*role) == "" || !known {
		return deskkit.Refused(fmt.Sprintf("refused: capacity requires --role naming a known loop (%s)",
			strings.Join(deskkit.WidthLoops(), ", ")))
	}
	var targets []string
	for _, r := range strings.Split(*repos, ",") {
		if r = strings.TrimSpace(r); r != "" {
			targets = append(targets, r)
		}
	}
	if len(targets) == 0 {
		return deskkit.Refused("refused: capacity requires --repo; the meters are per repo")
	}
	if *held < 0 || *continuation < 0 {
		return deskkit.Refused("refused: --held and --continuation are counts and cannot be negative")
	}
	if *inFlight < -1 {
		return deskkit.Refused("refused: --in-flight is a count; omit it (or pass -1) to assume the resolved width")
	}

	now := capacityNow().UTC()
	width, _, err := deskkit.ResolvedWidthAt(canonical, now)
	if err != nil {
		return err
	}
	max, maxWhy, err := deskkit.MaxWidth(canonical)
	if err != nil {
		return err
	}
	reserve, _, err := deskkit.ResolvedReserveAt(canonical, now)
	if err != nil {
		return err
	}
	reserved := 0
	for _, n := range reserve {
		reserved += n
	}
	proposed := 0
	if *depth > 0 {
		proposed = *depth
	}

	fmt.Fprintf(w, "capacity %s as of %s\n", canonical, now.Format(time.RFC3339))
	fmt.Fprintf(w, "scope: %s\n", deskkit.LedgerScope)
	fmt.Fprintf(w, "lanes: width %d (max %d, bound by %s); %d model-dispatch + %d reserved for continuation (%s)\n",
		width, max, maxWhy, width-reserved, reserved, deskkit.FormatReserve(reserve))

	// The binding landing reading across the named repos is the SMALLEST: the next item may
	// land on any of them. One unreadable repo blinds the whole answer — never a partial min.
	var binding *deskkit.LoopCapacity
	var blind []string
	for _, repo := range targets {
		c, cerr := deskkit.ReadLoopCapacity(canonical, repo, *inFlight, proposed, now)
		if cerr != nil {
			blind = append(blind, repo)
			fmt.Fprintf(w, "repo %s: COULD-NOT-CHECK — %s\n", repo, cerr.Error())
			continue
		}
		renderLoopCapacity(w, c)
		if binding == nil || (c.Blocked && !binding.Blocked) ||
			(!binding.Blocked && !c.Blocked && c.Items < binding.Items) {
			binding = c
		}
	}
	fmt.Fprintf(w, "provider/cross-host capacity: %s\n", deskkit.ProviderCapacityUnknown)
	if *held > 0 || *continuation > 0 {
		fmt.Fprintf(w, "not dispatch lanes: %d held on a human decision/action, %d continuation (uses the %d reserved slot(s)); "+
			"reported only — their landing writes are reserved only through --in-flight\n",
			*held, *continuation, reserved)
	}

	if len(blind) > 0 {
		fmt.Fprintf(w, "advice: COULD-NOT-CHECK landing capacity on %s — NOT advising a width change. "+
			"An unread meter is not spare capacity.\n", strings.Join(blind, ", "))
		return deskkit.Unverifiable("capacity: landing meters unreadable for "+strings.Join(blind, ", "), nil)
	}
	if *depth < 0 {
		return nil
	}
	target, why := deskkit.WidenBound(width, max, *depth, binding)
	if target > width {
		fmt.Fprintf(w, "advice: may widen to %d (bound by %s): deskroster set --role %s --width %d\n",
			target, why, canonical, target)
	} else {
		fmt.Fprintf(w, "advice: no widening — bound by %s. Limits are unchanged; widening never buys budget.\n", why)
	}
	return nil
}

func renderLoopCapacity(w io.Writer, c *deskkit.LoopCapacity) {
	s := c.Snapshot
	fmt.Fprintf(w, "repo %s: gate %s (tool %s), read by %s\n", c.Repo, s.Target.Gate, s.Target.Tool, s.Read)
	for _, m := range s.Meters {
		line := fmt.Sprintf("  meter %-9s %s: %d/%d", m.Tier, m.Target, m.Used, m.Limit)
		if m.Breaker {
			line = fmt.Sprintf("  meter %-9s %s: run %d/%d", m.Tier, m.Target, m.Used, m.Limit)
		} else {
			line += fmt.Sprintf(" used, %d remaining", m.Remaining)
		}
		line += " " + m.State
		if !m.FreeAt.IsZero() {
			line += " until " + m.FreeAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintln(w, line)
	}
	for _, u := range s.Unread {
		fmt.Fprintf(w, "  meter %s: not read (per target; this read names no target)\n", u)
	}
	if c.Blocked {
		fmt.Fprintf(w, "  remaining writes: BLOCKED until %s — no landing capacity on this repo\n",
			s.FreeAt.UTC().Format(time.RFC3339))
	} else {
		fmt.Fprintf(w, "  remaining writes: %d (gate admits one more: yes)\n", s.Remaining)
	}
	fmt.Fprintf(w, "  in-flight reservations: %d item(s) x %d = %d write(s) (%s)\n",
		c.InFlight, c.WritesPerItem, c.InFlight*c.WritesPerItem, c.InFlightSource)
	fmt.Fprintf(w, "  further items that fit: %s\n", itemsLabel(c))
	if c.Proposed > 0 {
		fits := "fits"
		if c.Blocked || c.Proposed > c.Items {
			fits = "does NOT fit"
		}
		fmt.Fprintf(w, "  proposed: %d item(s) x %d = %d write(s) — %s\n", c.Proposed, c.WritesPerItem, c.ProposedCost, fits)
	}
}

func itemsLabel(c *deskkit.LoopCapacity) string {
	if c.Blocked {
		return "none (blocked)"
	}
	return strconv.Itoa(c.Items)
}
