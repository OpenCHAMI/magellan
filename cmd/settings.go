package cmd

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/crawler"
	"github.com/openchami/magellan/pkg/power"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/stmcginnis/gofish"
)

var (
	settingsFormat        format.DataFormat = format.FORMAT_JSON
	settingsInputFormat   format.DataFormat = format.FORMAT_YAML
	settingsInventoryFile string
	settingsCACertPath    string
)

// Category names used by settings subcommands.
var settingsCategories = map[string]string{
	"NetworkProtocol":   "Network service settings (SSH, HTTPS, IPMI, NTP, etc.)",
	"EthernetInterface": "Network interface settings (IP, MAC, DHCP, etc.)",
	"ComputerSystem":    "System-level settings (boot order, asset tag, etc.)",
	"Manager":           "Manager properties (firmware version, model, etc.)",
	"Accounts":          "BMC user accounts (username, role, etc.)",
	"Reset":             "Factory reset the BMC manager",
}

var SettingsCmd = &cobra.Command{
	Use: "settings",
	Example: `  # list all available setting categories
  magellan settings list

  # list properties in the NetworkProtocol category
  magellan settings list NetworkProtocol

  # get a specific network setting from a BMC
  magellan settings get 172.16.0.105 NetworkProtocol SSH

  # set a network setting
  magellan settings set 172.16.0.105 NetworkProtocol SSH '{"ProtocolEnabled":true,"Port":22}'

  # get the first ethernet interface's IP settings
  magellan settings get 172.16.0.105 EthernetInterface 0

  # factory reset the BMC manager
  magellan settings reset 172.16.0.105

  # factory reset preserving network settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetwork`,
	Short: "Configure BMC properties through Redfish",
	Long: `Configure BMC properties through Redfish.

Supported categories:
  NetworkProtocol    SSH, HTTPS, IPMI, NTP, SNMP, and other network services
  EthernetInterface  IP addresses, MAC addresses, DHCP settings
  ComputerSystem     Boot order, asset tag, and other system-level settings
  Manager            Firmware version, model, and other manager properties
  Accounts           BMC user accounts, roles, and access control

See 'magellan-settings(1)' for more details. See 'magellan(1)' for a list of
available environment variables.
`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			log.Error().Err(err).Msg("failed to print help")
		}
	},
}

var SettingsListCmd = &cobra.Command{
	Use:   "list [category]",
	Short: "List available setting categories or properties",
	Long: `List available setting categories or properties within a category.

When called without arguments, lists all available categories.
When called with a category name, lists the properties available in that category.`,
	Example: `  # list all available categories
  magellan settings list

  # list properties in NetworkProtocol
  magellan settings list NetworkProtocol`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Available setting categories:")
			fmt.Fprintln(cmd.OutOrStdout())
			for name, desc := range settingsCategories {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s\n", name, desc)
			}
			return nil
		}
		category := args[0]
		if _, ok := settingsCategories[category]; !ok {
			return fmt.Errorf("unknown category %q; use 'magellan settings list' to see available categories", category)
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Category: %s\n", category)
		fmt.Fprintf(out, "Description: %s\n\n", settingsCategories[category])
		switch category {
		case "NetworkProtocol":
			fmt.Fprintln(out, "  DHCP        DHCPv4 protocol settings")
			fmt.Fprintln(out, "  DHCPv6      DHCPv6 protocol settings")
			fmt.Fprintln(out, "  FQDN        Fully qualified domain name")
			fmt.Fprintln(out, "  FTP         File Transfer Protocol settings")
			fmt.Fprintln(out, "  FTPS        FTP over SSL settings")
			fmt.Fprintln(out, "  HTTP        HTTP protocol settings")
			fmt.Fprintln(out, "  HTTPS       HTTPS/SSL protocol settings")
			fmt.Fprintln(out, "  HostName    Host name without domain")
			fmt.Fprintln(out, "  IPMI        IPMI over LAN settings")
			fmt.Fprintln(out, "  KVMIP       KVM-IP settings")
			fmt.Fprintln(out, "  NTP         NTP protocol settings")
			fmt.Fprintln(out, "  Proxy       HTTP/HTTPS proxy configuration")
			fmt.Fprintln(out, "  RDP         Remote Desktop Protocol settings")
			fmt.Fprintln(out, "  RFB         Remote Frame Buffer settings")
			fmt.Fprintln(out, "  SFTP        SFTP settings")
			fmt.Fprintln(out, "  SNMP        SNMP settings")
			fmt.Fprintln(out, "  SSDP        SSDP settings")
			fmt.Fprintln(out, "  SSH         Secure Shell settings")
			fmt.Fprintln(out, "  Telnet      Telnet settings")
			fmt.Fprintln(out, "  VirtualMedia Virtual Media settings")
			fmt.Fprintln(out, "  MDNS        Multicast DNS settings")
		case "EthernetInterface":
			fmt.Fprintln(out, "  0           First ethernet interface")
			fmt.Fprintln(out, "  1           Second ethernet interface (if present)")
			fmt.Fprintln(out, "  ...         Additional interfaces")
		case "ComputerSystem":
			fmt.Fprintln(out, "  Boot        Boot settings and order")
			fmt.Fprintln(out, "  BiosVersion BIOS version string")
			fmt.Fprintln(out, "  AssetTag    Asset tag identifier")
			fmt.Fprintln(out, "  Manufacturer Manufacturer name")
			fmt.Fprintln(out, "  Model       System model")
			fmt.Fprintln(out, "  SerialNumber Serial number")
			fmt.Fprintln(out, "  UUID        System UUID")
		case "Manager":
			fmt.Fprintln(out, "  ID          Manager identifier")
			fmt.Fprintln(out, "  Name        Manager name")
			fmt.Fprintln(out, "  FirmwareVersion Firmware version")
			fmt.Fprintln(out, "  ManagerType Manager type")
			fmt.Fprintln(out, "  Model       Manager model")
			fmt.Fprintln(out, "  SerialNumber Serial number")
		case "Accounts":
			fmt.Fprintln(out, "  (use 'get' to query specific accounts)")
		case "Reset":
			fmt.Fprintln(out, "  ResetType   ResetAll, PreserveNetwork, or PreserveNetworkAndUsers")
		}
		return nil
	},
}

var SettingsGetCmd = &cobra.Command{
	Use:   "get <node> <category> [property]",
	Short: "Get a BMC setting value",
	Long: `Get a BMC setting value by category and property.

For NetworkProtocol, specify the protocol name (e.g., SSH, HTTPS, IPMI, NTP).
For EthernetInterface, specify the interface index (0, 1, ...).
For ComputerSystem, specify the property name.
For Manager, specify the property name.
For Accounts, specify the account ID.`,
	Example: `  # get SSH protocol settings
  magellan settings get 172.16.0.105 NetworkProtocol SSH

  # get the first ethernet interface
  magellan settings get 172.16.0.105 EthernetInterface 0

  # get boot settings
  magellan settings get 172.16.0.105 ComputerSystem Boot

  # get all accounts
  magellan settings get 172.16.0.105 Accounts

  # get a specific account
  magellan settings get 172.16.0.105 Accounts 1`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeArg := args[0]
		category := args[1]

		client, err := settingsConnect(nodeArg)
		if err != nil {
			return fmt.Errorf("failed to connect to BMC for node %q: %w", nodeArg, err)
		}
		defer client.Logout()

		var result any
		switch category {
		case "NetworkProtocol":
			np, err := bmc.GetNetworkProtocol(client)
			if err != nil {
				return fmt.Errorf("failed to get network protocol: %w", err)
			}
			if len(args) > 2 {
				proto := args[2]
				field, ok := settingsField(np, proto)
				if !ok {
					return fmt.Errorf("unknown protocol %q", proto)
				}
				result = field.Interface()
			} else {
				result = np
			}
		case "EthernetInterface":
			ifaces, err := bmc.GetEthernetInterfaces(client)
			if err != nil {
				return fmt.Errorf("failed to get ethernet interfaces: %w", err)
			}
			if len(args) > 2 {
				idx := 0
				if _, err := fmt.Sscanf(args[2], "%d", &idx); err != nil {
					return fmt.Errorf("invalid interface index %q: %w", args[2], err)
				}
				if idx < 0 || idx >= len(ifaces) {
					return fmt.Errorf("interface index %d out of range (0-%d)", idx, len(ifaces)-1)
				}
				result = ifaces[idx]
			} else {
				result = ifaces
			}
		case "ComputerSystem":
			sys, err := bmc.GetDefaultComputerSystem(client)
			if err != nil {
				return fmt.Errorf("failed to get computer system: %w", err)
			}
			if len(args) > 2 {
				field, ok := settingsField(sys, args[2])
				if !ok {
					return fmt.Errorf("unknown property %q on ComputerSystem", args[2])
				}
				result = field.Interface()
			} else {
				result = sys
			}
		case "Manager":
			mgr, err := bmc.GetDefaultManager(client)
			if err != nil {
				return fmt.Errorf("failed to get manager: %w", err)
			}
			if len(args) > 2 {
				field, ok := settingsField(mgr, args[2])
				if !ok {
					return fmt.Errorf("unknown property %q on Manager", args[2])
				}
				result = field.Interface()
			} else {
				result = mgr
			}
		case "Accounts":
			accts, err := bmc.ListAccounts(client)
			if err != nil {
				return fmt.Errorf("failed to list accounts: %w", err)
			}
			if len(args) > 2 {
				found := false
				for _, acct := range accts {
					if acct.ID == args[2] {
						result = acct
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("account %q not found", args[2])
				}
			} else {
				result = accts
			}
		default:
			return fmt.Errorf("unknown category %q; use 'magellan settings list' to see available categories", category)
		}

		output, err := format.MarshalData(result, settingsFormat)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
		return nil
	},
}

var SettingsSetCmd = &cobra.Command{
	Use:   "set <node> <category> <property> <value>",
	Short: "Set a BMC setting value",
	Long: `Set a BMC setting value by category, property, and value.

The value should be a JSON string for complex types or a simple string for
scalar values.`,
	Example: `  # enable SSH on the BMC
  magellan settings set 172.16.0.105 NetworkProtocol SSH '{"ProtocolEnabled":true,"Port":22}'

  # update the first ethernet interface IP
  magellan settings set 172.16.0.105 EthernetInterface 0 '{"IPv4Addresses":[{"Address":"172.16.0.105","SubnetMask":"255.255.255.0","Gateway":"172.16.0.1"}]}'`,
	Args: cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeArg := args[0]
		category := args[1]
		property := args[2]
		value := args[3]

		client, err := settingsConnect(nodeArg)
		if err != nil {
			return fmt.Errorf("failed to connect to BMC for node %q: %w", nodeArg, err)
		}
		defer client.Logout()

		switch category {
		case "NetworkProtocol":
			if err := bmc.SetNetworkProtocol(client, property, value); err != nil {
				return fmt.Errorf("failed to set NetworkProtocol.%s: %w", property, err)
			}
		case "EthernetInterface":
			idx := 0
			if _, err := fmt.Sscanf(property, "%d", &idx); err != nil {
				return fmt.Errorf("invalid interface index %q: %w", property, err)
			}
			if err := bmc.SetEthernetInterface(client, idx, value); err != nil {
				return fmt.Errorf("failed to set EthernetInterface[%d]: %w", idx, err)
			}
		case "ComputerSystem":
			if err := bmc.SetComputerSystemProperty(client, property, value); err != nil {
				return fmt.Errorf("failed to set ComputerSystem.%s: %w", property, err)
			}
		case "Manager":
			if err := bmc.SetManagerProperty(client, property, value); err != nil {
				return fmt.Errorf("failed to set Manager.%s: %w", property, err)
			}
		case "Accounts":
			if err := bmc.UpdateAccount(client, property, value); err != nil {
				return fmt.Errorf("failed to update account %s: %w", property, err)
			}
		default:
			return fmt.Errorf("unknown category %q; use 'magellan settings list' to see available categories", category)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully set %s.%s\n", category, property)
		return nil
	},
}

var settingsPreserveConfig string

var SettingsResetCmd = &cobra.Command{
	Use:   "reset <node>",
	Short: "Factory reset the BMC manager",
	Long: `Factory reset the BMC manager.

By default, this resets all settings. Use --preserve-config to preserve
specific settings during the reset.`,
	Example: `  # factory reset all settings
  magellan settings reset 172.16.0.105

  # factory reset but preserve network settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetwork

  # factory reset but preserve network and user settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetworkAndUsers`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeArg := args[0]

		client, err := settingsConnect(nodeArg)
		if err != nil {
			return fmt.Errorf("failed to connect to BMC for node %q: %w", nodeArg, err)
		}
		defer client.Logout()

		if err := bmc.ResetManager(client, settingsPreserveConfig); err != nil {
			return fmt.Errorf("failed to reset manager: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Manager reset initiated (preserve-config: %s)\n", settingsPreserveConfig)
		return nil
	},
}

// settingsConnect resolves a node argument to a BMC connection. When an
// inventory file is provided, nodeArg is looked up by ClusterID or NodeID;
// otherwise nodeArg is treated as a direct IP address or hostname.
func settingsConnect(nodeArg string) (*gofish.APIClient, error) {
	address := nodeArg
	if settingsInventoryFile != "" {
		nodes, err := power.ParseInventory(settingsInventoryFile, settingsInputFormat)
		if err != nil {
			return nil, fmt.Errorf("failed to parse inventory file %s: %w", settingsInventoryFile, err)
		}

		var found *bmc.Node
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

	endpoint, err := settingsEndpoint(address)
	if err != nil {
		return nil, err
	}
	store, err := settingsCredentialStore()
	if err != nil {
		return nil, err
	}
	return crawler.GetBMCClient(crawler.CrawlerConfig{
		URI:             endpoint,
		CredentialStore: store,
		Insecure:        insecure,
		CACertPath:      settingsCACertPath,
	})
}

func settingsCredentialStore() (secrets.SecretStore, error) {
	if username != "" && password != "" {
		return secrets.NewStaticStore(username, password), nil
	}
	if secretsFile == "" {
		return nil, fmt.Errorf("BMC credentials are required; use --username/--password or --secrets-file")
	}
	store, err := secrets.OpenStore(secretsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open secrets file: %w", err)
	}
	return store, nil
}

func settingsEndpoint(address string) (string, error) {
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

func settingsField(resource any, name string) (reflect.Value, bool) {
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

func init() {
	SettingsGetCmd.Flags().VarP(&settingsFormat, "output-format", "F", "Set the output format (json|yaml).")
	SettingsResetCmd.Flags().StringVar(&settingsPreserveConfig, "preserve-config", "", "Preserve settings during reset (PreserveNetwork|PreserveNetworkAndUsers).")

	// Common flags for commands that connect to BMC
	for _, c := range []*cobra.Command{SettingsGetCmd, SettingsSetCmd, SettingsResetCmd} {
		c.Flags().StringVarP(&settingsInventoryFile, "inventory-file", "f", "", "File containing node inventory.")
		c.Flags().Var(&settingsInputFormat, "input-format", "Set the inventory input format (json|yaml).")
		c.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username.")
		c.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password.")
		c.Flags().StringVar(&secretsFile, "secrets-file", "secrets.json", "Set the secrets file with BMC credentials.")
		c.Flags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification during probe.")
		c.Flags().StringVar(&settingsCACertPath, "cacert", "", "Set the path to CA cert file (defaults to system CAs when blank).")
	}

	SettingsCmd.AddCommand(SettingsListCmd)
	SettingsCmd.AddCommand(SettingsGetCmd)
	SettingsCmd.AddCommand(SettingsSetCmd)
	SettingsCmd.AddCommand(SettingsResetCmd)

	rootCmd.AddCommand(SettingsCmd)
}
