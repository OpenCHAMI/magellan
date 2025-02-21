package magellan

import (
<<<<<<< HEAD
	"fmt"
	"net/url"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
=======
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/OpenCHAMI/magellan/pkg/client"
>>>>>>> 81116ec (refactor: moved internal functions to pkg and updated refs)
)

type UpdateParams struct {
	CollectParams
	FirmwarePath     string
	FirmwareVersion  string
	Component        string
	TransferProtocol string
<<<<<<< HEAD
	Insecure         bool
=======
>>>>>>> 81116ec (refactor: moved internal functions to pkg and updated refs)
}

// UpdateFirmwareRemote() uses 'gofish' to update the firmware of a BMC node.
// The function expects the firmware URL, firmware version, and component flags to be
// set from the CLI to perform a firmware update.
<<<<<<< HEAD
// Example:
// ./magellan update https://192.168.23.40 --username root --password 0penBmc
// --firmware-url http://192.168.23.19:1337/obmc-phosphor-image.static.mtd.tar
// --scheme TFTP
//
// being:
// q.URI https://192.168.23.40
// q.TransferProtocol TFTP
// q.FirmwarePath http://192.168.23.19:1337/obmc-phosphor-image.static.mtd.tar
=======
>>>>>>> 81116ec (refactor: moved internal functions to pkg and updated refs)
func UpdateFirmwareRemote(q *UpdateParams) error {
	// parse URI to set up full address
	uri, err := url.ParseRequestURI(q.URI)
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}
<<<<<<< HEAD

	// Connect to the Redfish service using gofish
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: uri.String(), Username: q.Username, Password: q.Password, Insecure: q.Insecure})
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
=======
	uri.User = url.UserPassword(q.Username, q.Password)

	// set up other vars
	updateUrl := fmt.Sprintf("%s/redfish/v1/UpdateService/Actions/SimpleUpdate", uri.String())
	headers := map[string]string{
		"Content-Type":  "application/json",
		"cache-control": "no-cache",
	}
	b := map[string]any{
		"UpdateComponent":  q.Component, // BMC, BIOS
		"TransferProtocol": q.TransferProtocol,
		"ImageURI":         q.FirmwarePath,
	}
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}
	res, body, err := client.MakeRequest(nil, updateUrl, "POST", data, headers)
	if err != nil {
		return fmt.Errorf("something went wrong: %v", err)
	} else if res == nil {
		return fmt.Errorf("no response returned (url: %s)", updateUrl)
	}
	if len(body) > 0 {
		fmt.Printf("%d: %v\n", res.StatusCode, string(body))
	}
>>>>>>> 81116ec (refactor: moved internal functions to pkg and updated refs)
	return nil
}

func GetUpdateStatus(q *UpdateParams) error {
	// parse URI to set up full address
	uri, err := url.ParseRequestURI(q.URI)
	if err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}
<<<<<<< HEAD

	// Connect to the Redfish service using gofish
	client, err := gofish.Connect(gofish.ClientConfig{Endpoint: uri.String(), Username: q.Username, Password: q.Password, Insecure: q.Insecure})
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

=======
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
>>>>>>> 81116ec (refactor: moved internal functions to pkg and updated refs)
	return nil
}
