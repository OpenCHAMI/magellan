// Package server exposes magellan's shared BMC service core (pkg/service) over a
// REST API, so other OpenCHAMI tools can delegate discovery, inventory, and
// power operations to a long-lived magellan daemon instead of each talking to
// BMCs directly (RFD #133). The CLI and this daemon drive the same in-process
// service.Service, so behavior stays identical across front-ends.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/OpenCHAMI/magellan/pkg/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// DefaultRequestTimeout bounds how long a single API request may run before the
// server cancels its context. BMC operations can be slow, so it is generous.
const DefaultRequestTimeout = 120 * time.Second

// shutdownGrace bounds how long graceful shutdown waits for in-flight requests.
const shutdownGrace = 15 * time.Second

// Config configures the daemon HTTP server.
type Config struct {
	// Addr is the listen address, e.g. ":8443".
	Addr string
	// TLSCert and TLSKey, when both set, enable HTTPS.
	TLSCert string
	TLSKey  string
	// AuthToken, when non-empty, requires a matching `Authorization: Bearer`
	// token on all /v1 routes. Empty disables auth (suitable behind a gateway).
	AuthToken string
	// RequestTimeout overrides DefaultRequestTimeout when > 0.
	RequestTimeout time.Duration
}

// Server is the magellan REST daemon. It is a thin HTTP front-end over a
// service.Service; it holds no BMC logic of its own.
type Server struct {
	svc     *service.Service
	cfg     Config
	handler http.Handler
}

// New builds a Server over the given service core.
func New(svc *service.Service, cfg Config) *Server {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = DefaultRequestTimeout
	}
	s := &Server{svc: svc, cfg: cfg}
	s.handler = s.routes()
	return s
}

// Handler returns the server's HTTP handler, primarily for testing.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(s.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(s.cfg.RequestTimeout))

	// Liveness/readiness are unauthenticated so orchestrators can probe them.
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)

	r.Route("/v1", func(r chi.Router) {
		if s.cfg.AuthToken != "" {
			r.Use(s.requireBearer)
		}
		r.Post("/inventory", s.handleInventory)
		r.Get("/power", s.handlePowerState)
		r.Get("/power/reset-types", s.handleResetTypes)
		r.Post("/power", s.handlePowerAction)
	})

	return r
}

// ListenAndServe runs the server until ctx is cancelled, then drains in-flight
// requests within shutdownGrace. It serves HTTPS when TLS cert/key are set.
func (s *Server) ListenAndServe(ctx context.Context) error {
	httpSrv := &http.Server{Addr: s.cfg.Addr, Handler: s.handler}

	errCh := make(chan error, 1)
	go func() {
		tls := s.cfg.TLSCert != "" && s.cfg.TLSKey != ""
		log.Info().Str("addr", s.cfg.Addr).Bool("tls", tls).Bool("auth", s.cfg.AuthToken != "").Msg("magellan daemon listening")
		var err error
		if tls {
			err = httpSrv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			err = httpSrv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info().Msg("shutting down magellan daemon")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
