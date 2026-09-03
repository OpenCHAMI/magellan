package bmc

import (
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
)

// ConnConfig holds everything needed to open a session to a single BMC.
//
// It is the canonical connection-configuration type for the BMC interaction
// layer. The crawler package aliases its CrawlerConfig to this type so existing
// callers continue to compile unchanged.
type ConnConfig struct {
	URI             string              // URI of the BMC (e.g. https://10.0.0.1)
	Insecure        bool                // Whether to ignore TLS verification errors
	CredentialStore secrets.SecretStore // Source of BMC credentials
	UseDefault      bool                // Retained for compatibility with existing callers
	// SecretID overrides the secret-store key used to look up credentials.
	// Callers that key credentials by something other than the BMC URI (e.g. an
	// inventory system's own identifier) set this instead.
	SecretID string
}

// credentialID is the secret-store key used to resolve this connection's
// credentials: the explicit SecretID when set, otherwise the BMC URI.
func (c ConnConfig) credentialID() string {
	if c.SecretID != "" {
		return c.SecretID
	}
	return c.URI
}

// GetUserPass resolves the BMC credentials for this connection from the
// configured secret store, falling back to the default credentials when no
// host-specific entry exists. It returns an error when the store is missing or
// the resolved credentials are blank.
func (c ConnConfig) GetUserPass() (BMCCredentials, error) {
	if c.CredentialStore == nil {
		return BMCCredentials{}, fmt.Errorf("credential store is invalid")
	}
	creds := GetBMCCredentialsOrDefault(c.CredentialStore, c.credentialID())
	if creds == (BMCCredentials{}) {
		return creds, fmt.Errorf("%s: credentials blank for BMC", c.URI)
	}
	return creds, nil
}
