package util

import (
	"path/filepath"
)

const (
	FORMAT_LIST = "list"
	FORMAT_JSON = "json"
	FORMAT_YAML = "yaml"
)

func DataFormatFromFileExt(path string, defaultFmt string) string {
	// Figure out the type of the contents (JSON or YAML) based on
	// the filname extension. The default format is passed in, so
	// if it doesn't match one of the cases, that's what we will
	// use. The defaultFmt value takes into account both the
	// standard default format (JSON) and any command line change
	// to that provided by options.
	switch filepath.Ext(path) {
	case ".json", ".JSON":
		// The file is a JSON file
		return FORMAT_JSON
	case ".yaml", ".yml", ".YAML", ".YML":
		// The file is a YAML file
		return FORMAT_YAML
	}
	return defaultFmt
}
