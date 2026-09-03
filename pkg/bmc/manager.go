package bmc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
)

// Manager is the single authority for opening sessions to BMCs. It owns the one
// place where gofish.Connect is called, performs vendor detection, and maintains
// an optional per-URI connection cache so callers can reuse open sessions.
type Manager struct {
	mu    sync.Mutex
	cache map[string]Client
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{cache: make(map[string]Client)}
}

// DefaultManager is the process-wide Manager used by the CLI call sites. The
// daemon may construct its own Manager instead.
var DefaultManager = NewManager()

// ConnectContext opens a new, uncached gofish session to the BMC described by
// cfg, scoping the session's request context to ctx. This is the single point in
// the codebase where gofish.Connect(Context) is invoked; all 404 and 401 errors
// are decorated here for consistent messaging. Because gofish binds the context
// at connection time, ctx governs the request deadline/cancellation for every
// call made through the returned client.
func (m *Manager) ConnectContext(ctx context.Context, cfg ConnConfig) (*gofish.APIClient, error) {
	creds, err := cfg.GetUserPass()
	if err != nil {
		log.Error().Err(err).Msg("failed to load BMC credentials")
		return nil, err
	}

	api, err := gofish.ConnectContext(ctx, gofish.ClientConfig{
		Endpoint:  cfg.URI,
		Username:  creds.Username,
		Password:  creds.Password,
		Insecure:  cfg.Insecure,
		BasicAuth: true,
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "404:") {
			err = fmt.Errorf("no ServiceRoot found.  This is probably not a BMC: %s", cfg.URI)
		}
		if strings.HasPrefix(err.Error(), "401:") {
			err = fmt.Errorf("authentication failed.  Check your username and password: %s", cfg.URI)
		}
		log.Error().Err(err).Msg("failed to connect to BMC")
		return nil, err
	}
	return api, nil
}

// Connect opens a new, uncached gofish session using a background context. It is
// retained for the raw-gofish call sites (crawler, collect) that do not yet
// thread a context; prefer ConnectContext where a context is available.
func (m *Manager) Connect(cfg ConnConfig) (*gofish.APIClient, error) {
	return m.ConnectContext(context.Background(), cfg)
}

// Client opens a new, uncached session and wraps it in a vendor-aware Client.
// The caller is responsible for calling Logout on the returned Client.
func (m *Manager) Client(ctx context.Context, cfg ConnConfig) (Client, error) {
	api, err := m.ConnectContext(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return clientFor(api), nil
}

// CachedClient returns a vendor-aware Client for the BMC, creating and caching a
// new session if one does not already exist. Cached sessions are kept open for
// efficiency and released together by LogoutAll.
//
// Note: ctx scopes the session only when a new one is opened; a cache hit
// returns a session bound to the context it was originally connected with. For
// per-request cancellation across many callers (e.g. the daemon), prefer Client
// to obtain an uncached, request-scoped session.
func (m *Manager) CachedClient(ctx context.Context, cfg ConnConfig) (Client, error) {
	// The key includes the credential identity so two callers targeting the same
	// BMC under different credentials never share a session.
	key := cfg.URI + "\x00" + cfg.credentialID()
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.cache[key]; ok {
		log.Debug().Msgf("found existing client for %s", cfg.URI)
		return c, nil
	}
	api, err := m.ConnectContext(ctx, cfg)
	if err != nil {
		return nil, err
	}
	c := clientFor(api)
	m.cache[key] = c
	log.Debug().Msgf("created new client for %s", cfg.URI)
	return c, nil
}

// LogoutAll logs out and evicts every cached session. It should be called as a
// post-execution cleanup step.
func (m *Manager) LogoutAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, c := range m.cache {
		log.Debug().Msgf("logging out client for %s", key)
		c.Logout()
		delete(m.cache, key)
	}
}
