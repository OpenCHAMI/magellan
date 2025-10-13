package magellan

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	defaultBaseURI      = "https://bmc.openchami.cluster"
	scheme              = "https"
	protocol            = "tcp"
	timeout             = 10
	serviceRootResponse = `{
    "@odata.etag": "W/\"1646860561\"",
    "@odata.id": "/redfish/v1/",
    "@odata.type": "#ServiceRoot.v1_2_0.ServiceRoot",
    "AccountService": {
        "@odata.id": "/redfish/v1/AccountService"
    },
    "CertificateService": {
        "@odata.id": "/redfish/v1/CertificateService"
    },
    "Chassis": {
        "@odata.id": "/redfish/v1/Chassis"
    },
    "Description": "The service root for all Redfish requests on this host",
    "EventService": {
        "@odata.id": "/redfish/v1/EventService"
    },
    "Id": "RootService",
    "JsonSchemas": {
        "@odata.id": "/redfish/v1/JsonSchemas"
    },
    "Links": {
        "Sessions": {
            "@odata.id": "/redfish/v1/SessionService/Sessions"
        }
    },
    "Managers": {
        "@odata.id": "/redfish/v1/Managers"
    },
    "Name": "Root Service",
    "RedfishVersion": "1.2.0",
    "Registries": {
        "@odata.id": "/redfish/v1/Registries"
    },
    "SessionService": {
        "@odata.id": "/redfish/v1/SessionService"
    },
    "Systems": {
        "@odata.id": "/redfish/v1/Systems"
    },
    "Tasks": {
        "@odata.id": "/redfish/v1/TaskService"
    },
    "UpdateService": {
        "@odata.id": "/redfish/v1/UpdateService"
    }
}`
)

type ScanTestClient struct {
	params *ScanParams
}

func NewScanTestClient() *ScanTestClient {
	return &ScanTestClient{}
}

func ServiceRoot(baseURI string) string {
	return fmt.Sprintf("%s/redfish/v1", baseURI)
}

func (c *ScanTestClient) PerformScan() []RemoteAsset {
	return ScanForAssets(c.params)
}

func TestScan(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		params    *ScanParams
		wantFound int
	}{
		{
			name: "no_hosts_found",
			params: &ScanParams{
				Scheme:         scheme,
				Protocol:       protocol,
				Concurrency:    1,
				Timeout:        timeout,
				DisableProbing: false,
				Insecure:       true,
				Include: []string{
					"bmcs",
				},
			},
			wantFound: 0,
		},
		{
			name: "single_host",
			params: &ScanParams{
				Scheme:         scheme,
				Protocol:       protocol,
				Concurrency:    1,
				Timeout:        timeout,
				DisableProbing: false,
				Insecure:       true,
				Include: []string{
					"bmcs",
				},
			},
			wantFound: 1,
		},
		{
			name: "multiple_hosts",
			params: &ScanParams{
				Scheme:         scheme,
				Protocol:       protocol,
				Concurrency:    1,
				Timeout:        timeout,
				DisableProbing: false,
				Insecure:       true,
				Include: []string{
					"bmcs",
				},
			},
		},
		{
			name: "subnet_scan",
			params: &ScanParams{
				Scheme:         scheme,
				Protocol:       protocol,
				Concurrency:    1,
				Timeout:        timeout,
				DisableProbing: false,
				Insecure:       true,
				Include: []string{
					"bmcs",
				},
			},
			wantFound: 5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock server
			var servers []*httptest.Server
			for range tc.wantFound {
				mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/redfish/v1/" {
						t.Fatalf("Expected to request '%s', got: %s", ServiceRoot("/"), r.URL.Path)
					}
					if r.Method != http.MethodGet {
						t.Fatalf("Expected GET request, got: %s", r.Method)
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(serviceRootResponse))
				}))
				defer mockServer.Close() // Close the server when the test finishes
				servers = append(servers, mockServer)
			}

			c := NewScanTestClient()
			c.params = tc.params

			// add target hosts from mock servers created
			for _, mockServer := range servers {
				c.params.TargetHosts = append(c.params.TargetHosts, []string{mockServer.URL})
			}

			// set the number of expected responses
			tc.wantFound = len(servers)

			found := c.PerformScan()

			assert.Len(t, found, tc.wantFound)
		})

	}

}

func TestGenerateHostsFromSubnet(t *testing.T) {
	t.Parallel()

	var (
		defaultSubnetMask = net.IPMask{255, 255, 255, 0}
		defaultPorts      = []int{443}
	)

	type TestCase struct {
		name           string
		subnet         string
		subnetMask     *net.IPMask
		ports          []int
		scheme         string
		wantTotalHosts int
	}

	var getExpectedServiceCount = func(tc TestCase) int {
		v, err := strconv.ParseInt(tc.subnetMask.String(), 16, 0)
		if err != nil {
			return -1
		}
		return ((int(math.Pow(2, 32)) - int(v)) * len(tc.ports)) - 1
	}

	cases := []TestCase{
		{
			name:           "basic",
			subnet:         "172.21.0.0",
			subnetMask:     &defaultSubnetMask,
			ports:          defaultPorts,
			scheme:         scheme,
			wantTotalHosts: 0,
		},
		{
			name:           "none",
			subnet:         "10.0.0.0",
			subnetMask:     &defaultSubnetMask,
			ports:          defaultPorts,
			scheme:         scheme,
			wantTotalHosts: 0,
		},
		{
			name:           "invalid subnet",
			subnet:         "invalid",
			subnetMask:     &defaultSubnetMask,
			ports:          defaultPorts,
			scheme:         scheme,
			wantTotalHosts: 0,
		},
		{
			name:           "no subnet",
			subnet:         "",
			subnetMask:     &defaultSubnetMask,
			ports:          defaultPorts,
			scheme:         scheme,
			wantTotalHosts: 0,
		},
		{
			name:           "cidr",
			subnet:         "172.21.0.0/24",
			subnetMask:     &defaultSubnetMask,
			ports:          defaultPorts,
			scheme:         scheme,
			wantTotalHosts: 0,
		},
		{
			name:           "additional ports",
			subnet:         "172.21.0.0/24",
			subnetMask:     &defaultSubnetMask,
			ports:          []int{443, 5000},
			scheme:         scheme,
			wantTotalHosts: 0,
		},
		{
			name:           "different scheme",
			subnet:         "172.21.0.0/24",
			subnetMask:     &defaultSubnetMask,
			ports:          defaultPorts,
			scheme:         "http",
			wantTotalHosts: 0,
		},
	}

	for _, tc := range cases {
		tc.wantTotalHosts = getExpectedServiceCount(tc)
		hosts := GenerateHostsWithSubnet(
			tc.subnet,
			tc.subnetMask,
			tc.ports,
			tc.scheme,
		)

		assert.Len(t, hosts, tc.wantTotalHosts)
	}
}
