package daemon

import (
	"errors"
	"net/http"
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

	// TODO: Generate endpoints based on the command tree under `rootCmd`

	// Launch server
	err := http.ListenAndServe(viper.GetString("daemon.endpoint"), router)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
