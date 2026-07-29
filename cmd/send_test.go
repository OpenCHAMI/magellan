package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenCHAMI/magellan/internal/format"
	"github.com/stretchr/testify/require"
)

func TestProcessDataArgsUsesInputFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		inputFormat format.DataFormat
		wantIDs     []string
	}{
		{
			name:        "inline json",
			args:        []string{`[{"ID":"x0","host":"https://bmc0.example.com"}]`},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"x0"},
		},
		{
			name:        "inline yaml",
			args:        []string{"- ID: x0\n  host: https://bmc0.example.com\n"},
			inputFormat: format.FORMAT_YAML,
			wantIDs:     []string{"x0"},
		},
		{
			name:        "empty args skipped",
			args:        []string{"", `[{"ID":"x0"}]`, ""},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"x0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processDataArgs(tt.args, tt.inputFormat)

			require.Len(t, got, len(tt.wantIDs))
			for i, wantID := range tt.wantIDs {
				require.Equal(t, wantID, got[i]["ID"])
			}
		})
	}
}

func TestProcessDataArgsReadsFilesAndFallbackFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonPath := writeTestFile(t, dir, "assets.json", `[{"ID":"json-file"}]`)
	yamlPath := writeTestFile(t, dir, "assets.yaml", "- ID: yaml-file\n")
	fallbackPath := writeTestFile(t, dir, "assets.txt", "- ID: fallback-yaml\n")
	emptyPath := writeTestFile(t, dir, "empty.json", "")

	tests := []struct {
		name        string
		args        []string
		inputFormat format.DataFormat
		wantIDs     []string
	}{
		{
			name:        "json file extension",
			args:        []string{"@" + jsonPath},
			inputFormat: format.FORMAT_YAML,
			wantIDs:     []string{"json-file"},
		},
		{
			name:        "yaml file extension",
			args:        []string{"@" + yamlPath},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"yaml-file"},
		},
		{
			name:        "extensionless fallback format",
			args:        []string{"@" + fallbackPath},
			inputFormat: format.FORMAT_YAML,
			wantIDs:     []string{"fallback-yaml"},
		},
		{
			name:        "multiple files",
			args:        []string{"@" + jsonPath, "@" + yamlPath},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"json-file", "yaml-file"},
		},
		{
			name:        "empty file skipped",
			args:        []string{"@" + emptyPath, "@" + jsonPath},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"json-file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processDataArgs(tt.args, tt.inputFormat)

			require.Len(t, got, len(tt.wantIDs))
			for i, wantID := range tt.wantIDs {
				require.Equal(t, wantID, got[i]["ID"])
			}
		})
	}
}

func TestProcessDataArgsKeepsMixedInlineAndFileInputs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonPath := writeTestFile(t, dir, "assets.json", `[{"ID":"file-json"}]`)
	yamlPath := writeTestFile(t, dir, "assets.yaml", "- ID: file-yaml\n")

	tests := []struct {
		name        string
		args        []string
		inputFormat format.DataFormat
		wantIDs     []string
	}{
		{
			name:        "inline before file",
			args:        []string{`[{"ID":"inline-json"}]`, "@" + jsonPath},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"inline-json", "file-json"},
		},
		{
			name:        "file before inline",
			args:        []string{"@" + jsonPath, `[{"ID":"inline-json"}]`},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"file-json", "inline-json"},
		},
		{
			name:        "yaml inline before yaml file",
			args:        []string{"- ID: inline-yaml\n", "@" + yamlPath},
			inputFormat: format.FORMAT_YAML,
			wantIDs:     []string{"inline-yaml", "file-yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processDataArgs(tt.args, tt.inputFormat)

			require.Len(t, got, len(tt.wantIDs))
			for i, wantID := range tt.wantIDs {
				require.Equal(t, wantID, got[i]["ID"])
			}
		})
	}
}

func TestProcessDataArgsSkipsInvalidInputs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalidPath := writeTestFile(t, dir, "invalid.json", `{`)
	validPath := writeTestFile(t, dir, "valid.json", `[{"ID":"valid-file"}]`)
	missingPath := filepath.Join(dir, "missing.json")

	tests := []struct {
		name        string
		args        []string
		inputFormat format.DataFormat
		wantIDs     []string
	}{
		{
			name:        "invalid inline skipped",
			args:        []string{`{`, `[{"ID":"valid-inline"}]`},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"valid-inline"},
		},
		{
			name:        "invalid file skipped",
			args:        []string{"@" + invalidPath, "@" + validPath},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"valid-file"},
		},
		{
			name:        "missing file skipped",
			args:        []string{"@" + missingPath, `[{"ID":"valid-inline"}]`},
			inputFormat: format.FORMAT_JSON,
			wantIDs:     []string{"valid-inline"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processDataArgs(tt.args, tt.inputFormat)

			require.Len(t, got, len(tt.wantIDs))
			for i, wantID := range tt.wantIDs {
				require.Equal(t, wantID, got[i]["ID"])
			}
		})
	}
}

func TestHandleArgsParsesStdinWithInputFormat(t *testing.T) {
	tests := []struct {
		name        string
		stdin       string
		inputFormat format.DataFormat
		wantID      string
	}{
		{
			name:        "json stdin",
			stdin:       `[{"ID":"stdin-json"}]`,
			inputFormat: format.FORMAT_JSON,
			wantID:      "stdin-json",
		},
		{
			name:        "yaml stdin",
			stdin:       "- ID: stdin-yaml\n",
			inputFormat: format.FORMAT_YAML,
			wantID:      "stdin-yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withStdin(t, tt.stdin)
			stdout := captureStdout(t)
			sendDataArgs = nil
			t.Cleanup(func() { sendDataArgs = nil })

			got := handleArgs(nil, tt.inputFormat)

			require.Empty(t, stdout())
			require.Len(t, got, 1)
			require.Equal(t, tt.wantID, got[0]["ID"])
		})
	}
}

func TestHandleArgsSkipsStdinWhenSendDataArgsAreSet(t *testing.T) {
	withStdin(t, `[{"ID":"stdin-json"}]`)
	sendDataArgs = []string{`[{"ID":"flag-json"}]`}
	t.Cleanup(func() { sendDataArgs = nil })

	got := handleArgs(nil, format.FORMAT_JSON)

	require.Nil(t, got)
}

func writeTestFile(t *testing.T, dir, name, contents string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0644))
	return path
}

func withStdin(t *testing.T, contents string) {
	t.Helper()

	oldStdin := os.Stdin
	readEnd, writeEnd, err := os.Pipe()
	require.NoError(t, err)
	_, err = writeEnd.WriteString(contents)
	require.NoError(t, err)
	require.NoError(t, writeEnd.Close())
	os.Stdin = readEnd
	t.Cleanup(func() {
		os.Stdin = oldStdin
		require.NoError(t, readEnd.Close())
	})
}

func captureStdout(t *testing.T) func() string {
	t.Helper()

	oldStdout := os.Stdout
	readEnd, writeEnd, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writeEnd

	return func() string {
		require.NoError(t, writeEnd.Close())
		os.Stdout = oldStdout
		out, err := io.ReadAll(readEnd)
		require.NoError(t, err)
		require.NoError(t, readEnd.Close())
		return string(out)
	}
}
