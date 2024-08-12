package client

import (
	"fmt"
	"net/http"
)

type MagellanClient struct {
	*http.Client
}

func (c *MagellanClient) Name() string {
	return "default"
}

// Add() is the default function that is called with a client with no implementation.
// This function will simply make a HTTP request including all the data passed as
// the first argument with no data processing or manipulation. The function sends
// the data to a set callback URL (which may be changed to use a configurable value
// instead).
func (c *MagellanClient) Add(data HTTPBody, headers HTTPHeader) error {
	if data == nil {
		return fmt.Errorf("no data found")
	}

	path := "/inventory/add"
	res, body, err := MakeRequest(c.Client, path, http.MethodPost, data, headers)
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("returned status code %d when POST'ing to endpoint", res.StatusCode)
		}
		fmt.Printf("%v (%v)\n%s\n", path, res.Status, string(body))
	}
	return err
}

func (c *MagellanClient) Update(data HTTPBody, headers HTTPHeader) error {
	if data == nil {
		return fmt.Errorf("no data found")
	}

	path := "/inventory/update"
	res, body, err := MakeRequest(c.Client, path, http.MethodPut, data, headers)
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("returned status code %d when PUT'ing to endpoint", res.StatusCode)
		}
		fmt.Printf("%v (%v)\n%s\n", path, res.Status, string(body))
	}
	return err
}
