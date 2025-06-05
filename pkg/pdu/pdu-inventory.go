package pdu

type PDUOutlet struct {
	ID         string `json:"id"`          // e.g., "35" or "BA35"
	Name       string `json:"name"`        // e.g., "Link1_Outlet_35"
	PowerState string `json:"power_state"` // e.g., "ON" or "OFF"
}

type PDUInventory struct {
	Hostname        string      `json:"hostname"`
	Model           string      `json:"model,omitempty"`
	SerialNumber    string      `json:"serial_number,omitempty"`
	FirmwareVersion string      `json:"firmware_version,omitempty"`
	Outlets         []PDUOutlet `json:"outlets"`
}
