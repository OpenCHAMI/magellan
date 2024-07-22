package magellan

import (
	"fmt"

	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/spf13/viper"
)

func LoadConfig(path string) error {
	dir, filename, ext := util.SplitPathForViper(path)
	// fmt.Printf("dir: %s\nfilename: %s\nextension: %s\n", dir, filename, ext)
	viper.AddConfigPath(dir)
	viper.SetConfigName(filename)
	viper.SetConfigType(ext)
	// ...no search paths set intentionally, so config has to be set explicitly
	// ...also, the config file will not save anything
	// ...and finally, parameters passed to CLI have precedence over config values
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
