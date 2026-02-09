package magellan

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenCHAMI/magellan/internal/format"
	"github.com/stretchr/testify/assert"
)

type CollectTestClient struct {
	assets *[]RemoteAsset
	params *CollectParams
}

func NewCollectTestClient() *CollectTestClient {
	return &CollectTestClient{}
}

func (c *CollectTestClient) PerformCollect() ([]map[string]any, error) {
	return CollectInventory(c.assets, c.params)
}

func TestCollect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		assets []RemoteAsset
		params *CollectParams
		want   int
	}{
		{
			name: "basic",
			assets: []RemoteAsset{
				{
					Host:        "",
					Port:        443,
					State:       true,
					Protocol:    "tcp",
					ServiceType: BMC,
				},
			},
			params: &CollectParams{
				Concurrency: 1,
				Timeout:     timeout,
				Insecure:    true,
				Format:      format.FORMAT_JSON,
				SecretStore: nil,
			},
			want: 0,
		},
		{
			name: "",
			assets: []RemoteAsset{
				{
					Host:        "",
					Port:        443,
					State:       true,
					Protocol:    "tcp",
					ServiceType: BMC,
				},
			},
			params: &CollectParams{},
		},
		{
			name: "",
			assets: []RemoteAsset{
				{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			// create mock Redfish servers
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "" {

				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(""))
			}))
			defer mockServer.Close()

			var (
				collection []map[string]any
				err        error
			)

			c := NewCollectTestClient()
			c.params = tc.params

			// add mock Redfish services
			// for _, mockServer := range servers {
			// 	c.assets = append(c.assets)
			// }

			collection, err = c.PerformCollect()

			assert.Error(t, nil, err)
			assert.Len(t, collection, tc.want)

		})
	}
}
