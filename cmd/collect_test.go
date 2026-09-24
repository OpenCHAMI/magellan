package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/internal/format"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// setStdin points the os.Stdin global at f for the duration of the test.
// Everything under test reads os.Stdin directly (Stat, bufio scanners), so
// swapping it here simulates terminals, pipes and redirected files.
func setStdin(t *testing.T, f *os.File) {
	t.Helper()
	orig := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = orig
		_ = f.Close()
	})
}

// pipeStdin simulates piped stdin: it returns the read end to hand to
// setStdin and the write end for the test to fill.
func pipeStdin(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	return r, w
}

// writeCache creates a scan cache containing assets and returns its path.
func writeCache(t *testing.T, assets ...magellan.RemoteAsset) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "assets.db")
	require.NoError(t, sqlite.InsertScannedAssets(path, assets...))
	return path
}

// stdinAssetJSON is what `magellan scan -F json | magellan collect` pipes.
const stdinAssetJSON = `[{"host":"https://from-stdin","port":5000,"protocol":"tcp","state":true,"timestamp":"2026-09-23T00:00:00Z"}]`

func TestIsStdinEmpty(t *testing.T) {
	t.Run("character device (terminal or /dev/null)", func(t *testing.T) {
		f, err := os.Open(os.DevNull)
		require.NoError(t, err)
		setStdin(t, f)

		empty, err := IsStdinEmpty()
		require.NoError(t, err)
		require.True(t, empty)
	})

	t.Run("empty regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.json")
		require.NoError(t, os.WriteFile(path, nil, 0o600))
		f, err := os.Open(path)
		require.NoError(t, err)
		setStdin(t, f)

		empty, err := IsStdinEmpty()
		require.NoError(t, err)
		require.True(t, empty)
	})

	t.Run("non-empty regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "data.json")
		require.NoError(t, os.WriteFile(path, []byte(stdinAssetJSON), 0o600))
		f, err := os.Open(path)
		require.NoError(t, err)
		setStdin(t, f)

		empty, err := IsStdinEmpty()
		require.NoError(t, err)
		require.False(t, empty)
	})

	t.Run("pipe", func(t *testing.T) {
		r, w := pipeStdin(t)
		_, err := w.WriteString("data")
		require.NoError(t, err)
		setStdin(t, r)

		empty, err := IsStdinEmpty()
		require.NoError(t, err)
		require.False(t, empty)
	})

	t.Run("stat failure", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "closed")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		setStdin(t, f)

		empty, err := IsStdinEmpty()
		require.Error(t, err)
		require.True(t, empty)
	})
}

// Regression test for OpenCHAMI/magellan#189: an explicit --cache must be
// read even when stdin is a pipe. Non-TTY invocations (CI, cmd/exec,
// `docker compose run`) hand the process a fifo, which the old
// IsStdinEmpty-based branch treated as piped data, so collect found nothing
// and exited 1 despite --cache being set.
func TestLoadCollectInputExplicitCacheBeatsPipedStdin(t *testing.T) {
	want := magellan.RemoteAsset{Host: "https://from-cache", Port: 443, Protocol: "tcp", State: true, Timestamp: time.Unix(1, 0)}
	cache := writeCache(t, want)

	r, w := pipeStdin(t)
	_, err := w.WriteString(stdinAssetJSON) // the pipe even carries data...
	require.NoError(t, err)
	require.NoError(t, w.Close()) // ...then hits EOF, like `docker compose run`
	setStdin(t, r)

	got, stdinInput, err := loadCollectInput(cache, true, nil, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want.Host, got[0].Host)
	require.Empty(t, stdinInput, "stdin must be ignored when --cache is set explicitly")
}

// A cache the user explicitly asked for must fail loudly instead of being
// skipped with a warning followed by the generic "no input" error (#189).
func TestLoadCollectInputExplicitCacheReadFailureIsError(t *testing.T) {
	r, _ := pipeStdin(t)
	setStdin(t, r)

	got, stdinInput, err := loadCollectInput(filepath.Join(t.TempDir(), "missing.db"), true, nil, format.FORMAT_JSON)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--cache")
	require.Nil(t, got)
	require.Nil(t, stdinInput)
}

// The documented `scan | collect` flow must keep working: with no cache
// requested, piped stdin is read as before.
func TestLoadCollectInputPipedStdinWithoutCache(t *testing.T) {
	r, w := pipeStdin(t)
	_, err := w.WriteString(stdinAssetJSON)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	setStdin(t, r)

	got, stdinInput, err := loadCollectInput(filepath.Join(t.TempDir(), "unused.db"), false, nil, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Len(t, stdinInput, 1)
	require.Equal(t, "https://from-stdin", stdinInput[0]["host"])
}

// With no explicit cache and nothing piped (terminal, /dev/null), the cache
// path is consulted -- the historical TTY behavior, now applied whether or
// not a TTY is present.
func TestLoadCollectInputFallsBackToCacheWithoutInput(t *testing.T) {
	want := magellan.RemoteAsset{Host: "https://from-cache", Port: 443, Protocol: "tcp", State: true, Timestamp: time.Unix(2, 0)}
	cache := writeCache(t, want)

	f, err := os.Open(os.DevNull)
	require.NoError(t, err)
	setStdin(t, f)

	got, stdinInput, err := loadCollectInput(cache, false, nil, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want.Host, got[0].Host)
	require.Empty(t, stdinInput)
}

// An empty pipe with an implicit cache path behaves like a terminal: after
// reading nothing from stdin, the cache is used (the #189 scenario without
// an explicit --cache).
func TestLoadCollectInputEmptyPipeFallsBackToCache(t *testing.T) {
	want := magellan.RemoteAsset{Host: "https://from-cache", Port: 443, Protocol: "tcp", State: true, Timestamp: time.Unix(3, 0)}
	cache := writeCache(t, want)

	r, w := pipeStdin(t)
	require.NoError(t, w.Close())
	setStdin(t, r)

	got, stdinInput, err := loadCollectInput(cache, false, nil, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want.Host, got[0].Host)
	require.Empty(t, stdinInput)
}

// Positional arguments still feed collect even though stdin contributes
// nothing (empty pipe), and no cache is consulted because input was found.
func TestLoadCollectInputPositionalArgs(t *testing.T) {
	r, w := pipeStdin(t)
	require.NoError(t, w.Close())
	setStdin(t, r)

	const arg = `{"host":"https://from-arg","port":443,"protocol":"tcp","state":true,"timestamp":"2026-09-23T00:00:00Z"}`
	got, stdinInput, err := loadCollectInput(filepath.Join(t.TempDir(), "unused.db"), false, []string{arg}, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "https://from-arg", got[0].Host)
	require.Empty(t, stdinInput)
}

// `--cache ""` explicitly disables the cache: only stdin and args count.
func TestLoadCollectInputExplicitlyDisabledCache(t *testing.T) {
	r, w := pipeStdin(t)
	_, err := w.WriteString(stdinAssetJSON)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	setStdin(t, r)

	got, stdinInput, err := loadCollectInput("", true, nil, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Len(t, stdinInput, 1)
}

func TestIsCacheExplicit(t *testing.T) {
	const defaultPath = "/tmp/test-default/assets.db"

	origCachePath := cachePath
	t.Cleanup(func() { cachePath = origCachePath })

	check := func(t *testing.T, args []string, resolved string, want bool) {
		t.Helper()
		cachePath = resolved // simulate what InitializeConfig left behind

		var seen bool
		root := &cobra.Command{Use: "magellan", SilenceUsage: true, SilenceErrors: true}
		root.PersistentFlags().String("cache", defaultPath, "")
		root.AddCommand(&cobra.Command{Use: "collect", Run: func(c *cobra.Command, _ []string) {
			seen = isCacheExplicit(c)
		}})
		root.SetArgs(append([]string{"collect"}, args...))
		require.NoError(t, root.Execute())
		require.Equal(t, want, seen)
	}

	t.Run("built-in default is implicit", func(t *testing.T) {
		check(t, nil, defaultPath, false)
	})
	t.Run("flag on the command line is explicit", func(t *testing.T) {
		check(t, []string{"--cache", "/custom.db"}, "/custom.db", true)
	})
	t.Run("environment or config override is explicit", func(t *testing.T) {
		check(t, nil, "/custom.db", true)
	})
}
