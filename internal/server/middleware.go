package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// requestLogger emits one structured zerolog line per request. It is the
// foundation of the RFD #133 audit trail: it records the requestor, endpoint,
// outcome, and latency of every API call, including the BMC-affecting ones.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		defer func() {
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.RequestURI()).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("duration", time.Since(start)).
				Str("requestor", r.RemoteAddr).
				Str("request_id", middleware.GetReqID(r.Context())).
				Msg("api request")
		}()
		next.ServeHTTP(ww, r)
	})
}

// requireBearer enforces a static bearer token on protected routes. It is a
// deliberately simple Phase-1 gate (suitable for a token shared with trusted
// callers or injected by a gateway); JWT validation via pkg/auth is a later
// enhancement. The comparison is constant-time to avoid leaking the token.
func (s *Server) requireBearer(next http.Handler) http.Handler {
	const prefix = "Bearer "
	want := []byte(s.cfg.AuthToken)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) ||
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, prefix)), want) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
