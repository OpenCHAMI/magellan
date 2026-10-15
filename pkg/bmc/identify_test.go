package bmc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openchami/magellan/pkg/test"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
	"github.com/stretchr/testify/require"
)

func TestIsBMC(t *testing.T) {
	require.False(t, IsBMC(nil))
	for _, managerType := range []schemas.ManagerType{schemas.BMCManagerType, schemas.ManagementControllerManagerType} {
		require.True(t, IsBMC(&schemas.Manager{ManagerType: managerType}))
	}
	require.False(t, IsBMC(&schemas.Manager{ManagerType: schemas.ManagerType("EnclosureManager")}))
}

func TestGetBMCManagers(t *testing.T) {
	for _, failManagers := range []bool{false, true} {
		name := "manager identity"
		if failManagers {
			name = "manager collection failure"
		}
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/redfish/v1/", test.Make(test.RESPONSE_ServiceRoot))
			mux.HandleFunc("/redfish/v1/Managers", func(w http.ResponseWriter, r *http.Request) {
				if failManagers {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				test.Make(`{"@odata.id":"/redfish/v1/Managers","Members":[{"@odata.id":"/redfish/v1/Managers/bmc"},{"@odata.id":"/redfish/v1/Managers/enclosure"}],"Members@odata.count":2}`)(w, r)
			})
			mux.HandleFunc("/redfish/v1/Managers/bmc", test.Make(`{
				"Id":"bmc", "ManagerType":"BMC", "Manufacturer":"Vendor",
				"Model":"Controller", "SerialNumber":"BMC-SERIAL",
				"FirmwareVersion":"1.2.3", "UUID":"bmc-uuid"
			}`))
			mux.HandleFunc("/redfish/v1/Managers/enclosure", test.Make(`{"Id":"enclosure","ManagerType":"EnclosureManager","SerialNumber":"OTHER-SERIAL"}`))
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			client, err := gofish.Connect(gofish.ClientConfig{
				Endpoint: srv.URL, Username: "test", Password: "test", BasicAuth: true,
			})
			require.NoError(t, err)
			t.Cleanup(client.Logout)
			managers, err := GetBMCManagers(client)
			if failManagers {
				require.ErrorContains(t, err, "failed to retrieve managers")
				require.Nil(t, managers)
				return
			}
			require.NoError(t, err)
			require.Len(t, managers, 1)
			require.Equal(t, "bmc", managers[0].ID)
			require.Equal(t, "Vendor", managers[0].Manufacturer)
			require.Equal(t, "Controller", managers[0].Model)
			require.Equal(t, "BMC-SERIAL", managers[0].SerialNumber)
			require.Equal(t, "1.2.3", managers[0].FirmwareVersion)
			require.Equal(t, schemas.BMCManagerType, managers[0].ManagerType)
			require.Equal(t, "bmc-uuid", managers[0].UUID)
		})
	}
}
