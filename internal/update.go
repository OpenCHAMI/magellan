package magellan

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

type UpdateParams struct {
	CollectParams
	FirmwarePath     string
	FirmwareVersion  string
	Component        string
	TransferProtocol string
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

	// Connect to the Redfish service using gofish (using insecure connection for this example)
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: uri.String(), Username: q.Username, Password: q.Password, Insecure: true})
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
		ImageURI:         q.FirmwarePath,
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
	uri.User = url.UserPassword(q.Username, q.Password)
	updateUrl := fmt.Sprintf("%s/redfish/v1/UpdateService", uri.String())
	res, body, err := client.MakeRequest(nil, updateUrl, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("something went wrong: %v", err)
	} else if res == nil {
		return fmt.Errorf("no response returned (url: %s)", updateUrl)
	} else if res.StatusCode != http.StatusOK {
		return fmt.Errorf("returned status code %d", res.StatusCode)
	}
	if len(body) > 0 {
		fmt.Printf("%v\n", string(body))
	}
	return nil
}
