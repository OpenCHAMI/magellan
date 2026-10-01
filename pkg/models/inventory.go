package models

type EthernetInterface struct {
	URI         string `json:"uri,omitempty"`         // URI of the interface
	MAC         string `json:"mac,omitempty"`         // MAC address of the interface
	IP          string `json:"ip,omitempty"`          // IP address of the interface
	Name        string `json:"name,omitempty"`        // Name of the interface
	Description string `json:"description,omitempty"` // Description of the interface
	Enabled     bool   `json:"enabled,omitempty"`     // Enabled interface
}

type NetworkAdapter struct {
	URI          string `json:"uri,omitempty"`          // URI of the adapter
	Manufacturer string `json:"manufacturer,omitempty"` // Manufacturer of the adapter
	Name         string `json:"name,omitempty"`         // Name of the adapter
	Model        string `json:"model,omitempty"`        // Model of the adapter
	Serial       string `json:"serial,omitempty"`       // Serial number of the adapter
	Description  string `json:"description,omitempty"`  // Description of the adapter
}

type NetworkInterface struct {
	URI         string         `json:"uri,omitempty"`         // URI of the interface
	Name        string         `json:"name,omitempty"`        // Name of the interface
	Description string         `json:"description,omitempty"` // Description of the interface
	Adapter     NetworkAdapter `json:"adapter,omitempty"`     // Adapter of the interface
}

type Manager struct {
	URI                    string              `json:"uri,omitempty"`
	UUID                   string              `json:"uuid,omitempty"`
	Name                   string              `json:"name,omitempty"`
	Description            string              `json:"description,omitempty"`
	Model                  string              `json:"model,omitempty"`
	Type                   string              `json:"type,omitempty"`
	FirmwareVersion        string              `json:"firmware_version,omitempty"`
	EthernetInterfaces     []EthernetInterface `json:"ethernet_interfaces,omitempty"`
	SerialConsoleSupported []string            `json:"serial_console"`
	CommandShellSupported  []string            `json:"command_shell"`
}

type Links struct {
	Chassis  []string `json:"chassis,omitempty"`
	Managers []string `json:"managers,omitempty"`
}

type Power struct {
	State           string   `json:"state,omitempty"`
	Mode            string   `json:"mode,omitempty"`
	RestorePolicy   string   `json:"restore_policy,omitempty"`
	PowerControlIDs []string `json:"power_control_ids,omitempty"`
}

type SerialConsoleConfig struct {
	Port    uint `json:"port,omitempty"`
	Enabled bool `json:"enabled,omitempty"`
}

type SerialConsole struct {
	IPMI   SerialConsoleConfig `json:"impi,omitempty"`
	Telnet SerialConsoleConfig `json:"telnet,omitempty"`
	SSH    SerialConsoleConfig `json:"ssh,omitempty"`
}

type InventoryDetail struct {
	URI                  string              `json:"uri,omitempty"`                  // URI of the BMC
	UUID                 string              `json:"uuid,omitempty"`                 // UUID of Node
	Manufacturer         string              `json:"manufacturer,omitempty"`         // Manufacturer of the Node
	SystemType           string              `json:"system_type,omitempty"`          // System type of the Node
	Name                 string              `json:"name,omitempty"`                 // Name of the Node
	ModelNumber          string              `json:"model,omitempty"`                // Model of the Node
	SerialNumber         string              `json:"serial,omitempty"`               // Serial number of the Node
	SerialConsole        SerialConsole       `json:"serial_console,omitempty"`       // Supported serial console types of the Node
	BiosVersion          string              `json:"bios_version,omitempty"`         // Version of the BIOS
	EthernetInterfaces   []EthernetInterface `json:"ethernet_interfaces,omitempty"`  // Ethernet interfaces of the Node
	NetworkInterfaces    []NetworkInterface  `json:"network_interfaces,omitempty"`   // Network interfaces of the Node
	Actions              []string            `json:"actions,omitempty"`              // Available actions for Node
	Power                Power               `json:"power,omitempty"`                // Power related settings of Node
	ProcessorCount       uint                `json:"processor_count,omitempty"`      // Processors of the Node
	ProcessorType        string              `json:"processor_type,omitempty"`       // Processor type of the Node
	MemoryTotal          float64             `json:"memory_total,omitempty"`         // Total memory of the Node in Gigabytes
	TrustedModules       []string            `json:"trusted_modules,omitempty"`      // Trusted modules of the Node
	TrustedComponents    []string            `json:"trusted_components,omitempty"`   // Trusted components of the Chassis
	Chassis_SKU          string              `json:"chassis_sku,omitempty"`          // SKU of the Chassis
	Chassis_Serial       string              `json:"chassis_serial,omitempty"`       // Serial number of the Chassis
	Chassis_AssetTag     string              `json:"chassis_asset_tag,omitempty"`    // Asset tag of the Chassis
	Chassis_Manufacturer string              `json:"chassis_manufacturer,omitempty"` // Manufacturer of the Chassis
	Chassis_Model        string              `json:"chassis_model,omitempty"`        // Model of the Chassis
	Links                Links               `json:"links,omitempty"`                // Links to specific resources
	NodeID               string              `json:"node_id,omitempty"`              // Node ID within the BMC, e.g. /redfish/v1/Systems/<ID>
}
