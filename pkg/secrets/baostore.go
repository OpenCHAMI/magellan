package secrets

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/openbao/openbao/api/v2"
	"github.com/rs/zerolog/log"
)

// baoSecretField is the single KV v2 data field magellan stores credentials
// under. The field value is the credential JSON ({"username":...,
// "password":...}) that bmc.BMCCredentials expects.
const baoSecretField = "value"

// baoDefaultMount is the KV v2 mount path used when neither StoreConfig.Mount
// nor BAO_MOUNT/VAULT_MOUNT is set.
const baoDefaultMount = "magellan"

// baoDefaultLoginPath is the AppRole login path used when neither
// StoreConfig.LoginPath nor BAO_LOGIN_PATH/VAULT_LOGIN_PATH is set.
const baoDefaultLoginPath = "auth/approle/login"

// BaoSecretStore implements SecretStore against an OpenBao (or HashiCorp
// Vault) KV v2 secrets engine. One implementation serves both products: the
// OpenBao API prefers BAO_* environment variables and falls back to the
// VAULT_* equivalents, so --secrets-backend bao and --secrets-backend vault
// differ only in naming convention.
//
// Secret IDs are full BMC URIs (e.g. "https://172.16.0.10"), which are not
// usable as KV paths directly, so they are base64url-encoded into a single
// path segment below the configured mount and prefix. The literal DEFAULT_KEY
// ("default") passes through unchanged. ListSecrets reverses the mapping.
type BaoSecretStore struct {
	client    *api.Client
	mount     string
	prefix    string
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}

// NewBaoSecretStore builds a BaoSecretStore from cfg. Connection details come
// from the environment: BAO_ADDR/VAULT_ADDR for the server, BAO_TOKEN/
// VAULT_TOKEN or BAO_ROLE_ID+BAO_SECRET_ID (AppRole) for credentials, and
// BAO_CACERT/VAULT_CACERT for the CA bundle. StoreConfig supplies a fallback CA
// bundle (CACert), disables TLS verification (InsecureTLS, i.e.
// --insecure=secrets), and set the mount path, prefix, and AppRole login
// path.
func NewBaoSecretStore(cfg StoreConfig) (*BaoSecretStore, error) {
	apiCfg := api.DefaultConfig()
	if apiCfg.Error != nil {
		return nil, fmt.Errorf("failed to configure OpenBao/Vault client: %w", apiCfg.Error)
	}

	// Preserve secret-manager CA files, bytes, or directories loaded by the
	// SDK. The BMC CA is a fallback only; insecure scope may disable verification.
	if cfg.CACert != "" || cfg.InsecureTLS {
		tlsCfg := &api.TLSConfig{}
		if cfg.CACert != "" && firstEnv("BAO_CACERT", "VAULT_CACERT", "BAO_CACERT_BYTES", "VAULT_CACERT_BYTES", "BAO_CAPATH", "VAULT_CAPATH") == "" {
			tlsCfg.CACert = cfg.CACert
		}
		if cfg.InsecureTLS {
			tlsCfg.Insecure = true
		}
		if err := apiCfg.ConfigureTLS(tlsCfg); err != nil {
			return nil, fmt.Errorf("failed to configure TLS for OpenBao/Vault: %w", err)
		}
	}

	client, err := api.NewClient(apiCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenBao/Vault client: %w", err)
	}

	// NewClient picks up BAO_TOKEN/VAULT_TOKEN automatically; fall back to
	// AppRole login when no token is available.
	var login *api.Secret
	if client.Token() == "" {
		login, err = baoAppRoleLogin(context.Background(), client, cfg)
		if err != nil {
			return nil, err
		}
	}

	mount := cfg.Mount
	if mount == "" {
		mount = firstEnv("BAO_MOUNT", "VAULT_MOUNT")
	}
	if mount == "" {
		mount = baoDefaultMount
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = firstEnv("BAO_PREFIX", "VAULT_PREFIX")
	}

	store := &BaoSecretStore{
		client: client,
		mount:  strings.Trim(mount, "/"),
		prefix: strings.Trim(prefix, "/"),
	}
	if login != nil {
		ctx, cancel := context.WithCancel(context.Background())
		store.cancel = cancel
		store.done = make(chan struct{})
		go store.maintainLogin(ctx, cfg, login)
	}
	return store, nil
}

// baoAppRoleLogin authenticates with AppRole credentials read from the
// BAO_ROLE_ID/BAO_SECRET_ID (or VAULT_ROLE_ID/VAULT_SECRET_ID) environment
// variables and installs the returned token on client.
func baoAppRoleLogin(ctx context.Context, client *api.Client, cfg StoreConfig) (*api.Secret, error) {
	roleID := firstEnv("BAO_ROLE_ID", "VAULT_ROLE_ID")
	secretID := firstEnv("BAO_SECRET_ID", "VAULT_SECRET_ID")
	if roleID == "" || secretID == "" {
		return nil, fmt.Errorf("no credentials for OpenBao/Vault: set BAO_TOKEN/VAULT_TOKEN or BAO_ROLE_ID and BAO_SECRET_ID")
	}

	loginPath := cfg.LoginPath
	if loginPath == "" {
		loginPath = firstEnv("BAO_LOGIN_PATH", "VAULT_LOGIN_PATH")
	}
	if loginPath == "" {
		loginPath = baoDefaultLoginPath
	}

	secret, err := client.Logical().WriteWithContext(ctx, loginPath, map[string]any{
		"role_id":   roleID,
		"secret_id": secretID,
	})
	if err != nil {
		return nil, fmt.Errorf("AppRole login to OpenBao/Vault failed: %w", err)
	}
	if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
		return nil, fmt.Errorf("AppRole login to OpenBao/Vault returned no client token")
	}
	client.SetToken(secret.Auth.ClientToken)
	return secret, nil
}

// Close stops AppRole renewal and cancels any in-flight reauthentication.
func (b *BaoSecretStore) Close() error {
	b.closeOnce.Do(func() {
		if b.cancel != nil {
			b.cancel()
			<-b.done
		}
	})
	return nil
}

func (b *BaoSecretStore) maintainLogin(ctx context.Context, cfg StoreConfig, secret *api.Secret) {
	defer close(b.done)
	for ctx.Err() == nil {
		// Tokens with no lease do not need renewal.
		if secret.Auth.LeaseDuration == 0 {
			return
		}
		watcher, err := b.client.NewLifetimeWatcher(&api.LifetimeWatcherInput{Secret: secret})
		if err == nil {
			go watcher.Start()
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case watchErr := <-watcher.DoneCh():
				watcher.Stop()
				if watchErr != nil {
					log.Warn().Err(watchErr).Msg("OpenBao/Vault AppRole token renewal ended; reauthenticating")
				}
			}
		}
		// A nonrenewable or exhausted token needs a new AppRole login.
		// Retry transient failures without spinning or logging credentials.
		// Very short leases may exhaust the watcher's grace immediately.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		for delay := time.Second; ctx.Err() == nil; {
			secret, err = baoAppRoleLogin(ctx, b.client, cfg)
			if err == nil {
				break
			}
			log.Warn().Msg("OpenBao/Vault AppRole reauthentication failed; retrying")
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			delay = min(delay*2, 30*time.Second)
		}
	}
}

// firstEnv returns the first non-empty environment variable value.
func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

// secretPath maps a secret ID to a KV v2 secret path relative to the mount.
// DEFAULT_KEY passes through unchanged; everything else is base64url-encoded
// so any URI (colons, slashes, dots) becomes one valid path segment.
func (b *BaoSecretStore) secretPath(secretID string) string {
	name := secretID
	if name != DEFAULT_KEY {
		name = base64.RawURLEncoding.EncodeToString([]byte(name))
	}
	if b.prefix != "" {
		return b.prefix + "/" + name
	}
	return name
}

// secretIDFromName is the inverse of secretPath for the listing keys
// returned by ListSecrets (which are relative to the configured prefix).
func (b *BaoSecretStore) secretIDFromName(name string) string {
	if name == DEFAULT_KEY {
		return name
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(name); err == nil {
		return string(decoded)
	}
	// Not a magellan-generated key; surface it verbatim.
	return name
}

// getSecret reads the credential JSON stored at a KV v2 path relative to the
// mount.
func (b *BaoSecretStore) getSecret(secretPath string) (string, error) {
	secret, err := b.client.KVv2(b.mount).Get(context.Background(), secretPath)
	if err != nil {
		return "", err
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("no secret found at %s/%s", b.mount, secretPath)
	}
	value, ok := secret.Data[baoSecretField].(string)
	if !ok {
		return "", fmt.Errorf("secret at %s/%s has no %q field", b.mount, secretPath, baoSecretField)
	}
	return value, nil
}

// GetSecretByID returns the credential JSON stored for secretID.
func (b *BaoSecretStore) GetSecretByID(secretID string) (string, error) {
	value, err := b.getSecret(b.secretPath(secretID))
	if err != nil {
		return "", fmt.Errorf("failed to retrieve secret %q: %w", secretID, err)
	}
	return value, nil
}

// StoreSecretByID writes the credential JSON under secretID's KV v2 path.
func (b *BaoSecretStore) StoreSecretByID(secretID, secret string) error {
	data := map[string]any{baoSecretField: secret}
	if _, err := b.client.KVv2(b.mount).Put(context.Background(), b.secretPath(secretID), data); err != nil {
		return fmt.Errorf("failed to store secret %q: %w", secretID, err)
	}
	return nil
}

// ListSecrets lists every secret ID below the configured prefix and reads
// each value, matching the local store's behavior. This requires a policy
// with read access to the KV v2 engine, not just list.
func (b *BaoSecretStore) ListSecrets() (map[string]string, error) {
	list, err := b.client.KVv2(b.mount).List(context.Background(), b.prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets at %s/%s: %w", b.mount, b.prefix, err)
	}

	secretsByID := make(map[string]string)
	if list == nil {
		return secretsByID, nil
	}
	for _, name := range list.Keys {
		// Sub-paths end in "/"; magellan stores secrets one level deep.
		if strings.HasSuffix(name, "/") {
			log.Debug().Str("path", b.mount+"/"+b.prefix).Str("key", name).Msg("skipping subdirectory in secrets listing")
			continue
		}
		secretPath := name
		if b.prefix != "" {
			secretPath = b.prefix + "/" + name
		}
		value, err := b.getSecret(secretPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read secret %q: %w", name, err)
		}
		secretsByID[b.secretIDFromName(name)] = value
	}
	return secretsByID, nil
}

// RemoveSecretByID deletes the secret and all of its versions.
func (b *BaoSecretStore) RemoveSecretByID(secretID string) error {
	if err := b.client.KVv2(b.mount).DeleteMetadata(context.Background(), b.secretPath(secretID)); err != nil {
		return fmt.Errorf("failed to remove secret %q: %w", secretID, err)
	}
	return nil
}
