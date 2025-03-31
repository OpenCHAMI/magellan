package secrets

const DEFAULT_KEY = "default"

type SecretStore interface {
	GetSecretByID(secretID string) (string, error)
	StoreSecretByID(secretID, secret string) error
	ListSecrets() (map[string]string, error)
	RemoveSecretByID(secretID string) error
}
