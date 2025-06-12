package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/jaws"
	"github.com/OpenCHAMI/magellan/pkg/pdu"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var mock bool

func transformToSMDFormat(inventory *pdu.PDUInventory) []map[string]any {
	smdOutlets := make([]map[string]any, 0)
	for _, outlet := range inventory.Outlets {
		rawOutlet := map[string]any{
			"id":          outlet.ID,
			"name":        outlet.Name,
			"state":       outlet.PowerState,
			"socket_type": "Cx",
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
		if mock {
			log.Info().Msg("Running in --mock mode. Generating hardcoded PDU payload to standard output.")

			type PDUInventoryForSMD struct {
				Model           string `json:"Model"`
				SerialNumber    string `json:"SerialNumber"`
				FirmwareVersion string `json:"FirmwareVersion"`
				Outlets         []any  `json:"Outlets"`
			}
			type PayloadForSMD struct {
				ID                 string             `json:"ID"`
				Type               string             `json:"Type"`
				FQDN               string             `json:"FQDN"`
				Hostname           string             `json:"Hostname"`
				Enabled            bool               `json:"Enabled"`
				RediscoverOnUpdate bool               `json:"RediscoverOnUpdate"`
				PDUInventory       PDUInventoryForSMD `json:"PDUInventory"`
			}

			mockPayload := PayloadForSMD{
				ID:                 "x3000m0",
				Type:               "Node",
				FQDN:               "x3000m0",
				Hostname:           "x3000m0",
				Enabled:            true,
				RediscoverOnUpdate: false,
				PDUInventory: PDUInventoryForSMD{
					Outlets: []any{
						map[string]string{"id": "BA35", "name": "Link1_Outlet_35", "state": "On", "socket_type": "Cx"},
						map[string]string{"id": "BA36", "name": "Link1_Outlet_36", "state": "Off", "socket_type": "Cx"},
					},
				},
			}
			payloadCollection := []PayloadForSMD{mockPayload}

			jsonData, err := json.MarshalIndent(payloadCollection, "", "  ")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to marshal mock payload")
			}

			fmt.Println(string(jsonData))
			return
		}
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
	pduCollectCmd.Flags().BoolVar(&mock, "mock", false, "Run in mock mode, sending hardcoded data to SMD")
}
