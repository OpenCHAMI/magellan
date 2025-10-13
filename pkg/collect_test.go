package magellan

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
				RemoteAsset{
					Host: "",
					Port: 443,
				},
			},
			params: &CollectParams{
				Concurrency: 1,
				Timeout:     timeout,
			},
			want: 0,
		},
		{
			name: "",
		},
		{
			name: "",
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
