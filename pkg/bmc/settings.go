package bmc

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// GetNetworkProtocol returns the ManagerNetworkProtocol from the first manager
// found on the BMC pointed to by client.
func GetNetworkProtocol(client *gofish.APIClient) (*schemas.ManagerNetworkProtocol, error) {
	service := client.GetService()
	managers, err := service.Managers()
	if err != nil {
		return nil, fmt.Errorf("failed to list managers: %w", err)
	}
	if len(managers) == 0 {
		return nil, fmt.Errorf("no managers found on BMC")
	}
	return managers[0].NetworkProtocol()
}

// SetNetworkProtocol applies JSON-encoded properties to a named network protocol
// (e.g., "SSH", "HTTPS", "IPMI") on the first manager. Since ManagerNetworkProtocol
// does not expose an Update() method, this builds a patch payload and sends it
// directly via the Entity Patch method.
func SetNetworkProtocol(client *gofish.APIClient, protocolName, jsonData string) error {
	np, err := GetNetworkProtocol(client)
	if err != nil {
		return err
	}

	npVal := reflect.ValueOf(np).Elem()
	field := npVal.FieldByName(protocolName)
	if !field.IsValid() {
		return fmt.Errorf("unknown network protocol %q", protocolName)
	}

	if !field.CanAddr() {
		return fmt.Errorf("protocol %q field is not addressable", protocolName)
	}

	// Decode the JSON into the field to validate it
	if err := json.Unmarshal([]byte(jsonData), field.Addr().Interface()); err != nil {
		return fmt.Errorf("failed to parse JSON for protocol %q: %w", protocolName, err)
	}

	// Build the patch payload with only the protocol name
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
		return fmt.Errorf("failed to parse JSON payload: %w", err)
	}

	patchData := map[string]any{
		protocolName: payload,
	}
	return np.Patch(np.ODataID, patchData)
}

// GetEthernetInterfaces returns all EthernetInterface resources from the first
// manager on the BMC.
func GetEthernetInterfaces(client *gofish.APIClient) ([]schemas.EthernetInterface, error) {
	service := client.GetService()
	managers, err := service.Managers()
	if err != nil {
		return nil, fmt.Errorf("failed to list managers: %w", err)
	}
	if len(managers) == 0 {
		return nil, fmt.Errorf("no managers found on BMC")
	}
	ifaces, err := managers[0].EthernetInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get ethernet interfaces: %w", err)
	}
	result := make([]schemas.EthernetInterface, len(ifaces))
	for i := range ifaces {
		result[i] = *ifaces[i]
	}
	return result, nil
}

// SetEthernetInterface applies JSON-encoded properties to the Nth ethernet
// interface (0-indexed) of the first manager.
func SetEthernetInterface(client *gofish.APIClient, index int, jsonData string) error {
	service := client.GetService()
	managers, err := service.Managers()
	if err != nil {
		return fmt.Errorf("failed to list managers: %w", err)
	}
	if len(managers) == 0 {
		return fmt.Errorf("no managers found on BMC")
	}
	ifaces, err := managers[0].EthernetInterfaces()
	if err != nil {
		return fmt.Errorf("failed to get ethernet interfaces: %w", err)
	}
	if index < 0 || index >= len(ifaces) {
		return fmt.Errorf("ethernet interface index %d out of range (0-%d)", index, len(ifaces)-1)
	}

	if err := json.Unmarshal([]byte(jsonData), ifaces[index]); err != nil {
		return fmt.Errorf("failed to parse JSON for ethernet interface %d: %w", index, err)
	}

	return ifaces[index].Update()
}

// GetComputerSystem returns the ComputerSystem matching the given systemID.
func GetComputerSystem(client *gofish.APIClient, systemID string) (*schemas.ComputerSystem, error) {
	service := client.GetService()
	systems, err := service.Systems()
	if err != nil {
		return nil, fmt.Errorf("failed to list systems: %w", err)
	}
	for _, sys := range systems {
		if sys.ID == systemID {
			return sys, nil
		}
	}
	return nil, fmt.Errorf("computer system %q not found", systemID)
}

// SetComputerSystem applies JSON-encoded properties to the named ComputerSystem.
func SetComputerSystem(client *gofish.APIClient, systemID, jsonData string) error {
	sys, err := GetComputerSystem(client, systemID)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(jsonData), sys); err != nil {
		return fmt.Errorf("failed to parse JSON for ComputerSystem %q: %w", systemID, err)
	}

	return sys.Update()
}

// GetManager returns the Manager matching the given name (e.g. "BMC", "1").
func GetManager(client *gofish.APIClient, name string) (*schemas.Manager, error) {
	service := client.GetService()
	managers, err := service.Managers()
	if err != nil {
		return nil, fmt.Errorf("failed to list managers: %w", err)
	}
	for _, mgr := range managers {
		if mgr.ID == name || mgr.Name == name {
			return mgr, nil
		}
	}
	return nil, fmt.Errorf("manager %q not found", name)
}

// SetManager applies JSON-encoded properties to the named Manager.
func SetManager(client *gofish.APIClient, name, jsonData string) error {
	mgr, err := GetManager(client, name)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(jsonData), mgr); err != nil {
		return fmt.Errorf("failed to parse JSON for Manager %q: %w", name, err)
	}

	return mgr.Update()
}

// ListAccounts returns all ManagerAccount resources from the AccountService.
func ListAccounts(client *gofish.APIClient) ([]schemas.ManagerAccount, error) {
	service := client.GetService()
	acctSvc, err := service.AccountService()
	if err != nil {
		return nil, fmt.Errorf("failed to get account service: %w", err)
	}
	accts, err := acctSvc.Accounts()
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	result := make([]schemas.ManagerAccount, len(accts))
	for i := range accts {
		result[i] = *accts[i]
	}
	return result, nil
}

// UpdateAccount applies JSON-encoded properties to the account matching accountID.
func UpdateAccount(client *gofish.APIClient, accountID, jsonData string) error {
	accts, err := ListAccounts(client)
	if err != nil {
		return err
	}
	for i := range accts {
		if accts[i].ID == accountID {
			if err := json.Unmarshal([]byte(jsonData), &accts[i]); err != nil {
				return fmt.Errorf("failed to parse JSON for account %q: %w", accountID, err)
			}
			return accts[i].Update()
		}
	}
	return fmt.Errorf("account %q not found", accountID)
}

// ResetManager performs a factory reset on the first manager.
// preserveConfig can be: "" (reset all), "PreserveNetwork", or "PreserveNetworkAndUsers".
func ResetManager(client *gofish.APIClient, preserveConfig string) error {
	service := client.GetService()
	managers, err := service.Managers()
	if err != nil {
		return fmt.Errorf("failed to list managers: %w", err)
	}
	if len(managers) == 0 {
		return fmt.Errorf("no managers found on BMC")
	}

	var resetType schemas.ResetToDefaultsType
	switch preserveConfig {
	case "PreserveNetwork":
		resetType = schemas.PreserveNetworkResetToDefaultsType
	case "PreserveNetworkAndUsers":
		resetType = schemas.PreserveNetworkAndUsersResetToDefaultsType
	default:
		resetType = schemas.ResetAllResetToDefaultsType
	}

	log.Info().Msgf("resetting manager %s to defaults (type: %s)", managers[0].ID, resetType)
	_, err = managers[0].ResetToDefaults(resetType)
	return err
}

// GetProtocolNames returns the names of protocol fields on ManagerNetworkProtocol.
func GetProtocolNames() []string {
	var names []string
	t := reflect.TypeOf(schemas.ManagerNetworkProtocol{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// Skip embedded types and non-protocol fields
		if field.Anonymous || field.Name[0] == 'O' || field.Name == "Status" || field.Name == "HostName" || field.Name == "FQDN" {
			continue
		}
		names = append(names, field.Name)
	}
	return names
}

// GetProtocolProperties returns the property names of a nested protocol setting.
func GetProtocolProperties(client *gofish.APIClient, protocolName string) ([]string, error) {
	np, err := GetNetworkProtocol(client)
	if err != nil {
		return nil, err
	}

	npVal := reflect.ValueOf(np).Elem()
	field := npVal.FieldByName(protocolName)
	if !field.IsValid() {
		return nil, fmt.Errorf("unknown protocol %q", protocolName)
	}

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil, nil
		}
		field = field.Elem()
	}

	var props []string
	t := field.Type()
	for i := 0; i < t.NumField(); i++ {
		props = append(props, t.Field(i).Name)
	}
	return props, nil
}

// GetEthernetInterfaceProperties returns the property names for an ethernet interface.
func GetEthernetInterfaceProperties(client *gofish.APIClient, index int) ([]string, error) {
	ifaces, err := GetEthernetInterfaces(client)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(ifaces) {
		return nil, fmt.Errorf("interface index %d out of range (0-%d)", index, len(ifaces)-1)
	}

	var props []string
	t := reflect.TypeOf(ifaces[index])
	for i := 0; i < t.NumField(); i++ {
		props = append(props, t.Field(i).Name)
	}
	return props, nil
}

// GetComputerSystemProperties returns the property names for a ComputerSystem.
func GetComputerSystemProperties(client *gofish.APIClient, systemID string) ([]string, error) {
	sys, err := GetComputerSystem(client, systemID)
	if err != nil {
		return nil, err
	}

	var props []string
	t := reflect.TypeOf(sys).Elem()
	for i := 0; i < t.NumField(); i++ {
		props = append(props, t.Field(i).Name)
	}
	return props, nil
}

// GetManagerProperties returns the property names for a Manager.
func GetManagerProperties(client *gofish.APIClient, name string) ([]string, error) {
	mgr, err := GetManager(client, name)
	if err != nil {
		return nil, err
	}

	var props []string
	t := reflect.TypeOf(mgr).Elem()
	for i := 0; i < t.NumField(); i++ {
		props = append(props, t.Field(i).Name)
	}
	return props, nil
}

// GetAccountProperties returns the property names for a ManagerAccount.
func GetAccountProperties(client *gofish.APIClient) ([]string, error) {
	accts, err := ListAccounts(client)
	if err != nil {
		return nil, err
	}
	if len(accts) == 0 {
		return nil, fmt.Errorf("no accounts found")
	}

	var props []string
	t := reflect.TypeOf(accts[0])
	for i := 0; i < t.NumField(); i++ {
		props = append(props, t.Field(i).Name)
	}
	return props, nil
}

// setStructField sets a field on a gofish struct by name and value.
// The value is JSON-marshaled and then unmarshaled into the target field
// to handle type conversions (e.g. string -> int, string -> []string).
func setStructField(field *reflect.Value, key string, value any) error {
	switch v := value.(type) {
	case string:
		if len(v) > 0 && (v[0] == '{' || v[0] == '[') {
			jsonBytes := []byte(v)
			if err := json.Unmarshal(jsonBytes, field.Addr().Interface()); err != nil {
				return fmt.Errorf("failed to parse value for %q: %w", key, err)
			}
		} else {
			if err := json.Unmarshal([]byte(fmt.Sprintf(`"%s"`, v)), field.Addr().Interface()); err != nil {
				if err2 := json.Unmarshal([]byte(v), field.Addr().Interface()); err2 != nil {
					return fmt.Errorf("failed to set %q: %w", key, err)
				}
			}
		}
	default:
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value for %q: %w", key, err)
		}
		if err := json.Unmarshal(jsonBytes, field.Addr().Interface()); err != nil {
			return fmt.Errorf("failed to set %q: %w", key, err)
		}
	}
	return nil
}
