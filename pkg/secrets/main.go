package secrets

type SecretStore interface {
	GetSecretByID(secretID string) (string, error)
	StoreSecretByID(secretID, secret string) error
	ListSecrets() (map[string]string, error)
	RemoveSecretByID(secretID string) error
}
