package cmd

import (
	"bytes"
	"testing"
)

// TestPowerCommandExecutes guards against shorthand-flag collisions between the
// power subcommand and the root command's persistent flags. pflag merges the
// root persistent flags into a subcommand's flagset at execution time and
// panics on a duplicate shorthand (e.g. the historical --list-reset-types/-l vs
// --log-level/-l clash). Executing `power --help` forces that merge without
// running the command body, so any reintroduced collision fails here instead of
// at runtime for every user.
func TestPowerCommandExecutes(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"power", "--help"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	// A shorthand collision surfaces as a panic during flag merge; the test
	// fails (rather than the process aborting) if that regresses.
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("executing `power --help` returned an error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected help output for `power --help`, got none")
	}
}
