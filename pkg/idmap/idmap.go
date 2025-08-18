// Package magellan implements the core routines for the tools.
package idmap

import (
	"github.com/OpenCHAMI/magellan/internal/format"
	"github.com/rs/zerolog/log"
)

// This file is the top-level of the BMC ID Mapping infrastructure. It
// defines the interface into all BMC ID Mappers, the structure of the
// parameters for ID Mapping, and a function for picking a BMC ID
// Mapper to use.
//
// Implementations of specific ID mappers are found in files with the
// prefix 'idmap_' in this directory. To extend the BMC ID Mapping
// capability, create a new mapper in its own file, and then add the
// mapper selection logic for your new mapper to pickIDMapper() here.

// The structure passed into the GetMappedID() function as the key
// options for a mapper. Currently only contains the IPv4 address of
// the BMC in question.
type MapperKeys struct {
	IPv4Addr string
}

type Mapper interface {
	Initialize() (Mapper, error)
	GetMappedID(keys *MapperKeys) string
}

// Select the correct BMC ID Mapper based on the parameters to
// 'collect'.
// func PickIDMapper(params *magellan.CollectParams) idMapper {
func PickIDMapper(bmcIDMap string, idMapFormat format.DataFormat) Mapper {
	// If the parameters contain a BMC ID Map (user defined
	// mapping of a key to a BMC ID) then we use the userProvidedMapper
	// implementaiton of an ID Mapper. Until other BMC ID schemes
	// are implemented, the other case is simply to use the
	// generated XNAME mapper, generatedXNAMEMapper.
	var (
		mapper     Mapper
		mapperName string
		err        error
	)

	if bmcIDMap != "" {
		// Always use the user provided mapper if a user
		// provided map is present.
		mapperName = "userProvidedMapper"
		mapper = userProvidedMapper{
			IDMapStr:    bmcIDMap,
			IDMapFormat: idMapFormat,
		}
	} else {
		// Currently the only other mapper is the generated
		// XNAMEs mapper, since no user provided mapper was
		// offered, use that instead.
		mapperName = "generatedXNAMEMapper"
		mapper = generatedXNAMEMapper{}
	}
	mapper, err = mapper.Initialize()
	if err != nil {
		log.Error().Err(err).Str("Mapper Name", mapperName).Msg("failed to initialized BMC ID Mapper")
	}
	return mapper
}
