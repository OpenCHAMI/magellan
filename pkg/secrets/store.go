package secrets

import "fmt"

// Backend selects which SecretStore implementation NewStore constructs. It
// implements pflag.Value so it can back the --secrets-backend flag.
type Backend string

// Supported secret store backends.
const (
	BackendLocal Backend = "local" // encrypted JSON file on disk
	BackendBao   Backend = "bao"   // OpenBao KV v2 secrets engine
	BackendVault Backend = "vault" // HashiCorp Vault KV v2 secrets engine
)

// BackendHelpMap maps backend names to one-line descriptions. It drives the
// --secrets-backend flag help and shell completion.
var BackendHelpMap = map[string]string{
	string(BackendLocal): "Encrypted JSON file on disk (requires MASTER_KEY)",
	string(BackendBao):   "OpenBao KV v2 secrets engine (requires BAO_ADDR and BAO_TOKEN or AppRole env vars)",
	string(BackendVault): "HashiCorp Vault KV v2 secrets engine (requires VAULT_ADDR and VAULT_TOKEN or AppRole env vars)",
}

// Set validates v and stores it as the backend. It implements pflag.Value.
func (b *Backend) Set(v string) error {
	switch Backend(v) {
	case BackendLocal, BackendBao, BackendVault:
		*b = Backend(v)
		return nil
	default:
		return fmt.Errorf("invalid secrets backend %q (valid: local, bao, vault)", v)
	}
}

// String returns the current backend as a string. It implements pflag.Value.
func (b Backend) String() string {
	return string(b)
}

// Type returns the pflag value type name. It implements pflag.Value.
func (b Backend) Type() string {
	return "Backend"
}

// Valid reports whether b is a known backend.
func (b Backend) Valid() bool {
	switch b {
	case BackendLocal, BackendBao, BackendVault:
		return true
	}
	return false
}

// StoreConfig holds the backend-independent settings NewStore needs to open a
// SecretStore. Fields that do not apply to the selected backend are ignored;
// the zero value selects the local backend.
type StoreConfig struct {
	// Backend selects the implementation (local, bao, vault). Empty means local.
	Backend Backend
	// LocalFile is the path to the encrypted JSON file (local backend only).
	LocalFile string
	// CACert is a path to a PEM CA bundle used to verify the secret manager's
	// TLS certificate (bao/vault backends only). BAO_CACERT/VAULT_CACERT take
	// precedence when set in the environment.
	CACert string
	// InsecureTLS skips TLS certificate verification for the secret manager
	// connection (bao/vault backends only), i.e. --insecure=secrets.
	InsecureTLS bool
	// Mount is the KV v2 mount path (bao/vault backends). Empty defaults to
	// BAO_MOUNT/VAULT_MOUNT or "magellan".
	Mount string
	// Prefix is an optional path prefix below the mount (bao/vault backends).
	// Empty defaults to BAO_PREFIX/VAULT_PREFIX.
	Prefix string
	// LoginPath is the AppRole login path used when no token is available
	// (bao/vault backends). Empty defaults to BAO_LOGIN_PATH/VAULT_LOGIN_PATH
	// or "auth/approle/login".
	LoginPath string
}

// NewStore opens a SecretStore for the configured backend.
//
// # Adding a new secret store backend
//
//  1. Implement the SecretStore interface in a new
//     pkg/secrets/<name>store.go.
//  2. Add a Backend constant (e.g. BackendFoo Backend = "foo") and a matching
//     BackendHelpMap entry; the help map drives --secrets-backend shell
//     completion automatically.
//  3. Add a case below mapping StoreConfig fields to your constructor.
//     Backend-specific StoreConfig fields must be optional: zero values must
//     produce a clear error, never a panic.
//  4. Add tests mirroring baostore_test.go (table-driven round-trip through
//     the four interface methods, plus auth/config error paths).
//  5. Document the backend's environment variables in README.md and
//     man/magellan.1.sc.
//
// No command files change: every consumer resolves stores through
// cmd.openSecretStore(), and the --secrets-backend flag/SECRETS_BACKEND env
// wiring is backend-agnostic.
func NewStore(cfg StoreConfig) (SecretStore, error) {
	switch cfg.Backend {
	case BackendLocal, "":
		return OpenStore(cfg.LocalFile)
	case BackendBao, BackendVault:
		return NewBaoSecretStore(cfg)
	default:
		return nil, fmt.Errorf("unknown secrets backend %q (valid: local, bao, vault)", cfg.Backend)
	}
}
