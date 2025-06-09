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
	smdRecords := make([]map[string]any, 0)

	rtsHostname := fmt.Sprintf("%s-rts:8083", inventory.Hostname)
	pduBank := "B"

	for _, outlet := range inventory.Outlets {
		smdID := fmt.Sprintf("%sp1v%s", inventory.Hostname, outlet.ID)
		odataID := fmt.Sprintf("/redfish/v1/PowerEquipment/RackPDUs/%s/Outlets/%s", pduBank, outlet.ID)
		redfishURL := fmt.Sprintf("%s%s", rtsHostname, odataID)
		powerControlTarget := fmt.Sprintf("%s/Actions/Outlet.PowerControl", odataID)

		record := map[string]any{
			"ID":                    smdID,
			"Type":                  "CabinetPDUPowerConnector",
			"RedfishType":           "Outlet",
			"RedfishSubtype":        "Cx",
			"OdataID":               odataID,
			"RedfishEndpointID":     inventory.Hostname,
			"Enabled":               true,
			"RedfishEndpointFQDN":   rtsHostname,
			"RedfishURL":            redfishURL,
			"ComponentEndpointType": "ComponentEndpointOutlet",
			"RedfishOutletInfo": map[string]any{
				"Name": outlet.Name,
				"Actions": map[string]any{
					"#Outlet.PowerControl": map[string]any{
						"PowerState@Redfish.AllowableValues": []string{"On", "Off"},
						"target":                             powerControlTarget,
					},
				},
			},
		}
		smdRecords = append(smdRecords, record)
	}
	return smdRecords
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
				ID:                 "x9999m0",
				Type:               "CabinetPDUController",
				FQDN:               "x9999m0-rts.mock:8083",
				Hostname:           "x9999m0-rts.mock:8083",
				Enabled:            true,
				RediscoverOnUpdate: false,
				PDUInventory: PDUInventoryForSMD{
					Model:           "MOCK-PRO2",
					SerialNumber:    "MOCK-SN-12345",
					FirmwareVersion: "v9.9z",
					Outlets: []any{
						map[string]string{"id": "ZA01", "name": "Mock_Server_01", "state": "On", "socket_type": "Cx"},
						map[string]string{"id": "ZA02", "name": "Mock_Server_02", "state": "Off", "socket_type": "Cx"},
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
