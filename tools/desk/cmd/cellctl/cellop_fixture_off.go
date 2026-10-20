//go:build !cellopfixture

package main

import "github.com/spf13/cobra"

// The release build carries no fixture verb and no stand-ins: `cellctl cellop-fixture` is an
// unknown command here (release-build row 11). The fixture lives in cellop_fixture.go behind
// `//go:build cellopfixture`, so a tagless compile contains none of it.

// cellopFixtureBuild lets a test state which build it is running under.
const cellopFixtureBuild = false

// cellopFixtureCommands registers nothing in the release build.
func cellopFixtureCommands(*cobra.Command) {}

// cellopFixtureTrace records nothing in the release build.
func cellopFixtureTrace(*cobra.Command) {}
