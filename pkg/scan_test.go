package magellan

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/test"
	"github.com/stretchr/testify/assert"
)

const (
	scheme   = "https"
	protocol = "tcp"
	timeout  = 10
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
					_, err := w.Write([]byte(test.RESPONSE_ServiceRoot))
					assert.NoError(t, err)
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
		wantPorts      int
	}

	var getExpectedHostCount = func(tc TestCase) int {
		if net.ParseIP(tc.subnet) == nil {
			if _, _, err := net.ParseCIDR(tc.subnet); err != nil {
				return 0
			}
		}
		v, err := strconv.ParseInt(tc.subnetMask.String(), 16, 0)
		if err != nil {
			return -1
		}
		return (int(math.Pow(2, 32)) - int(v)) - 1
	}

	cases := []TestCase{
		{
			name:       "basic",
			subnet:     "172.21.0.0",
			subnetMask: &defaultSubnetMask,
			ports:      defaultPorts,
			scheme:     scheme,
		},
		{
			name:       "none",
			subnet:     "10.0.0.0",
			subnetMask: &defaultSubnetMask,
			ports:      defaultPorts,
			scheme:     scheme,
		},
		{
			name:       "invalid subnet",
			subnet:     "invalid",
			subnetMask: &defaultSubnetMask,
			ports:      defaultPorts,
			scheme:     scheme,
		},
		{
			name:       "no subnet",
			subnet:     "",
			subnetMask: &defaultSubnetMask,
			ports:      defaultPorts,
			scheme:     scheme,
		},
		{
			name:       "cidr",
			subnet:     "172.21.0.0/24",
			subnetMask: &defaultSubnetMask,
			ports:      defaultPorts,
			scheme:     scheme,
		},
		{
			name:       "additional ports",
			subnet:     "172.21.0.0/24",
			subnetMask: &defaultSubnetMask,
			ports:      []int{443, 5000},
			scheme:     scheme,
		},
		{
			name:       "different scheme",
			subnet:     "172.21.0.0/24",
			subnetMask: &defaultSubnetMask,
			ports:      defaultPorts,
			scheme:     "http",
		},
	}

	for _, tc := range cases {
		tc.wantTotalHosts = getExpectedHostCount(tc)
		tc.wantPorts = len(tc.ports)
		hosts := GenerateHostsWithSubnet(
			tc.subnet,
			tc.subnetMask,
			tc.ports,
			tc.scheme,
		)

		assert.Len(t, hosts, tc.wantTotalHosts)
		for _, hostPorts := range hosts {
			assert.Len(t, hostPorts, tc.wantPorts)
		}
	}
}
