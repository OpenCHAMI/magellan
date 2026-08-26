package bmc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/stmcginnis/gofish"
	"github.com/stretchr/testify/require"
)

type capturedRequest struct {
	method  string
	path    string
	payload map[string]any
}

type redfishSettingsFixture struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	writes []capturedRequest
}

func newRedfishSettingsFixture(t *testing.T) *redfishSettingsFixture {
	t.Helper()
	f := &redfishSettingsFixture{t: t}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	return f
}

func (f *redfishSettingsFixture) client() *gofish.APIClient {
	f.t.Helper()
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: f.server.URL, BasicAuth: true})
	require.NoError(f.t, err)
	return client
}

func (f *redfishSettingsFixture) capturedWrites() []capturedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]capturedRequest(nil), f.writes...)
}

func (f *redfishSettingsFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		var payload map[string]any
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&payload))
		f.mu.Lock()
		f.writes = append(f.writes, capturedRequest{method: r.Method, path: r.URL.Path, payload: payload})
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	responses := map[string]string{
		"/redfish/v1/": `{
			"@odata.id":"/redfish/v1/", "Id":"RootService", "Name":"Root Service",
			"Managers":{"@odata.id":"/redfish/v1/Managers"},
			"Systems":{"@odata.id":"/redfish/v1/Systems"},
			"AccountService":{"@odata.id":"/redfish/v1/AccountService"}
		}`,
		"/redfish/v1/Managers": `{
			"@odata.id":"/redfish/v1/Managers", "Name":"Managers",
			"Members":[{"@odata.id":"/redfish/v1/Managers/BMC-A"},{"@odata.id":"/redfish/v1/Managers/BMC-B"}],
			"Members@odata.count":2
		}`,
		"/redfish/v1/Managers/BMC-A": `{
			"@odata.id":"/redfish/v1/Managers/BMC-A", "Id":"BMC-A", "Name":"Primary BMC",
			"FirmwareVersion":"1.2.3", "DateTime":"2026-01-01T00:00:00Z",
			"NetworkProtocol":{"@odata.id":"/redfish/v1/Managers/BMC-A/NetworkProtocol"},
			"EthernetInterfaces":{"@odata.id":"/redfish/v1/Managers/BMC-A/EthernetInterfaces"},
			"Actions":{"#Manager.ResetToDefaults":{"target":"/redfish/v1/Managers/BMC-A/Actions/Manager.ResetToDefaults"}}
		}`,
		"/redfish/v1/Managers/BMC-B": `{
			"@odata.id":"/redfish/v1/Managers/BMC-B", "Id":"BMC-B", "Name":"Secondary BMC",
			"FirmwareVersion":"9.9.9"
		}`,
		"/redfish/v1/Managers/BMC-A/NetworkProtocol": `{
			"@odata.id":"/redfish/v1/Managers/BMC-A/NetworkProtocol", "Id":"NetworkProtocol", "Name":"Manager Network Protocol",
			"FQDN":"bmc-a.example.com", "SSH":{"ProtocolEnabled":false,"Port":22}
		}`,
		"/redfish/v1/Managers/BMC-A/EthernetInterfaces": `{
			"@odata.id":"/redfish/v1/Managers/BMC-A/EthernetInterfaces", "Name":"Ethernet Interfaces",
			"Members":[{"@odata.id":"/redfish/v1/Managers/BMC-A/EthernetInterfaces/1"}], "Members@odata.count":1
		}`,
		"/redfish/v1/Managers/BMC-A/EthernetInterfaces/1": `{
			"@odata.id":"/redfish/v1/Managers/BMC-A/EthernetInterfaces/1", "Id":"1", "Name":"Management Ethernet Interface",
			"HostName":"bmc-a", "IPv6Enabled":false
		}`,
		"/redfish/v1/Systems": `{
			"@odata.id":"/redfish/v1/Systems", "Name":"Systems",
			"Members":[{"@odata.id":"/redfish/v1/Systems/Node0"}], "Members@odata.count":1
		}`,
		"/redfish/v1/Systems/Node0": `{
			"@odata.id":"/redfish/v1/Systems/Node0", "Id":"Node0", "Name":"Compute Node",
			"AssetTag":"old-tag", "Boot":{"BootOrder":["PXE","Disk"]}
		}`,
		"/redfish/v1/AccountService": `{
			"@odata.id":"/redfish/v1/AccountService", "Id":"AccountService", "Name":"Account Service",
			"Accounts":{"@odata.id":"/redfish/v1/AccountService/Accounts"}
		}`,
		"/redfish/v1/AccountService/Accounts": `{
			"@odata.id":"/redfish/v1/AccountService/Accounts", "Name":"Accounts",
			"Members":[{"@odata.id":"/redfish/v1/AccountService/Accounts/1"}], "Members@odata.count":1
		}`,
		"/redfish/v1/AccountService/Accounts/1": `{
			"@odata.id":"/redfish/v1/AccountService/Accounts/1", "Id":"1", "Name":"Administrator",
			"UserName":"root", "Enabled":true, "RoleId":"Administrator"
		}`,
	}
	response, ok := responses[r.URL.Path]
	if !ok {
		http.Error(w, fmt.Sprintf("unexpected path %s", r.URL.Path), http.StatusNotFound)
		return
	}
	_, _ = w.Write([]byte(response))
}

func TestSettingsGettersUseDefaultResources(t *testing.T) {
	f := newRedfishSettingsFixture(t)
	client := f.client()

	np, err := GetNetworkProtocol(client)
	require.NoError(t, err)
	require.Equal(t, "bmc-a.example.com", np.FQDN)

	interfaces, err := GetEthernetInterfaces(client)
	require.NoError(t, err)
	require.Len(t, interfaces, 1)
	require.Equal(t, "bmc-a", interfaces[0].HostName)

	system, err := GetDefaultComputerSystem(client)
	require.NoError(t, err)
	require.Equal(t, "Node0", system.ID)

	manager, err := GetDefaultManager(client)
	require.NoError(t, err)
	require.Equal(t, "BMC-A", manager.ID)

	accounts, err := ListAccounts(client)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, "root", accounts[0].UserName)
}

func TestSettingsPropertyPatchPayloads(t *testing.T) {
	f := newRedfishSettingsFixture(t)
	client := f.client()

	require.NoError(t, SetNetworkProtocol(client, "SSH", `{"ProtocolEnabled":true,"Port":2222}`))
	require.NoError(t, SetNetworkProtocol(client, "FQDN", "new-bmc.example.com"))
	require.NoError(t, SetComputerSystemProperty(client, "AssetTag", "new-tag"))
	require.NoError(t, SetComputerSystemProperty(client, "Boot", `{"BootOrder":["Disk","PXE"]}`))
	require.NoError(t, SetManagerProperty(client, "DateTime", "2026-08-26T12:00:00Z"))

	writes := f.capturedWrites()
	require.Len(t, writes, 5)
	require.Equal(t, capturedRequest{http.MethodPatch, "/redfish/v1/Managers/BMC-A/NetworkProtocol", map[string]any{
		"SSH": map[string]any{"ProtocolEnabled": true, "Port": float64(2222)},
	}}, writes[0])
	require.Equal(t, "new-bmc.example.com", writes[1].payload["FQDN"])
	require.Equal(t, "new-tag", writes[2].payload["AssetTag"])
	require.Equal(t, map[string]any{"BootOrder": []any{"Disk", "PXE"}}, writes[3].payload["Boot"])
	require.Equal(t, "2026-08-26T12:00:00Z", writes[4].payload["DateTime"])
}

func TestSettingsPropertyValidationPreventsWrites(t *testing.T) {
	f := newRedfishSettingsFixture(t)
	client := f.client()

	require.ErrorContains(t, SetNetworkProtocol(client, "Unknown", `{}`), "unknown network protocol")
	require.ErrorContains(t, SetNetworkProtocol(client, "SSH", `{`), "failed to parse value")
	require.ErrorContains(t, SetComputerSystemProperty(client, "Unknown", "value"), "unknown property")
	require.ErrorContains(t, SetComputerSystemProperty(client, "Boot", `{`), "failed to parse value")
	require.ErrorContains(t, SetManagerProperty(client, "Unknown", "value"), "unknown property")
	require.Empty(t, f.capturedWrites())
}

func TestExportedFieldRejectsInternalFields(t *testing.T) {
	type resource struct {
		Visible string
		hidden  string
	}
	value := &resource{Visible: "value", hidden: "secret"}
	field, ok := exportedField(value, "Visible")
	require.True(t, ok)
	require.Equal(t, "value", field.String())
	_, ok = exportedField(value, "hidden")
	require.False(t, ok)
}

func TestDecodePropertyValueKeepsBareStringScalars(t *testing.T) {
	field := reflect.ValueOf("")
	for _, value := range []string{"123", "true", "null"} {
		decoded, err := decodePropertyValue(field, value)
		require.NoError(t, err)
		require.Equal(t, value, decoded)
	}
}

func TestSettingsResourceUpdates(t *testing.T) {
	f := newRedfishSettingsFixture(t)
	client := f.client()

	require.NoError(t, SetEthernetInterface(client, 0, `{"IPv6Enabled":true}`))
	require.ErrorContains(t, SetEthernetInterface(client, 1, `{}`), "out of range")
	require.NoError(t, UpdateAccount(client, "1", `{"Enabled":false}`))
	require.ErrorContains(t, UpdateAccount(client, "missing", `{}`), "not found")

	writes := f.capturedWrites()
	require.Len(t, writes, 2)
	require.Equal(t, true, writes[0].payload["IPv6Enabled"])
	require.Equal(t, false, writes[1].payload["Enabled"])
}

func TestResetManagerValidatesPreservationMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected string
	}{
		{name: "reset all", mode: "", expected: "ResetAll"},
		{name: "preserve network", mode: "PreserveNetwork", expected: "PreserveNetwork"},
		{name: "preserve network and users", mode: "PreserveNetworkAndUsers", expected: "PreserveNetworkAndUsers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newRedfishSettingsFixture(t)
			require.NoError(t, ResetManager(f.client(), tt.mode))
			writes := f.capturedWrites()
			require.Len(t, writes, 1)
			require.Equal(t, http.MethodPost, writes[0].method)
			require.Equal(t, tt.expected, writes[0].payload["ResetType"])
		})
	}

	f := newRedfishSettingsFixture(t)
	require.ErrorContains(t, ResetManager(f.client(), "PreserveNetwrok"), "invalid preserve configuration")
	require.Empty(t, f.capturedWrites())
}
