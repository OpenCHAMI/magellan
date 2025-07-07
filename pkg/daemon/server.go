package daemon

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func RunServer(ctx context.Context, done chan error, rfEndpoint string) {
	rfMux := http.NewServeMux()
	rfMux.HandleFunc("/", placeholderLogHandler)

	rfServer := &http.Server{
		Addr:         rfEndpoint,
		Handler:      rfMux,
		WriteTimeout: 10 * time.Second,
	}

	// Launch server
	serverExit := make(chan error, 1)
	go func() {
		log.Info().Msgf("starting Redfish callback server on %s", rfServer.Addr)
		serverExit <- rfServer.ListenAndServe()
		// serverExit <- http.ListenAndServe(rfEndpoint, router)
	}()

	// Wait for finish signal
	var err error = nil
	select {
	case err = <-serverExit:
		log.Error().Err(err).Msg("server exited")
	case <-ctx.Done():
		log.Info().Msg("shutting down server")
	}

	// Cleanly shut down the server
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()
	// NOTE: Calling Shutdown() causes ListenAndServe() to return
	// immediately. However, existing connections may not be closed for a
	// little while, so we need to wait for Shutdown() itself to return.
	rfServer.Shutdown(shutdownCtx)

	// Return error information (via channel, since this is invoked as a goroutine)
	// nil means we handled a shutdown request; anything else is an error
	// passed on from ListenAndServe()
	done <- err
}

func placeholderLogHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Msgf("Handling request from %s: %s %s", r.RemoteAddr, r.Method, r.RequestURI)
	w.Write(nil)
}
