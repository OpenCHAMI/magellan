package magellan

import (
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/crawler"
)

type CrawlTestClient struct {
	config *crawler.CrawlerConfig
}

func NewCrawlTestClient() *CrawlTestClient {
	return &CrawlTestClient{}
}

func TestCrawl(t *testing.T) {
	t.Parallel()
	// TODO: initialize secret store for test case

	cases := []struct {
		name   string
		config *crawler.CrawlerConfig
	}{
		{
			name: "basic",
			config: &crawler.CrawlerConfig{
				URI:             "",
				Insecure:        true,
				CredentialStore: nil,
				UseDefault:      true,
			},
		},
		{
			name: "secrets",
			config: &crawler.CrawlerConfig{
				URI:             "",
				Insecure:        true,
				CredentialStore: nil,
				UseDefault:      true,
			},
		},
		{
			name: "no_default",
			config: &crawler.CrawlerConfig{
				URI:             "",
				Insecure:        true,
				CredentialStore: nil,
				UseDefault:      false,
			},
		},
	}

	for _, tc := range cases {
		c := NewCrawlTestClient()
		c.config = tc.config
	}
}
