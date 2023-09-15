package smd

// See ref for API docs:
//	https://github.com/Cray-HPE/hms-smd/blob/master/docs/examples.adoc
//	https://github.com/alexlovelltroy/hms-smd
import (
	"davidallendj/magellan/internal/api"
	"fmt"
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
	_, body, err := api.MakeRequest(url, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("could not get endpoint: %v", err)
	}
	// fmt.Println(res)
	fmt.Println(string(body))
	return nil
}

func GetComponentEndpoint(xname string) error {
	url := makeEndpointUrl("/Inventory/ComponentsEndpoints/" + xname)
	res, body, err := api.MakeRequest(url, "GET", nil, nil)
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

	// var ep hms.RedfishEP
	// _ = ep
	// Add redfish endpoint via POST `/hsm/v2/Inventory/RedfishEndpoints` endpoint
	url := makeEndpointUrl("/Inventory/RedfishEndpoints")
	res, body, _ := api.MakeRequest(url, "POST", data, headers)
	fmt.Println("smd url: ", url)
	fmt.Println("res: ", res)
	fmt.Println("body: ", string(body))
	return nil
}

func UpdateRedfishEndpoint() {
	// Update redfish endpoint via PUT `/hsm/v2/Inventory/RedfishEndpoints` endpoint
}
