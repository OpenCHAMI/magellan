package bmc

import (
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
)

// Manager opens sessions to BMCs using the shared connection helper,
// performs vendor detection, and maintains
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

// Connect opens a new, uncached gofish session to the BMC described by cfg using
// the shared credential and TLS configuration helper.
func (m *Manager) Connect(cfg ConnConfig) (*gofish.APIClient, error) {
	return ConnectWithCredentials(cfg.URI, cfg.CredentialStore, cfg.Insecure, cfg.CACertPath)
}

// Client opens a new, uncached session and wraps it in a vendor-aware Client.
// The caller is responsible for calling Logout on the returned Client.
func (m *Manager) Client(cfg ConnConfig) (Client, error) {
	api, err := m.Connect(cfg)
	if err != nil {
		return nil, err
	}
	return clientFor(api), nil
}

// CachedClient returns a vendor-aware Client for the BMC, creating and caching a
// new session keyed by URI if one does not already exist. Cached sessions are
// kept open for efficiency and released together by LogoutAll.
func (m *Manager) CachedClient(cfg ConnConfig) (Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.cache[cfg.URI]; ok {
		log.Debug().Msgf("found existing client for %s", cfg.URI)
		return c, nil
	}
	api, err := m.Connect(cfg)
	if err != nil {
		return nil, err
	}
	c := clientFor(api)
	m.cache[cfg.URI] = c
	log.Debug().Msgf("created new client for %s", cfg.URI)
	return c, nil
}

// LogoutAll logs out and evicts every cached session. It should be called as a
// post-execution cleanup step.
func (m *Manager) LogoutAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for uri, c := range m.cache {
		log.Debug().Msgf("logging out client for %s", uri)
		c.Logout()
		delete(m.cache, uri)
	}
}
