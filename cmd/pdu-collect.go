package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/jaws"
	"github.com/OpenCHAMI/magellan/pkg/pdu"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var pduCollectCmd = &cobra.Command{
	Use:   "collect [hosts...]",
	Short: "Collect inventory from JAWS-based PDUs",
	Long:  `Connects to one or more PDUs with a JAWS interface to collect hardware inventory.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error().Msg("no PDU hosts provided")
			return
		}

		if username == "" || password == "" {
			log.Error().Msg("--username and --password are required for PDU collection")
			return
		}

		collection := make([]*pdu.PDUInventory, 0)
		for _, host := range args {
			log.Info().Msgf("Collecting from PDU: %s", host)
			config := jaws.CrawlerConfig{
				URI:      host,
				Username: username,
				Password: password,
			}

			inventory, err := jaws.CrawlPDU(config)
			if err != nil {
				log.Error().Err(err).Msgf("failed to crawl PDU %s", host)
				continue
			}
			collection = append(collection, inventory)
		}

		output, err := json.MarshalIndent(collection, "", "    ")
		if err != nil {
			log.Error().Err(err).Msgf("failed to marshal PDU collection to JSON")
		}
		fmt.Println(string(output))
	},
}

func init() {
	PduCmd.AddCommand(pduCollectCmd)

	pduCollectCmd.Flags().StringVarP(&username, "username", "u", "", "Set the PDU username")
	pduCollectCmd.Flags().StringVarP(&password, "password", "p", "", "Set the PDU password")
}
