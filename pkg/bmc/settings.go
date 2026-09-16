package bmc

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// Connection configuration for the settings commands. These are populated from
// CLI flags (username, password, secrets-file, insecure, cacert) and consumed
// by Connect/SettingsCredentialStore.
var (
	SettingsUsername    string
	SettingsPassword    string
	SettingsSecretsFile string
	SettingsInsecure    bool
	SettingsCACertPath  string
)

// SettingsCategories maps category names to a short description of each.
var SettingsCategories = map[string]string{
	"NetworkProtocol":   "Network service settings (SSH, HTTPS, IPMI, NTP, etc.)",
	"EthernetInterface": "Network interface settings (IP, MAC, DHCP, etc.)",
	"ComputerSystem":    "System-level settings (boot order, asset tag, etc.)",
	"Manager":           "Manager properties (firmware version, model, etc.)",
	"Accounts":          "BMC user accounts (username, role, etc.)",
	"Reset":             "Factory reset the BMC manager",
}

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

	supported, err := managers[0].GetSupportedResetToDefaultsTypes()
	if err != nil {
		return fmt.Errorf("failed to query reset-to-defaults support: %w", err)
	}
	if len(supported) == 0 {
		return fmt.Errorf("BMC %q does not support resetting to default via Manager.ResetToDefaults", managers[0].ID)
	}
	found := false
	for _, st := range supported {
		if st == resetType {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("BMC %q does not support reset type %q (supported: %v)", managers[0].ID, resetType, supported)
	}

	log.Info().Msgf("resetting manager %s to defaults (type: %s)", managers[0].ID, resetType)
	_, err = managers[0].ResetToDefaults(resetType)
	return err
}

// ListProtocols returns the names of the network protocols present on the
// ManagerNetworkProtocol of the first manager found on the BMC pointed to by
// client. Unlike GetProtocolNames, this verifies that a manager and its
// network protocol resource exist on the live BMC.
func ListProtocols(client *gofish.APIClient) ([]string, error) {
	np, err := GetNetworkProtocol(client)
	if err != nil {
		return nil, err
	}
	var names []string
	t := reflect.TypeOf(np).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// Skip embedded types and non-protocol fields
		if field.Anonymous || field.Name[0] == 'O' || field.Name == "Status" || field.Name == "HostName" || field.Name == "FQDN" {
			continue
		}
		names = append(names, field.Name)
	}
	return names, nil
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
	if field.Kind() == reflect.Pointer {
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
	if value.Kind() == reflect.Pointer {
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

// listSettingsCategories connects to the BMC and prints the categories for
// which data is actually present.
func ListSettingsCategories(client *gofish.APIClient, out io.Writer) error {
	var present []string
	for _, name := range []string{"NetworkProtocol", "EthernetInterface", "ComputerSystem", "Manager", "Accounts"} {
		var err error
		switch name {
		case "NetworkProtocol":
			_, err = GetNetworkProtocol(client)
		case "EthernetInterface":
			_, err = GetEthernetInterfaces(client)
		case "ComputerSystem":
			_, err = GetDefaultComputerSystem(client)
		case "Manager":
			_, err = GetDefaultManager(client)
		case "Accounts":
			_, err = ListAccounts(client)
		}
		if err != nil {
			continue
		}
		present = append(present, name)
	}

	_, _ = fmt.Fprintln(out, "Available setting categories on BMC:")
	_, _ = fmt.Fprintln(out)
	for _, name := range present {
		_, _ = fmt.Fprintf(out, "  %-20s %s\n", name, SettingsCategories[name])
	}
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Use 'magellan settings list <node> <category>' to inspect items in a category.")
	return nil
}

// listSettingsItems prints the items present under a category on the BMC.
func ListSettingsItems(client *gofish.APIClient, out io.Writer, category string) error {
	switch category {
	case "NetworkProtocol":
		names, err := ListProtocols(client)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(out, "Network protocols:")
		for _, name := range names {
			_, _ = fmt.Fprintf(out, "  %-15s (use 'magellan settings list <node> NetworkProtocol %s' for properties)\n", name, name)
		}
	case "EthernetInterface":
		ifaces, err := GetEthernetInterfaces(client)
		if err != nil {
			return err
		}
		if len(ifaces) == 0 {
			_, _ = fmt.Fprintln(out, "  (no ethernet interfaces found)")
			return nil
		}
		_, _ = fmt.Fprintln(out, "Ethernet interfaces:")
		for i := range ifaces {
			_, _ = fmt.Fprintf(out, "  %-3d %-15s %s (use 'magellan settings list <node> EthernetInterface %d' for properties)\n", i, ifaces[i].Name, ifaces[i].ID, i)
		}
	case "ComputerSystem":
		systems, err := client.GetService().Systems()
		if err != nil {
			return err
		}
		if len(systems) == 0 {
			_, _ = fmt.Fprintln(out, "  (no computer systems found)")
			return nil
		}
		_, _ = fmt.Fprintln(out, "Computer systems:")
		for _, sys := range systems {
			_, _ = fmt.Fprintf(out, "  %-15s %s (use 'magellan settings list <node> ComputerSystem %s' for properties)\n", sys.ID, sys.Name, sys.ID)
		}
	case "Manager":
		managers, err := client.GetService().Managers()
		if err != nil {
			return err
		}
		if len(managers) == 0 {
			_, _ = fmt.Fprintln(out, "  (no managers found)")
			return nil
		}
		_, _ = fmt.Fprintln(out, "Managers:")
		for _, mgr := range managers {
			_, _ = fmt.Fprintf(out, "  %-15s %s (use 'magellan settings list <node> Manager %s' for properties)\n", mgr.ID, mgr.Name, mgr.ID)
		}
	case "Accounts":
		accts, err := ListAccounts(client)
		if err != nil {
			return err
		}
		if len(accts) == 0 {
			_, _ = fmt.Fprintln(out, "  (no accounts found)")
			return nil
		}
		_, _ = fmt.Fprintln(out, "Accounts:")
		for i := range accts {
			_, _ = fmt.Fprintf(out, "  %-10s %-20s enabled=%v role=%s (use 'magellan settings list <node> Accounts %s' for properties)\n", accts[i].ID, accts[i].UserName, accts[i].Enabled, accts[i].RoleID, accts[i].ID)
		}
	case "Reset":
		_, _ = fmt.Fprintln(out, "  Reset is an action, not a listable resource. Use 'magellan settings reset <node>' to perform a factory reset.")
	}
	return nil
}

// listSettingsProperties prints the property names available at the resolved
// item/path of a category on the BMC. When the resolved value is a struct, its
// exported fields are listed; otherwise a message directs the user to 'get'.
func ListSettingsProperties(client *gofish.APIClient, out io.Writer, category, item string, path []string) error {
	resolved, err := ResolveListItem(client, category, item)
	if err != nil {
		return err
	}

	final, err := ResolveSettingsPath(resolved, path)
	if err != nil {
		return err
	}

	value := final
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			_, _ = fmt.Fprintln(out, "  (value is nil)")
			return nil
		}
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		_, _ = fmt.Fprintf(out, "  %s is a %s; use 'magellan settings get' to read it\n", item, value.Kind())
		return nil
	}

	_, _ = fmt.Fprintf(out, "Properties of %s.%s:", category, item)
	for _, name := range path {
		_, _ = fmt.Fprintf(out, ".%s", name)
	}
	_, _ = fmt.Fprintln(out)
	t := value.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" || f.Anonymous {
			continue
		}
		_, _ = fmt.Fprintf(out, "  %s\n", f.Name)
	}
	return nil
}

// resolveListItem resolves the list item for a category using list semantics:
// ComputerSystem and Manager items are identified by resource ID/name.
func ResolveListItem(client *gofish.APIClient, category, item string) (any, error) {
	switch category {
	case "NetworkProtocol":
		np, err := GetNetworkProtocol(client)
		if err != nil {
			return nil, err
		}
		field, ok := SettingsField(np, item)
		if !ok {
			return nil, fmt.Errorf("unknown protocol %q", item)
		}
		return field.Interface(), nil
	case "EthernetInterface":
		ifaces, err := GetEthernetInterfaces(client)
		if err != nil {
			return nil, err
		}
		idx := 0
		if _, err := fmt.Sscanf(item, "%d", &idx); err != nil {
			return nil, fmt.Errorf("invalid interface index %q: %w", item, err)
		}
		if idx < 0 || idx >= len(ifaces) {
			return nil, fmt.Errorf("interface index %d out of range (0-%d)", idx, len(ifaces)-1)
		}
		return ifaces[idx], nil
	case "ComputerSystem":
		return GetComputerSystem(client, item)
	case "Manager":
		return GetManager(client, item)
	case "Accounts":
		accts, err := ListAccounts(client)
		if err != nil {
			return nil, err
		}
		for i := range accts {
			if accts[i].ID == item {
				return accts[i], nil
			}
		}
		return nil, fmt.Errorf("account %q not found", item)
	case "Reset":
		return nil, fmt.Errorf("reset is an action, not a listable resource")
	default:
		return nil, fmt.Errorf("unknown category %q", category)
	}
}

// resolveCategoryCollection returns the full set of resources for a category
// (used when no item is specified).
func ResolveCategoryCollection(client *gofish.APIClient, category string) (any, error) {
	switch category {
	case "NetworkProtocol":
		np, err := GetNetworkProtocol(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get network protocol: %w", err)
		}
		return np, nil
	case "EthernetInterface":
		ifaces, err := GetEthernetInterfaces(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get ethernet interfaces: %w", err)
		}
		return ifaces, nil
	case "ComputerSystem":
		sys, err := GetDefaultComputerSystem(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get computer system: %w", err)
		}
		return sys, nil
	case "Manager":
		mgr, err := GetDefaultManager(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get manager: %w", err)
		}
		return mgr, nil
	case "Accounts":
		accts, err := ListAccounts(client)
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}
		return accts, nil
	default:
		return nil, fmt.Errorf("unknown category %q", category)
	}
}

// resolveCategoryItem resolves the category-level item into a resource value,
// following the same per-category semantics used to identify an item.
func ResolveCategoryItem(client *gofish.APIClient, category, item string) (any, error) {
	switch category {
	case "NetworkProtocol":
		np, err := GetNetworkProtocol(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get network protocol: %w", err)
		}
		field, ok := SettingsField(np, item)
		if !ok {
			return nil, fmt.Errorf("unknown protocol %q", item)
		}
		return field.Interface(), nil
	case "EthernetInterface":
		ifaces, err := GetEthernetInterfaces(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get ethernet interfaces: %w", err)
		}
		idx := 0
		if _, err := fmt.Sscanf(item, "%d", &idx); err != nil {
			return nil, fmt.Errorf("invalid interface index %q: %w", item, err)
		}
		if idx < 0 || idx >= len(ifaces) {
			return nil, fmt.Errorf("interface index %d out of range (0-%d)", idx, len(ifaces)-1)
		}
		return ifaces[idx], nil
	case "ComputerSystem":
		sys, err := GetDefaultComputerSystem(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get computer system: %w", err)
		}
		field, ok := SettingsField(sys, item)
		if !ok {
			return nil, fmt.Errorf("unknown property %q on ComputerSystem", item)
		}
		return field.Interface(), nil
	case "Manager":
		mgr, err := GetDefaultManager(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get manager: %w", err)
		}
		field, ok := SettingsField(mgr, item)
		if !ok {
			return nil, fmt.Errorf("unknown property %q on Manager", item)
		}
		return field.Interface(), nil
	case "Accounts":
		accts, err := ListAccounts(client)
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}
		for i := range accts {
			if accts[i].ID == item {
				return accts[i], nil
			}
		}
		return nil, fmt.Errorf("account %q not found", item)
	default:
		return nil, fmt.Errorf("unknown category %q", category)
	}
}

// ConnectWithCredentials connects to a BMC using the provided configuration.
// The function performs the following steps:
//  1. Initializes a gofish client with the provided configuration.
//  2. Attempts to connect to the BMC using the gofish client.
//  3. Handles specific connection errors such as 404 (ServiceRoot not found)
//     and 401 (authentication failed).
//  4. Returns the active gofish client.
func ConnectWithCredentials(uri string, store secrets.SecretStore, insecure bool, caCertPath string) (*gofish.APIClient, error) {
	if store == nil {
		return nil, fmt.Errorf("credential store is invalid")
	}
	masterCreds := GetBMCCredentialsOrDefault(store, uri)
	if masterCreds == (BMCCredentials{}) {
		return nil, fmt.Errorf("%s: credentials blank for BMC", uri)
	}

	clientConfig := gofish.ClientConfig{
		Endpoint:  uri,
		Username:  masterCreds.Username,
		Password:  masterCreds.Password,
		Insecure:  insecure,
		BasicAuth: true,
	}
	if caCertPath != "" {
		caCert, readErr := os.ReadFile(caCertPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read CA certificate %q: %w", caCertPath, readErr)
		}
		certPool, poolErr := x509.SystemCertPool()
		if poolErr != nil {
			certPool = x509.NewCertPool()
		}
		if ok := certPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to parse CA certificate %q", caCertPath)
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    certPool,
		}
		clientConfig.HTTPClient = &http.Client{Transport: transport}
	}

	// initialize gofish client
	client, err := gofish.Connect(clientConfig)
	if err != nil {
		if strings.HasPrefix(err.Error(), "404:") {
			err = fmt.Errorf("no ServiceRoot found.  This is probably not a BMC: %s", uri)
		}
		if strings.HasPrefix(err.Error(), "401:") {
			err = fmt.Errorf("authentication failed.  Check your username and password: %s", uri)
		}
		event := log.Error()
		event.Err(err)
		event.Msg("failed to connect to BMC")
		return nil, err
	}
	return client, nil
}

// Connect resolves a node argument to a BMC connection. When an
// inventory file is provided, nodeArg is looked up by ClusterID or NodeID;
// otherwise nodeArg is treated as a direct IP address or hostname.
func Connect(nodeArg string, inventoryFile string, inputFormat format.DataFormat) (*gofish.APIClient, error) {
	address := nodeArg
	if inventoryFile != "" {
		nodes, err := ParseInventory(inventoryFile, inputFormat)
		if err != nil {
			return nil, fmt.Errorf("failed to parse inventory file %s: %w", inventoryFile, err)
		}

		var found *Node
		for i := range nodes {
			if nodes[i].ClusterID == nodeArg || nodes[i].NodeID == nodeArg {
				found = &nodes[i]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("node %q not found in inventory", nodeArg)
		}
		address = found.BmcIP
	}

	endpoint, err := SettingsEndpoint(address)
	if err != nil {
		return nil, err
	}
	store, err := SettingsCredentialStore()
	if err != nil {
		return nil, err
	}
	return ConnectWithCredentials(endpoint, store, SettingsInsecure, SettingsCACertPath)
}

func SettingsCredentialStore() (secrets.SecretStore, error) {
	if SettingsUsername != "" && SettingsPassword != "" {
		return secrets.NewStaticStore(SettingsUsername, SettingsPassword), nil
	}
	if SettingsSecretsFile == "" {
		return nil, fmt.Errorf("BMC credentials are required; use --username/--password or --secrets-file")
	}
	store, err := secrets.OpenStore(SettingsSecretsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open secrets file: %w", err)
	}
	return store, nil
}

func SettingsEndpoint(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", fmt.Errorf("BMC address cannot be empty")
	}
	if parsed, err := url.Parse(address); err == nil && strings.Contains(address, "://") {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", fmt.Errorf("unsupported BMC URL scheme %q", parsed.Scheme)
		}
		if parsed.Host == "" {
			return "", fmt.Errorf("invalid BMC URL %q", address)
		}
		return strings.TrimRight(parsed.String(), "/"), nil
	}

	host := address
	if ip := net.ParseIP(address); ip != nil && strings.Contains(address, ":") {
		host = "[" + address + "]"
	}
	endpoint := (&url.URL{Scheme: "https", Host: host}).String()
	if endpoint == "https:" || endpoint == "https://" {
		return "", fmt.Errorf("invalid BMC address %q", address)
	}
	return endpoint, nil
}

func SettingsField(resource any, name string) (reflect.Value, bool) {
	value := reflect.ValueOf(resource)
	if value.Kind() == reflect.Pointer {
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

// settingsFieldByName walks into a struct (handling pointers) and returns the
// exported field matching the given name.
func SettingsFieldByName(value reflect.Value, name string) (reflect.Value, bool) {
	if value.Kind() == reflect.Pointer {
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

// resolveSettingsPath walks a starting value down a sequence of field names,
// returning the final value. Returns an error if any segment is not a valid
// exported field on the current struct.
func ResolveSettingsPath(start any, path []string) (reflect.Value, error) {
	current := reflect.ValueOf(start)
	for _, name := range path {
		field, ok := SettingsFieldByName(current, name)
		if !ok {
			return reflect.Value{}, fmt.Errorf("unknown property %q", name)
		}
		current = field
	}
	return current, nil
}
