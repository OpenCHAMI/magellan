package bmc

import (
	"fmt"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// IsBMC reports whether a Redfish Manager represents a BMC.
func IsBMC(manager *schemas.Manager) bool {
	if manager == nil {
		return false
	}
	return manager.ManagerType == schemas.BMCManagerType || manager.ManagerType == schemas.ManagementControllerManagerType
}

// GetBMCManagers retrieves all Redfish Manager resources that represent BMCs.
func GetBMCManagers(client *gofish.APIClient) ([]*schemas.Manager, error) {
	var bmcManagers []*schemas.Manager
	managers, err := client.Service.Managers()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve managers: %v", err)
	}
	for _, manager := range managers {
		if !IsBMC(manager) {
			continue
		}
		bmcManagers = append(bmcManagers, manager)
	}
	return bmcManagers, nil
}
