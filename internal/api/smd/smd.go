package smd

// See ref for API docs:
//	https://github.com/Cray-HPE/hms-smd/blob/master/docs/examples.adoc
//	https://github.com/alexlovelltroy/hms-smd
import (
	"fmt"

	"github.com/OpenCHAMI/magellan/internal/util"
	// hms "github.com/alexlovelltroy/hms-smd"
)

var (
	Host         = "http://localhost"
	BaseEndpoint = "/hsm/v2"
	Port         = 27779
)

func makeEndpointUrl(endpoint string) string {
	return Host + ":" + fmt.Sprint(Port) + BaseEndpoint + endpoint
}

func GetRedfishEndpoints() error {
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	_, body, err := util.MakeRequest(url, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("could not get endpoint: %v", err)
	}
	// fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func GetComponentEndpoint(xname string) error {
	url := makeEndpointUrl("/Inventory/ComponentsEndpoints/" + xname)
	res, body, err := util.MakeRequest(url, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("could not get endpoint: %v", err)
	}
	fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func AddRedfishEndpoint(data []byte, headers map[string]string) error {
	if data == nil {
		return fmt.Errorf("could not add redfish endpoint: no data found")
	}

	// Add redfish endpoint via POST `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	res, body, err := util.MakeRequest(url, "POST", data, headers)
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("returned status code %d when adding endpoint", res.StatusCode)
		}
		fmt.Printf("%v (%v)\n%s\n", url, res.Status, string(body))
	}
	return err
}

func UpdateRedfishEndpoint(xname string, data []byte, headers map[string]string) error {
	if data == nil {
		return fmt.Errorf("could not add redfish endpoint: no data found")
	}
	// Update redfish endpoint via PUT `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := makeEndpointUrl("/Inventory/RedfishEndpoints/" + xname)
	res, body, err := util.MakeRequest(url, "PUT", data, headers)
	fmt.Printf("%v (%v)\n%s\n", url, res.Status, string(body))
	if res != nil {
		statusOk := res.StatusCode >= 200 && res.StatusCode < 300
		if !statusOk {
			return fmt.Errorf("could not update redfish endpoint")
		}
	}
	return err
}
