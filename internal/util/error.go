package util

import (
	"errors"
	"fmt"
)

// FormatErrorList() is a wrapper function that unifies error list formatting
// and makes printing error lists consistent.
//
// NOTE: The error returned IS NOT an error in itself and may be a bit misleading.
// Instead, it is a single condensed error composed of all of the errors included
// in the errList argument.
func FormatErrorList(errList []error) error {
	var errmsg string
	for i, e := range errList {
		// NOTE: for multi-error formating, we want to include \n here
		errmsg = fmt.Sprintf("\t[%d] %v\n", i, e)
	}
	return errors.New(errmsg)
}

// HasErrors() is a simple wrapper function to check if an error list contains
// errors. Having a function that clearly states its purpose helps to improve
// readibility although it may seem pointless.
func HasErrors(errList []error) bool {
	return len(errList) > 0
}
