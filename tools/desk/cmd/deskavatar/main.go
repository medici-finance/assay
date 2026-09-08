// Command deskavatar generates the deterministic, on-brand App-avatar set an
// adopter uploads for their Assay Apps. It renders one octagon-framed tile per
// App, runs a 20 px legibility proof over the set BEFORE writing anything, and
// refuses (exit 5) if any pair would be indistinguishable in a PR timeline.
//
// It is offline and deterministic: no network, no time, no randomness. The same
// org login always produces byte-identical files.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/avatar"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("deskavatar", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		org      = fs.String("org", "", "org login the avatars are hued from (required)")
		tier     = fs.String("tier", "team", "avatar set: team (read+act) or family (six roles)")
		out      = fs.String("out", "", "output directory (required)")
		sizesCSV = fs.String("sizes", "512", "comma-separated PNG sizes in px")
		avatarPN = fs.String("avatar", "", "optional org avatar PNG used as the identity field")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: deskavatar --org <login> --tier <team|family> --out <dir> [--sizes 200,512,1000] [--avatar org.png]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *org == "" || *out == "" {
		fs.Usage()
		return 2
	}

	sizes, err := parseSizes(*sizesCSV)
	if err != nil {
		fmt.Fprintln(stderr, "deskavatar:", err)
		return 2
	}

	var avatarPNG []byte
	if *avatarPN != "" {
		avatarPNG, err = os.ReadFile(*avatarPN)
		if err != nil {
			fmt.Fprintln(stderr, "deskavatar: reading --avatar:", err)
			return 2
		}
	}

	set, err := avatar.Generate(*org, avatar.Tier(*tier), avatar.Options{Sizes: sizes, Avatar: avatarPNG})
	if err != nil {
		var pe *avatar.ProofError
		if errors.As(err, &pe) {
			// The single control fired: the set would be indistinguishable at
			// 20 px. Nothing is written.
			fmt.Fprintln(stderr, "deskavatar:", err)
			return 5
		}
		fmt.Fprintln(stderr, "deskavatar:", err)
		return 1
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(stderr, "deskavatar:", err)
		return 1
	}
	single := len(sizes) == 1
	for _, av := range set {
		svgPath := filepath.Join(*out, av.App+".svg")
		if err := os.WriteFile(svgPath, av.SVG, 0o644); err != nil {
			fmt.Fprintln(stderr, "deskavatar:", err)
			return 1
		}
		for _, sz := range sizes {
			name := av.App + ".png"
			if !single {
				name = fmt.Sprintf("%s-%d.png", av.App, sz)
			}
			if err := os.WriteFile(filepath.Join(*out, name), av.PNGs[sz], 0o644); err != nil {
				fmt.Fprintln(stderr, "deskavatar:", err)
				return 1
			}
		}
	}
	return 0
}

func parseSizes(csv string) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid size %q (want positive integers)", tok)
		}
		if n > avatar.MaxRenderSize {
			return nil, fmt.Errorf("size %d exceeds the maximum %d px", n, avatar.MaxRenderSize)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no sizes given")
	}
	sort.Ints(out)
	return out, nil
}
