// Package magellan implements the core routines for the tools.
package magellan

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/OpenCHAMI/magellan/internal/log"

	"github.com/OpenCHAMI/magellan/internal/api/smd"
	"github.com/OpenCHAMI/magellan/internal/util"

	"github.com/Cray-HPE/hms-xname/xnames"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stmcginnis/gofish"
	_ "github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	"golang.org/x/exp/slices"
)

const (
	IPMI_PORT  = 623
	SSH_PORT   = 22
	HTTPS_PORT = 443
)

// QueryParams is a collections of common parameters passed to the CLI.
// Each CLI subcommand has a corresponding implementation function that
// takes an object as an argument. However, the implementation may not
// use all of the properties within the object.
type QueryParams struct {
	Host         string   // set by the 'host' flag
	Port         int      // set by the 'port' flag
	Protocol     string   // set by the 'protocol' flag
	Username     string   // set the BMC username with the 'username' flag
	Password     string   // set the BMC password with the 'password' flag
	Drivers      []string // DEPRECATED: TO BE REMOVED!!!
	Concurrency  int      // set the of concurrent jobs with the 'concurrency' flag
	Preferred    string   // DEPRECATED: TO BE REMOVED!!!
	Timeout      int      // set the timeout with the 'timeout' flag
	CaCertPath   string   // set the cert path with the 'cacert' flag
	Verbose      bool     // set whether to include verbose output with 'verbose' flag
	IpmitoolPath string   // DEPRECATED: TO BE REMOVE!!!
	OutputPath   string   // set the path to save output with 'output' flag
	ForceUpdate  bool     // set whether to force updating SMD with 'force-update' flag
	AccessToken  string   // set the access token to include in request with 'access-token' flag
}

// This is the main function used to collect information from the BMC nodes via Redfish.
// The function expects a list of hosts found using the `ScanForAssets()` function.
//
// Requests can be made to several of the nodes using a goroutine by setting the q.Concurrency
// property value between 1 and 255.
func CollectAll(probeStates *[]ScannedResult, l *log.Logger, q *QueryParams) error {
	// check for available probe states
	if probeStates == nil {
		return fmt.Errorf("no probe states found")
	}
	if len(*probeStates) <= 0 {
		return fmt.Errorf("no probe states found")
	}

	// make the output directory to store files
	outputPath := path.Clean(q.OutputPath)
	outputPath, err := util.MakeOutputDirectory(outputPath)
	if err != nil {
		l.Log.Errorf("failed to make output directory: %v", err)
	}

	// collect bmc information asynchronously
	var (
		offset         = 0
		wg             sync.WaitGroup
		found          = make([]string, 0, len(*probeStates))
		done           = make(chan struct{}, q.Concurrency+1)
		chanProbeState = make(chan ScannedResult, q.Concurrency+1)
		client         = smd.NewClient(
			smd.WithSecureTLS(q.CaCertPath),
		)
	)
	wg.Add(q.Concurrency)
	for i := 0; i < q.Concurrency; i++ {
		go func() {
			for {
				ps, ok := <-chanProbeState
				if !ok {
					wg.Done()
					return
				}
				q.Host = ps.Host
				q.Port = ps.Port

				// generate custom xnames for bmcs
				node := xnames.Node{
					Cabinet:       1000,
					Chassis:       1,
					ComputeModule: 7,
					NodeBMC:       offset,
				}
				offset += 1

				gofishClient, err := connectGofish(q)
				if err != nil {
					l.Log.Errorf("failed to connect to BMC (%v:%v): %v", q.Host, q.Port, err)
				}
				defer gofishClient.Logout()

				// data to be sent to smd
				data := map[string]any{
					"ID":   fmt.Sprintf("%v", node.String()[:len(node.String())-2]),
					"Type": "",
					"Name": "",
					"FQDN": ps.Host,
					"User": q.Username,
					// "Password":           q.Pass,
					"MACRequired":        true,
					"RediscoverOnUpdate": false,
				}

				// chassis
				if gofishClient != nil {
					chassis, err := CollectChassis(gofishClient, q)
					if err != nil {
						l.Log.Errorf("failed to collect chassis: %v", err)
						continue
					}
					data["Chassis"] = chassis

					// systems
					systems, err := CollectSystems(gofishClient, q)
					if err != nil {
						l.Log.Errorf("failed to collect systems: %v", err)
					}
					data["Systems"] = systems

					// add other fields from systems
					if len(systems) > 0 {
						system := systems[0]["Data"].(*redfish.ComputerSystem)
						if system == nil {
							l.Log.Errorf("invalid system data (data is nil)")
						} else {
							data["Name"] = system.Name
						}
					}
				} else {
					l.Log.Errorf("invalid client (client is nil)")
					continue
				}

				headers := make(map[string]string)
				headers["Content-Type"] = "application/json"

				// use access token in authorization header if we have it
				if q.AccessToken != "" {
					headers["Authorization"] = "Bearer " + q.AccessToken
				}

				body, err := json.MarshalIndent(data, "", "    ")
				if err != nil {
					l.Log.Errorf("failed to marshal output to JSON: %v", err)
				}

				if q.Verbose {
					fmt.Printf("%v\n", string(body))
				}

				// write JSON data to file if output path is set
				if outputPath != "" {
					err = os.WriteFile(path.Clean(outputPath+"/"+q.Host+".json"), body, os.ModePerm)
					if err != nil {
						l.Log.Errorf("failed to write data to file: %v", err)
					}
				}

				// add all endpoints to smd
				err = client.AddRedfishEndpoint(body, headers)
				if err != nil {
					l.Log.Error(err)

					// try updating instead
					if q.ForceUpdate {
						err = client.UpdateRedfishEndpoint(data["ID"].(string), body, headers)
						if err != nil {
							l.Log.Error(err)
						}
					}
				}

				// got host information, so add to list of already probed hosts
				found = append(found, ps.Host)
			}
		}()
	}

	// use the found results to query bmc information
	for _, ps := range *probeStates {
		// skip if found info from host
		foundHost := slices.Index(found, ps.Host)
		if !ps.State || foundHost >= 0 {
			continue
		}
		chanProbeState <- ps
	}

	// handle goroutine paths
	go func() {
		select {
		case <-done:
			wg.Done()
			break
		default:
			time.Sleep(1000)
		}
	}()

	close(chanProbeState)
	wg.Wait()
	close(done)

	return nil
}

// CollectEthernetInterfaces() collects all of the ethernet interfaces found
// from all systems from under the "/redfish/v1/Systems" endpoint.
//
// TODO: This function needs to be refactored entirely...if not deleted
// in favor of using crawler.CrawlBM() instead.
func CollectEthernetInterfaces(c *gofish.APIClient, q *QueryParams, systemID string) ([]byte, error) {
	// TODO: add more endpoints to test for ethernet interfaces
	// /redfish/v1/Chassis/{ChassisID}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions/{NetworkDeviceFunctionId}/EthernetInterfaces/{EthernetInterfaceId}
	// /redfish/v1/Managers/{ManagerId}/EthernetInterfaces/{EthernetInterfaceId}
	// /redfish/v1/Systems/{ComputerSystemId}/EthernetInterfaces/{EthernetInterfaceId}
	// /redfish/v1/Systems/{ComputerSystemId}/OperatingSystem/Containers/EthernetInterfaces/{EthernetInterfaceId}
	systems, err := c.Service.Systems()
	if err != nil {
		return nil, fmt.Errorf("failed to get systems: (%v:%v): %v", q.Host, q.Port, err)
	}

	var (
		interfaces []*redfish.EthernetInterface
		errList    []error
	)

	// get all of the ethernet interfaces in our systems
	for _, system := range systems {
		eth, err := system.EthernetInterfaces()
		if err != nil {
			errList = append(errList, err)
		}

		interfaces = append(interfaces, eth...)
	}

	// print any report errors
	err = util.FormatErrorList(errList)
	if util.HasErrors(errList) {
		return nil, fmt.Errorf("failed to get ethernet interfaces with %d error(s): \n%v", len(errList), err)
	}

	data := map[string]any{"EthernetInterfaces": interfaces}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return b, nil
}

// CollectChassis() fetches all chassis related information from each node specified
// via the Redfish API. Like the other collect functions, this function uses the gofish
// library to make requests to each node. Additionally, all of the network adapters found
// are added to the output as well.
//
// Returns a map that represents a Chassis object with NetworkAdapters.
func CollectChassis(c *gofish.APIClient, q *QueryParams) ([]map[string]any, error) {
	rfChassis, err := c.Service.Chassis()
	if err != nil {
		return nil, fmt.Errorf("failed to query chassis (%v:%v): %v", q.Host, q.Port, err)
	}

	var chassis []map[string]any
	for _, ch := range rfChassis {
		networkAdapters, err := ch.NetworkAdapters()
		if err != nil {
			return nil, fmt.Errorf("failed to get network adapters: %v", err)
		}

		chassis = append(chassis, map[string]any{
			"Data":            ch,
			"NetworkAdapters": networkAdapters,
		})
	}

	return chassis, nil
}

// TODO: DELETE ME!!!
func CollectStorage(c *gofish.APIClient, q *QueryParams) ([]byte, error) {
	systems, err := c.Service.StorageSystems()
	if err != nil {
		return nil, fmt.Errorf("failed to query storage systems (%v:%v): %v", q.Host, q.Port, err)
	}

	services, err := c.Service.StorageServices()
	if err != nil {
		return nil, fmt.Errorf("failed to query storage services (%v:%v): %v", q.Host, q.Port, err)
	}

	data := map[string]any{
		"Storage": map[string]any{
			"Systems":  systems,
			"Services": services,
		},
	}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return b, nil
}

// CollectSystems pulls system information from each BMC node via Redfish using the
// `gofish` library.
//
// The process of collecting this info is as follows:
// 1. check if system has ethernet interfaces
// 1.a. if yes, create system data and ethernet interfaces JSON
// 1.b. if no, try to get data using manager instead
// 2. check if manager has "ManagerForServices" and "EthernetInterfaces" properties
// 2.a. if yes, query both properties to use in next step
// 2.b. for each service, query its data and add the ethernet interfaces
// 2.c. add the system to list of systems to marshal and return
func CollectSystems(c *gofish.APIClient, q *QueryParams) ([]map[string]any, error) {
	rfSystems, err := c.Service.Systems()
	if err != nil {
		return nil, fmt.Errorf("failed to get systems (%v:%v): %v", q.Host, q.Port, err)
	}

	var systems []map[string]any

	for _, system := range rfSystems {
		eths, err := system.EthernetInterfaces()
		if err != nil {
			return nil, fmt.Errorf("failed to get system ethernet interfaces: %v", err)
		}

		// try and get ethernet interfaces through manager if empty
		if len(eths) <= 0 {
			if q.Verbose {
				fmt.Printf("no system ethernet interfaces found...trying to get from managers interface\n")
			}
			managedBy, err := system.ManagedBy()
			if err != nil {
				return nil, fmt.Errorf("failed to get system managers for '%s': %w", system.Name, err)
			}
			for _, manager := range managedBy {
				// try getting ethernet interface from all managers until one is found
				eths, err = manager.EthernetInterfaces()
				if err != nil {
					return nil, fmt.Errorf("failed to get system manager ethernet interfaces: %v", err)
				}
				if len(eths) > 0 {
					break
				}
			}
		}

		// add network interfaces to system
		rfNetworkInterfaces, err := system.NetworkInterfaces()
		if err != nil {
			return nil, fmt.Errorf("failed to get system network interfaces: %v", err)
		}

		// get the network adapter ID for each network interface
		var networkInterfaces []map[string]any
		for _, rfNetworkInterface := range rfNetworkInterfaces {
			networkAdapter, err := rfNetworkInterface.NetworkAdapter()
			if err != nil {
				return nil, fmt.Errorf("failed to get network adapter: %v", err)
			}

			networkInterfaces = append(networkInterfaces, map[string]any{
				"Data":             rfNetworkInterface,
				"NetworkAdapterId": networkAdapter.ID,
			})
		}

		// add system to collection of systems
		systems = append(systems, map[string]any{
			"Data":               system,
			"EthernetInterfaces": eths,
			"NetworkInterfaces":  networkInterfaces,
		})
	}

	// do manual requests if systems is empty to only get necessary info as last resort
	// /redfish/v1/Systems

	// /redfish/v1/Systems/Members
	// /redfish/v1/Systems/
	// fmt.Printf("system count: %d\n", len(systems))
	// if len(systems) == 0 {
	// 	url := baseRedfishUrl(q) + "/Systems"
	// 	if q.Verbose {
	// 		fmt.Printf("%s\n", url)
	// 	}
	// 	res, body, err := util.MakeRequest(nil, url, "GET", nil, nil)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to make request: %v", err)
	// 	} else if res.StatusCode != http.StatusOK {
	// 		return nil, fmt.Errorf("request returned status code %d", res.StatusCode)
	// 	}

	// 	// sweet syntatic sugar type aliases
	// 	type System = map[string]any
	// 	type Member = map[string]string

	// 	// get all the systems
	// 	var (
	// 		tempSystems System
	// 		interfaces  []*redfish.EthernetInterface
	// 		errList     []error
	// 	)
	// 	err = json.Unmarshal(body, &tempSystems)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to unmarshal systems: %v", err)
	// 	}

	// 	// then, get all the members within a system
	// 	members, ok := tempSystems["Members"]
	// 	if ok {
	// 		for _, member := range members.([]Member) {
	// 			id, ok := member["@odata.id"]
	// 			if ok {
	// 				// /redfish/v1/Systems/Self (or whatever)
	// 				// memberEndpoint := fmt.Sprintf("%s%s", url, id)
	// 				// res, body, err := util.MakeRequest(nil, baseRedfishUrl(q)+memberEndpoint, http.MethodGet, nil, nil)
	// 				// if err != nil {
	// 				// 	continue
	// 				// } else if res.StatusCode != http.StatusOK {
	// 				// 	continue
	// 				// }
	// 				// TODO: extract EthernetInterfaces from Systems then query

	// 				// get all of the ethernet interfaces in our systems
	// 				ethernetInterface, err := redfish.ListReferencedEthernetInterfaces(c, id+"/EthernetInterfaces/")
	// 				if err != nil {
	// 					errList = append(errList, err)
	// 					continue
	// 				}
	// 				interfaces = append(interfaces, ethernetInterface...)
	// 			} else {
	// 				return nil, fmt.Errorf("no ID found for member")
	// 			}
	// 			if util.HasErrors(errList) {
	// 				return nil, util.FormatErrorList(errList)
	// 			}
	// 		}
	// 		i, err := json.Marshal(interfaces)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("failed to unmarshal interface: %v", err)
	// 		}
	// 		temp = append(temp, map[string]any{
	// 			"Data":               nil,
	// 			"EthernetInterfaces": string(i),
	// 		})
	// 	} else {
	// 		return nil, fmt.Errorf("no members found in systems")
	// 	}

	// } else {
	// 	b, err := json.Marshal(systems)
	// 	if err != nil {
	// 		fmt.Printf("failed to marshal systems: %v", err)
	// 	}
	// 	fmt.Printf("systems: %v\n", string(b))

	// 	// query the system's ethernet interfaces
	// 	// var temp []map[string]any
	// 	var errList []error
	// 	for _, system := range systems {
	// 		interfaces, err := CollectEthernetInterfaces(c, q, system.ID)
	// 		if err != nil {
	// 			errList = append(errList, fmt.Errorf("failed to collect ethernet interface: %v", err))
	// 			continue
	// 		}
	// 		var i map[string]any
	// 		err = json.Unmarshal(interfaces, &i)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("failed to unmarshal interface: %v", err)
	// 		}
	// 		temp = append(temp, map[string]any{
	// 			"Data":               system,
	// 			"EthernetInterfaces": i["EthernetInterfaces"],
	// 		})
	// 	}
	// 	if util.HasErrors(errList) {
	// 		err = util.FormatErrorList(errList)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("multiple errors occurred: %v", err)
	// 		}
	// 	}
	// }

	return systems, nil
}

// TODO: DELETE ME!!!
func CollectRegisteries(c *gofish.APIClient, q *QueryParams) ([]byte, error) {
	registries, err := c.Service.Registries()
	if err != nil {
		return nil, fmt.Errorf("failed to query storage systems (%v:%v): %v", q.Host, q.Port, err)
	}

	data := map[string]any{"Registries": registries}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return b, nil
}

// TODO: MAYBE DELETE???
func CollectProcessors(q *QueryParams) ([]byte, error) {
	url := baseRedfishUrl(q) + "/Systems"
	res, body, err := util.MakeRequest(nil, url, "GET", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("something went wrong: %v", err)
	} else if res == nil {
		return nil, fmt.Errorf("no response returned (url: %s)", url)
	} else if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned status code %d", res.StatusCode)
	}

	// convert to not get base64 string
	var procs map[string]json.RawMessage
	var members []map[string]any
	err = json.Unmarshal(body, &procs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal processors: %v", err)
	}
	err = json.Unmarshal(procs["Members"], &members)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal processor members: %v", err)
	}

	// request data about each processor member on node
	for _, member := range members {
		var oid = member["@odata.id"].(string)
		var infoUrl = url + oid
		res, _, err := util.MakeRequest(nil, infoUrl, "GET", nil, nil)
		if err != nil {
			return nil, fmt.Errorf("something went wrong: %v", err)
		} else if res == nil {
			return nil, fmt.Errorf("no response returned (url: %s)", url)
		} else if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("returned status code %d", res.StatusCode)
		}
	}

	data := map[string]any{"Processors": procs}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return b, nil
}

func connectGofish(q *QueryParams) (*gofish.APIClient, error) {
	config, err := makeGofishConfig(q)
	if err != nil {
		return nil, fmt.Errorf("failed to make gofish config: %v", err)
	}
	c, err := gofish.Connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redfish endpoint: %v", err)
	}
	if c != nil {
		c.Service.ProtocolFeaturesSupported = gofish.ProtocolFeaturesSupported{
			ExpandQuery: gofish.Expand{
				ExpandAll: true,
				Links:     true,
			},
		}
	}
	return c, err
}

func makeGofishConfig(q *QueryParams) (gofish.ClientConfig, error) {
	var (
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		url = baseRedfishUrl(q)
	)
	return gofish.ClientConfig{
		Endpoint:            url,
		Username:            q.Username,
		Password:            q.Password,
		Insecure:            true,
		TLSHandshakeTimeout: q.Timeout,
		HTTPClient:          client,
		// MaxConcurrentRequests: int64(q.Threads),  // NOTE: this was added in latest version of gofish
	}, nil
}

func makeJson(object any) ([]byte, error) {
	b, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}
	return []byte(b), nil
}

func baseRedfishUrl(q *QueryParams) string {
	url := fmt.Sprintf("%s://", q.Protocol)
	if q.Username != "" && q.Password != "" {
		url += fmt.Sprintf("%s:%s@", q.Username, q.Password)
	}
	return fmt.Sprintf("%s%s:%d", url, q.Host, q.Port)
}
