package format

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type DataFormat string

const (
	FORMAT_LIST DataFormat = "list"
	FORMAT_JSON DataFormat = "json"
	FORMAT_YAML DataFormat = "yaml"
	FORMAT_DB   DataFormat = "db"
)

func (df DataFormat) String() string {
	return string(df)
}

func (df *DataFormat) Set(v string) error {
	switch DataFormat(v) {
	case FORMAT_LIST, FORMAT_JSON, FORMAT_YAML, FORMAT_DB:
		*df = DataFormat(v)
		return nil
	default:
		return fmt.Errorf("must be one of %v", []DataFormat{
			FORMAT_LIST, FORMAT_JSON, FORMAT_YAML, FORMAT_DB,
		})
	}
}

func (df DataFormat) Type() string {
	return "DataFormat"
}

// MarshalData marshals arbitrary data into a byte slice formatted as outFormat.
// If a marshalling error occurs or outFormat is unknown, an error is returned.
//
// Supported values are: json, list, yaml
func Marshal(data interface{}, outFormat DataFormat) ([]byte, error) {
	switch outFormat {
	case FORMAT_JSON:
		if bytes, err := json.MarshalIndent(data, "", "  "); err != nil {
			return nil, fmt.Errorf("failed to marshal data into JSON: %w", err)
		} else {
			return bytes, nil
		}
	case FORMAT_YAML:
		if bytes, err := yaml.Marshal(data); err != nil {
			return nil, fmt.Errorf("failed to marshal data into YAML: %w", err)
		} else {
			return bytes, nil
		}
	case FORMAT_LIST, FORMAT_DB:
		return nil, fmt.Errorf("this data format cannot be marshaled")
	default:
		return nil, fmt.Errorf("unknown data format: %s", outFormat)
	}
}

// UnmarshalData unmarshals a byte slice formatted as inFormat into an interface
// v. If an unmarshalling error occurs or inFormat is unknown, an error is
// returned.
//
// Supported values are: json, list, yaml
func Unmarshal(data []byte, v interface{}, inFormat DataFormat) error {
	switch inFormat {
	case FORMAT_JSON:
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to unmarshal data into JSON: %w", err)
		}
	case FORMAT_YAML:
		if err := yaml.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to unmarshal data into YAML: %w", err)
		}
	case FORMAT_LIST, FORMAT_DB:
		return fmt.Errorf("this data format cannot be unmarshaled")
	default:
		return fmt.Errorf("unknown data format: %s", inFormat)
	}

	return nil
}

func DataFormatFromFileExt(path string, defaultFmt DataFormat) DataFormat {
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
