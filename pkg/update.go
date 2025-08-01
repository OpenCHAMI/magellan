package magellan

import (
	"fmt"
	"net/url"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

type UpdateParams struct {
	CollectParams
	URI              string   // Set from the positional paramters to update
	FirmwareURI      string   // set from the --firmware-url flag
	TransferProtocol string   // set from the --scheme flag
	Insecure         bool     // set from the --insecure flag
}

// UpdateFirmwareRemote() uses 'gofish' to update the firmware of a BMC node.
// The function expects the firmware URL, firmware version, and component flags to be
// set from the CLI to perform a firmware update.
// Example:
// ./magellan update https://192.168.23.40 --username root --password 0penBmc
// --firmware-url http://192.168.23.19:1337/obmc-phosphor-image.static.mtd.tar
// --scheme TFTP
//
// being:
// q.URI https://192.168.23.40
// q.TransferProtocol TFTP
// q.FirmwarePath http://192.168.23.19:1337/obmc-phosphor-image.static.mtd.tar
func UpdateFirmwareRemote(q *UpdateParams) error {
	// parse URI to set up full address
	uri, err := url.ParseRequestURI(q.URI)
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	// Get BMC credentials from secret store in update parameters
	bmcCreds, err := bmc.GetBMCCredentials(q.SecretStore, q.URI)
	if err != nil {
		return fmt.Errorf("failed to get BMC credentials: %w", err)
	}

	// Connect to the Redfish service using gofish
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: uri.String(), Username: bmcCreds.Username, Password: bmcCreds.Password, Insecure: q.Insecure})
	if err != nil {
		return fmt.Errorf("failed to connect to Redfish service: %w", err)
	}
	defer client.Logout()

	// Retrieve the UpdateService from the Redfish client
	updateService, err := client.Service.UpdateService()
	if err != nil {
		return fmt.Errorf("failed to get update service: %w", err)
	}

	// Build the update request payload
	req := redfish.SimpleUpdateParameters{
		ImageURI:         q.FirmwareURI,
		TransferProtocol: redfish.TransferProtocolType(q.TransferProtocol),
	}

	// Execute the SimpleUpdate action
	err = updateService.SimpleUpdate(&req)
	if err != nil {
		return fmt.Errorf("firmware update failed: %w", err)
	}
	fmt.Println("Firmware update initiated successfully.")

	return nil
}

func GetUpdateStatus(q *UpdateParams) error {
	// parse URI to set up full address
	uri, err := url.ParseRequestURI(q.URI)
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	// Get BMC credentials from secret store in update parameters
	bmcCreds, err := bmc.GetBMCCredentials(q.SecretStore, q.URI)
	if err != nil {
		return fmt.Errorf("failed to get BMC credentials: %w", err)
	}

	// Connect to the Redfish service using gofish
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: uri.String(), Username: bmcCreds.Username, Password: bmcCreds.Password, Insecure: q.Insecure})
	if err != nil {
		return fmt.Errorf("failed to connect to Redfish service: %w", err)
	}
	defer client.Logout()

	// Retrieve the UpdateService from the Redfish client
	updateService, err := client.Service.UpdateService()
	if err != nil {
		return fmt.Errorf("failed to get update service: %w", err)
	}

	// Get the update status
	status := updateService.Status
	fmt.Printf("Update Status: %v\n", status)

	return nil
}
