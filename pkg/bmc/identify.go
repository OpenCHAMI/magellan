package bmc

import (
	"fmt"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// BMCInfo describes a Redfish Manager, not the ComputerSystem represented by Node.
type BMCInfo struct {
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
	ManagerType     string `json:"manager_type"`
	UUID            string `json:"uuid"`
}

// IsBMC reports whether a Redfish Manager represents a BMC.
func IsBMC(manager *schemas.Manager) bool {
	if manager == nil {
		return false
	}
	return manager.ManagerType == schemas.BMCManagerType || manager.ManagerType == schemas.ManagementControllerManagerType
}

// GetBMCInfo retrieves identity details for all BMC managers.
func GetBMCInfo(client *gofish.APIClient) ([]BMCInfo, error) {
	var bmcList []BMCInfo
	managers, err := client.Service.Managers()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve managers: %v", err)
	}
	for _, manager := range managers {
		if !IsBMC(manager) {
			continue
		}
		bmcList = append(bmcList, BMCInfo{
			Manufacturer:    manager.Manufacturer,
			Model:           manager.Model,
			SerialNumber:    manager.SerialNumber,
			FirmwareVersion: manager.FirmwareVersion,
			ManagerType:     string(manager.ManagerType),
			UUID:            manager.UUID,
		})
	}
	return bmcList, nil
}
