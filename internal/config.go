package magellan

import (
	"fmt"

	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/spf13/viper"
)

// LoadConfig() will load a YAML config file at the specified path. There are some general
// considerations about how this is done with spf13/viper:
//
// 1. There are intentionally no search paths set, so config path has to be set explicitly
// 2. No data will be written to the config file from the tool
// 3. Parameters passed as CLI flags and envirnoment variables should always have
// precedence over values set in the config.
func LoadConfig(path string) error {
	dir, filename, ext := util.SplitPathForViper(path)
	// fmt.Printf("dir: %s\nfilename: %s\nextension: %s\n", dir, filename, ext)
	viper.AddConfigPath(dir)
	viper.SetConfigName(filename)
	viper.SetConfigType(ext)
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return fmt.Errorf("config file not found: %w", err)
		} else {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	return nil
}
