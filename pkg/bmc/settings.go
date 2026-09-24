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
	"sort"
	"strconv"
	"strings"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
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

// nonProtocolProperties lists top-level ManagerNetworkProtocol keys that may
// hold JSON objects but are not individually manageable network protocols.
var nonProtocolProperties = map[string]bool{
	"Status":  true,
	"Oem":     true,
	"Links":   true,
	"Actions": true,
}

// redfishResource fetches a Redfish resource and decodes it into generic JSON
// so the settings feature operates on the exact payload the BMC returned,
// rather than on gofish Go structs.
func redfishResource(client *gofish.APIClient, path string) (map[string]any, error) {
	resp, err := client.Get(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s for %s", resp.Status, path)
	}
	var doc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", path, err)
	}
	return doc, nil
}

// redfishCollection fetches a Redfish collection resource and resolves each
// member reference into its full resource document.
func redfishCollection(client *gofish.APIClient, path string) ([]map[string]any, error) {
	doc, err := redfishResource(client, path)
	if err != nil {
		return nil, err
	}
	var members []map[string]any
	for _, ref := range asSlice(doc["Members"]) {
		uri := oDataID(ref)
		if uri == "" {
			continue
		}
		member, err := redfishResource(client, uri)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, nil
}

// patchResource sends a PATCH request with the given JSON payload to a
// Redfish resource URI. URIs come from the BMC's own payload (@odata.id), so
// requests always target the resource the data was read from.
func patchResource(client *gofish.APIClient, uri string, payload map[string]any) error {
	resp, err := client.Patch(uri, payload)
	if err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	return nil
}

// managerMembers returns the raw JSON of every Manager resource advertised by
// the service root.
func managerMembers(client *gofish.APIClient) ([]map[string]any, error) {
	root, err := redfishResource(client, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch service root: %w", err)
	}
	managersPath := redfishLink(root, "Managers")
	if managersPath == "" {
		return nil, fmt.Errorf("service root does not expose a Managers collection")
	}
	managers, err := redfishCollection(client, managersPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list managers: %w", err)
	}
	return managers, nil
}

// systemMembers returns the raw JSON of every ComputerSystem resource
// advertised by the service root.
func systemMembers(client *gofish.APIClient) ([]map[string]any, error) {
	root, err := redfishResource(client, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch service root: %w", err)
	}
	systemsPath := redfishLink(root, "Systems")
	if systemsPath == "" {
		return nil, fmt.Errorf("service root does not expose a Systems collection")
	}
	systems, err := redfishCollection(client, systemsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list systems: %w", err)
	}
	return systems, nil
}

// asMap type-asserts a decoded JSON value to an object.
func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// asSlice type-asserts a decoded JSON value to an array.
func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

// oDataID returns the @odata.id reference of a decoded Redfish value, or an
// empty string when the value is not a link.
func oDataID(v any) string {
	if m := asMap(v); m != nil {
		if id, ok := m["@odata.id"].(string); ok {
			return id
		}
	}
	return ""
}

// redfishLink returns the @odata.id value of a link property within a Redfish
// JSON object, or an empty string if the key is absent.
func redfishLink(doc map[string]any, key string) string {
	link := asMap(doc[key])
	if link == nil {
		return ""
	}
	id, _ := link["@odata.id"].(string)
	return id
}

// decodeSettingValue parses a command-line value for a Redfish property, using
// the property's existing JSON value to decide how to interpret the input:
// bare values become strings when the property currently holds a string;
// otherwise the input is parsed as JSON.
func decodeSettingValue(current any, value string) (any, error) {
	trimmed := strings.TrimSpace(value)
	if _, isString := current.(string); isString && !strings.HasPrefix(trimmed, `"`) {
		return value, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, `"`) {
			return nil, err
		}
		return value, nil
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

// GetNetworkProtocol returns the ManagerNetworkProtocol resource of the first
// manager found on the BMC pointed to by client, as raw Redfish JSON.
func GetNetworkProtocol(client *gofish.APIClient) (map[string]any, error) {
	mgr, err := GetDefaultManager(client)
	if err != nil {
		return nil, err
	}
	npPath := redfishLink(mgr, "NetworkProtocol")
	if npPath == "" {
		return nil, fmt.Errorf("manager does not expose a NetworkProtocol resource")
	}
	return redfishResource(client, npPath)
}

// SetNetworkProtocol applies JSON-encoded properties to a named network
// protocol (e.g., "SSH", "HTTPS", "IPMI") on the first manager. The protocol
// name must match a property of the actual NetworkProtocol payload and is used
// verbatim as the PATCH key.
func SetNetworkProtocol(client *gofish.APIClient, protocolName, jsonData string) error {
	np, err := GetNetworkProtocol(client)
	if err != nil {
		return err
	}
	current, ok := np[protocolName]
	if !ok {
		return fmt.Errorf("unknown network protocol %q", protocolName)
	}
	payload, err := decodeSettingValue(current, jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse value for protocol %q: %w", protocolName, err)
	}
	return patchResource(client, oDataID(np), map[string]any{protocolName: payload})
}

// GetEthernetInterfaces returns all EthernetInterface resources from the first
// manager on the BMC, as raw Redfish JSON.
func GetEthernetInterfaces(client *gofish.APIClient) ([]map[string]any, error) {
	mgr, err := GetDefaultManager(client)
	if err != nil {
		return nil, err
	}
	ifacesPath := redfishLink(mgr, "EthernetInterfaces")
	if ifacesPath == "" {
		return nil, fmt.Errorf("manager does not expose an EthernetInterfaces collection")
	}
	ifaces, err := redfishCollection(client, ifacesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get ethernet interfaces: %w", err)
	}
	return ifaces, nil
}

// SetEthernetInterface applies JSON-encoded properties to the Nth ethernet
// interface (0-indexed) of the first manager.
func SetEthernetInterface(client *gofish.APIClient, index int, jsonData string) error {
	ifaces, err := GetEthernetInterfaces(client)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(ifaces) {
		return fmt.Errorf("ethernet interface index %d out of range (0-%d)", index, len(ifaces)-1)
	}
	payload, err := decodeObject(jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse JSON for ethernet interface %d: %w", index, err)
	}
	return patchResource(client, oDataID(ifaces[index]), payload)
}

// GetComputerSystem returns the ComputerSystem matching the given systemID.
func GetComputerSystem(client *gofish.APIClient, systemID string) (map[string]any, error) {
	systems, err := systemMembers(client)
	if err != nil {
		return nil, err
	}
	for _, sys := range systems {
		if fmt.Sprint(sys["Id"]) == systemID {
			return sys, nil
		}
	}
	return nil, fmt.Errorf("computer system %q not found", systemID)
}

// GetDefaultComputerSystem returns the first ComputerSystem exposed by the BMC.
func GetDefaultComputerSystem(client *gofish.APIClient) (map[string]any, error) {
	systems, err := systemMembers(client)
	if err != nil {
		return nil, err
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
	uri := oDataID(sys)
	if uri == "" {
		return fmt.Errorf("computer system %q missing @odata.id", systemID)
	}
	return patchResource(client, uri, payload)
}

// SetComputerSystemProperty applies a value to a named property on the first
// ComputerSystem exposed by the BMC.
func SetComputerSystemProperty(client *gofish.APIClient, propertyName, value string) error {
	sys, err := GetDefaultComputerSystem(client)
	if err != nil {
		return err
	}
	current, ok := sys[propertyName]
	if !ok {
		return fmt.Errorf("unknown property %q on ComputerSystem", propertyName)
	}
	payload, err := decodeSettingValue(current, value)
	if err != nil {
		return fmt.Errorf("failed to parse value for ComputerSystem.%s: %w", propertyName, err)
	}
	return patchResource(client, oDataID(sys), map[string]any{propertyName: payload})
}

// GetManager returns the Manager matching the given name (e.g. "BMC", "1").
func GetManager(client *gofish.APIClient, name string) (map[string]any, error) {
	managers, err := managerMembers(client)
	if err != nil {
		return nil, err
	}
	for _, mgr := range managers {
		if fmt.Sprint(mgr["Id"]) == name || fmt.Sprint(mgr["Name"]) == name {
			return mgr, nil
		}
	}
	return nil, fmt.Errorf("manager %q not found", name)
}

// GetDefaultManager returns the first Manager exposed by the BMC.
func GetDefaultManager(client *gofish.APIClient) (map[string]any, error) {
	managers, err := managerMembers(client)
	if err != nil {
		return nil, err
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
	uri := oDataID(mgr)
	if uri == "" {
		return fmt.Errorf("manager %q missing @odata.id", name)
	}
	return patchResource(client, uri, payload)
}

// SetManagerProperty applies a value to a named property on the first Manager
// exposed by the BMC.
func SetManagerProperty(client *gofish.APIClient, propertyName, value string) error {
	mgr, err := GetDefaultManager(client)
	if err != nil {
		return err
	}
	current, ok := mgr[propertyName]
	if !ok {
		return fmt.Errorf("unknown property %q on Manager", propertyName)
	}
	payload, err := decodeSettingValue(current, value)
	if err != nil {
		return fmt.Errorf("failed to parse value for Manager.%s: %w", propertyName, err)
	}
	return patchResource(client, oDataID(mgr), map[string]any{propertyName: payload})
}

// ListAccounts returns all ManagerAccount resources from the AccountService,
// as raw Redfish JSON.
func ListAccounts(client *gofish.APIClient) ([]map[string]any, error) {
	root, err := redfishResource(client, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch service root: %w", err)
	}
	acctSvcPath := redfishLink(root, "AccountService")
	if acctSvcPath == "" {
		return nil, fmt.Errorf("service root does not expose an AccountService")
	}
	acctSvc, err := redfishResource(client, acctSvcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get account service: %w", err)
	}
	acctsPath := redfishLink(acctSvc, "Accounts")
	if acctsPath == "" {
		return nil, fmt.Errorf("account service does not expose an Accounts collection")
	}
	accts, err := redfishCollection(client, acctsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	return accts, nil
}

// UpdateAccount applies JSON-encoded properties to the account matching
// accountID.
func UpdateAccount(client *gofish.APIClient, accountID, jsonData string) error {
	accts, err := ListAccounts(client)
	if err != nil {
		return err
	}
	for _, acct := range accts {
		if fmt.Sprint(acct["Id"]) == accountID {
			payload, err := decodeObject(jsonData)
			if err != nil {
				return fmt.Errorf("failed to parse JSON for account %q: %w", accountID, err)
			}
			uri := oDataID(acct)
			if uri == "" {
				return fmt.Errorf("account %q missing @odata.id", accountID)
			}
			return patchResource(client, uri, payload)
		}
	}
	return fmt.Errorf("account %q not found", accountID)
}

// ResetManager performs a factory reset on the first manager.
// preserveConfig can be: "" (reset all), "PreserveNetwork", or "PreserveNetworkAndUsers".
func ResetManager(client *gofish.APIClient, preserveConfig string) error {
	resetType := ""
	switch preserveConfig {
	case "":
		resetType = "ResetAll"
	case "PreserveNetwork":
		resetType = "PreserveNetwork"
	case "PreserveNetworkAndUsers":
		resetType = "PreserveNetworkAndUsers"
	default:
		return fmt.Errorf("invalid preserve configuration %q", preserveConfig)
	}

	managers, err := managerMembers(client)
	if err != nil {
		return fmt.Errorf("failed to list managers: %w", err)
	}
	if len(managers) == 0 {
		return fmt.Errorf("no managers found on BMC")
	}
	mgr := managers[0]

	action := asMap(asMap(mgr["Actions"])["#Manager.ResetToDefaults"])
	target, _ := action["target"].(string)
	if target == "" {
		return fmt.Errorf("BMC %q does not support resetting to default via Manager.ResetToDefaults", mgr["Id"])
	}
	var supported []string
	for _, v := range asSlice(action["ResetType@Redfish.AllowableValues"]) {
		if s, ok := v.(string); ok {
			supported = append(supported, s)
		}
	}
	if len(supported) == 0 {
		return fmt.Errorf("BMC %q does not support resetting to default via Manager.ResetToDefaults", mgr["Id"])
	}
	found := false
	for _, st := range supported {
		if st == resetType {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("BMC %q does not support reset type %q (supported: %v)", mgr["Id"], resetType, supported)
	}

	log.Info().Msgf("resetting manager %s to defaults (type: %s)", mgr["Id"], resetType)
	resp, err := client.Post(target, map[string]any{"ResetType": resetType})
	if resp != nil {
		if err := resp.Body.Close(); err != nil {
			return err
		}
	}
	return err
}

// ListProtocols returns the names of the network protocols present on the
// ManagerNetworkProtocol of the first manager found on the BMC pointed to by
// client. Protocol names are derived from the object properties of the actual
// NetworkProtocol payload, so they match the property names a curl call would
// return.
func ListProtocols(client *gofish.APIClient) ([]string, error) {
	np, err := GetNetworkProtocol(client)
	if err != nil {
		return nil, err
	}
	var names []string
	for name, value := range np {
		if strings.HasPrefix(name, "@") || nonProtocolProperties[name] {
			continue
		}
		if _, ok := value.(map[string]any); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
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
			_, _ = fmt.Fprintf(out, "  %-3d %-15s %s (use 'magellan settings list <node> EthernetInterface %d' for properties)\n", i, ifaces[i]["Name"], ifaces[i]["Id"], i)
		}
	case "ComputerSystem":
		systems, err := systemMembers(client)
		if err != nil {
			return err
		}
		if len(systems) == 0 {
			_, _ = fmt.Fprintln(out, "  (no computer systems found)")
			return nil
		}
		_, _ = fmt.Fprintln(out, "Computer systems:")
		for _, sys := range systems {
			_, _ = fmt.Fprintf(out, "  %-15s %s (use 'magellan settings list <node> ComputerSystem %s' for properties)\n", sys["Id"], sys["Name"], sys["Id"])
		}
	case "Manager":
		managers, err := managerMembers(client)
		if err != nil {
			return err
		}
		if len(managers) == 0 {
			_, _ = fmt.Fprintln(out, "  (no managers found)")
			return nil
		}
		_, _ = fmt.Fprintln(out, "Managers:")
		for _, mgr := range managers {
			_, _ = fmt.Fprintf(out, "  %-15s %s (use 'magellan settings list <node> Manager %s' for properties)\n", mgr["Id"], mgr["Name"], mgr["Id"])
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
			_, _ = fmt.Fprintf(out, "  %-10s %-20s enabled=%v role=%s (use 'magellan settings list <node> Accounts %s' for properties)\n", accts[i]["Id"], accts[i]["UserName"], accts[i]["Enabled"], accts[i]["RoleId"], accts[i]["Id"])
		}
	case "Reset":
		_, _ = fmt.Fprintln(out, "  Reset is an action, not a listable resource. Use 'magellan settings reset <node>' to perform a factory reset.")
	}
	return nil
}

// listSettingsProperties prints the property names available at the resolved
// item/path of a category on the BMC. Property names come from the keys of the
// actual Redfish JSON payload, so they match what a curl call would return.
func ListSettingsProperties(client *gofish.APIClient, out io.Writer, category, item string, path []string) error {
	resolved, err := ResolveListItem(client, category, item)
	if err != nil {
		return err
	}

	final, err := ResolveSettingsPath(resolved, path)
	if err != nil {
		return err
	}

	if obj, ok := final.(map[string]any); ok {
		_, _ = fmt.Fprintf(out, "Properties of %s.%s:", category, item)
		for _, name := range path {
			_, _ = fmt.Fprintf(out, ".%s", name)
		}
		_, _ = fmt.Fprintln(out)
		var keys []string
		for name := range obj {
			if strings.HasPrefix(name, "@") {
				continue
			}
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			_, _ = fmt.Fprintf(out, "  %s\n", name)
		}
		return nil
	}

	_, _ = fmt.Fprintf(out, "  %s is a %s; use 'magellan settings get' to read it\n", item, jsonTypeName(final))
	return nil
}

// jsonTypeName returns a readable name for a decoded JSON value.
func jsonTypeName(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
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
		value, ok := np[item]
		if !ok {
			return nil, fmt.Errorf("unknown protocol %q", item)
		}
		return value, nil
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
		for _, acct := range accts {
			if fmt.Sprint(acct["Id"]) == item {
				return acct, nil
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
		value, ok := np[item]
		if !ok {
			return nil, fmt.Errorf("unknown protocol %q", item)
		}
		return value, nil
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
		value, ok := sys[item]
		if !ok {
			return nil, fmt.Errorf("unknown property %q on ComputerSystem", item)
		}
		return value, nil
	case "Manager":
		mgr, err := GetDefaultManager(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get manager: %w", err)
		}
		value, ok := mgr[item]
		if !ok {
			return nil, fmt.Errorf("unknown property %q on Manager", item)
		}
		return value, nil
	case "Accounts":
		accts, err := ListAccounts(client)
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}
		for _, acct := range accts {
			if fmt.Sprint(acct["Id"]) == item {
				return acct, nil
			}
		}
		return nil, fmt.Errorf("account %q not found", item)
	default:
		return nil, fmt.Errorf("unknown category %q", category)
	}
}

// ResolveSettingsPath walks start down a sequence of Redfish JSON property
// names, returning the JSON value at the end of the path. Numeric path
// segments select array elements. Property names and values come directly from
// the BMC's payload.
func ResolveSettingsPath(start any, path []string) (any, error) {
	current := start
	for _, name := range path {
		switch node := current.(type) {
		case map[string]any:
			value, ok := node[name]
			if !ok {
				return nil, fmt.Errorf("unknown property %q", name)
			}
			current = value
		case []any:
			idx, err := strconv.Atoi(name)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("invalid index %q into array", name)
			}
			current = node[idx]
		default:
			return nil, fmt.Errorf("unknown property %q", name)
		}
	}
	return current, nil
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
