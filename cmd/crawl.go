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
	Run: func(cmd *cobra.Command, args []string) {
		var (
			uri   = args[0]
			store secrets.SecretStore
			err   error
		)

		if username != "" && password != "" {
			// First, try and load credentials from --username and --password if both are set.
			log.Debug().Str("uri", uri).Msgf("--username and --password specified, using them for BMC credentials")
			store = secrets.NewStaticStore(username, password)
		} else {
			// Alternatively, locate specific credentials (falling back to default) and override those
			// with --username or --password if either are passed.
			log.Debug().Str("uri", uri).Msgf("one or both of --username and --password NOT passed, attempting to obtain missing credentials from secret store at %s", secretsFile)
			if store, err = secrets.OpenStore(secretsFile); err != nil {
				log.Error().Str("uri", uri).Err(err).Msg("failed to open local secrets store")
			}

			// Either none of the flags were passed or only one of them were; get
			// credentials from secrets store to fill in the gaps.
			//
			// Attempt to get URI-specific credentials.
			var nodeCreds secrets.StaticStore
			if uriCreds, err := store.GetSecretByID(uri); err != nil {
				// Specific credentials for URI not found, fetch default.
				log.Warn().Str("uri", uri).Msg("specific credentials not found, falling back to default")
				defaultSecret, err := store.GetSecretByID(secrets.DEFAULT_KEY)
				if err != nil {
					// We've exhausted all options, the credentials will be blank unless
					// overridden by a CLI flag.
					log.Warn().Str("uri", uri).Err(err).Msg("no default credentials were set, they will be blank unless overridden by CLI flags")
				} else {
					// Default credentials found, use them.
					var creds crawler.BMCUsernamePassword
					if err = json.Unmarshal([]byte(defaultSecret), &creds); err != nil {
						log.Warn().Str("uri", uri).Err(err).Msg("failed to unmarshal default secrets store credentials")
					} else {
						log.Info().Str("uri", uri).Msg("default credentials found, using")
						nodeCreds.Username = creds.Username
						nodeCreds.Password = creds.Password
					}
				}
			} else {
				// Specific URI credentials found, use them.
				var creds crawler.BMCUsernamePassword
				if err = json.Unmarshal([]byte(uriCreds), &creds); err != nil {
					log.Warn().Str("uri", uri).Err(err).Msg("failed to unmarshal uri credentials")
				} else {
					nodeCreds.Username = creds.Username
					nodeCreds.Password = creds.Password
					log.Info().Str("uri", uri).Msg("specific credentials found, using")
				}
			}

			// If either of the flags were passed, override the fetched
			// credentials with them.
			if username != "" {
				log.Info().Str("uri", uri).Msg("--username was set, overriding username for this BMC")
				nodeCreds.Username = username
			}
			if password != "" {
				log.Info().Str("uri", uri).Msg("--password was set, overriding password for this BMC")
				nodeCreds.Password = password
			}

			store = &nodeCreds
		}

		systems, err := crawler.CrawlBMCForSystems(crawler.CrawlerConfig{
			URI:             uri,
			CredentialStore: store,
			Insecure:        insecure,
			UseDefault:      true,
		})
		if err != nil {
			log.Error().Err(err).Msg("failed to crawl BMC")
		}
		// Marshal the inventory details to JSON
		jsonData, err := json.MarshalIndent(systems, "", "  ")
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal JSON")
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

	checkBindFlagError(viper.BindPFlag("crawl.insecure", CrawlCmd.Flags().Lookup("insecure")))

	rootCmd.AddCommand(CrawlCmd)
}
