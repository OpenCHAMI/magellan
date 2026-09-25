package magellan

import (
	"testing"

	"github.com/OpenCHAMI/magellan/internal/format"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/stretchr/testify/require"
)

func TestCollectInventoryValidation(t *testing.T) {
	params := &CollectParams{Concurrency: 1, OutputFormat: format.FORMAT_JSON}
	_, err := CollectInventory(nil, params)
	require.Error(t, err)

	empty := []RemoteAsset{}
	_, err = CollectInventory(&empty, params)
	require.Error(t, err)

	assets := []RemoteAsset{{Host: "https://127.0.0.1", Port: 443, State: true}}
	_, err = CollectInventory(&assets, nil)
	require.Error(t, err)
	_, err = CollectInventory(&assets, &CollectParams{Concurrency: 0})
	require.Error(t, err)
}

func TestCollectInventorySkipsInactiveAssets(t *testing.T) {
	assets := []RemoteAsset{{Host: "https://127.0.0.1", Port: 443, State: false}}
	got, err := CollectInventory(&assets, &CollectParams{
		Concurrency:  1,
		OutputFormat: format.FORMAT_JSON,
	})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestFindMACAddressWithIPRejectsNilTarget(t *testing.T) {
	// a nil target can never equal an interface address; the lookup must
	// fail before connecting to the BMC (hostname scan targets only reach
	// this point unresolved when DNS resolution failed — see
	// CollectInventory, which resolves the scan target first).
	_, err := FindMACAddressWithIP(crawler.CrawlerConfig{}, nil)
	require.ErrorContains(t, err, "target IP is nil")
}
