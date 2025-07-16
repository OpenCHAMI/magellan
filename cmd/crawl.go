package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var crawlOutputFormat string

// The `crawl` command walks a collection of Redfish endpoints to collect
// specfic inventory detail. This command only expects host names and does
// not require a scan to be performed beforehand.
var CrawlCmd = &cobra.Command{
	Use: "crawl [uri]",
	Example: `  magellan crawl https://bmc.example.com
  magellan crawl https://bmc.example.com -i -u username -p password`,
	Short: "Crawl a single BMC for inventory information",
	Long:  "Crawl a single BMC for inventory information with URI.\n\n NOTE: This command does not scan subnets, store scan information in cache, nor make a request to a specified host. It is used only to retrieve inventory data directly. Otherwise, use 'scan' and 'collect' instead.",
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
	PreRunE: func(cmd *cobra.Command, args []string) (error) {
		// Validate the specified file format
		if crawlOutputFormat != util.FORMAT_JSON && crawlOutputFormat != util.FORMAT_YAML {
			return fmt.Errorf("specified format '%s' is invalid, must be (json|yaml)", crawlOutputFormat)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		var (
			uri    = args[0]
			store  secrets.SecretStore
			output []byte
			err    error
		)

		if username != "" && password != "" {
			// First, try and load credentials from --username and --password if both are set.
			log.Debug().Str("id", uri).Msgf("--username and --password specified, using them for BMC credentials")
			store = secrets.NewStaticStore(username, password)
		} else {
			// Alternatively, locate specific credentials (falling back to default) and override those
			// with --username or --password if either are passed.
			log.Debug().Str("id", uri).Msgf("one or both of --username and --password NOT passed, attempting to obtain missing credentials from secret store at %s", secretsFile)
			if store, err = secrets.OpenStore(secretsFile); err != nil {
				log.Error().Str("id", uri).Err(err).Msg("failed to open local secrets store")
			}

			// Either none of the flags were passed or only one of them were; get
			// credentials from secrets store to fill in the gaps.
			bmcCreds, _ := bmc.GetBMCCredentials(store, uri)
			nodeCreds := secrets.StaticStore{
				Username: bmcCreds.Username,
				Password: bmcCreds.Password,
			}

			// If either of the flags were passed, override the fetched
			// credentials with them.
			if username != "" {
				log.Info().Str("id", uri).Msg("--username was set, overriding username for this BMC")
				nodeCreds.Username = username
			}
			if password != "" {
				log.Info().Str("id", uri).Msg("--password was set, overriding password for this BMC")
				nodeCreds.Password = password
			}

			store = &nodeCreds
		}

		var (
			systems  []crawler.InventoryDetail
			managers []crawler.Manager
			config   = crawler.CrawlerConfig{
				URI:             uri,
				CredentialStore: store,
				Insecure:        insecure,
				UseDefault:      true,
			}
		)

		systems, err = crawler.CrawlBMCForSystems(config)
		if err != nil {
			log.Error().Err(err).Msg("failed to crawl BMC for systems")
		}
		managers, err = crawler.CrawlBMCForManagers(config)
		if err != nil {
			log.Error().Err(err).Msg("failed to crawl BMC for managers")
		}

		data := map[string]any{
			"Systems":  systems,
			"Managers": managers,
		}

		switch crawlOutputFormat {
		case util.FORMAT_JSON:
			// Marshal the inventory details to JSON
			output, err = json.MarshalIndent(data, "", "  ")
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal JSON")
				return
			}
		case util.FORMAT_YAML:
			// Marshal the inventory details to JSON
			output, err = yaml.Marshal(data)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal JSON")
				return
			}
		default:
			log.Error().Str("hint", "Try setting --format/-F to 'json' or 'yaml'").Msg("unrecognized format")
			os.Exit(1)
		}

		// Print the pretty JSON or YAML
		fmt.Println(string(output))
	},
}

func init() {
	CrawlCmd.Flags().StringVarP(&username, "username", "u", "", "Set the username for the BMC")
	CrawlCmd.Flags().StringVarP(&password, "password", "p", "", "Set the password for the BMC")
	CrawlCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL errors")
	CrawlCmd.Flags().StringVarP(&secretsFile, "secrets-file", "f", "secrets.json", "Set path to the node secrets file")
	CrawlCmd.Flags().StringVarP(&crawlOutputFormat, "format", "F", util.FORMAT_JSON, "Set the output format (json|yaml)")

	checkBindFlagError(viper.BindPFlag("crawl.insecure", CrawlCmd.Flags().Lookup("insecure")))
	checkBindFlagError(viper.BindPFlag("crawl.insecure", CrawlCmd.Flags().Lookup("insecure")))
	checkBindFlagError(viper.BindPFlag("crawl.insecure", CrawlCmd.Flags().Lookup("insecure")))

	rootCmd.AddCommand(CrawlCmd)
}
