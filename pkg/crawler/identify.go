package crawler

import (
	"fmt"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

// BMCInfo represents relevant information about a BMC
type BMCInfo struct {
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
	ManagerType     string `json:"manager_type"`
	UUID            string `json:"uuid"`
}

// IsBMC checks if a given Manager is a BMC based on its type and associations
func IsBMC(manager *redfish.Manager) bool {
	if manager == nil {
		return false
	}

	// Valid BMC types in Redfish
	bmcTypes := map[string]bool{
		"BMC":                  true,
		"ManagementController": true, // Some BMCs use this type
	}

	// Check if ManagerType matches a BMC type
	if !bmcTypes[string(manager.ManagerType)] {
		return false
	}

	return false // Otherwise, it's likely a chassis manager or other device
}

// GetBMCInfo retrieves details of all available BMCs
func GetBMCInfo(client *gofish.APIClient) ([]BMCInfo, error) {
	var bmcList []BMCInfo

	// Retrieve all managers (BMCs and other managers)
	managers, err := client.Service.Managers()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve managers: %v", err)
	}

	// Iterate through each manager and collect BMC details
	for _, manager := range managers {
		if !IsBMC(manager) {
			continue // Skip if it's not a BMC
		}

		bmc := BMCInfo{
			Manufacturer:    manager.Manufacturer,
			Model:           manager.Model,
			SerialNumber:    manager.SerialNumber,
			FirmwareVersion: manager.FirmwareVersion,
			ManagerType:     string(manager.ManagerType), // Convert ManagerType to string
			UUID:            manager.UUID,
		}

		bmcList = append(bmcList, bmc)
	}

	return bmcList, nil
}
