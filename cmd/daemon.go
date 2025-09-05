package cmd

import (
	"github.com/OpenCHAMI/magellan/pkg/daemon"
	"github.com/spf13/cobra"
)

// The `daemon` command launches a long-running server that exposes all other commands as HTTP endpoints.
var daemonCmd = &cobra.Command{
	Use: "daemon",
	Example: `  // basic launch
  magellan daemon
  // launch with a custom configuration
  magellan daemon -c custom-settings.yml`,
	Short: "Launch a long-running web server, e.g. for container use",
	Long:  "Exposes all other commands as HTTP endpoints, so that Magellan functionality can be controlled remotely by authorized users.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Don't expose the `daemon` command itself; that could lead to very weird recursion scenarios.
		// This should apply to any subcommands, as well.
		rootCmd.RemoveCommand(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.RunServer(rootCmd)
	},
}

func init() {
	addFlag("daemon.endpoint", daemonCmd, "endpoint", "e", "localhost:80", "Root endpoint for the daemon to listen on")

	// TODO: All options for all other commands *could* apply here. Do we
	// try to handle that, or only allow the user to specify a config file?

	rootCmd.AddCommand(daemonCmd)
}
