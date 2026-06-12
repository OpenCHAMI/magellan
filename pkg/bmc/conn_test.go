package bmc

import (
	"strings"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
)

func TestGetUserPass(t *testing.T) {
	t.Run("nil store errors", func(t *testing.T) {
		cfg := ConnConfig{URI: "https://bmc.example", CredentialStore: nil}
		_, err := cfg.GetUserPass()
		if err == nil || !strings.Contains(err.Error(), "credential store is invalid") {
			t.Fatalf("GetUserPass with nil store err = %v, want 'credential store is invalid'", err)
		}
	})

	t.Run("blank credentials error", func(t *testing.T) {
		// A store that resolves to empty username/password must be reported as an
		// error rather than silently producing an unauthenticated connection.
		cfg := ConnConfig{URI: "https://bmc.example", CredentialStore: secrets.NewStaticStore("", "")}
		_, err := cfg.GetUserPass()
		if err == nil || !strings.Contains(err.Error(), "credentials blank") {
			t.Fatalf("GetUserPass with blank creds err = %v, want 'credentials blank'", err)
		}
	})

	t.Run("valid credentials returned", func(t *testing.T) {
		cfg := ConnConfig{URI: "https://bmc.example", CredentialStore: secrets.NewStaticStore("alice", "s3cret")}
		creds, err := cfg.GetUserPass()
		if err != nil {
			t.Fatalf("GetUserPass unexpected error: %v", err)
		}
		if creds.Username != "alice" || creds.Password != "s3cret" {
			t.Fatalf("GetUserPass creds = %+v, want {alice s3cret}", creds)
		}
	})
}
