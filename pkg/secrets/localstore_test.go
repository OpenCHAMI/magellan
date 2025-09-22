package secrets

import (
	"encoding/hex"
	"os"
	"testing"

	"github.com/rs/zerolog/log"
)

func TestNewLocalSecretStore(t *testing.T) {
	masterKey, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	filename := "test_secrets.json"
	defer func() {
		if err = os.Remove(filename); err != nil {
			log.Warn().Err(err).Msg("could not close response resource")
		}
	}()

	store, err := NewLocalSecretStore(masterKey, filename, true)
	if err != nil {
		t.Fatalf("Failed to create LocalSecretStore: %v", err)
	}

	if store.filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, store.filename)
	}

	if hex.EncodeToString(store.masterKey) != masterKey {
		t.Errorf("Expected master key %s, got %s", masterKey, hex.EncodeToString(store.masterKey))
	}
}

func TestGenerateMasterKey(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	if len(key) != 64 { // 32 bytes in hex representation
		t.Errorf("Expected key length 64, got %d", len(key))
	}
}

func TestStoreAndGetSecretByID(t *testing.T) {
	masterKey, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	filename := "test_secrets.json"
	defer func() {
		if err = os.Remove(filename); err != nil {
			log.Warn().Err(err).Msg("could not close response resource")
		}
	}()

	store, err := NewLocalSecretStore(masterKey, filename, true)
	if err != nil {
		t.Fatalf("Failed to create LocalSecretStore: %v", err)
	}

	secretID := "test_secret"
	secretValue := "my_secret_value"

	err = store.StoreSecretByID(secretID, secretValue)
	if err != nil {
		t.Fatalf("Failed to store secret: %v", err)
	}

	retrievedSecret, err := store.GetSecretByID(secretID)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	if retrievedSecret != secretValue {
		t.Errorf("Expected secret value %s, got %s", secretValue, retrievedSecret)
	}
}

func TestStoreAndGetSecretJSON(t *testing.T) {
	masterKey, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	filename := "test_secrets.json"
	defer func() {
		if err = os.Remove(filename); err != nil {
			log.Warn().Err(err).Msg("could not close response resource")
		}
	}()

	store, err := NewLocalSecretStore(masterKey, filename, true)
	if err != nil {
		t.Fatalf("Failed to create LocalSecretStore: %v", err)
	}

	secretID := "json_creds"
	jsonSecret := `{"username":"testUser","password":"testPass"}`

	if err := store.StoreSecretByID(secretID, jsonSecret); err != nil {
		t.Fatalf("Failed to store JSON secret: %v", err)
	}

	retrieved, err := store.GetSecretByID(secretID)
	if err != nil {
		t.Fatalf("Failed to get JSON secret by ID: %v", err)
	}

	if retrieved != jsonSecret {
		t.Errorf("Expected %s, got %s", jsonSecret, retrieved)
	}
}

func TestListSecrets(t *testing.T) {
	masterKey, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	filename := "test_secrets.json"

	store, err := NewLocalSecretStore(masterKey, filename, true)
	if err != nil {
		t.Fatalf("Failed to create LocalSecretStore: %v", err)
	}
	defer func() {
		if err = os.Remove(filename); err != nil {
			log.Warn().Err(err).Msg("could not close response resource")
		}
	}()

	secretID1 := "test_secret_1"
	secretValue1 := "my_secret_value_1"
	secretID2 := "test_secret_2"
	secretValue2 := "my_secret_value_2"

	err = store.StoreSecretByID(secretID1, secretValue1)
	if err != nil {
		t.Fatalf("Failed to store secret: %v", err)
	}

	err = store.StoreSecretByID(secretID2, secretValue2)
	if err != nil {
		t.Fatalf("Failed to store secret: %v", err)
	}

	secrets, err := store.ListSecrets()
	if err != nil {
		t.Fatalf("Failed to list secrets: %v", err)
	}

	if len(secrets) != 2 {
		t.Errorf("Expected 2 secrets, got %d", len(secrets))
	}

	if secrets[secretID1] != store.Secrets[secretID1] {
		t.Errorf("Expected secret value %s, got %s", store.Secrets[secretID1], secrets[secretID1])
	}

	if secrets[secretID2] != store.Secrets[secretID2] {
		t.Errorf("Expected secret value %s, got %s", store.Secrets[secretID2], secrets[secretID2])
	}
	if err = os.Remove(filename); err != nil {
		log.Warn().Err(err).Msg("could not close response resource")
	}
}
