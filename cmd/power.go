package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OpenCHAMI/magellan/internal/format"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/power"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stmcginnis/gofish/schemas"
)

var (
	list_reset_types bool
	reset_type       string
	operation        string
	powerFormat      format.DataFormat = format.FORMAT_JSON
)

// The `power` command gets and sets power states for a single BMC node using flexible identifiers.
var PowerCmd = &cobra.Command{
	Use: "power <identifier>",
	Example: `  // Power control by IP address (no inventory file needed)
  magellan power 10.0.0.101 -u admin -p password
  magellan power 10.0.0.101 -o off
	magellan power 192.168.1.100 --list-reset-types

  // Power control by UUID (from inventory file)
	magellan power 3894755a-8e4c-41d6-a6eb-3c5f4b7d2e10 -o on -f nodes.json

  // Power control by serial number (from inventory file)
	magellan power CN75120A3G -o hard-restart -f nodes.json

  // Power control by MAC address (from inventory file)
  magellan power aa:bb:cc:dd:ee:ff -o soft-off -f nodes.json
	magellan power aa-bb-cc-dd-ee-ff -o off -f nodes.json

  // Power control by xname (from inventory file)
  magellan power x1000c0s0b3n0 -o off -f nodes.json
  magellan power x5506c0s172b105n1 --list-reset-types -f nodes.json`,
	Short: "Get and set node power states using flexible identifiers",
	Long: `Control power states of individual nodes through their BMC using natural identifiers.

Supported identifier types (auto-detected):
  - IP address: Connect directly to BMC (no inventory file needed)
  - UUID: Redfish system UUID from inventory
	- Serial number: System serial number from inventory
  - MAC address: Any NIC MAC address from inventory
  - XName: Cray-format cluster identifier from inventory

For IP addresses, magellan connects directly without requiring a pre-collected inventory.
For other identifiers, an inventory file from 'magellan collect' is required.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		// Validate: exactly one node identifier
		if len(args) == 0 {
			log.Fatal().Msg("exactly one node identifier required. Usage: magellan power <identifier> [flags]")
		}
		if len(args) > 1 {
			log.Fatal().Msgf("multiple nodes not supported - specify one identifier at a time. Received %d identifiers: %v", len(args), args)
		}

		identifier := args[0]

		// Validate operation flag value
		if operation != "" && !bmc.KnownOperation(bmc.Operation(operation)) {
			log.Fatal().Msgf("unknown power operation %q (known operations: %v)", operation, bmc.Operations())
		}

		// Setup credentials (from flags or secrets file)
		store := setupCredentialStore()

		// Detect identifier type
		identifierType := bmc.DetectIdentifierType(identifier)
		log.Debug().Msgf("detected identifier type: %s", identifierType)

		// PATH 1: Direct IP connection (no inventory file needed)
		if identifierType == bmc.IdentifierIP {
			err := handleDirectIPConnection(ctx, identifier, store)
			if err != nil {
				log.Fatal().Err(err).Msg("direct IP connection failed")
			}
			return
		}

		// PATH 2: Inventory-based lookup (for UUID, Serial, MAC, XName)
		log.Info().Msg("non-IP identifier detected, loading inventory for lookup")

		// Load inventory
		nodes := loadInventory()

		// Find node by identifier
		node, err := findNodeByIdentifier(nodes, identifier)
		if err != nil {
			log.Fatal().Err(err).Msg("node lookup failed")
		}

		log.Debug().Msgf("found node in inventory: ClusterID=%s, BmcIP=%s, NodeID=%s", node.ClusterID, node.BmcIP, node.NodeID)

		// Connect to the node's BMC
		config := crawler.CrawlerConfig{
			URI:             "https://" + node.BmcIP,
			CredentialStore: store,
			Insecure:        insecure,
		}

		client, err := bmc.DefaultManager.Client(ctx, config)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to connect to BMC at %s for node %s", node.BmcIP, identifier)
		}
		defer client.Logout()

		// Perform power action
		err = performPowerAction(ctx, client, node.NodeID, identifier)
		if err != nil {
			log.Fatal().Err(err).Msgf("power action failed for node %s", identifier)
		}
	},
}

// setupCredentialStore creates and configures the credential store from CLI flags or secrets file.
// It handles username/password overrides and returns a configured SecretStore.
func setupCredentialStore() secrets.SecretStore {
	var store secrets.SecretStore
	var err error

	if username != "" && password != "" {
		log.Debug().Msg("using credentials from --username and --password flags")
		store = secrets.NewStaticStore(username, password)
	} else {
		log.Debug().Msgf("loading credentials from secret store at %s", secretsFile)
		if store, err = secrets.OpenStore(secretsFile); err != nil {
			log.Error().Err(err).Msg("failed to open local secrets store")
		}

		// Override username/password if either flag is set
		if username != "" {
			log.Info().Msg("--username passed, overriding all usernames from secret store")
		}
		if password != "" {
			log.Info().Msg("--password passed, overriding all passwords from secret store")
		}
		switch s := store.(type) {
		case *secrets.StaticStore:
			if username != "" {
				s.Username = username
			}
			if password != "" {
				s.Password = password
			}
		case *secrets.LocalSecretStore:
			for k := range s.Secrets {
				if creds, err := bmc.GetBMCCredentials(store, k); err != nil {
					log.Error().Str("id", k).Err(err).Msg("failed to override BMC credentials")
				} else {
					if username != "" {
						creds.Username = username
					}
					if password != "" {
						creds.Password = password
					}
					if newCreds, err := json.Marshal(creds); err != nil {
						log.Error().Str("id", k).Err(err).Msg("failed to marshal updated credentials")
					} else {
						err = s.StoreSecretByID(k, string(newCreds))
						if err != nil {
							log.Error().Err(err).Str("id", k).Msg("failed to store secret by ID")
						}
					}
				}
			}
		}
	}

	return store
}

// loadInventory reads and parses the node inventory from the configured file or default location.
func loadInventory() []bmc.Node {
	var datafile string
	if viper.IsSet("inventory-file") {
		datafile = viper.GetString("inventory-file")
	} else {
		datafile = viper.GetString("collect.output-file")
		log.Info().Msgf("using default inventory file from 'collect' command: %s", datafile)
	}

	nodes, err := power.ParseInventory(datafile, powerFormat)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to parse inventory file %s. Hint: for IP-based operations, no inventory file is needed; for other identifiers, run 'magellan collect' first", datafile)
	}

	return nodes
}

// findNodeByIdentifier searches the inventory for a node matching the given identifier.
// It automatically detects the identifier type and searches the appropriate field(s).
func findNodeByIdentifier(nodes []bmc.Node, identifier string) (*bmc.Node, error) {
	identifierType := bmc.DetectIdentifierType(identifier)

	switch identifierType {
	case bmc.IdentifierXName:
		for i := range nodes {
			if nodes[i].ClusterID == identifier {
				return &nodes[i], nil
			}
		}

	case bmc.IdentifierIP:
		for i := range nodes {
			if nodes[i].BmcIP == identifier {
				return &nodes[i], nil
			}
		}
	case bmc.IdentifierUUID:
		for i := range nodes {
			if strings.EqualFold(nodes[i].UUID, identifier) {
				return &nodes[i], nil
			}
		}

	case bmc.IdentifierSerial:
		for i := range nodes {
			if strings.EqualFold(nodes[i].SerialNumber, identifier) {
				return &nodes[i], nil
			}
		}

	case bmc.IdentifierMAC:
		// Normalize both input and stored MACs (lowercase, colon separator)
		normalizedInput := strings.ToLower(strings.ReplaceAll(identifier, "-", ":"))
		for i := range nodes {
			for _, mac := range nodes[i].MACAddresses {
				normalizedMAC := strings.ToLower(strings.ReplaceAll(mac, "-", ":"))
				if normalizedMAC == normalizedInput {
					return &nodes[i], nil
				}
			}
		}

	case bmc.IdentifierUnknown:
		return nil, fmt.Errorf("unable to determine identifier type for %q", identifier)
	}

	return nil, fmt.Errorf("node with %s %q not found in inventory. Try: run 'magellan collect' to refresh inventory, or use the BMC IP address directly", identifierType, identifier)
}

// handleDirectIPConnection connects directly to a BMC by IP address without requiring an inventory file.
// This enables quick single-node operations: magellan power 10.0.0.101 -o off
func handleDirectIPConnection(ctx context.Context, ipAddr string, store secrets.SecretStore) error {
	log.Info().Msgf("connecting directly to BMC at %s (no inventory file required)", ipAddr)

	// Build connection config
	config := crawler.CrawlerConfig{
		URI:             "https://" + ipAddr,
		CredentialStore: store,
		Insecure:        insecure,
	}

	// Connect to BMC
	client, err := bmc.DefaultManager.Client(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to connect to BMC at %s: %w. Check: IP address is correct, credentials are valid, network connectivity is working", ipAddr, err)
	}
	defer client.Logout()

	// Get systems from BMC
	systems, err := client.Gofish().GetService().Systems()
	if err != nil {
		return fmt.Errorf("failed to get computer systems from BMC: %w", err)
	}
	if len(systems) == 0 {
		return fmt.Errorf("no computer systems found on BMC at %s", ipAddr)
	}

	// Use first system (simple approach)
	systemID := systems[0].ID
	if len(systems) > 1 {
		log.Info().Msgf("BMC has %d computer systems, using first system: %s", len(systems), systemID)
	}

	// Perform the requested power action
	return performPowerAction(ctx, client, systemID, ipAddr)
}

// performPowerAction executes the requested power operation (or query) on a computer system.
// This consolidates the action logic used by both direct IP and inventory-based paths.
func performPowerAction(ctx context.Context, client bmc.Client, systemID string, displayName string) error {
	// Action: List supported reset types
	if list_reset_types {
		types, err := client.GetResetTypes(ctx, systemID)
		if err != nil {
			return fmt.Errorf("failed to get reset types: %w", err)
		}
		fmt.Printf("%s: supported reset types: %v\n", displayName, types)
		return nil
	}

	// Action: Vendor-neutral operation
	if operation != "" {
		_, err := client.ResetOperation(ctx, systemID, bmc.Operation(operation))
		if err != nil {
			return fmt.Errorf("failed to perform operation %q: %w", operation, err)
		}
		fmt.Printf("%s: operation %q completed successfully\n", displayName, operation)
		return nil
	}

	// Action: Raw Redfish reset type
	if reset_type != "" {
		_, err := client.Reset(ctx, systemID, schemas.ResetType(reset_type))
		if err != nil {
			return fmt.Errorf("failed to perform reset type %q: %w", reset_type, err)
		}
		fmt.Printf("%s: reset type %q completed successfully\n", displayName, reset_type)
		return nil
	}

	// Default action: Query power state
	state, err := client.GetPowerState(ctx, systemID)
	if err != nil {
		return fmt.Errorf("failed to get power state: %w", err)
	}
	fmt.Printf("%s: %s\n", displayName, state)
	return nil
}

func init() {
	// Alternative actions from the default power-state query.
	// NOTE: no "-l" shorthand here — it is reserved globally for --log-level on
	// the root command (cmd/root.go). Defining it again panics pflag at execution
	// time when the persistent flags are merged into this subcommand's flagset.
	PowerCmd.Flags().BoolVar(&list_reset_types, "list-reset-types", false, "List supported Redfish reset types")
	PowerCmd.Flags().StringVarP(&reset_type, "reset-type", "r", "", "Raw Redfish reset type to perform (no validation/fallback; prefer --operation)")
	PowerCmd.Flags().StringVarP(&operation, "operation", "o", "", "Vendor-neutral power operation (on|off|soft-off|force-off|soft-restart|hard-restart|init)")
	PowerCmd.MarkFlagsMutuallyExclusive("reset-type", "list-reset-types", "operation")

	// Normal config options
	PowerCmd.Flags().StringP("inventory-file", "f", "", "YAML file containing node inventory")
	PowerCmd.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username")
	PowerCmd.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password")
	PowerCmd.Flags().String("secrets-file", "", "Set path to the node secrets file")
	PowerCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL errors")
	PowerCmd.Flags().String("cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")
	PowerCmd.Flags().VarP(&powerFormat, "format", "F", "Set the output format (json|yaml)")

	checkRegisterFlagCompletionError(PowerCmd.RegisterFlagCompletionFunc("format", completionFormatData))

	// Bind flags to config properties
	checkBindFlagError(viper.BindPFlag("power.cacert", PowerCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlag("power.format", PowerCmd.Flags().Lookup("format")))
	checkBindFlagError(viper.BindPFlags(PowerCmd.Flags()))

	rootCmd.AddCommand(PowerCmd)
}
