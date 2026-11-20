package cmd

import (
	"github.com/openchami/magellan/pkg/secrets"
)

// openSecretStore resolves the configured secrets backend (--secrets-backend,
// default local) and opens a SecretStore. filename is the local store path,
// which is ignored by remote backends. For the bao/vault backends, the CA
// bundle comes from BAO_CACERT/VAULT_CACERT or --cacert, and TLS verification
// is skipped when --insecure includes the secrets scope.
func openSecretStore(filename string) (secrets.SecretStore, error) {
	return secrets.NewStore(secrets.StoreConfig{
		Backend:     secretsBackend,
		LocalFile:   filename,
		CACert:      cacertPath,
		InsecureTLS: insecureScope.Secrets(),
	})
}
