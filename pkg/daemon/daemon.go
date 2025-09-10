package daemon

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RunServer(rootCmd *cobra.Command) error {
	// Set up router
	router := chi.NewRouter()
	router.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.StripSlashes,
		middleware.Timeout(60*time.Second),
	)

	// Generate endpoints based on the command tree under `rootCmd`
	createCommandTree(router, "", rootCmd)

	// Launch server
	err := http.ListenAndServe(viper.GetString("daemon.endpoint"), router)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Add an endpoint for the given command, and repeat recursively for any subcommands
func createCommandTree(router *chi.Mux, endpoint string, cmd *cobra.Command) {
	endpoint = endpoint + "/" + cmd.Name()
	// Create a shallow copy of the relevant Cobra command, and set its
	// output destination to this HTTP request's ResponseWriter
	// NOTE: Without this, two parallel HTTP requests could race to call
	// SetOut(), with the winner being overridden by the loser. That would
	// cause the winning request to receive no response, and the losing
	// request to get the output from both command invocations
	targetCmd := cmd
	router.Get(endpoint, createHelpHandler(targetCmd))
	router.Post(endpoint, createCommandHandler(targetCmd))
	for _, childCmd := range cmd.Commands() {
		if childCmd.Runnable() || childCmd.HasSubCommands() {
			createCommandTree(router, endpoint, childCmd)
		}
	}
}

// Create an HTTP request handler that displays help for the given command
func createHelpHandler(cmd *cobra.Command) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd.SetOut(w)
		_ = cmd.Help()
		// Help() always returns nil; not sure why the function signature includes an error
	}
}

// Create an HTTP request handler that executes the given command
func createCommandHandler(cmd *cobra.Command) func(w http.ResponseWriter, r *http.Request) {
	// NOTE: Unset our shallow copy's parent command, so that it can
	// execute on its own. Otherwise, the Execute() call later will
	// traverse up the command tree
	parent := cmd.Parent()
	if parent != nil {
		parent.RemoveCommand(cmd)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		cmd.SetOut(w)

		// Split out each body line as a separate argument
		body, err := io.ReadAll(r.Body)
		var args []string
		if err == nil {
			args = strings.Split(string(body), "\n")
		} else {
			args = []string{}
		}
		cmd.SetArgs(args)

		// Run the actual command
		err = cmd.Execute()
		if err != nil {
			// FIXME: Fails since the first call to Write() creates a 200 status
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}
