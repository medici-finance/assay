package cli

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Source is where a resolved setting came from.
type Source int

const (
	Unset Source = iota
	Default
	Config
	Env
	Flag
)

func (s Source) String() string {
	switch s {
	case Default:
		return "default"
	case Config:
		return "config"
	case Env:
		return "env"
	case Flag:
		return "flag"
	}
	return "unset"
}

// Kind is a setting's type, which is the type its handler reads.
type Kind int

const (
	String Kind = iota
	Bool
	Int
	Duration
	StringArray
)

// Binding declares one setting and every source it may be resolved from. Only the sources
// named here are consulted; nothing is inferred from the key.
type Binding struct {
	Key   string // lower-case [a-z][a-z0-9-]*; the name handlers read
	Kind  Kind
	Usage string
	// Flag is the long flag spelling ("" = no flag); Short an optional one-letter shorthand.
	Flag  string
	Short string
	// Persistent registers the flag for the command and every subcommand.
	Persistent bool
	Hidden     bool
	// Env lists the environment variable names consulted, first present wins.
	Env []string
	// Config allows the key from the caller's config map.
	Config bool
	// Default is used when no other allowed source sets the key; nil means no default.
	// Its Go type must match Kind (string, bool, int, time.Duration, []string).
	Default any
	// Order is the per-key precedence, highest first. Empty means Flag, Env, Config, Default
	// restricted to the sources this Binding declares. Every listed source must be declared.
	Order []Source
	// Secret marks a credential: it may never be a flag, and its value never appears in an
	// error.
	Secret bool
}

// Set is the declared settings of one command, in declaration order.
type Set struct {
	cmd      *cobra.Command
	bindings []Binding
	byKey    map[string]int
}

var keyRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Declare validates bindings and registers their flags on cmd. An invalid declaration is a
// programming error and panics, so it fails the command's own tests rather than a caller.
func Declare(cmd *cobra.Command, bindings ...Binding) *Set {
	s := &Set{cmd: cmd, byKey: map[string]int{}}
	for _, b := range bindings {
		if !keyRE.MatchString(b.Key) {
			panic(fmt.Sprintf("cli: key %q is not lower-case [a-z][a-z0-9-]*", b.Key))
		}
		if _, dup := s.byKey[b.Key]; dup {
			panic(fmt.Sprintf("cli: key %q declared twice", b.Key))
		}
		if b.Secret && b.Flag != "" {
			panic(fmt.Sprintf("cli: key %q is Secret and cannot be a flag: credentials never travel on argv", b.Key))
		}
		if b.Short != "" && (b.Flag == "" || len(b.Short) != 1) {
			panic(fmt.Sprintf("cli: key %q: Short needs a Flag and is one letter", b.Key))
		}
		for _, e := range b.Env {
			if e == "" || strings.ContainsAny(e, "= \t\x00") {
				panic(fmt.Sprintf("cli: key %q: invalid environment variable name %q", b.Key, e))
			}
		}
		if b.Default != nil && !kindMatches(b.Kind, b.Default) {
			panic(fmt.Sprintf("cli: key %q: Default type %T does not match its Kind", b.Key, b.Default))
		}
		b.Order = orderFor(b)
		if b.Flag != "" {
			registerFlag(cmd, b)
		}
		s.byKey[b.Key] = len(s.bindings)
		s.bindings = append(s.bindings, b)
	}
	return s
}

func declared(b Binding, src Source) bool {
	switch src {
	case Flag:
		return b.Flag != ""
	case Env:
		return len(b.Env) > 0
	case Config:
		return b.Config
	case Default:
		return b.Default != nil
	}
	return false
}

func orderFor(b Binding) []Source {
	if len(b.Order) == 0 {
		var o []Source
		for _, src := range []Source{Flag, Env, Config, Default} {
			if declared(b, src) {
				o = append(o, src)
			}
		}
		return o
	}
	seen := map[Source]bool{}
	for _, src := range b.Order {
		if seen[src] {
			panic(fmt.Sprintf("cli: key %q: source %s listed twice in Order", b.Key, src))
		}
		if !declared(b, src) {
			panic(fmt.Sprintf("cli: key %q: Order lists %s, which the binding does not declare", b.Key, src))
		}
		seen[src] = true
	}
	for _, src := range []Source{Flag, Env, Config, Default} {
		if declared(b, src) && !seen[src] {
			panic(fmt.Sprintf("cli: key %q declares %s but leaves it out of Order", b.Key, src))
		}
	}
	return append([]Source(nil), b.Order...)
}

func kindMatches(k Kind, v any) bool {
	switch k {
	case String:
		_, ok := v.(string)
		return ok
	case Bool:
		_, ok := v.(bool)
		return ok
	case Int:
		_, ok := v.(int)
		return ok
	case Duration:
		_, ok := v.(time.Duration)
		return ok
	case StringArray:
		_, ok := v.([]string)
		return ok
	}
	return false
}

func registerFlag(cmd *cobra.Command, b Binding) {
	fs := cmd.Flags()
	if b.Persistent {
		fs = cmd.PersistentFlags()
	}
	if fs.Lookup(b.Flag) != nil {
		panic(fmt.Sprintf("cli: key %q: flag --%s already defined", b.Key, b.Flag))
	}
	// The pflag default only renders in help; resolution takes defaults from the Default
	// source, and a flag counts as a source only when it was given on the command line.
	switch b.Kind {
	case String:
		d, _ := b.Default.(string)
		fs.StringP(b.Flag, b.Short, d, b.Usage)
	case Bool:
		d, _ := b.Default.(bool)
		fs.BoolP(b.Flag, b.Short, d, b.Usage)
	case Int:
		d, _ := b.Default.(int)
		fs.IntP(b.Flag, b.Short, d, b.Usage)
	case Duration:
		d, _ := b.Default.(time.Duration)
		fs.DurationP(b.Flag, b.Short, d, b.Usage)
	case StringArray:
		d, _ := b.Default.([]string)
		fs.StringArrayP(b.Flag, b.Short, d, b.Usage)
	default:
		panic(fmt.Sprintf("cli: key %q: unknown Kind %d", b.Key, b.Kind))
	}
	if b.Hidden {
		_ = fs.MarkHidden(b.Flag)
	}
}

// Inputs are the non-argv sources for one Resolve.
type Inputs struct {
	// Config is the map the command's existing config parser produced (cell.env, a roster
	// line, a YAML file it already reads). Keys are binding keys. Undeclared keys are left to
	// that parser and ignored here; a declared key whose binding does not allow Config is an
	// error. A nil value counts as absent.
	Config map[string]any
	// LookupEnv reads one environment variable. Nil means os.LookupEnv. Only the names the
	// bindings list are ever looked up.
	LookupEnv func(string) (string, bool)
}

// Values are one invocation's resolved settings.
type Values struct {
	set  *Set
	vals map[string]resolved
}

type resolved struct {
	src Source
	v   any
}

// Resolve reads every declared setting from its allowed sources through a fresh Viper
// instance and returns typed values. Call it inside RunE: never before help or version.
func (s *Set) Resolve(in Inputs) (*Values, error) {
	lookup := in.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	for k, v := range in.Config {
		i, ok := s.byKey[k]
		if !ok || v == nil {
			continue
		}
		if !s.bindings[i].Config {
			return nil, fmt.Errorf("setting %q cannot be set from config", k)
		}
	}
	// Each source lives under its own prefix so one instance holds all four layers and
	// IsSet answers "did THIS source set the key" — the per-key Order then picks.
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	envLayer := map[string]any{}
	cfgLayer := map[string]any{}
	for _, b := range s.bindings {
		if b.Flag != "" {
			f := s.lookupFlag(b.Flag)
			if f != nil && f.Changed {
				val, err := flagValue(b, f)
				if err != nil {
					return nil, err
				}
				v.Set("flag::"+b.Key, val)
			}
		}
		for _, name := range b.Env {
			if val, ok := lookup(name); ok {
				envLayer[b.Key] = val
				break
			}
		}
		if b.Config {
			if val, ok := in.Config[b.Key]; ok && val != nil {
				cfgLayer[b.Key] = val
			}
		}
		if b.Default != nil {
			v.SetDefault("default::"+b.Key, b.Default)
		}
	}
	if err := v.MergeConfigMap(map[string]any{"env": envLayer, "config": cfgLayer}); err != nil {
		return nil, fmt.Errorf("resolving settings: %v", err)
	}
	out := &Values{set: s, vals: map[string]resolved{}}
	for _, b := range s.bindings {
		for _, src := range b.Order {
			k := prefix(src) + "::" + b.Key
			if !v.IsSet(k) {
				continue
			}
			typed, err := convert(b.Kind, v.Get(k))
			if err != nil {
				return nil, fmt.Errorf("setting %q from %s: not a valid %s", b.Key, src, kindName(b.Kind))
			}
			out.vals[b.Key] = resolved{src: src, v: typed}
			break
		}
	}
	return out, nil
}

func (s *Set) lookupFlag(name string) *pflag.Flag {
	if f := s.cmd.Flags().Lookup(name); f != nil {
		return f
	}
	return s.cmd.PersistentFlags().Lookup(name)
}

// flagValue reads the flag's typed value: a stringArray keeps every element byte-identical,
// including an explicitly empty one.
func flagValue(b Binding, f *pflag.Flag) (any, error) {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	fs.AddFlag(f)
	switch b.Kind {
	case String:
		return fs.GetString(f.Name)
	case Bool:
		return fs.GetBool(f.Name)
	case Int:
		return fs.GetInt(f.Name)
	case Duration:
		return fs.GetDuration(f.Name)
	case StringArray:
		// GetStringArray re-reads the value through its CSV rendering, which drops an
		// explicitly empty element; the slice value itself is exact.
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			return sv.GetSlice(), nil
		}
		return fs.GetStringArray(f.Name)
	}
	return nil, fmt.Errorf("setting %q: unknown kind", b.Key)
}

func prefix(src Source) string {
	switch src {
	case Flag:
		return "flag"
	case Env:
		return "env"
	case Config:
		return "config"
	}
	return "default"
}

func kindName(k Kind) string {
	return [...]string{"string", "bool", "int", "duration", "string list"}[k]
}

func convert(k Kind, raw any) (any, error) {
	switch k {
	case String:
		switch raw.(type) {
		case string, int, int64, float64, bool:
			return cast.ToStringE(raw)
		}
		return nil, fmt.Errorf("not a scalar")
	case Bool:
		return cast.ToBoolE(raw)
	case Int:
		if s, ok := raw.(string); ok {
			// The same syntax an int flag accepts (Go flag and pflag both use base 0).
			n, err := strconv.ParseInt(s, 0, strconv.IntSize)
			return int(n), err
		}
		return cast.ToIntE(raw)
	case Duration:
		if s, ok := raw.(string); ok {
			return time.ParseDuration(s)
		}
		return cast.ToDurationE(raw)
	case StringArray:
		switch t := raw.(type) {
		case string:
			// One environment or config value is one element: never split, so a path with a
			// comma or a space survives.
			return []string{t}, nil
		case []string:
			return append([]string(nil), t...), nil
		case []any:
			out := make([]string, 0, len(t))
			for _, e := range t {
				s, ok := e.(string)
				if !ok {
					return nil, fmt.Errorf("element is not a string")
				}
				out = append(out, s)
			}
			return out, nil
		}
		return nil, fmt.Errorf("not a list")
	}
	return nil, fmt.Errorf("unknown kind")
}

// IsSet reports whether any allowed source set key.
func (v *Values) IsSet(key string) bool { v.binding(key); _, ok := v.vals[key]; return ok }

// Source reports which source set key, or Unset.
func (v *Values) Source(key string) Source { v.binding(key); return v.vals[key].src }

func (v *Values) binding(key string) Binding {
	i, ok := v.set.byKey[key]
	if !ok {
		panic(fmt.Sprintf("cli: key %q was never declared", key))
	}
	return v.set.bindings[i]
}

func (v *Values) get(key string, k Kind) any {
	if b := v.binding(key); b.Kind != k {
		panic(fmt.Sprintf("cli: key %q is a %s, read as a %s", key, kindName(b.Kind), kindName(k)))
	}
	return v.vals[key].v
}

// String returns key's value, "" when unset.
func (v *Values) String(key string) string { s, _ := v.get(key, String).(string); return s }

// Bool returns key's value, false when unset.
func (v *Values) Bool(key string) bool { b, _ := v.get(key, Bool).(bool); return b }

// Int returns key's value, 0 when unset.
func (v *Values) Int(key string) int { n, _ := v.get(key, Int).(int); return n }

// Duration returns key's value, 0 when unset.
func (v *Values) Duration(key string) time.Duration {
	d, _ := v.get(key, Duration).(time.Duration)
	return d
}

// Strings returns a copy of key's value, nil when unset.
func (v *Values) Strings(key string) []string {
	s, _ := v.get(key, StringArray).([]string)
	if s == nil {
		return nil
	}
	return append([]string(nil), s...)
}
