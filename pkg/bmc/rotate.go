package bmc

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/openchami/magellan/pkg/secrets"
)

var accountRotationMu sync.Mutex

// RotateStoredAccount changes the stored login account's password and saves it
// only after a fresh authenticated account read succeeds. It has no recovery
// protocol: callers must retain the replacement password independently.
func RotateStoredAccount(endpoint, accountID, newPassword string, store secrets.SecretStore, insecure bool, caCert string) error {
	accountRotationMu.Lock()
	defer accountRotationMu.Unlock()

	if store == nil || endpoint == "" || accountID == "" || newPassword == "" {
		return fmt.Errorf("endpoint, account ID, password, and writable store are required")
	}
	if _, static := store.(*secrets.StaticStore); static {
		return fmt.Errorf("rotation requires a persistent credential store")
	}
	record, err := loadRotationCredential(store, endpoint)
	if err != nil {
		return fmt.Errorf("read exact BMC credential entry (no default fallback): %w", err)
	}
	if record.Pending != nil {
		return fmt.Errorf("a rotation is already pending for BMC %s; run 'magellan settings recover-rotation %s' before starting another", endpoint, endpoint)
	}
	creds := record.BMCCredentials
	if creds.Username == "" || creds.Password == "" || creds.Password == newPassword {
		return fmt.Errorf("stored credentials must be nonempty and replacement password must differ")
	}
	if err := verifyAccountCredential(endpoint, accountID, creds.Username, creds.Password, insecure, caCert); err != nil {
		return fmt.Errorf("verify target account before rotation: %w", err)
	}
	record.Pending = &pendingAccountRotation{AccountID: accountID, NewPassword: newPassword}
	if err := saveRotationCredential(store, endpoint, record); err != nil {
		return fmt.Errorf("could not persist rotation intent; BMC was not changed: %w", err)
	}
	if err := patchAccountPassword(endpoint, accountID, creds, newPassword, insecure, caCert); err != nil {
		return fmt.Errorf("Redfish password PATCH returned an error; the BMC may still have applied the change. A pending rotation was saved at %s. Do not retry blindly; run 'magellan settings recover-rotation %s' to check which password works: %w", endpoint, endpoint, err)
	}
	if err := verifyAccountCredential(endpoint, accountID, creds.Username, newPassword, insecure, caCert); err != nil {
		return fmt.Errorf("BMC may have changed; new credential verification failed. Pending rotation remains at %s; run 'magellan settings recover-rotation %s': %w", endpoint, endpoint, err)
	}
	if err := finalizeAccountRotation(store, endpoint, record); err != nil {
		return fmt.Errorf("BMC password is verified, but credential-store finalization failed. Pending rotation remains at %s; run 'magellan settings recover-rotation %s': %w", endpoint, endpoint, err)
	}
	return nil
}

type pendingAccountRotation struct {
	AccountID   string `json:"account_id"`
	NewPassword string `json:"new_password"`
}

type rotationCredential struct {
	BMCCredentials
	Pending *pendingAccountRotation `json:"pending_rotation,omitempty"`
}

func loadRotationCredential(store secrets.SecretStore, endpoint string) (rotationCredential, error) {
	value, err := store.GetSecretByID(endpoint)
	if err != nil {
		return rotationCredential{}, err
	}
	var record rotationCredential
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return rotationCredential{}, err
	}
	return record, nil
}

func saveRotationCredential(store secrets.SecretStore, endpoint string, record rotationCredential) error {
	value, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return store.StoreSecretByID(endpoint, string(value))
}

func patchAccountPassword(endpoint, accountID string, current BMCCredentials, newPassword string, insecure bool, caCert string) error {
	client, err := ConnectWithCredentials(endpoint, secrets.NewStaticStore(current.Username, current.Password), insecure, caCert)
	if err != nil {
		return fmt.Errorf("connect with previous credential: %w", err)
	}
	defer client.Logout()
	account, err := ResolveCategoryItem(client, "Accounts", accountID)
	if err != nil {
		return fmt.Errorf("read target account: %w", err)
	}
	if asMap(account)["UserName"] != current.Username {
		return fmt.Errorf("target account must match the stored login username")
	}
	payload, err := json.Marshal(map[string]string{"Password": newPassword})
	if err != nil {
		return err
	}
	return UpdateAccount(client, accountID, string(payload))
}

func verifyAccountCredential(endpoint, accountID, username, password string, insecure bool, caCert string) error {
	client, err := ConnectWithCredentials(endpoint, secrets.NewStaticStore(username, password), insecure, caCert)
	if err != nil {
		return err
	}
	defer client.Logout()
	account, err := ResolveCategoryItem(client, "Accounts", accountID)
	if err != nil {
		return err
	}
	if asMap(account)["UserName"] != username {
		return fmt.Errorf("account username does not match stored credential")
	}
	return nil
}

func finalizeAccountRotation(store secrets.SecretStore, endpoint string, record rotationCredential) error {
	if record.Pending == nil {
		return fmt.Errorf("no pending rotation")
	}
	record.Password = record.Pending.NewPassword
	record.Pending = nil
	return saveRotationCredential(store, endpoint, record)
}

// RecoverStoredAccountRotation inspects a durable pending rotation. If the new
// password works, it commits it to the store. If only the old password works,
// it retries the PATCH, verifies the new credential, and commits it. If neither
// credential authenticates, it leaves the pending state untouched for manual
// recovery with an independent BMC administrator account.
func RecoverStoredAccountRotation(endpoint string, store secrets.SecretStore, insecure bool, caCert string) error {
	accountRotationMu.Lock()
	defer accountRotationMu.Unlock()

	if store == nil || endpoint == "" {
		return fmt.Errorf("BMC endpoint and credential store are required")
	}
	record, err := loadRotationCredential(store, endpoint)
	if err != nil {
		return fmt.Errorf("read BMC credential and pending rotation: %w", err)
	}
	if record.Pending == nil || record.Pending.AccountID == "" || record.Pending.NewPassword == "" {
		return fmt.Errorf("no valid pending rotation found for BMC %s", endpoint)
	}
	if err := verifyAccountCredential(endpoint, record.Pending.AccountID, record.Username, record.Pending.NewPassword, insecure, caCert); err == nil {
		if err := finalizeAccountRotation(store, endpoint, record); err != nil {
			return fmt.Errorf("new BMC password works but storing it failed; pending rotation remains: %w", err)
		}
		return nil
	} else if !isCredentialRejected(err) {
		return fmt.Errorf("could not determine whether pending BMC password works; pending rotation remains: %w", err)
	}

	if err := verifyAccountCredential(endpoint, record.Pending.AccountID, record.Username, record.Password, insecure, caCert); err != nil {
		if isCredentialRejected(err) {
			return fmt.Errorf("neither previous nor pending password authenticates; pending rotation remains. Use independent administrator access to inspect or reset the account: %w", err)
		}
		return fmt.Errorf("could not determine whether previous BMC password works; pending rotation remains: %w", err)
	}

	current := record.BMCCredentials
	if err := patchAccountPassword(endpoint, record.Pending.AccountID, current, record.Pending.NewPassword, insecure, caCert); err != nil {
		if verifyErr := verifyAccountCredential(endpoint, record.Pending.AccountID, record.Username, record.Pending.NewPassword, insecure, caCert); verifyErr == nil {
			if commitErr := finalizeAccountRotation(store, endpoint, record); commitErr != nil {
				return fmt.Errorf("new BMC password works after ambiguous PATCH but storing it failed; pending rotation remains: %w", commitErr)
			}
			return nil
		}
		return fmt.Errorf("retry PATCH failed and outcome remains ambiguous; pending rotation remains. Do not retry blindly; use independent administrator access if needed: %w", err)
	}
	if err := verifyAccountCredential(endpoint, record.Pending.AccountID, record.Username, record.Pending.NewPassword, insecure, caCert); err != nil {
		return fmt.Errorf("retry PATCH returned but new credential verification failed; pending rotation remains: %w", err)
	}
	if err := finalizeAccountRotation(store, endpoint, record); err != nil {
		return fmt.Errorf("new BMC password works but storing it failed; pending rotation remains: %w", err)
	}
	return nil
}

func isCredentialRejected(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "authentication failed") || strings.Contains(message, "401:") || strings.Contains(strings.ToLower(message), "unauthorized")
}
