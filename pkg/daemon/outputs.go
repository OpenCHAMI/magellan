package daemon

import (
	"fmt"
)

func OutputToSMD(power PowerInfo) {
	OutputToStdout(power)  // FIXME:
}

func OutputToStdout(power PowerInfo) {
	fmt.Printf("%s has PowerState %s\n", power.Xname, power.State)
}
