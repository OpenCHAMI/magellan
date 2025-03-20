package secrets

import "fmt"

type StaticStore struct {
	Username string
	Password string
}

// NewStaticStore creates a new StaticStore with the given username and password.
func NewStaticStore(username, password string) *StaticStore {
	return &StaticStore{
		Username: username,
		Password: password,
	}
}

func (s *StaticStore) GetSecretByID(secretID string) (string, error) {
	return fmt.Sprintf(`{"username":"%s","password":"%s"}`, s.Username, s.Password), nil
}

func (s *StaticStore) StoreSecretByID(secretID, secret string) error {
	return nil
}

func (s *StaticStore) ListSecrets() (map[string]string, error) {
	return map[string]string{
		"static_creds": fmt.Sprintf(`{"username":"%s","password":"%s"}`, s.Username, s.Password),
	}, nil
}

func (s *StaticStore) RemoveSecretByID(secretID string) error {
	// Nothing to do here, since nothing is being stored. With different implementations, we could return an error when no secret is found for a specific ID.
	return nil
}
