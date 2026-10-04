// Package cli is the desk tools' small Cobra/Viper adapter: the one place a desk command's
// parsing, generated help, version output and allowlisted configuration binding are wired
// (desktools-v2 spec §9; the contract is docs/streams/desktools-v2/cli-contract.md).
//
// WHAT IT OWNS. Turning argv into a fresh Cobra tree's parse, rendering help from that tree,
// printing the version before anything else runs, and resolving each declared setting from
// its allowed sources into a typed value the handler reads. Exit codes for parse errors
// (usage, 2) and the boundary between a parse error and a handler error.
//
// WHAT IT DOES NOT OWN. Admission, custody and every external effect stay with their current
// owners: a handler receives typed options and calls the same validators it called before.
// A source this package resolved is never thereby admissible — the roster, custody and cell
// parsers still decide. The adapter reads no file, no credential and no cell itself; a
// handler passes it the config map its existing parser already produced.
//
// THE RULES THE CODE ENFORCES.
//
//   - Fresh per invocation: Run builds a new command tree and Resolve a new Viper instance on
//     every call; nothing is package-level mutable, and the Viper global is never touched.
//   - Help and version first: both are answered by Cobra before any RunE, so a config loader,
//     credential read or effect placed in a handler cannot run for -h, --help, help <cmd> or
//     --version, even when the config it would read is missing or malformed.
//   - Allowlist only: a setting is resolved only from the sources its Binding names — a flag
//     spelling, explicit environment variable names, the caller's config map, a default. No
//     AutomaticEnv, no config-file search, no remote config.
//   - Unset is not empty: an environment variable or config key that is present with an empty
//     value resolves as set-to-empty from that source; an absent one falls through.
//   - No secrets on argv: a Binding marked Secret cannot declare a flag.
//   - Errors name the key and the source, never the value.
package cli
