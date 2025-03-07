package secrets

import (
	"testing"
)

func TestDeriveAESKey(t *testing.T) {
	masterKey := []byte("testmasterkey")
	secretID := "mySecretID"
	key1 := deriveAESKey(masterKey, secretID)
	key2 := deriveAESKey(masterKey, secretID)

	if len(key1) != 32 {
		t.Errorf("derived key should be 32 bytes, got %d", len(key1))
	}
	if string(key1) != string(key2) {
		t.Errorf("keys derived from same secretID should match")
	}
}

func TestEncryptDecryptAESGCM(t *testing.T) {
	masterKey := []byte("anotherTestMasterKey")
	secretID := "testSecret"
	plaintext := "Hello, secrets!"

	key := deriveAESKey(masterKey, secretID)

	encrypted, err := encryptAESGCM(key, []byte(plaintext))
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	decrypted, err := decryptAESGCM(key, encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}
