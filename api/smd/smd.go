package smd

// See ref for API docs:
//	https://github.com/Cray-HPE/hms-smd/blob/master/docs/examples.adoc
//	https://github.com/alexlovelltroy/hms-smd
import (
	"davidallendj/magellan/api"
	"fmt"
)

const (
	Host = "http://localhost"
	BaseEndpoint = "/hsm/v2"
	Port = 27779
)

func makeEndpointUrl(endpoint string) string {
	return Host + ":" + fmt.Sprint(Port) + BaseEndpoint + endpoint
}

func GetRedfishEndpoints() error {
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	_, body, err := api.MakeRequest(url, "GET", nil)
	if err != nil {
		return fmt.Errorf("could not get endpoint: %v", err)
	}
	// fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func GetComponentEndpoint(xname string) error {
	url := makeEndpointUrl("/Inventory/ComponentsEndpoints/" + xname)
	res, body, err := api.MakeRequest(url, "GET", nil)
	if err != nil {
		return fmt.Errorf("could not get endpoint: %v", err)
	}
	fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func AddRedfishEndpoint(inventory []byte) error {
	if inventory == nil {
		return fmt.Errorf("could not add redfish endpoint: no data found")
	}
	// Add redfish endpoint via POST `/hsm/v2/Inventory/RedfishEndpoints` endpoint 
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	res, body, _ := api.MakeRequest(url, "POST", inventory)
	fmt.Println("res: ", res)
	fmt.Println("body: ", string(body))
	return nil
}

func UpdateRedfishEndpoint() {
	// Update redfish endpoint via PUT `/hsm/v2/Inventory/RedfishEndpoints` endpoint
}