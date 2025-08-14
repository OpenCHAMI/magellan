package util

import (
	"fmt"
	"io/fs"
	"os"
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
