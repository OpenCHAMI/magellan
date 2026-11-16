package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/magellan/internal/format"

	"github.com/openchami/magellan/pkg/secrets"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestSecretsListIDsOnly(t *testing.T) {
	oldFile, oldBackend, oldFormat := secretsCommandFile, secretsBackend, secretsListFormat
	oldOut := secretsListCmd.OutOrStdout()
	t.Cleanup(func() {
		secretsCommandFile, secretsBackend, secretsListFormat = oldFile, oldBackend, oldFormat
		secretsListCmd.SetOut(oldOut)
	})
	key, err := secrets.GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", key)
	secretsCommandFile = filepath.Join(t.TempDir(), "secrets.json")
	secretsBackend = secrets.BackendLocal
	store, err := secrets.OpenStore(secretsCommandFile)
	require.NoError(t, err)
	for _, id := range []string{"z", "a"} {
		require.NoError(t, store.StoreSecretByID(id, `{"username":"private-user","password":"private-password"}`))
	}
	for _, tc := range []struct {
		format format.DataFormat
		want   string
	}{
		{format.FORMAT_LIST, "a\nz\n"},
		{format.FORMAT_JSON, ""},
		{format.FORMAT_YAML, ""},
	} {
		var buf bytes.Buffer
		secretsListCmd.SetOut(&buf)
		secretsListFormat = tc.format
		secretsListCmd.Run(secretsListCmd, nil)
		out := buf.String()
		require.NotContains(t, out, "private-")
		switch tc.format {
		case format.FORMAT_JSON:
			require.JSONEq(t, `["a","z"]`, out)
		case format.FORMAT_YAML:
			require.YAMLEq(t, "- a\n- z\n", out)
		default:
			require.Equal(t, tc.want, out)
		}
	}
}

func TestSecretsStoreFileIsolation(t *testing.T) {
	// Verify the actual initialized value, not only the flag's advertised
	// default: another command's flag registration used to clear this value.
	require.Equal(t, "secrets.json", secretsCommandFile)
	oldFile, oldInput, oldFormat, oldShared := secretsCommandFile, secretsStoreInputFile, secretsStoreFormat, secretsFile
	oldBackend := secretsBackend
	t.Cleanup(func() {
		secretsCommandFile, secretsStoreInputFile, secretsStoreFormat, secretsFile = oldFile, oldInput, oldFormat, oldShared
		secretsBackend = oldBackend
	})
	secretsBackend = secrets.BackendLocal
	// Other commands may change their destination without affecting secrets.
	secretsFile = "unrelated-store.json"
	key, err := secrets.GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", key)
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "credentials.json")
	const credentials = `{"username":"review-user","password":"review-password"}`
	require.NoError(t, os.WriteFile(input, []byte(credentials), 0600))
	for _, tc := range []struct{ name, file string }{
		{"default", "secrets.json"},
		{"explicit", "custom-secrets.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := pflag.NewFlagSet("store", pflag.ContinueOnError)
			for _, f := range []*pflag.Flag{
				secretsCmd.PersistentFlags().Lookup("file"),
				secretsStoreCmd.Flags().Lookup("input-format"),
				secretsStoreCmd.Flags().Lookup("input-file"),
			} {
				changed := f.Changed
				t.Cleanup(func() { f.Changed = changed })
				flags.AddFlag(f)
			}
			args := []string{"review-check", "--input-format=json", "-i", input}
			if tc.name == "explicit" {
				args = append(args, "--file", tc.file)
			}
			require.NoError(t, flags.Parse(args))
			secretsStoreCmd.Run(secretsStoreCmd, flags.Args())
			store, err := secrets.OpenStore(tc.file)
			require.NoError(t, err)
			value, err := store.GetSecretByID("review-check")
			require.NoError(t, err)
			require.JSONEq(t, credentials, value)
		})
	}
	data, err := os.ReadFile(input)
	require.NoError(t, err)
	require.Equal(t, credentials, string(data), "input must not be used as the store")
	_, err = os.Stat(secretsFile)
	require.True(t, os.IsNotExist(err), "other commands' store must remain untouched")
}
