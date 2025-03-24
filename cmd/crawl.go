package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"

	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The `crawl` command walks a collection of Redfish endpoints to collect
// specfic inventory detail. This command only expects host names and does
// not require a scan to be performed beforehand.
var CrawlCmd = &cobra.Command{
	Use:   "crawl [uri]",
	Short: "Crawl a single BMC for inventory information",
	Long: "Crawl a single BMC for inventory information with URI. This command does NOT scan subnets nor store scan information\n" +
		"in cache after completion. To do so, use the 'collect' command instead\n\n" +
		"Examples:\n" +
		"  magellan crawl https://bmc.example.com\n" +
		"  magellan crawl https://bmc.example.com -i -u username -p password",
	Args: func(cmd *cobra.Command, args []string) error {
		// Validate that the only argument is a valid URI
		var err error
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		args[0], err = urlx.Sanitize(args[0])
		if err != nil {
			return fmt.Errorf("failed to sanitize URI: %w", err)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// try and load credentials from local store first
		store, err := secrets.OpenStore(secretsFile)
		if err != nil {
			log.Error().Err(err).Msg("failed to open store")
			// try and use the `username` and `password` arguments instead
			store = &secrets.StaticStore{
				Username: username,
				Password: password,
			}
		}
		systems, err := crawler.CrawlBMCForSystems(crawler.CrawlerConfig{
			URI:             args[0],
			CredentialStore: store,
			Insecure:        insecure,
		})
		if err != nil {
			log.Error().Err(err).Msg("error crawling BMC")
		}
		// Marshal the inventory details to JSON
		jsonData, err := json.MarshalIndent(systems, "", "  ")
		if err != nil {
			log.Error().Err(err).Msg("error marshalling to JSON:")
			return
		}

		// Print the pretty JSON
		fmt.Println(string(jsonData))
	},
}

func init() {
	CrawlCmd.Flags().StringVarP(&username, "username", "u", "", "Set the username for the BMC")
	CrawlCmd.Flags().StringVarP(&password, "password", "p", "", "Set the password for the BMC")
	CrawlCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL errors")
	CrawlCmd.Flags().StringVarP(&secretsFile, "file", "f", "nodes.json", "set the secrets file with BMC credentials")

	checkBindFlagError(viper.BindPFlag("crawl.username", CrawlCmd.Flags().Lookup("username")))
	checkBindFlagError(viper.BindPFlag("crawl.password", CrawlCmd.Flags().Lookup("password")))
	checkBindFlagError(viper.BindPFlag("crawl.insecure", CrawlCmd.Flags().Lookup("insecure")))

	rootCmd.AddCommand(CrawlCmd)
}
