package magellan

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
	bmclib "github.com/bmc-toolbox/bmclib/v2"
	"github.com/jacobweinstock/registrar"
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

// NOTE: ...params were getting too long...
type QueryParams struct {
	Host          string
	Port          int
	Protocol      string
	User          string
	Pass          string
	Drivers       []string
	Threads       int
	Preferred     string
	Timeout       int
	WithSecureTLS bool
	CertPoolFile  string
	Verbose       bool
	IpmitoolPath  string
	OutputPath    string
	ForceUpdate   bool
}

func NewClient(l *log.Logger, q *QueryParams) (*bmclib.Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{
		Transport: tr,
	}

	// init client
	clientOpts := []bmclib.Option{
		// bmclib.WithSecureTLS(nil),
		bmclib.WithHTTPClient(&httpClient),
		// bmclib.WithLogger(),
		bmclib.WithRedfishHTTPClient(&httpClient),
		// bmclib.WithDellRedfishUseBasicAuth(true),
		bmclib.WithRedfishPort(fmt.Sprint(q.Port)),
		bmclib.WithRedfishUseBasicAuth(true),
		bmclib.WithIpmitoolPort(fmt.Sprint(IPMI_PORT)),
		bmclib.WithIpmitoolPath(q.IpmitoolPath),
	}

	// only work if valid cert is provided
	if q.WithSecureTLS && q.CertPoolFile != "" {
		pool := x509.NewCertPool()
		data, err := os.ReadFile(q.CertPoolFile)
		if err != nil {
			return nil, fmt.Errorf("could not read cert pool file: %v", err)
		}
		pool.AppendCertsFromPEM(data)
		// a nil pool uses the system certs
		clientOpts = append(clientOpts, bmclib.WithSecureTLS(pool))
	}
	url := ""
	fmt.Println(url)
	if q.User != "" && q.Pass != "" {
		url += fmt.Sprintf("%s://%s:%s@%s", q.Protocol, q.User, q.Pass, q.Host)
	} else {
		url += q.Host
	}
	client := bmclib.NewClient(url, q.User, q.Pass, clientOpts...)
	ds := registrar.Drivers{}
	for _, driver := range q.Drivers {
		ds = append(ds, client.Registry.Using(driver)...) // ipmi, gofish, redfish
	}
	client.Registry.Drivers = ds

	return client, nil
}

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
		l.Log.Errorf("could not make output directory: %v", err)
	}

	found := make([]string, 0, len(*probeStates))
	done := make(chan struct{}, q.Threads+1)
	chanProbeState := make(chan ScannedResult, q.Threads+1)

	// collect bmc information asynchronously
	var offset = 0
	var wg sync.WaitGroup
	wg.Add(q.Threads)
	for i := 0; i < q.Threads; i++ {
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

				// bmclibClient, err := NewClient(l, q)
				// if err != nil {
				// 	l.Log.Errorf("could not make client: %v", err)
				// }

				gofishClient, err := connectGofish(q)
				if err != nil {
					l.Log.Errorf("could not connect to bmc (%v:%v): %v", q.Host, q.Port, err)
				}

				// data to be sent to smd
				data := map[string]any{
					"ID":                 fmt.Sprintf("%v", node.String()[:len(node.String())-2]),
					"Type":               "",
					"Name":               "",
					"FQDN":               ps.Host,
					"User":               q.User,
					"Password":           q.Pass,
					"MACRequired":        true,
					"RediscoverOnUpdate": false,
				}

				// unmarshal json to send in correct format
				var rm map[string]json.RawMessage

				// inventories
				// if bmclibClient != nil {
				// 	inventory, err := CollectInventory(bmclibClient, q)
				// 	if err != nil {
				// 		l.Log.Errorf("could not query inventory (%v:%v): %v", q.Host, q.Port, err)
				// 	}
				// 	json.Unmarshal(inventory, &rm)
				// 	data["Inventory"] = rm["Inventory"]
				// }

				// chassis
				if gofishClient != nil {
					chassis, err := CollectChassis(gofishClient, q)
					if err != nil {
						l.Log.Errorf("could not query chassis: %v", err)
						continue
					}
					json.Unmarshal(chassis, &rm)
					data["Chassis"] = rm["Chassis"]

					// systems
					systems, err := CollectSystems(gofishClient, q)
					if err != nil {
						l.Log.Errorf("could not query systems: %v", err)
					}
					json.Unmarshal(systems, &rm)
					data["Systems"] = rm["Systems"]

					// add other fields from systems
					if len(rm["Systems"]) > 0 {
						var s map[string][]interface{}
						json.Unmarshal(rm["Systems"], &s)
						data["Name"] = s["Name"]
					}
				}

				headers := make(map[string]string)
				headers["Content-Type"] = "application/json"

				body, err := json.MarshalIndent(data, "", "    ")
				if err != nil {
					l.Log.Errorf("could not marshal JSON: %v", err)
				}

				if q.Verbose {
					fmt.Printf("%v\n", string(body))
				}

				// write JSON data to file
				err = os.WriteFile(path.Clean(outputPath+"/"+q.Host+".json"), body, os.ModePerm)
				if err != nil {
					l.Log.Errorf("could not write data to file: %v", err)
				}

				// add all endpoints to smd
				err = smd.AddRedfishEndpoint(body, headers)
				if err != nil {
					l.Log.Error(err)

					// try updating instead
					if q.ForceUpdate {
						err = smd.UpdateRedfishEndpoint(data["ID"].(string), body, headers)
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

func CollectMetadata(client *bmclib.Client, q *QueryParams) ([]byte, error) {
	// client, err := NewClient(l, q)

	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not connect to bmc: %v", err)
	}

	defer client.Close(ctx)

	metadata := client.GetMetadata()
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not get metadata: %v", err)
	}

	// retrieve inventory data
	b, err := json.MarshalIndent(metadata, "", "    ")
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	ctxCancel()
	return b, nil
}

func CollectInventory(client *bmclib.Client, q *QueryParams) ([]byte, error) {
	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.PreferProvider(q.Preferred).Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open client: %v", err)
	}

	inventory, err := client.Inventory(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not get inventory: %v", err)
	}

	// retrieve inventory data
	data := map[string]any{"Inventory": inventory}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	ctxCancel()
	return b, nil
}

func CollectPowerState(client *bmclib.Client, q *QueryParams) ([]byte, error) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.PreferProvider(q.Preferred).Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open client: %v", err)
	}

	powerState, err := client.GetPowerState(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not get inventory: %v", err)
	}

	// retrieve inventory data
	data := map[string]any{"PowerState": powerState}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	ctxCancel()
	return b, nil

}

func CollectUsers(client *bmclib.Client, q *QueryParams) ([]byte, error) {
	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not connect to bmc: %v", err)
	}

	defer client.Close(ctx)

	users, err := client.ReadUsers(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not get users: %v", err)
	}

	// retrieve inventory data
	data := map[string]any{"Users": users}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	ctxCancel()
	return b, nil
}

func CollectBios(client *bmclib.Client, q *QueryParams) ([]byte, error) {
	// client, err := NewClient(l, q)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not make query: %v", err)
	// }
	b, err := makeRequest(client, client.GetBiosConfiguration, q.Timeout)
	return b, err
}

func CollectEthernetInterfaces(c *gofish.APIClient, q *QueryParams, systemID string) ([]byte, error) {
	systems, err := c.Service.Systems()
	if err != nil {
		return nil, fmt.Errorf("could not query storage systems (%v:%v): %v", q.Host, q.Port, err)
	}

	var interfaces []*redfish.EthernetInterface
	for _, system := range systems {
		i, err := redfish.ListReferencedEthernetInterfaces(c, "/redfish/v1/Systems/"+system.ID+"/EthernetInterfaces/")
		if err != nil {
			continue
		}
		interfaces = append(interfaces, i...)
	}

	if len(interfaces) <= 0 {
		return nil, fmt.Errorf("could not get ethernet interfaces: %v", err)
	}

	data := map[string]any{"EthernetInterfaces": interfaces}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	return b, nil
}

func CollectChassis(c *gofish.APIClient, q *QueryParams) ([]byte, error) {
	chassis, err := c.Service.Chassis()
	if err != nil {
		return nil, fmt.Errorf("could not query chassis (%v:%v): %v", q.Host, q.Port, err)
	}

	data := map[string]any{"Chassis": chassis}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	return b, nil
}

func CollectStorage(c *gofish.APIClient, q *QueryParams) ([]byte, error) {
	systems, err := c.Service.StorageSystems()
	if err != nil {
		return nil, fmt.Errorf("could not query storage systems (%v:%v): %v", q.Host, q.Port, err)
	}

	services, err := c.Service.StorageServices()
	if err != nil {
		return nil, fmt.Errorf("could not query storage services (%v:%v): %v", q.Host, q.Port, err)
	}

	data := map[string]any{
		"Storage": map[string]any{
			"Systems":  systems,
			"Services": services,
		},
	}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	return b, nil
}

func CollectSystems(c *gofish.APIClient, q *QueryParams) ([]byte, error) {
	systems, err := c.Service.Systems()
	if err != nil {
		return nil, fmt.Errorf("could not query systems (%v:%v): %v", q.Host, q.Port, err)
	}

	// query the system's ethernet interfaces
	var temp []map[string]any
	for _, system := range systems {
		interfaces, err := CollectEthernetInterfaces(c, q, system.ID)
		if err != nil {
			continue
		}
		var i map[string]any
		json.Unmarshal(interfaces, &i)
		temp = append(temp, map[string]any{
			"Data":               system,
			"EthernetInterfaces": i["EthernetInterfaces"],
		})
	}

	data := map[string]any{"Systems": temp}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	return b, nil
}

func CollectRegisteries(c *gofish.APIClient, q *QueryParams) ([]byte, error) {
	registries, err := c.Service.Registries()
	if err != nil {
		return nil, fmt.Errorf("could not query storage systems (%v:%v): %v", q.Host, q.Port, err)
	}

	data := map[string]any{"Registries": registries}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	return b, nil
}

func CollectProcessors(q *QueryParams) ([]byte, error) {
	url := baseRedfishUrl(q) + "/Systems"
	res, body, err := util.MakeRequest(url, "GET", nil, nil)
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
	json.Unmarshal(body, &procs)
	json.Unmarshal(procs["Members"], &members)

	// request data about each processor member on node
	for _, member := range members {
		var oid = member["@odata.id"].(string)
		var infoUrl = url + oid
		res, _, err := util.MakeRequest(infoUrl, "GET", nil, nil)
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
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	return b, nil
}

func connectGofish(q *QueryParams) (*gofish.APIClient, error) {
	config := makeGofishConfig(q)
	c, err := gofish.Connect(config)
	if err != nil {

		return nil, fmt.Errorf("could not connect to redfish endpoint: %v", err)
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

func makeGofishConfig(q *QueryParams) gofish.ClientConfig {
	url := baseRedfishUrl(q)
	return gofish.ClientConfig{
		Endpoint:            url,
		Username:            q.User,
		Password:            q.Pass,
		Insecure:            !q.WithSecureTLS,
		TLSHandshakeTimeout: q.Timeout,
	}
}

func makeRequest[T any](client *bmclib.Client, fn func(context.Context) (T, error), timeout int) ([]byte, error) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open client: %v", err)
	}

	defer client.Close(ctx)

	response, err := fn(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not get response: %v", err)
	}

	ctxCancel()
	return makeJson(response)
}

func makeJson(object any) ([]byte, error) {
	b, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}
	return []byte(b), nil
}

func baseRedfishUrl(q *QueryParams) string {
	url := fmt.Sprintf("%s://", q.Protocol)
	if q.User != "" && q.Pass != "" {
		url += fmt.Sprintf("%s:%s@", q.User, q.Pass)
	}
	return fmt.Sprintf("%s%s:%d", url, q.Host, q.Port)
}
