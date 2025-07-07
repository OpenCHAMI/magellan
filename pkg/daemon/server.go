package daemon

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func RunServer(rfEndpoint string) {
	rfMux := http.NewServeMux()
	rfMux.HandleFunc("/", placeholderLogHandler)

	rfServer := &http.Server{
		Addr:         rfEndpoint,
		Handler:      rfMux,
		WriteTimeout: 10 * time.Second,
	}
	log.Info().Msgf("starting Redfish callback server on %s", rfServer.Addr)
	log.Fatal().Err(rfServer.ListenAndServe()).Msg("server closed")
}

func placeholderLogHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Msgf("Handling request from %s: %s %s", r.RemoteAddr, r.Method, r.RequestURI)
	w.Write(nil)
}
