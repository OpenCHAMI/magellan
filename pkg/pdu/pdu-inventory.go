package pdu

type PDUOutlet struct {
	ID         string `json:"id"`          // e.g., "35" or "BA35"
	Name       string `json:"name"`        // e.g., "Link1_Outlet_35"
	PowerState string `json:"power_state"` // e.g., "ON" or "OFF"
	SocketType string `json:"socket_type"`
}

type PDUInventory struct {
	Hostname string      `json:"hostname"`
	Outlets  []PDUOutlet `json:"outlets"`
}
