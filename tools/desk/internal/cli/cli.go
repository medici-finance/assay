package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ExitUsage is the exit code for a parse error: an unknown flag or command, a missing or
// malformed flag value, or a positional-argument count the command refuses. It is the code
// Go's flag package and the hand-rolled desk parsers already use.
const ExitUsage = 2

// IO is the three streams an invocation writes to and reads from. Tests pass buffers; a
// main passes os.Stdin, os.Stdout and os.Stderr.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Options shapes one Run.
type Options struct {
	IO IO
	// Version is printed by --version on the root command. Empty disables the flag.
	Version string
	// VersionTemplate overrides Cobra's "<name> version <v>" line; legacy tools that print a
	// bare version keep their text with "{{.Version}}\n".
	VersionTemplate string
	// UsageExit is the exit code for a parse error. Zero means ExitUsage.
	UsageExit int
	// ExitCode maps a handler error to its exit code. Nil means: an error carrying an
	// ExitCode() int method keeps that code, any other error exits 1.
	ExitCode func(error) int
	// GoFlagCompat accepts the Go flag package's single-dash long spellings (-repo x,
	// -repo=x) for every long flag the tree defines, as legacy callers write them.
	GoFlagCompat bool
}

// NewRoot returns a root command with the adapter's fixed settings: completion disabled,
// Cobra's own error and usage printing off (Run prints parse errors once, and a handler
// error is the handler's to report).
func NewRoot(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:               use,
		Short:             short,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
}

// handlerError marks an error returned by a handler, so Run can tell it from a parse error.
type handlerError struct{ err error }

func (h handlerError) Error() string { return h.err.Error() }
func (h handlerError) Unwrap() error { return h.err }

// Run builds a fresh tree with build, executes args against it and returns the exit code.
// It never calls os.Exit and never reads os.Args: the caller passes os.Args[1:].
func Run(build func() *cobra.Command, args []string, opts Options) int {
	root := build()
	if root == nil {
		panic("cli: build returned nil")
	}
	usageExit := opts.UsageExit
	if usageExit == 0 {
		usageExit = ExitUsage
	}
	stdout, stderr := opts.IO.Out, opts.IO.Err
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	in := opts.IO.In
	if in == nil {
		in = strings.NewReader("")
	}
	root.SetIn(in)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.CompletionOptions.DisableDefaultCmd = true
	if opts.Version != "" {
		root.Version = opts.Version
		// Declared here, without Cobra's -v shorthand: legacy tools never gave -v that meaning.
		if root.Flags().Lookup("version") == nil {
			root.Flags().Bool("version", false, "print the version and exit")
		}
		if opts.VersionTemplate != "" {
			root.SetVersionTemplate(opts.VersionTemplate)
		}
	}
	// Cobra's help subcommand is runnable, unlike --help, and ordinarily inherits
	// persistent hooks. Identify that command before wrapping so help cannot load
	// configuration or credentials through a parent hook (including traversal mode).
	root.InitDefaultHelpCmd()
	help, _, helpErr := root.Find([]string{"help"})
	if helpErr != nil || help == root {
		help = nil
	}
	wrapHandlers(root, help)
	if args == nil {
		args = []string{}
	}
	if opts.GoFlagCompat {
		args = normalizeGoFlags(root, args)
	}
	root.SetArgs(args)
	_, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	var h handlerError
	if errors.As(err, &h) {
		if opts.ExitCode != nil {
			return opts.ExitCode(h.err)
		}
		var coded interface{ ExitCode() int }
		if errors.As(h.err, &coded) {
			return coded.ExitCode()
		}
		return 1
	}
	fmt.Fprintf(stderr, "%s: %v\nRun '%s --help' for usage.\n", root.Name(), err, root.Name())
	return usageExit
}

// wrapHandlers marks every handler error in the tree. Hooks Cobra runs after parsing
// (pre-run, run, post-run) are handlers; argument validation and flag parsing are not.
func wrapHandlers(c, help *cobra.Command) {
	wrap := func(f func(*cobra.Command, []string) error, persistent bool) func(*cobra.Command, []string) error {
		if f == nil {
			return nil
		}
		return func(cmd *cobra.Command, args []string) error {
			if persistent && cmd == help {
				return nil
			}
			if err := f(cmd, args); err != nil {
				return handlerError{err}
			}
			return nil
		}
	}
	wrapPersistent := func(f func(*cobra.Command, []string)) func(*cobra.Command, []string) {
		if f == nil {
			return nil
		}
		return func(cmd *cobra.Command, args []string) {
			if cmd != help {
				f(cmd, args)
			}
		}
	}
	c.PersistentPreRunE = wrap(c.PersistentPreRunE, true)
	c.PersistentPreRun = wrapPersistent(c.PersistentPreRun)
	c.PreRunE = wrap(c.PreRunE, false)
	c.RunE = wrap(c.RunE, false)
	c.PostRunE = wrap(c.PostRunE, false)
	c.PersistentPostRunE = wrap(c.PersistentPostRunE, true)
	c.PersistentPostRun = wrapPersistent(c.PersistentPostRun)
	for _, sub := range c.Commands() {
		wrapHandlers(sub, help)
	}
}

// normalizeGoFlags rewrites -name and -name=value to --name forms for every long flag the
// tree defines (plus help and version), leaving shorthand clusters, values and everything
// after "--" untouched. A token that is the separate value of a preceding non-boolean long
// flag is never rewritten, so "--message -repo" keeps "-repo" as the message.
func normalizeGoFlags(root *cobra.Command, args []string) []string {
	long := map[string]bool{"help": false, "version": false} // name -> takes a value
	var collect func(c *cobra.Command)
	collect = func(c *cobra.Command) {
		visit := func(f *pflag.Flag) {
			long[f.Name] = f.NoOptDefVal == ""
		}
		c.Flags().VisitAll(visit)
		c.PersistentFlags().VisitAll(visit)
		for _, sub := range c.Commands() {
			collect(sub)
		}
	}
	collect(root)
	out := make([]string, 0, len(args))
	valueNext := false
	for i, a := range args {
		if valueNext {
			out = append(out, a)
			valueNext = false
			continue
		}
		if a == "--" {
			out = append(out, args[i:]...)
			return out
		}
		name := ""
		switch {
		case strings.HasPrefix(a, "--"):
			name = strings.TrimPrefix(a, "--")
		case strings.HasPrefix(a, "-") && len(a) > 2:
			cand := strings.TrimPrefix(a, "-")
			n := cand
			if j := strings.IndexByte(cand, '='); j >= 0 {
				n = cand[:j]
			}
			if _, ok := long[n]; ok {
				a = "-" + a
				name = cand
			}
		}
		if name != "" && !strings.Contains(name, "=") && long[name] {
			valueNext = true
		}
		out = append(out, a)
	}
	return out
}

// Main is the production entry: Run with the process streams and os.Args[1:].
func Main(build func() *cobra.Command, opts Options) int {
	if opts.IO.In == nil {
		opts.IO.In = os.Stdin
	}
	if opts.IO.Out == nil {
		opts.IO.Out = os.Stdout
	}
	if opts.IO.Err == nil {
		opts.IO.Err = os.Stderr
	}
	return Run(build, os.Args[1:], opts)
}
