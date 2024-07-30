package client

// See ref for API docs:
//	https://github.com/OpenCHAMI/hms-smd/blob/master/docs/examples.adoc
//	https://github.com/OpenCHAMI/hms-smd
import (
	"fmt"
	"net/http"

	"github.com/OpenCHAMI/magellan/internal/util"
)

var (
	Host         = "http://localhost"
	BaseEndpoint = "/hsm/v2"
	Port         = 27779
)

func (c *Client) GetRedfishEndpoints(header util.HTTPHeader) error {
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	_, body, err := util.MakeRequest(c.Client, url, http.MethodGet, nil, header)
	if err != nil {
		return fmt.Errorf("failed to get endpoint: %v", err)
	}
	// fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func (c *Client) GetComponentEndpoint(xname string) error {
	url := makeEndpointUrl("/Inventory/ComponentsEndpoints/" + xname)
	res, body, err := c.MakeRequest(url, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get endpoint: %v", err)
	}
	fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func (c *Client) AddRedfishEndpoint(data map[string]any, headers util.HTTPHeader) error {
	if data == nil {
		return fmt.Errorf("failed to add redfish endpoint: no data found")
	}

	// Add redfish endpoint via POST `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	res, body, err := c.Post(url, data, headers)
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("returned status code %d when adding endpoint", res.StatusCode)
		}
		fmt.Printf("%v (%v)\n%s\n", url, res.Status, string(body))
	}
	return err
}

func (c *Client) UpdateRedfishEndpoint(xname string, data []byte, headers map[string]string) error {
	if data == nil {
		return fmt.Errorf("failed to add redfish endpoint: no data found")
	}
	// Update redfish endpoint via PUT `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := makeEndpointUrl("/Inventory/RedfishEndpoints/" + xname)
	res, body, err := c.MakeRequest(url, "PUT", data, headers)
	fmt.Printf("%v (%v)\n%s\n", url, res.Status, string(body))
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("failed to update redfish endpoint (returned %s)", res.Status)
		}
	}
	return err
}

func makeEndpointUrl(endpoint string) string {
	return Host + ":" + fmt.Sprint(Port) + BaseEndpoint + endpoint
}
