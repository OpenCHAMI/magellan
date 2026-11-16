package secrets

import (
	"fmt"
	"sort"
)

// SecretIDLister lists identifiers without fetching secret values. It is
// optional to preserve compatibility with existing SecretStore implementations.
type SecretIDLister interface {
	ListSecretIDs() ([]string, error)
}

// ListSecretIDs returns sorted IDs without falling back to value retrieval.
func ListSecretIDs(store SecretStore) ([]string, error) {
	lister, ok := store.(SecretIDLister)
	if !ok {
		return nil, fmt.Errorf("secret store does not support IDs-only listing")
	}
	ids, err := lister.ListSecretIDs()
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	sort.Strings(ids)
	return ids, nil
}

const DEFAULT_KEY = "default"

type SecretStore interface {
	GetSecretByID(secretID string) (string, error)
	StoreSecretByID(secretID, secret string) error
	ListSecrets() (map[string]string, error)
	RemoveSecretByID(secretID string) error
}
