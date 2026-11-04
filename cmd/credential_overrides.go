package cmd

import (
	"encoding/json"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
)

// credentialOverrides applies invocation-local overrides without modifying the
// underlying store. Errors are preserved so normal default lookup still works.
type credentialOverrides struct {
	secrets.SecretStore
	username, password string
}

func (s credentialOverrides) GetSecretByID(id string) (string, error) {
	creds, err := bmc.GetBMCCredentials(s.SecretStore, id)
	if err != nil {
		return "", err
	}
	if s.username != "" {
		creds.Username = s.username
	}
	if s.password != "" {
		creds.Password = s.password
	}
	data, err := json.Marshal(creds)
	return string(data), err
}
