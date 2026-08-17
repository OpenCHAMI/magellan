package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/stretchr/testify/require"
)

func TestScannedAssetLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "assets.db")
	_, err := GetScannedAssets(path)
	require.Error(t, err)
	require.Error(t, InsertScannedAssets(path, nil...))

	a := magellan.RemoteAsset{Host: "https://b", Port: 443, Protocol: "tcp", State: true, Timestamp: time.Unix(2, 0)}
	b := magellan.RemoteAsset{Host: "https://a", Port: 5000, Protocol: "tcp", State: true, Timestamp: time.Unix(1, 0)}
	require.NoError(t, InsertScannedAssets(path, a, b))
	got, err := GetScannedAssets(path)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, b.Host, got[0].Host)

	a.State = false
	require.NoError(t, InsertScannedAssets(path, a))
	got, err = GetScannedAssets(path)
	require.NoError(t, err)
	require.False(t, got[1].State)

	require.NoError(t, DeleteScannedAssets(path, a))
	got, err = GetScannedAssets(path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Error(t, DeleteScannedAssets(path, nil...))
}
