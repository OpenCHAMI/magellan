package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PathExists() is a wrapper function that simplifies checking
// if a file or directory already exists at the provided path.
//
// Returns whether the path exists and no error if successful,
// otherwise, it returns false with an error.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// SplitPathForViper() is an utility function to split a path into 3 parts:
// - directory
// - filename
// - extension
// The intent was to break a path into a format that's more easily consumable
// by spf13/viper's API. See the "LoadConfig()" function in internal/config.go
// for more details.
//
// TODO: Rename function to something more generalized.
func SplitPathForViper(path string) (string, string, string) {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return filepath.Dir(path), strings.TrimSuffix(filename, ext), strings.TrimPrefix(ext, ".")
}

// MakeOutputDirectory() creates a new directory at the path argument if
// the path does not exist
//
// TODO: Refactor this function for hive partitioning or possibly move into
// the logging package.
// TODO: Add an option to force overwriting the path.
func MakeOutputDirectory(path string) (string, error) {
	// get the current data + time using Go's stupid formatting
	t := time.Now()
	dirname := t.Format("2006-01-01 15:04:05")
	final := path + "/" + dirname

	// check if path is valid and directory
	pathExists, err := PathExists(final)
	if err != nil {
		return final, fmt.Errorf("failed to check for existing path: %v", err)
	}
	if pathExists {
		// make sure it is directory with 0o644 permissions
		return final, fmt.Errorf("found existing path: %v", final)
	}

	// create directory with data + time
	err = os.MkdirAll(final, 0766)
	if err != nil {
		return final, fmt.Errorf("failed to make directory: %v", err)
	}
	return final, nil
}
