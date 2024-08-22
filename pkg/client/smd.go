package client

// See ref for API docs:
//	https://github.com/OpenCHAMI/hms-smd/blob/master/docs/examples.adoc
//	https://github.com/OpenCHAMI/hms-smd
import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

type SmdClient struct {
	*http.Client
	URI   string
	Xname string
}

func (c SmdClient) Name() string {
	return "smd"
}

func (c SmdClient) RootEndpoint(endpoint string) string {
	return fmt.Sprintf("%s/hsm/v2%s", c.URI, endpoint)
}

func (c SmdClient) GetClient() *http.Client {
	return c.Client
}

// Add() has a similar function definition to that of the default implementation,
// but also allows further customization and data/header manipulation that would
// be specific and/or unique to SMD's API.
func (c SmdClient) Add(data HTTPBody, headers HTTPHeader) error {
	if data == nil {
		return fmt.Errorf("failed to add redfish endpoint: no data found")
	}

	// Add redfish endpoint via POST `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := c.RootEndpoint("/Inventory/RedfishEndpoints")
	res, body, err := MakeRequest(c.Client, url, http.MethodPost, data, headers)
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			if len(body) > 0 {
				return fmt.Errorf("%d: %s", res.StatusCode, string(body))
			} else {
				return fmt.Errorf("returned status code %d when adding endpoint", res.StatusCode)
			}
		}
		log.Debug().Msgf("%v (%v)\n%s\n", url, res.Status, string(body))
	}
	return err
}

func (c SmdClient) Update(data HTTPBody, headers HTTPHeader) error {
	if data == nil {
		return fmt.Errorf("failed to add redfish endpoint: no data found")
	}
	// Update redfish endpoint via PUT `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := c.RootEndpoint("/Inventory/RedfishEndpoints/" + c.Xname)
	res, body, err := MakeRequest(c.Client, url, http.MethodPut, data, headers)
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			if len(body) > 0 {
				return fmt.Errorf("%d: %s", res.StatusCode, string(body))
			} else {
				return fmt.Errorf("failed to update redfish endpoint (returned %s)", res.Status)
			}
		}
		log.Debug().Msgf("%v (%v)\n%s\n", url, res.Status, string(body))
	}
	return err
}
