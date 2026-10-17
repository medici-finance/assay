package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var cliVerbs = []string{"prs", "actions", "reviews", "queue", "throughput", "health", "dispatch", "todo", "next", "next-up", "awaiting", "nextup", "scope", "policydrift", "stalled", "diff", "files"}

type cliRecorder struct {
	starts, guards, calls, reads int
	sub, tier                    string
	rest                         []string
	age, floor                   int
	merge, unreviewed            time.Duration
	sources                      map[string]string
}

func cliRecord(t *testing.T) *cliRecorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	t.Setenv("ASSAY_TELEMETRY", "0")
	r := &cliRecorder{}
	d, g, h, l, o := boardDispatch, boardGuard, boardHeader, boardLookupEnv, boardResolved
	boardGuard = func() error { r.guards++; return nil }
	boardHeader = func() Header { return Header{} }
	boardLookupEnv = func(k string) (string, bool) {
		r.reads++
		if k != rateFloorEnv {
			t.Errorf("undeclared environment lookup %q", k)
		}
		return os.LookupEnv(k)
	}
	boardResolved = func(v *cli.Values) {
		r.sources = map[string]string{}
		for _, b := range boardBindings() {
			r.sources[b.Key] = v.Source(b.Key).String()
		}
	}
	boardDispatch = func(sub string, rest []string, _ Header, m, u time.Duration, age int, tier string) (*Report, error) {
		r.calls++
		r.sub, r.rest, r.merge, r.unreviewed, r.age, r.tier, r.floor = sub, append([]string(nil), rest...), m, u, age, tier, rateFloor
		return &Report{value: map[string]string{"board": "unchanged"}, render: func(w io.Writer) { fmt.Fprintln(w, "board unchanged") }}, nil
	}
	t.Cleanup(func() { boardDispatch, boardGuard, boardHeader, boardLookupEnv, boardResolved = d, g, h, l, o })
	return r
}
func (r *cliRecorder) exec(a ...string) (int, string, string) {
	var out, err bytes.Buffer
	n := execute(a, &out, &err, func(io.Writer) int { r.starts++; return 0 })
	return n, out.String(), err.String()
}

// regression: desktools-v2/20
func TestCLIHelpOffline(t *testing.T) {
	forms := [][]string{{"--version"}, {"version"}}
	for _, v := range cliVerbs {
		forms = append(forms, clicontract.HelpForms(v)...)
	}
	for _, cfg := range clicontract.MalformedConfigs {
		t.Run(cfg.Name, func(t *testing.T) {
			r := cliRecord(t)
			p := filepath.Join(os.Getenv("HOME"), ".config", "assay")
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatal(err)
			}
			if cfg.Content != "" {
				if err := os.WriteFile(filepath.Join(p, "roster.env"), []byte(cfg.Content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, a := range forms {
				n, out, err := r.exec(a...)
				if len(a) == 1 && (a[0] == "--version" || a[0] == "version") && out != versionLine()+"\n" {
					t.Errorf("version output %q", out)
				}
				if n != 0 || out == "" || err != "" {
					t.Errorf("%v exit=%d out=%q err=%q", a, n, out, err)
				}
			}
			if r.starts+r.guards+r.calls+r.reads != 0 {
				t.Fatalf("help effects: %+v", r)
			}
			if n, _, e := r.exec("prs"); n != 0 || r.starts != 1 || r.guards != 1 || r.calls != 1 {
				t.Fatalf("positive control exit=%d effects=%+v: %s", n, r, e)
			}
		})
	}
	// Exercise main itself: hostile config, telemetry armed, stubs for possible children.
	bin := filepath.Join(t.TempDir(), "deskboard")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	home := t.TempDir()
	cfg := filepath.Join(home, ".config", "assay")
	os.MkdirAll(cfg, 0755)
	os.WriteFile(filepath.Join(cfg, "roster.env"), []byte("bad\x00config"), 0600)
	for _, a := range [][]string{{"--help"}, {"help", "files"}, {"dispatch", "--help"}, {"--version"}, {"version"}} {
		c := exec.Command(bin, a...)
		c.Dir = home
		c.Env = []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + t.TempDir(), "KUBECONFIG=/dev/null", "ASSAY_TELEMETRY=1"}
		var out, err bytes.Buffer
		c.Stdout = &out
		c.Stderr = &err
		if e := c.Run(); e != nil || out.Len() == 0 || err.Len() != 0 {
			t.Errorf("main %v: %v stdout=%q stderr=%q", a, e, out.String(), err.String())
		}
	}
	filepath.WalkDir(home, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			t.Error(e)
		}
		if p != home && p != filepath.Join(home, ".config") && p != cfg && p != filepath.Join(cfg, "roster.env") {
			t.Errorf("help wrote %s", p)
		}
		return nil
	})
}

// regression: desktools-v2/20
func TestCLIConfigFlow(t *testing.T) {
	var matrix struct {
		Flags map[string]any `json:"flags"`
		Floor []struct {
			Name      string
			Env, Flag *string
			Want      int
			Refused   bool
		} `json:"rate-floor"`
	}
	b, e := os.ReadFile("testdata/cli-matrix.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &matrix); e != nil {
		t.Fatal(e)
	}
	for _, c := range matrix.Floor {
		t.Run(c.Name, func(t *testing.T) {
			r := cliRecord(t)
			t.Setenv(rateFloorEnv, "")
			if c.Env != nil {
				t.Setenv(rateFloorEnv, *c.Env)
			} else {
				os.Unsetenv(rateFloorEnv)
			}
			a := []string{"prs"}
			if c.Flag != nil {
				a = append(a, "--rate-floor="+*c.Flag)
			}
			n, _, err := r.exec(a...)
			if c.Refused {
				if n != 5 || r.calls+r.starts+r.guards != 0 {
					t.Fatalf("exit %d effects=%+v: %s", n, r, err)
				}
				return
			}
			if n != 0 || r.floor != c.Want {
				t.Fatalf("exit %d floor=%d want=%d: %s", n, r.floor, c.Want, err)
			}
		})
	}
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range boardBindings() {
			if b.Config || b.Secret || (b.Key != "rate-floor-env" && len(b.Env) > 0) {
				t.Errorf("unexpected sources for %s", b.Key)
			}
		}
	})
	t.Run("all-defaults", func(t *testing.T) {
		r := cliRecord(t)
		t.Setenv(rateFloorEnv, "")
		n, _, e := r.exec("prs")
		if n != 0 || r.age != 48 || r.merge != 20*time.Minute || r.unreviewed != 30*time.Minute {
			t.Fatalf("defaults %+v exit=%d: %s", r, n, e)
		}
		for k := range matrix.Flags {
			if r.sources[k] != "default" {
				t.Errorf("%s from %s", k, r.sources[k])
			}
		}
	})
	t.Run("flags", func(t *testing.T) {
		r := cliRecord(t)
		n, _, e := r.exec("--merge-now-threshold", "1h", "dispatch", "--unreviewed-threshold=2h", "--min-age-hours", "3", "--tier", "any")
		if n != 0 || r.merge != time.Hour || r.unreviewed != 2*time.Hour || r.age != 3 || r.tier != "any" {
			t.Fatalf("typed options %+v exit %d: %s", r, n, e)
		}
	})
}

// regression: desktools-v2/20
func TestCLILegacyForms(t *testing.T) {
	for _, sub := range cliVerbs {
		t.Run(sub, func(t *testing.T) {
			r := cliRecord(t)
			a := []string{sub}
			if sub == "files" || sub == "diff" || sub == "reviews" {
				a = append(a, "example-org/project", "42")
			}
			n, out, e := r.exec(a...)
			if n != 0 || r.sub != sub || r.calls != 1 || out == "" {
				t.Fatalf("%v exit %d handler %q: %s", a, n, r.sub, e)
			}
		})
	}
	for _, f := range clicontract.OptionForms {
		for _, p := range clicontract.PathValues {
			t.Run(f.Name+"/"+p.Name, func(t *testing.T) {
				r := cliRecord(t)
				a := append([]string{"prs"}, f.Args("out", p.Value)...)
				// Capture path at the Viper boundary; avoid opening any fixture path.
				old := boardResolved
				boardResolved = func(v *cli.Values) {
					if v.String("out") != p.Value {
						t.Errorf("path %q want %q", v.String("out"), p.Value)
					}
					old(v)
				}
				boardGuard = func() error { return fmt.Errorf("stop after resolution") }
				n, _, _ := r.exec(a...)
				if f.GoFlagOnly {
					if n != 5 || r.sources != nil {
						t.Errorf("single-dash form accepted: %v", a)
					}
				} else if n != 6 || r.sources["out"] != "flag" {
					t.Errorf("path %v exit=%d sources=%v", a, n, r.sources)
				}
			})
		}
	}
	for _, a := range [][]string{{"prs", "--wat"}, {"prs", "-table"}, {"prs", "--out"}, {"stalled", "--min-age-hours=0"}, {"actions", "--merge-now-threshold=bad"}, {"actions", "--unreviewed-threshold=bad"}, {"dispatch", "--tier=fast"}, {"dispatch", "--tier="}, {"reviews"}, {"files", "o/r", "bad"}, {"completion"}, {"__complete"}, {"__completeNoDesc"}, {"prs", "extra"}} {
		t.Run(strings.Join(a, " "), func(t *testing.T) {
			r := cliRecord(t)
			n, _, e := r.exec(a...)
			if n != 5 || r.starts+r.guards+r.calls != 0 {
				t.Fatalf("invalid %v exit=%d effects=%+v: %s", a, n, r, e)
			}
		})
	}
	t.Run("termination", func(t *testing.T) {
		r := cliRecord(t)
		n, _, e := r.exec("files", "example-org/project", "42", "--", "--help")
		if n != 0 || !reflect.DeepEqual(r.rest, []string{"example-org/project", "42", "--help"}) {
			t.Fatalf("termination exit=%d rest=%v %s", n, r.rest, e)
		}
	})
	t.Run("repeat-and-alias", func(t *testing.T) {
		r := cliRecord(t)
		n, _, e := r.exec("--min-age-hours=3", "--throughput", "--min-age-hours", "7")
		if n != 0 || r.sub != "throughput" || r.age != 7 {
			t.Fatalf("legacy alias %+v exit=%d: %s", r, n, e)
		}
	})
	t.Run("render-parity", func(t *testing.T) {
		r := cliRecord(t)
		for _, a := range [][]string{{"prs"}, {"stalled", "--json"}} {
			n, out, e := r.exec(a...)
			if n != 0 || out != "{\n  \"board\": \"unchanged\"\n}\n" {
				t.Fatalf("JSON parity %v exit %d out=%q: %s", a, n, out, e)
			}
		}
		n, out, e := r.exec("prs", "--table")
		if n != 0 || out != "board unchanged\n" {
			t.Fatalf("table parity exit %d out=%q: %s", n, out, e)
		}
	})
}

// regression: desktools-v2/20
func TestCLITierDomainBoundary(t *testing.T) {
	t.Setenv("DESK_ROOTS", "")
	_, err := dispatch("dispatch", nil, Header{}, 0, 0, 48, "fast")
	if !deskkit.IsRefused(err) {
		t.Fatal("the domain admitted an invalid tier with the CLI bypassed")
	}
}
