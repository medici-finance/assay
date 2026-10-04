package clicontract

// Reusable Windows/POSIX CLI fixtures (desktools-v2/15 task 6). Every migration child runs
// its own command through these tables in its own package tests; they do not wait for the
// platform brief. They are data, not assertions: the child's test owns the expectation that
// a value arrives unchanged at its typed handler.

// Opt returns a pointer to s, so a fixture can say "present and empty" (Opt("")) apart
// from "absent" (nil).
func Opt(s string) *string { return &s }

// PathValues are option values every path-taking flag, environment variable and config key
// must deliver byte-identical, on every platform: the adapter never cleans, splits, expands
// or re-separates a value. The Windows forms are passed through on POSIX too — a POSIX
// host still receives Windows paths from a config file or a cross-platform caller.
var PathValues = []struct{ Name, Value string }{
	{"posix-abs", "/var/lib/assay/cell.env"},
	{"posix-space", "/tmp/a dir/with space.txt"},
	{"posix-rel", "./rel/../x"},
	{"win-drive", `C:\Users\op\cell.env`},
	{"win-drive-space", `C:\Program Files\assay\x y.txt`},
	{"win-unc", `\\srv\share\dir\f.md`},
	{"win-rel", `..\rel\x`},
	{"win-forward", "C:/mixed/sep/f"},
	{"dash-leading", "-not-a-flag"},
	{"equals-comma", "a=b,c=d"},
	{"unicode", "dossier/übersicht-ß.md"},
}

// OptionForm is one way to spell "flag F has value V" on a command line.
type OptionForm struct {
	Name string
	// GoFlagOnly forms are accepted only when the command opts into Go-flag compatibility.
	GoFlagOnly bool
	Args       func(flag, value string) []string
}

// OptionForms are the spellings the contract defines for a value flag.
var OptionForms = []OptionForm{
	{Name: "long-equals", Args: func(f, v string) []string { return []string{"--" + f + "=" + v} }},
	{Name: "long-space", Args: func(f, v string) []string { return []string{"--" + f, v} }},
	{Name: "go-equals", GoFlagOnly: true, Args: func(f, v string) []string { return []string{"-" + f + "=" + v} }},
	{Name: "go-space", GoFlagOnly: true, Args: func(f, v string) []string { return []string{"-" + f, v} }},
}

// PrecedenceCase is one row of the per-key precedence matrix for a key declared from flag,
// environment, config and default, highest first. nil is absent; Opt("") is explicitly
// empty, which wins over every lower source exactly like a non-empty value.
type PrecedenceCase struct {
	Name              string
	Flag, Env, Config *string
	Want, WantSource  string
}

// PrecedenceDefault is the default every PrecedenceCase assumes the key declares.
const PrecedenceDefault = "dflt"

// Precedence is the matrix for order flag > env > config > default.
var Precedence = []PrecedenceCase{
	{Name: "nothing-set", Want: PrecedenceDefault, WantSource: "default"},
	{Name: "config-only", Config: Opt("cfg"), Want: "cfg", WantSource: "config"},
	{Name: "env-over-config", Env: Opt("env"), Config: Opt("cfg"), Want: "env", WantSource: "env"},
	{Name: "flag-over-all", Flag: Opt("flg"), Env: Opt("env"), Config: Opt("cfg"), Want: "flg", WantSource: "flag"},
	{Name: "empty-config-beats-default", Config: Opt(""), Want: "", WantSource: "config"},
	{Name: "empty-env-beats-config", Env: Opt(""), Config: Opt("cfg"), Want: "", WantSource: "env"},
	{Name: "empty-flag-beats-env", Flag: Opt(""), Env: Opt("env"), Want: "", WantSource: "flag"},
	{Name: "unset-env-falls-through", Config: Opt("cfg"), Want: "cfg", WantSource: "config"},
}

// HelpForms are the argument vectors that must reach help with zero effects, for a tool
// with subcommand sub. GoFlagHelpForms add the single-dash spellings a Go-flag caller uses.
func HelpForms(sub string) [][]string {
	return [][]string{
		{"-h"}, {"--help"}, {"help"}, {"help", sub},
		{sub, "-h"}, {sub, "--help"},
	}
}

// GoFlagHelpForms are the extra help spellings for a command with Go-flag compatibility.
func GoFlagHelpForms(sub string) [][]string {
	return [][]string{{"-help"}, {sub, "-help"}}
}

// VersionForms reach version output with zero effects.
var VersionForms = [][]string{{"--version"}}

// GoFlagVersionForms add the Go-flag spelling.
var GoFlagVersionForms = [][]string{{"-version"}}

// MalformedConfigs are config inputs a loader would reject. Help and version must succeed
// with each of them in place, because they never call the loader.
var MalformedConfigs = []struct{ Name, Content string }{
	{"absent", ""},
	{"not-key-value", "this is not a config\x00\n"},
	{"unterminated-quote", "KEY=\"open\n"},
}

// EnvFoldsCase reports whether the platform's environment lookup ignores case. On Windows
// the process environment is case-insensitive, so a binding listing ASSAY_REPO also sees
// assay_repo; on every other platform the names differ. A child asserts its env bindings
// with this, never by assuming one platform.
func EnvFoldsCase(goos string) bool { return goos == "windows" }
