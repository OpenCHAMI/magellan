package idmap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/magellan/internal/format"
	"github.com/stretchr/testify/require"
)

func TestGetBMCIDMap(t *testing.T) {
	got, err := getBMCIDMap("", format.FORMAT_JSON)
	require.NoError(t, err)
	require.Nil(t, got)

	jsonMap := `{"map_key":"bmc-ip-addr","id_map":{"192.0.2.1":"x0c0s0b0"}}`
	got, err = getBMCIDMap(jsonMap, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Equal(t, "x0c0s0b0", got.IDMap["192.0.2.1"])
	_, err = getBMCIDMap("{", format.FORMAT_JSON)
	require.Error(t, err)

	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "map.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("map_key: bmc-ip-addr\nid_map:\n  192.0.2.2: x1c0s0b0\n"), 0o600))
	got, err = getBMCIDMap("@"+yamlPath, format.FORMAT_JSON)
	require.NoError(t, err)
	require.Equal(t, "x1c0s0b0", got.IDMap["192.0.2.2"])
	_, err = getBMCIDMap("@"+filepath.Join(dir, "missing.json"), format.FORMAT_JSON)
	require.Error(t, err)
}

func TestUserProvidedMapper(t *testing.T) {
	mapper := userProvidedMapper{IDMapStr: `{"map_key":"bmc-ip-addr","id_map":{"192.0.2.1":"node-a"}}`, IDMapFormat: format.FORMAT_JSON}
	initialized, err := mapper.Initialize()
	require.NoError(t, err)
	require.Equal(t, "node-a", initialized.GetMappedID(&MapperKeys{IPv4Addr: "192.0.2.1"}))
	require.Empty(t, initialized.GetMappedID(&MapperKeys{IPv4Addr: "192.0.2.2"}))

	bad := userProvidedMapper{IDMapStr: `{"map_key":"hostname","id_map":{}}`, IDMapFormat: format.FORMAT_JSON}
	_, err = bad.Initialize()
	require.Error(t, err)
	require.Equal(t, "selector", getBMCID(nil, "selector"))
	require.Empty(t, (userProvidedMapper{}).GetMappedID(&MapperKeys{}))
}

func TestGeneratedMapperAndSelection(t *testing.T) {
	mapper := PickIDMapper("", format.FORMAT_JSON)
	require.IsType(t, generatedXNAMEMapper{}, mapper)
	require.NotEmpty(t, mapper.GetMappedID(&MapperKeys{IPv4Addr: "192.0.2.1"}))
	require.Empty(t, mapper.GetMappedID(&MapperKeys{IPv4Addr: "invalid"}))
	require.Equal(t, "x0c0s0b0", ipAddrIntToXname(0))

	mapper = PickIDMapper(`{"map_key":"bmc-ip-addr","id_map":{"192.0.2.1":"provided"}}`, format.FORMAT_JSON)
	require.Equal(t, "provided", mapper.GetMappedID(&MapperKeys{IPv4Addr: "192.0.2.1"}))
}
