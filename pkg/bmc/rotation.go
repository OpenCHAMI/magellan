package bmc

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stmcginnis/gofish"
)

// RotationTarget provides the BMC operations needed to rotate one account.
type RotationTarget interface {
	AccountUsername(accountID, username, password string) (string, error)
	SetAccountPassword(accountID, username, password, newPassword string) error
	Authenticate(username, password string) error
}

type pendingCredentialRotation struct {
	AccountID   string    `json:"account_id"`
	NewPassword string    `json:"new_password"`
	StartedAt   time.Time `json:"started_at"`
}

type rotationCredentialRecord struct {
	Username string                     `json:"username"`
	Password string                     `json:"password"`
	Pending  *pendingCredentialRotation `json:"pending_rotation,omitempty"`
}

type credentialRotationLock struct {
	mu   sync.Mutex
	refs int
}

var credentialRotationLocks = struct {
	sync.Mutex
	locks map[string]*credentialRotationLock
}{locks: make(map[string]*credentialRotationLock)}

func lockCredentialRotation(secretID string) func() {
	credentialRotationLocks.Lock()
	lock := credentialRotationLocks.locks[secretID]
	if lock == nil {
		lock = &credentialRotationLock{}
		credentialRotationLocks.locks[secretID] = lock
	}
	lock.refs++
	credentialRotationLocks.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		credentialRotationLocks.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(credentialRotationLocks.locks, secretID)
		}
		credentialRotationLocks.Unlock()
	}
}

// GenerateBMCPassword returns a 32-character URL-safe password. The generated
// value is intended for BMCs with no site-specific password policy configured.
func GenerateBMCPassword() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate BMC password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// RotateAccountPassword updates the password for the account used by the
// supplied BMC credential, then persists the new credential. A pending marker
// is written first so a later RecoverAccountPasswordRotation call can resolve
// crashes and ambiguous Redfish responses.
func RotateAccountPassword(store secrets.SecretStore, secretID, accountID, newPassword string, target RotationTarget) error {
	if store == nil || target == nil {
		return fmt.Errorf("credential store and rotation target are required")
	}
	if secretID == "" || accountID == "" || newPassword == "" {
		return fmt.Errorf("BMC secret ID, account ID, and new password are required")
	}
	unlock := lockCredentialRotation(secretID)
	defer unlock()

	record, err := loadRotationRecord(store, secretID)
	if err != nil {
		return err
	}
	if record.Pending != nil {
		return fmt.Errorf("a password rotation is already pending for account %q; recover it before starting another", record.Pending.AccountID)
	}
	if record.Password == newPassword {
		return fmt.Errorf("new password must differ from the current password")
	}

	username, err := target.AccountUsername(accountID, record.Username, record.Password)
	if err != nil {
		return fmt.Errorf("verify BMC account before rotation: %w", err)
	}
	if username != record.Username {
		return fmt.Errorf("account %q is %q, not the credential-store username", accountID, username)
	}

	record.Pending = &pendingCredentialRotation{
		AccountID:   accountID,
		NewPassword: newPassword,
		StartedAt:   time.Now().UTC(),
	}
	if err := saveRotationRecord(store, secretID, record); err != nil {
		return fmt.Errorf("persist rotation intent before changing BMC: %w", err)
	}

	if err := target.SetAccountPassword(accountID, record.Username, record.Password, newPassword); err != nil {
		if authErr := target.Authenticate(record.Username, newPassword); authErr == nil {
			return commitRotation(store, secretID, record)
		}
		return fmt.Errorf("BMC password update outcome is uncertain; rotation intent was retained for recovery: %w", err)
	}
	if err := target.Authenticate(record.Username, newPassword); err != nil {
		return fmt.Errorf("BMC accepted the password update but the new credential could not be verified; rotation intent was retained for recovery: %w", err)
	}
	return commitRotation(store, secretID, record)
}

// RecoverAccountPasswordRotation resumes an interrupted rotation. It first
// tests the pending credential; if the BMC still accepts the old one, it
// retries the idempotent Redfish password PATCH and verifies the new one.
func RecoverAccountPasswordRotation(store secrets.SecretStore, secretID string, target RotationTarget) error {
	if store == nil || target == nil || secretID == "" {
		return fmt.Errorf("credential store, BMC secret ID, and rotation target are required")
	}
	unlock := lockCredentialRotation(secretID)
	defer unlock()
	record, err := loadRotationRecord(store, secretID)
	if err != nil {
		return err
	}
	if record.Pending == nil {
		return fmt.Errorf("no password rotation is pending for BMC %q", secretID)
	}
	pending := record.Pending

	if err := target.Authenticate(record.Username, pending.NewPassword); err == nil {
		return commitRotation(store, secretID, record)
	}
	if err := target.Authenticate(record.Username, record.Password); err != nil {
		return fmt.Errorf("neither old nor pending BMC credentials authenticate; rotation intent was retained: %w", err)
	}
	username, err := target.AccountUsername(pending.AccountID, record.Username, record.Password)
	if err != nil {
		return fmt.Errorf("verify BMC account during rotation recovery: %w", err)
	}
	if username != record.Username {
		return fmt.Errorf("pending account %q is %q, not the credential-store username", pending.AccountID, username)
	}
	if err := target.SetAccountPassword(pending.AccountID, record.Username, record.Password, pending.NewPassword); err != nil {
		if authErr := target.Authenticate(record.Username, pending.NewPassword); authErr == nil {
			return commitRotation(store, secretID, record)
		}
		return fmt.Errorf("retrying BMC password update failed; rotation intent was retained: %w", err)
	}
	if err := target.Authenticate(record.Username, pending.NewPassword); err != nil {
		return fmt.Errorf("retried BMC password update but new credential could not be verified; rotation intent was retained: %w", err)
	}
	return commitRotation(store, secretID, record)
}

func loadRotationRecord(store secrets.SecretStore, secretID string) (rotationCredentialRecord, error) {
	value, err := store.GetSecretByID(secretID)
	if err != nil && secretID != secrets.DEFAULT_KEY {
		value, err = store.GetSecretByID(secrets.DEFAULT_KEY)
	}
	if err != nil {
		return rotationCredentialRecord{}, fmt.Errorf("load BMC credentials for %q: %w", secretID, err)
	}
	var record rotationCredentialRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return rotationCredentialRecord{}, fmt.Errorf("decode BMC credentials for %q: %w", secretID, err)
	}
	if record.Username == "" || record.Password == "" {
		return rotationCredentialRecord{}, fmt.Errorf("BMC credentials for %q are blank", secretID)
	}
	return record, nil
}

func saveRotationRecord(store secrets.SecretStore, secretID string, record rotationCredentialRecord) error {
	value, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return store.StoreSecretByID(secretID, string(value))
}

func commitRotation(store secrets.SecretStore, secretID string, record rotationCredentialRecord) error {
	record.Password = record.Pending.NewPassword
	record.Pending = nil
	if err := saveRotationRecord(store, secretID, record); err != nil {
		return fmt.Errorf("BMC password is active but credential-store commit failed; rotation intent was retained for recovery: %w", err)
	}
	return nil
}

// RedfishRotationTarget implements RotationTarget using Redfish account
// resources and the configured BMC TLS settings.
type RedfishRotationTarget struct {
	URI        string
	Insecure   bool
	CACertPath string
}

func (r RedfishRotationTarget) AccountUsername(accountID, username, password string) (string, error) {
	client, err := r.connect(username, password)
	if err != nil {
		return "", err
	}
	defer client.Logout()
	accounts, err := ListAccounts(client)
	if err != nil {
		return "", err
	}
	for _, account := range accounts {
		if fmt.Sprint(account["Id"]) == accountID {
			value, _ := account["UserName"].(string)
			if value == "" {
				return "", fmt.Errorf("account %q has no UserName", accountID)
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("account %q not found", accountID)
}

func (r RedfishRotationTarget) SetAccountPassword(accountID, username, password, newPassword string) error {
	client, err := r.connect(username, password)
	if err != nil {
		return err
	}
	defer client.Logout()
	payload, err := json.Marshal(map[string]string{"Password": newPassword})
	if err != nil {
		return err
	}
	return UpdateAccount(client, accountID, string(payload))
}

func (r RedfishRotationTarget) Authenticate(username, password string) error {
	client, err := r.connect(username, password)
	if err != nil {
		return err
	}
	client.Logout()
	return nil
}

func (r RedfishRotationTarget) connect(username, password string) (*gofish.APIClient, error) {
	store := secrets.NewStaticStore(username, password)
	return ConnectWithCredentials(r.URI, store, r.Insecure, r.CACertPath)
}

var _ RotationTarget = RedfishRotationTarget{}
