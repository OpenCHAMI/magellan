package util

import (
	"fmt"
	"io/fs"
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
func PathExists(path string) (fs.FileInfo, bool) {
	fi, err := os.Stat(path)
	return fi, !os.IsNotExist(err)
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
// the path does not exist.
//
// Returns the final path that was created if no errors occurred. Otherwise,
// it returns an empty string with an error.
func MakeOutputDirectory(path string, overwrite bool) (string, error) {
	// get the current data + time using Go's stupid formatting
	t := time.Now()
	dirname := t.Format("2006-01-01")
	final := path + "/" + dirname

	// check if path is valid and directory
	_, pathExists := PathExists(final)
	if pathExists && !overwrite {
		// make sure it is directory with 0o644 permissions
		return "", fmt.Errorf("found existing path: %v", final)
	}

	// create directory with data + time
	err := os.MkdirAll(final, 0766)
	if err != nil {
		return "", fmt.Errorf("failed to make directory: %v", err)
	}
	return final, nil
}
