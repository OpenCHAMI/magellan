package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/OpenCHAMI/magellan/pkg/jaws"
	"github.com/OpenCHAMI/magellan/pkg/pdu"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func transformToSMDFormat(inventory *pdu.PDUInventory) []map[string]any {
	smdOutlets := make([]map[string]any, 0)
	for _, outlet := range inventory.Outlets {
		var letterPart, numberPart string
		splitIndex := strings.IndexFunc(outlet.ID, unicode.IsDigit)

		if splitIndex == -1 {
			log.Warn().Msgf("could not parse outlet ID format for '%s', skipping outlet", outlet.ID)
			continue
		}
		letterPart = outlet.ID[:splitIndex]
		numberPart = outlet.ID[splitIndex:]

		var pValue int
		if len(letterPart) > 1 {
			pValue = int(unicode.ToUpper(rune(letterPart[1])) - 'A')
		}
		
		idSuffix := fmt.Sprintf("p%dv%s", pValue, numberPart)

		rawOutlet := map[string]any{
			"original_id": outlet.ID,
			"id_suffix":   idSuffix,
			"name":        outlet.Name,
			"state":       outlet.PowerState,
			"socket_type": outlet.SocketType,
		}
		smdOutlets = append(smdOutlets, rawOutlet)
	}

	pduRecord := map[string]any{
		"ID":                 inventory.Hostname,
		"Type":               "Node",
		"FQDN":               inventory.Hostname,
		"Hostname":           inventory.Hostname,
		"Enabled":            true,
		"RediscoverOnUpdate": false,
		"PDUInventory": map[string]any{
			"Outlets": smdOutlets,
		},
	}

	return []map[string]any{pduRecord}
}

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

		allSmdRecords := make([]map[string]any, 0)

		for _, host := range args {
			log.Info().Msgf("Collecting from PDU: %s", host)
			config := jaws.CrawlerConfig{
				URI:      host,
				Username: username,
				Password: password,
				Insecure: true,
			}

			inventory, err := jaws.CrawlPDU(config)
			if err != nil {
				log.Error().Err(err).Msgf("failed to crawl PDU %s", host)
				continue
			}

			smdRecords := transformToSMDFormat(inventory)

			allSmdRecords = append(allSmdRecords, smdRecords...)
		}

		output, err := json.MarshalIndent(allSmdRecords, "", "  ")
		if err != nil {
			log.Error().Err(err).Msgf("failed to marshal SMD records to JSON")
		}
		fmt.Println(string(output))
	},
}

func init() {
	PduCmd.AddCommand(pduCollectCmd)

	pduCollectCmd.Flags().StringVarP(&username, "username", "u", "", "Set the PDU username")
	pduCollectCmd.Flags().StringVarP(&password, "password", "p", "", "Set the PDU password")
}
