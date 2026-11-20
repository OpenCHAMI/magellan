package bmc

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
)

type rotationMemoryStore struct {
	mu          sync.Mutex
	values      map[string]string
	writes      int
	failOnWrite int
}

func newRotationMemoryStore(values map[string]string) *rotationMemoryStore {
	return &rotationMemoryStore{values: values}
}

func (s *rotationMemoryStore) GetSecretByID(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[id]
	if !ok {
		return "", fmt.Errorf("secret %q not found", id)
	}
	return value, nil
}

func (s *rotationMemoryStore) StoreSecretByID(id, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	if s.writes == s.failOnWrite {
		return errors.New("injected store failure")
	}
	s.values[id] = value
	return nil
}

func (s *rotationMemoryStore) ListSecrets() (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := make(map[string]string, len(s.values))
	for key, value := range s.values {
		copy[key] = value
	}
	return copy, nil
}

func (s *rotationMemoryStore) RemoveSecretByID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, id)
	return nil
}

func (s *rotationMemoryStore) value(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[id]
}

type rotationFakeTarget struct {
	mu               sync.Mutex
	username         string
	accountUsername  string
	password         string
	applyThenFail    bool
	failNextPatch    bool
	updateCalls      int
	activeUpdates    int
	maxActiveUpdates int
	updateEntered    chan struct{}
	continueUpdate   chan struct{}
}

func (t *rotationFakeTarget) AccountUsername(_, username, password string) (string, error) {
	if err := t.Authenticate(username, password); err != nil {
		return "", err
	}
	if t.accountUsername != "" {
		return t.accountUsername, nil
	}
	return t.username, nil
}

func (t *rotationFakeTarget) SetAccountPassword(_, username, password, newPassword string) error {
	t.mu.Lock()
	t.activeUpdates++
	if t.activeUpdates > t.maxActiveUpdates {
		t.maxActiveUpdates = t.activeUpdates
	}
	t.updateCalls++
	call := t.updateCalls
	if t.updateEntered != nil && call == 1 {
		close(t.updateEntered)
	}
	t.mu.Unlock()

	if t.continueUpdate != nil && call == 1 {
		<-t.continueUpdate
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	defer func() { t.activeUpdates-- }()
	if username != t.username || password != t.password {
		return errors.New("old credential rejected")
	}
	t.password = newPassword
	if t.failNextPatch || t.applyThenFail {
		t.failNextPatch = false
		return errors.New("injected Redfish timeout")
	}
	return nil
}

func (t *rotationFakeTarget) Authenticate(username, password string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if username != t.username || password != t.password {
		return errors.New("authentication rejected")
	}
	return nil
}

func rotationRecordJSON(t *testing.T, value string) rotationCredentialRecord {
	t.Helper()
	var record rotationCredentialRecord
	require.NoError(t, json.Unmarshal([]byte(value), &record))
	return record
}

func TestRotateAccountPasswordCommitsAfterVerification(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old"}`,
	})
	target := &rotationFakeTarget{username: "admin", password: "old"}

	require.NoError(t, RotateAccountPassword(store, "https://bmc.example", "1", "new", target))
	record := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "admin", record.Username)
	require.Equal(t, "new", record.Password)
	require.Nil(t, record.Pending)
}

func TestRotateAccountPasswordHandlesAppliedPatchTimeout(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old"}`,
	})
	target := &rotationFakeTarget{username: "admin", password: "old", applyThenFail: true}

	require.NoError(t, RotateAccountPassword(store, "https://bmc.example", "1", "new", target))
	record := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "new", record.Password)
	require.Nil(t, record.Pending)
	require.Equal(t, 1, target.updateCalls)
}

func TestRecoverAccountPasswordRotationRetriesWhenBMCStillHasOldPassword(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old","pending_rotation":{"account_id":"1","new_password":"new"}}`,
	})
	target := &rotationFakeTarget{username: "admin", password: "old"}

	require.NoError(t, RecoverAccountPasswordRotation(store, "https://bmc.example", target))
	record := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "new", record.Password)
	require.Nil(t, record.Pending)
	require.Equal(t, 1, target.updateCalls)
}

func TestRecoverAccountPasswordRotationCommitsWhenBMCAlreadyChanged(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old","pending_rotation":{"account_id":"1","new_password":"new"}}`,
	})
	target := &rotationFakeTarget{username: "admin", password: "new"}

	require.NoError(t, RecoverAccountPasswordRotation(store, "https://bmc.example", target))
	record := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "new", record.Password)
	require.Nil(t, record.Pending)
	require.Zero(t, target.updateCalls)
}

func TestPendingRotationRecordKeepsExistingCredentialReadCompatibility(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old","pending_rotation":{"account_id":"1","new_password":"new"}}`,
	})

	credentials, err := GetBMCCredentials(store, "https://bmc.example")
	require.NoError(t, err)
	require.Equal(t, BMCCredentials{Username: "admin", Password: "old"}, credentials)
}

func TestRotateAccountPasswordRetainsIntentWhenStoreCommitFails(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old"}`,
	})
	store.failOnWrite = 2
	target := &rotationFakeTarget{username: "admin", password: "old"}

	err := RotateAccountPassword(store, "https://bmc.example", "1", "new", target)
	require.ErrorContains(t, err, "credential-store commit failed")
	interrupted := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "old", interrupted.Password)
	require.NotNil(t, interrupted.Pending)

	require.NoError(t, RecoverAccountPasswordRotation(store, "https://bmc.example", target))
	recovered := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "new", recovered.Password)
	require.Nil(t, recovered.Pending)
}

func TestRotateAccountPasswordDoesNotChangeBMCWhenIntentWriteFails(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old"}`,
	})
	store.failOnWrite = 1
	target := &rotationFakeTarget{username: "admin", password: "old"}

	err := RotateAccountPassword(store, "https://bmc.example", "1", "new", target)
	require.ErrorContains(t, err, "persist rotation intent")
	require.Equal(t, "old", target.password)
	require.Equal(t, 0, target.updateCalls)
}

func TestRotateAccountPasswordStoresHostOverrideInsteadOfChangingDefault(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		secrets.DEFAULT_KEY: `{"username":"admin","password":"old"}`,
	})
	target := &rotationFakeTarget{username: "admin", password: "old"}

	require.NoError(t, RotateAccountPassword(store, "https://bmc.example", "1", "new", target))
	require.Equal(t, `{"username":"admin","password":"old"}`, store.value(secrets.DEFAULT_KEY))
	record := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "new", record.Password)
}

func TestRotateAccountPasswordRejectsAnotherAccount(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old"}`,
	})
	target := &rotationFakeTarget{username: "admin", accountUsername: "other-user", password: "old"}

	err := RotateAccountPassword(store, "https://bmc.example", "2", "new", target)
	require.ErrorContains(t, err, "not the credential-store username")
	require.Equal(t, 0, target.updateCalls)
	require.NotContains(t, store.value("https://bmc.example"), "pending_rotation")
}

func TestConcurrentCredentialRotationsAreSerialized(t *testing.T) {
	store := newRotationMemoryStore(map[string]string{
		"https://bmc.example": `{"username":"admin","password":"old"}`,
	})
	target := &rotationFakeTarget{
		username:       "admin",
		password:       "old",
		updateEntered:  make(chan struct{}),
		continueUpdate: make(chan struct{}),
	}
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() {
		firstDone <- RotateAccountPassword(store, "https://bmc.example", "1", "first", target)
	}()
	<-target.updateEntered
	go func() {
		secondDone <- RotateAccountPassword(store, "https://bmc.example", "1", "second", target)
	}()
	close(target.continueUpdate)

	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)
	require.Equal(t, 1, target.maxActiveUpdates)
	record := rotationRecordJSON(t, store.value("https://bmc.example"))
	require.Equal(t, "second", record.Password)
	require.Nil(t, record.Pending)
}

func TestGenerateBMCPassword(t *testing.T) {
	password, err := GenerateBMCPassword()
	require.NoError(t, err)
	require.Len(t, password, 32)
	require.Regexp(t, `^[A-Za-z0-9_-]+$`, password)
}
