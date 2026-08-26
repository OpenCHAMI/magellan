package bmc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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

	field, ok := exportedField(np, protocolName)
	if !ok {
		return fmt.Errorf("unknown network protocol %q", protocolName)
	}

	payload, err := decodePropertyValue(field, jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse value for protocol %q: %w", protocolName, err)
	}

	patchData := map[string]any{
		protocolName: payload,
	}
	if err := np.Patch(np.ODataID, patchData); err != nil {
		return fmt.Errorf("failed to update network protocol %q: %w", protocolName, err)
	}
	return nil
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

	payload, err := decodeObject(jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse JSON for ethernet interface %d: %w", index, err)
	}
	if err := ifaces[index].Patch(ifaces[index].ODataID, payload); err != nil {
		return fmt.Errorf("failed to update ethernet interface %d: %w", index, err)
	}
	return nil
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

// GetDefaultComputerSystem returns the first ComputerSystem exposed by the BMC.
func GetDefaultComputerSystem(client *gofish.APIClient) (*schemas.ComputerSystem, error) {
	service := client.GetService()
	systems, err := service.Systems()
	if err != nil {
		return nil, fmt.Errorf("failed to list systems: %w", err)
	}
	if len(systems) == 0 {
		return nil, fmt.Errorf("no computer systems found on BMC")
	}
	return systems[0], nil
}

// SetComputerSystem applies JSON-encoded properties to the named ComputerSystem.
func SetComputerSystem(client *gofish.APIClient, systemID, jsonData string) error {
	sys, err := GetComputerSystem(client, systemID)
	if err != nil {
		return err
	}

	payload, err := decodeObject(jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse JSON for ComputerSystem %q: %w", systemID, err)
	}
	if err := sys.Patch(sys.ODataID, payload); err != nil {
		return fmt.Errorf("failed to update ComputerSystem %q: %w", systemID, err)
	}
	return nil
}

// SetComputerSystemProperty applies a value to a named property on the first
// ComputerSystem exposed by the BMC.
func SetComputerSystemProperty(client *gofish.APIClient, propertyName, value string) error {
	sys, err := GetDefaultComputerSystem(client)
	if err != nil {
		return err
	}

	field, ok := exportedField(sys, propertyName)
	if !ok {
		return fmt.Errorf("unknown property %q on ComputerSystem", propertyName)
	}
	payload, err := decodePropertyValue(field, value)
	if err != nil {
		return fmt.Errorf("failed to parse value for ComputerSystem.%s: %w", propertyName, err)
	}
	if err := sys.Patch(sys.ODataID, map[string]any{propertyName: payload}); err != nil {
		return fmt.Errorf("failed to update ComputerSystem.%s: %w", propertyName, err)
	}
	return nil
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

// GetDefaultManager returns the first Manager exposed by the BMC.
func GetDefaultManager(client *gofish.APIClient) (*schemas.Manager, error) {
	service := client.GetService()
	managers, err := service.Managers()
	if err != nil {
		return nil, fmt.Errorf("failed to list managers: %w", err)
	}
	if len(managers) == 0 {
		return nil, fmt.Errorf("no managers found on BMC")
	}
	return managers[0], nil
}

// SetManager applies JSON-encoded properties to the named Manager.
func SetManager(client *gofish.APIClient, name, jsonData string) error {
	mgr, err := GetManager(client, name)
	if err != nil {
		return err
	}

	payload, err := decodeObject(jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse JSON for Manager %q: %w", name, err)
	}
	if err := mgr.Patch(mgr.ODataID, payload); err != nil {
		return fmt.Errorf("failed to update Manager %q: %w", name, err)
	}
	return nil
}

// SetManagerProperty applies a value to a named property on the first Manager
// exposed by the BMC.
func SetManagerProperty(client *gofish.APIClient, propertyName, value string) error {
	mgr, err := GetDefaultManager(client)
	if err != nil {
		return err
	}

	field, ok := exportedField(mgr, propertyName)
	if !ok {
		return fmt.Errorf("unknown property %q on Manager", propertyName)
	}
	payload, err := decodePropertyValue(field, value)
	if err != nil {
		return fmt.Errorf("failed to parse value for Manager.%s: %w", propertyName, err)
	}
	if err := mgr.Patch(mgr.ODataID, map[string]any{propertyName: payload}); err != nil {
		return fmt.Errorf("failed to update Manager.%s: %w", propertyName, err)
	}
	return nil
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
			payload, err := decodeObject(jsonData)
			if err != nil {
				return fmt.Errorf("failed to parse JSON for account %q: %w", accountID, err)
			}
			if err := accts[i].Patch(accts[i].ODataID, payload); err != nil {
				return fmt.Errorf("failed to update account %q: %w", accountID, err)
			}
			return nil
		}
	}
	return fmt.Errorf("account %q not found", accountID)
}

// ResetManager performs a factory reset on the first manager.
// preserveConfig can be: "" (reset all), "PreserveNetwork", or "PreserveNetworkAndUsers".
func ResetManager(client *gofish.APIClient, preserveConfig string) error {
	var resetType schemas.ResetToDefaultsType
	switch preserveConfig {
	case "":
		resetType = schemas.ResetAllResetToDefaultsType
	case "PreserveNetwork":
		resetType = schemas.PreserveNetworkResetToDefaultsType
	case "PreserveNetworkAndUsers":
		resetType = schemas.PreserveNetworkAndUsersResetToDefaultsType
	default:
		return fmt.Errorf("invalid preserve configuration %q", preserveConfig)
	}

	service := client.GetService()
	managers, err := service.Managers()
	if err != nil {
		return fmt.Errorf("failed to list managers: %w", err)
	}
	if len(managers) == 0 {
		return fmt.Errorf("no managers found on BMC")
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

	field, ok := exportedField(np, protocolName)
	if !ok {
		return nil, fmt.Errorf("unknown protocol %q", protocolName)
	}

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil, nil
		}
		field = field.Elem()
	}

	if field.Kind() != reflect.Struct {
		return nil, fmt.Errorf("protocol %q does not contain nested properties", protocolName)
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

// decodePropertyValue parses a command-line value and verifies that it can be
// represented by the corresponding gofish schema field. Bare values are
// treated as strings, while valid JSON scalars, objects, and arrays retain
// their JSON types.
func decodePropertyValue(field reflect.Value, value string) (any, error) {
	trimmed := strings.TrimSpace(value)
	var parsed any
	if field.Kind() == reflect.String && !strings.HasPrefix(trimmed, `"`) {
		parsed = value
	} else if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, `"`) {
			return nil, err
		}
		parsed = value
	}

	encoded, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	target := reflect.New(field.Type())
	if err := json.Unmarshal(encoded, target.Interface()); err != nil {
		return nil, err
	}
	return parsed, nil
}

func decodeObject(value string) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("value must be a JSON object")
	}
	return payload, nil
}

func exportedField(resource any, name string) (reflect.Value, bool) {
	value := reflect.ValueOf(resource)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	fieldType, ok := value.Type().FieldByName(name)
	if !ok || fieldType.PkgPath != "" || fieldType.Anonymous {
		return reflect.Value{}, false
	}
	return value.FieldByIndex(fieldType.Index), true
}
