package bmc

import (
	"encoding/json"
	"fmt"

	"github.com/openchami/magellan/pkg/secrets"
)

// RotateStoredAccount changes the stored login account's password and saves it
// only after a fresh authenticated account read succeeds. It has no recovery
// protocol: callers must retain the replacement password independently.
func RotateStoredAccount(endpoint, accountID, newPassword string, store secrets.SecretStore, insecure bool, caCert string) error {
	if store == nil || endpoint == "" || accountID == "" || newPassword == "" {
		return fmt.Errorf("endpoint, account ID, password, and writable store are required")
	}
	if _, static := store.(*secrets.StaticStore); static {
		return fmt.Errorf("rotation requires a persistent credential store")
	}
	creds, err := GetBMCCredentials(store, endpoint)
	if err != nil {
		return fmt.Errorf("read exact BMC credential entry (no default fallback): %w", err)
	}
	if creds.Username == "" || creds.Password == "" || creds.Password == newPassword {
		return fmt.Errorf("stored credentials must be nonempty and replacement password must differ")
	}
	client, err := ConnectWithCredentials(endpoint, secrets.NewStaticStore(creds.Username, creds.Password), insecure, caCert)
	if err != nil {
		return fmt.Errorf("connect with current credential: %w", err)
	}
	defer client.Logout()
	account, err := ResolveCategoryItem(client, "Accounts", accountID)
	if err != nil {
		return fmt.Errorf("read target account: %w", err)
	}
	if asMap(account)["UserName"] != creds.Username {
		return fmt.Errorf("target account must match the stored login username")
	}
	payload, err := json.Marshal(map[string]string{"Password": newPassword})
	if err != nil {
		return err
	}
	if err := UpdateAccount(client, accountID, string(payload)); err != nil {
		return fmt.Errorf("password PATCH failed; outcome may be uncertain, retain both passwords and check manually: %w", err)
	}
	updated := BMCCredentials{Username: creds.Username, Password: newPassword}
	verification, err := ConnectWithCredentials(endpoint, secrets.NewStaticStore(updated.Username, updated.Password), insecure, caCert)
	if err != nil {
		return fmt.Errorf("BMC may have changed; new credential connection failed, store unchanged: %w", err)
	}
	defer verification.Logout()
	verified, err := ResolveCategoryItem(verification, "Accounts", accountID)
	if err != nil || asMap(verified)["UserName"] != updated.Username {
		return fmt.Errorf("BMC may have changed; authenticated account verification failed, store unchanged")
	}
	value, err := json.Marshal(updated)
	if err != nil {
		return err
	}
	if err := store.StoreSecretByID(endpoint, string(value)); err != nil {
		return fmt.Errorf("BMC password changed but OpenBao write failed; use the retained password file to repair the credential entry: %w", err)
	}
	return nil
}
