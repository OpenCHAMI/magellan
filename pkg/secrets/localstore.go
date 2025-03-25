package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Structure to store encrypted secrets in a JSON file
type LocalSecretStore struct {
	mu        sync.RWMutex
	masterKey []byte
	filename  string
	Secrets   map[string]string `json:"secrets"`
}

func NewLocalSecretStore(masterKeyHex, filename string, create bool) (*LocalSecretStore, error) {
	var secrets map[string]string

	masterKey, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("unable to generate masterkey from hex representation: %v", err)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if !create {
			return nil, fmt.Errorf("file %s does not exist", filename)
		}
		file, err := os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("unable to create file %s: %v", filename, err)
		}
		file.Close()
		secrets = make(map[string]string)
	}

	if secrets == nil {
		secrets, err = loadSecrets(filename)
		if err != nil {
			return nil, fmt.Errorf("unable to load secrets from file: %v", err)
		}
	}

	return &LocalSecretStore{
		masterKey: masterKey,
		filename:  filename,
		Secrets:   secrets,
	}, nil
}

// GenerateMasterKey creates a 32-byte random key and returns it as a hex string.
func GenerateMasterKey() (string, error) {
	key := make([]byte, 32) // 32 bytes for AES-256
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// GetSecretByID decrypts the secret using the master key and returns it
func (l *LocalSecretStore) GetSecretByID(secretID string) (string, error) {
	l.mu.RLock()
	encrypted, exists := l.Secrets[secretID]
	l.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("no secret found for %s", secretID)
	}

	derivedKey := deriveAESKey(l.masterKey, secretID)
	return decryptAESGCM(derivedKey, encrypted)
}

// StoreSecretByID encrypts the secret using the master key and stores it in the JSON file
func (l *LocalSecretStore) StoreSecretByID(secretID, secret string) error {
	derivedKey := deriveAESKey(l.masterKey, secretID)
	encryptedSecret, err := encryptAESGCM(derivedKey, []byte(secret))
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.Secrets[secretID] = encryptedSecret
	err = SaveSecrets(l.filename, l.Secrets)
	l.mu.Unlock()
	return err
}

// ListSecrets returns a copy of secret IDs to secrets stored in memory
func (l *LocalSecretStore) ListSecrets() (map[string]string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	secretsCopy := make(map[string]string)
	for key, value := range l.Secrets {
		secretsCopy[key] = value
	}
	return secretsCopy, nil
}

// RemoveSecretByID removes the specified secretID stored locally
func (l *LocalSecretStore) RemoveSecretByID(secretID string) error {
	l.mu.RLock()
	// Let user know if there was nothing to delete
	_, err := l.GetSecretByID(secretID)
	if err != nil {
		return err
	}
	delete(l.Secrets, secretID)
	l.mu.RUnlock()
	return nil
}

// openStore tries to create or open the LocalSecretStore based on the environment
// variable MASTER_KEY. If not found, it prints an error.
func OpenStore(filename string) (SecretStore, error) {
	if filename == "" {
		return nil, fmt.Errorf("path to secret store required")
	}

	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		return nil, fmt.Errorf("MASTER_KEY environment variable not set")
	}

	store, err := NewLocalSecretStore(masterKey, filename, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create new local secret store: %v", err)
	}
	return store, nil
}

// Saves secrets back to the JSON file
func SaveSecrets(jsonFile string, store map[string]string) error {
	file, err := os.OpenFile(jsonFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(store)
}

// Loads the secrets JSON file
func loadSecrets(jsonFile string) (map[string]string, error) {
	file, err := os.Open(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("unable to open secret file %s:%v", jsonFile, err)
	}
	defer file.Close()

	store := make(map[string]string)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&store)
	return store, err
}
