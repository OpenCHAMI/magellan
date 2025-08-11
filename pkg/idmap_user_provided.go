// Package magellan implements the core routines for the tools.
package magellan

import (
	"encoding/json"
	"fmt"
	"os"
	"github.com/rs/zerolog/log"
	"github.com/OpenCHAMI/magellan/internal/util"
	"gopkg.in/yaml.v3"
)

// bmcIDMap contains the mapping of host address strings to BMC Identifiers
// supplied by the --bmc-id-map option to collect. IDMap is the mapping itself,
// MapKey specifies what string to use as the key to the map. For now, that is
// always 'bmc-ip-addr'. In the future other options may be available.
type bmcIDMap struct {
	IDMap      map[string]string `json:"id_map" yaml:"id_map"`
	MapKey     string `json:"map_key" yaml:"map_key"`
}

type userProvidedMapper struct {
	IDMapStr string
	IDMapFormat string
	IDMap  *bmcIDMap
}

func getBMCIDMap(data string, format string)(*bmcIDMap, error) {
	// If no mapping is provided, there is no error, but there is
	// also no mapping, just return nil with no error and let the
	// caller pass that around.
	if data == "" {
		return nil, nil
	}

	var bmcIDMap bmcIDMap
	// First, check whether 'data' specifies a file (i.e. starts
	// with '@'). If not, it should be a JSON string containing the
	// map data. Otherwise, strip the '@' and fall through.
	if data[0] != '@' {
		err := json.Unmarshal([]byte(data), &bmcIDMap)
		if err != nil {
			return nil, err
		}
		return &bmcIDMap, nil
	}

	// The map data is in a file. Get the path from what comes
	// after the '@' and process it.
	path := data[1:]

	// Read in the contents of the map file, since we are going to
	// do that no matter what type it is...
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading BMC ID mapping file '%s': %v", path, err)
	}

	// Decode the file based on the appropriate format.
	switch util.DataFormatFromFileExt(path, format) {
	case util.FORMAT_JSON:
		// Read in JSON file
		err := json.Unmarshal(input, &bmcIDMap)
		if err != nil {
			return nil, err
		}
	case util.FORMAT_YAML:
		// Read in YAML file
		err := yaml.Unmarshal(input, &bmcIDMap)
		if err != nil {
			return nil, err
		}
	}
	return &bmcIDMap, nil
}


// Generate a BMC ID string associated with 'selector' in the provided
// 'bmcIDMap'. If there is no map, then return the selector string
// itself.  If the map is present but the host is not present in the
// map, then log a warning and return an empty string indicating that
// the BMC ID was not composed.
func getBMCID(bmcIDMap *bmcIDMap, selector string)(string) {
	if bmcIDMap == nil {
		return selector
	}
	// Go does not error out on string map references that do not
	// match the selector, it simply produces an empty
	// string. Recognize that case and log it, then return an
	// empty string.
	bmcID := bmcIDMap.IDMap[selector]
	if bmcID == "" {
		log.Warn().Msgf("no mapping found from host selector '%v' to a BMC ID", selector)
		return ""
	}
	return bmcID
}

func (mapper userProvidedMapper)initialize()(idMapper, error) {
	// Get the host to BMC ID mapping
	idMap, err := getBMCIDMap(mapper.IDMapStr, mapper.IDMapFormat)
	if err != nil {
		log.Error().Err(err).Str("User Specified BMC ID Map", mapper.IDMapStr).Msg("failed to decode user supplied BMC ID Mapping")
		return mapper, err
	}
	mapper.IDMap = idMap

	// For now, the only valid key is the IPv4 address, verify
	// that is what is in the mapping structure. If it is not,
	// return an error. When more map key options become available
	// add them here...
	switch mapper.IDMap.MapKey {
	case "bmc-ip-addr":
		break
	default:
		return mapper, fmt.Errorf("invalid 'map_key' field '%s' in BMC ID Map a valid value is 'bmc-ip-addr", mapper.IDMap.MapKey)
	}
	return mapper, nil
}

func (mapper userProvidedMapper)getMappedID(keys *idMapperKeys)(string) {
	// Get the map key. We already validated the key name in
	// initialize() so the default here should never happen. Log a
	// debug message if it does and return no ID.
	if mapper.IDMap == nil {
		// Somehow the IDMap isn't there. Must have failed
		// initialization. Log an error and return nil. There
		// was an error logged in initialize() that should
		// explain this.
		log.Error().Str("ID Mapper Keys", fmt.Sprintf("%#v", *keys)).Msg("BMC ID Mapping is missing, skipping BMC")
		return ""
	}
	var selector string
	switch mapper.IDMap.MapKey {
	case "bmc-ip-addr":
		selector = keys.IPv4Addr
	default:
		log.Debug().Msgf("failed to interpret map key name '%s' - THIS SHOULD NEVER HAPPEN!", mapper.IDMap.MapKey)
		return ""
	}
	return getBMCID(mapper.IDMap, selector)
}
