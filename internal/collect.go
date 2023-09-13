package magellan

import (
	"context"
	"crypto/x509"
	"davidallendj/magellan/internal/api/smd"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Cray-HPE/hms-xname/xnames"
	bmclib "github.com/bmc-toolbox/bmclib/v2"
	"github.com/jacobweinstock/registrar"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/stmcginnis/gofish"
	_ "github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	"golang.org/x/exp/slices"
)

const (
	IPMI_PORT    = 623
	SSH_PORT     = 22
	HTTPS_PORT   = 443
	REDFISH_PORT = 5000
)

type BMCProbeResult struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	State    bool   `json:"state"`
}

// NOTE: ...params were getting too long...
type QueryParams struct {
	Host          string
	Port          int
	User          string
	Pass          string
	Drivers       []string
	Threads			int
	Preferred		string
	Timeout       int
	WithSecureTLS bool
	CertPoolFile  string
	Verbose       bool
	IpmitoolPath string
}

func NewClient(l *Logger, q *QueryParams) (*bmclib.Client, error) {
	// NOTE: bmclib.NewClient(host, port, user, pass)
	// ...seems like the `port` params doesn't work like expected depending on interface

	// tr := &http.Transport{
	// 	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	// }
	// httpClient := http.Client{
	// 	Transport: tr,
	// }

	// init client
	clientOpts := []bmclib.Option{
		// bmclib.WithSecureTLS(nil),
		// bmclib.WithHTTPClient(&httpClient),
		// bmclib.WithLogger(),
		// bmclib.WithRedfishHTTPClient(&httpClient),
		bmclib.WithDellRedfishUseBasicAuth(true),
		bmclib.WithRedfishPort(fmt.Sprint(q.Port)),
		bmclib.WithRedfishUseBasicAuth(true),
		bmclib.WithIpmitoolPort(fmt.Sprint(IPMI_PORT)),
		bmclib.WithIpmitoolPath(q.IpmitoolPath),
	}

	// only work if valid cert is provided
	if q.WithSecureTLS {
		var pool *x509.CertPool
		if q.CertPoolFile != "" {
			pool = x509.NewCertPool()
			data, err := os.ReadFile(q.CertPoolFile)
			if err != nil {
				return nil, fmt.Errorf("could not read cert pool file: %v", err)
			}
			pool.AppendCertsFromPEM(data)
		}
		// a nil pool uses the system certs
		clientOpts = append(clientOpts, bmclib.WithSecureTLS(pool))
	}
	// url := fmt.Sprintf("https://%s:%s@%s", q.User, q.Pass, q.Host)
	url := ""
	// if q.WithSecureTLS {
	// url = "https://"
	// } else {
	// 	url = "http://"
	// }

	if q.User == "" && q.Pass == "" {
		url += fmt.Sprintf("%s:%s@%s", q.User, q.Pass, q.Host)
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

func CollectInfo(probeStates *[]BMCProbeResult, l *Logger, q *QueryParams) error {
	if probeStates == nil {
		return fmt.Errorf("no probe states found")
	}
	if len(*probeStates) <= 0 {
		return fmt.Errorf("no probe states found")
	}
	
	// generate custom xnames for bmcs
	node := xnames.Node{
		Cabinet:		1000,
		Chassis:		1,
		ComputeModule:	7,
		NodeBMC:		1,
		Node:			0,
	}

	found 			:= make([]string, 0, len(*probeStates))
	done 			:= make(chan struct{}, q.Threads+1)
	chanProbeState 	:= make(chan BMCProbeResult, q.Threads+1)

	//
	var wg sync.WaitGroup
	wg.Add(q.Threads)
	for i := 0; i < q.Threads; i++ {
		go func() {
			for {
				ps, ok := <- chanProbeState
				if !ok {
					wg.Done()
					return
				}
				q.Host = ps.Host
				q.Port = ps.Port

				logrus.Printf("querying %v:%v (%v)\n", ps.Host, ps.Port, ps.Protocol)

				client, err := NewClient(l, q)
				if err != nil {
					l.Log.Errorf("could not make client: %v", err)
					continue 
				}

				// metadata
				// _, err = magellan.QueryMetadata(client, l, &q)
				// if err != nil {
				// 	l.Log.Errorf("could not query metadata: %v\n", err)
				// }

				// inventories
				inventory, err := QueryInventory(client, l, q)
				if err != nil {
					l.Log.Errorf("could not query inventory: %v", err) 
				}

				// chassis
				_, err = QueryChassis(client, l, q)
				if err != nil {
					l.Log.Errorf("could not query chassis: %v", err)
				}

				node.NodeBMC += 1

				headers := make(map[string]string)
				headers["Content-Type"] = "application/json"

				data := make(map[string]any)
				data["ID"] 					= fmt.Sprintf("%v", node)
				data["Type"]				= ""
				data["Name"]				= ""
				data["FQDN"]				= ps.Host
				data["RediscoverOnUpdate"] 	= false
				data["Inventory"] 			= inventory

				b, err := json.MarshalIndent(data, "", "    ")
				if err != nil {
					l.Log.Errorf("could not marshal JSON: %v", err)
				}

				// add all endpoints to smd
				err = smd.AddRedfishEndpoint(b, headers)
				if err != nil {
					l.Log.Errorf("could not add redfish endpoint: %v", err)
				}

				// confirm the inventories were added
				err = smd.GetRedfishEndpoints()
				if err != nil {
					l.Log.Errorf("could not get redfish endpoints: %v", err)
				}

				// users
				// user, err := magellan.QueryUsers(client, l, &q)
				// if err != nil {
				// 	l.Log.Errorf("could not query users: %v\n", err)
				// }
				// users = append(users, user)

				// bios
				// _, err = magellan.QueryBios(client, l, &q)
				// if err != nil {
				// 	l.Log.Errorf("could not query bios: %v\n", err)
				// }

				// _, err = magellan.QueryPowerState(client, l, &q)
				// if err != nil {
				// 	l.Log.Errorf("could not query power state: %v\n", err)
				// }

				// got host information, so add to list of already probed hosts
				found = append(found, ps.Host)
			}
		}()
	}

	// use the found results to query bmc information
	for _, ps := range *probeStates {
		// skip if found info from host
		foundHost := slices.Index(found, ps.Host)
		if !ps.State || foundHost >= 0{
			continue
		}
		chanProbeState <- ps
	}

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

func QueryMetadata(client *bmclib.Client, l *Logger, q *QueryParams) ([]byte, error) {
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

	if q.Verbose {
		fmt.Printf("metadata: %v\n", string(b))
	}
	ctxCancel()
	return b, nil
}

func QueryInventory(client *bmclib.Client, l *Logger, q *QueryParams) ([]byte, error) {
	// discover.ScanAndConnect(url, user, pass, clientOpts)

	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.PreferProvider(q.Preferred).Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open client: %v", err)
	}
	defer client.Close(ctx)

	inventory, err := client.Inventory(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not get inventory: %v", err)
	}

	// retrieve inventory data
	b, err := json.MarshalIndent(inventory, "", "    ")
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	if q.Verbose {
		fmt.Printf("inventory: %v\n", string(b))
	}
	ctxCancel()
	return b, nil
}

func QueryPowerState(client *bmclib.Client, l *Logger, q *QueryParams) ([]byte, error) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.PreferProvider(q.Preferred).Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open client: %v", err)
	}
	defer client.Close(ctx)

	inventory, err := client.GetPowerState(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not get inventory: %v", err)
	}

	// retrieve inventory data
	b, err := json.MarshalIndent(inventory, "", "    ")
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	if q.Verbose {
		fmt.Printf("power state: %v\n", string(b))
	}
	ctxCancel()
	return b, nil

}

func QueryUsers(client *bmclib.Client, l *Logger, q *QueryParams) ([]byte, error) {
	// discover.ScanAndConnect(url, user, pass, clientOpts)
	// client, err := NewClient(l, q)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not make query: %v", err)
	// }

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
	b, err := json.MarshalIndent(users, "", "    ")
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	// return b, nil
	ctxCancel()
	if q.Verbose {
		fmt.Printf("users: %v\n", string(b))
	}
	return b, nil
}

func QueryBios(client *bmclib.Client, l *Logger, q *QueryParams) ([]byte, error) {
	// client, err := NewClient(l, q)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not make query: %v", err)
	// }
	b, err := makeRequest(client, client.GetBiosConfiguration, q.Timeout)
	if q.Verbose {
		fmt.Printf("bios: %v\n", string(b))
	}
	return b, err
}

func QueryEthernetInterfaces(client *bmclib.Client, l *Logger, q *QueryParams) ([]byte, error) {
	config := gofish.ClientConfig{

	}
	c, err := gofish.Connect(config)
	if err != nil {

	}

	redfish.ListReferencedEthernetInterfaces(c, "")
	return []byte{}, nil
}

func QueryChassis(client *bmclib.Client, l *Logger, q *QueryParams) ([]byte, error) {
	config := gofish.ClientConfig {
		Endpoint: "https://" + q.Host,
		Username: q.User,
		Password: q.Pass,
		Insecure: q.WithSecureTLS,
	}
	c, err := gofish.Connect(config)
	if err != nil {
		return nil, fmt.Errorf("could not connect to bmc: %v", err)
	}
	chassis, err := c.Service.Chassis()
	if err != nil {
		return nil, fmt.Errorf("could not query chassis: %v", err)
	}

	b, err := json.MarshalIndent(chassis, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}

	if q.Verbose {
		fmt.Printf("chassis: %v\n", string(b))
	}
	return b, nil
}

func makeRequest[T interface{}](client *bmclib.Client, fn func(context.Context) (T, error), timeout int) ([]byte, error) {
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

func makeJson(object interface{}) ([]byte, error) {
	b, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON: %v", err)
	}
	return []byte(b), nil
}
