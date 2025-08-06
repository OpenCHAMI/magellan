// Package magellan implements the core routines for the tools.
package magellan

import (
	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/rs/zerolog/log"
	"github.com/Cray-HPE/hms-xname/xnames"
)

// An XNAME can address up to 10,000 Cabinets (0 - 9,999). I chose 8K
// (8192) as the number I would map into. That makes 512K BMCs per
// cabinet distributed across 8 chassis per cabinet (a total of 64K
// chassis) which leaves 64K BMCs per chassis. Splitting this down the
// middle leaves us 256 shelves (ComputeModules) per chassis (a total
// of 16M shelvs) and 256 BMCs per shelf (4G total BMCs) wich
// completely consumes the 32 bit address space of an IPv4 address.
//
// It is more intuitive to do the math to decompose the IP address
// with shifting and masking than with division and modulo, so I
// define shifts and masks here...
const (
	cabinetShift int = 19
	cabinetMask int = 0x1FFF
	chassisShift int = 16
	chassisMask int = 0x7
	shelfShift int = 24
	shelfMask int = 0xFF
	bmcShift int = 0
	bmcMask = 0xff
)

type generatedXNAMEMapper struct {
	// This mapper is purely computational, it has no state.
}

func ipAddrIntToXname(ipInt int)(string) {
	// Compute the fields for the XNAME structure from the integer
	// IP address
	return xnames.NodeBMC {
		Cabinet: ((ipInt >> cabinetShift) & cabinetMask),
		Chassis: ((ipInt >> chassisShift) & chassisMask),
		ComputeModule: ((ipInt >> shelfShift) & shelfMask),
		NodeBMC: ((ipInt >> bmcShift) & bmcMask),
	}.String()
}

func (mapper generatedXNAMEMapper)initialize()(idMapper, error) {
	// Nothing to do here, everything done in this mapper is algorithmic
	// based on the IPv4 address.
	return mapper, nil
}

func (mapper generatedXNAMEMapper)getMappedID(keys *idMapperKeys)(string) {
	ipAddrInt, err := util.IPAddrStrToInt(keys.IPv4Addr)
	if err !=  nil {
		log.Error().Err(err).Str("IPv4 address", keys.IPv4Addr).Msg("failed to generate XNAME from IP address")
		// Failed to translate the IP into an XNAME for some
		// reason. The logic for this is to return an empty
		// string as the ID which will make the caller skip
		// this BMC.
		return ""
	}
	return ipAddrIntToXname(ipAddrInt)
}
