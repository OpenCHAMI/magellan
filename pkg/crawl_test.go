package magellan

import (
	"net/http/httptest"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/OpenCHAMI/magellan/pkg/test"
	"github.com/go-chi/chi/v5"
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
	t.Parallel()

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
			c.config = tc.config

			crawler.CrawlBMCForSystems(*c.config)
			crawler.CrawlBMCForManagers(*c.config)
		})
	}
}
