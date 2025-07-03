package daemon

import (
	"fmt"

	"github.com/stmcginnis/gofish/redfish"
)

func OutputToSMD(host string, power redfish.PowerSubsystem) {
	OutputToStdout(host, power)  // FIXME:
}

func OutputToStdout(host string, power redfish.PowerSubsystem) {
	fmt.Printf("%s has PowerSubsystem: %v", host, power)
}
