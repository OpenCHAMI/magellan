package magellan

import (
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/magellan/pkg/crawler"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/openchami/magellan/pkg/test"
	"github.com/stretchr/testify/require"
)

type CrawlTestClient struct {
	config *crawler.CrawlerConfig
}

func NewCrawlTestClient() *CrawlTestClient {
	store := secrets.NewStaticStore("test", "test")
	return &CrawlTestClient{
		config: &crawler.CrawlerConfig{
			URI:             "http://localhost:5000",
			Insecure:        true,
			CredentialStore: store,
		},
	}
}

func TestCrawl(t *testing.T) {
	cases := []struct {
		name   string
		config *crawler.CrawlerConfig
	}{
		{
			name: "basic",
			config: &crawler.CrawlerConfig{
				UseDefault: true,
			},
		},
		{
			name: "no_default",
			config: &crawler.CrawlerConfig{
				UseDefault: false,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := chi.NewMux()
			mux.HandleFunc("/redfish/v1", test.Make(test.RESPONSE_ServiceRoot))
			mux.HandleFunc("/redfish/v1/Systems", test.Make(test.RESPONSE_Systems))
			mux.HandleFunc("/redfish/v1/Node0/EthernetInterfaces", test.Make(test.RESPONSE_EthernetInterface))

			// create a mock server to simulator a Redfish service
			mockServer := httptest.NewServer(mux)
			defer mockServer.Close()

			c := NewCrawlTestClient()
			c.config.URI = mockServer.URL
			c.config.UseDefault = tc.config.UseDefault

			// The mock only implements a subset of Redfish, so the crawler should
			// report that the full service cannot be discovered.
			_, err := crawler.CrawlBMCForSystems(*c.config)
			require.Error(t, err)
			_, err = crawler.CrawlBMCForManagers(*c.config)
			require.Error(t, err)
		})
	}
}
