package magellan

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	bmclib "github.com/bmc-toolbox/bmclib/v2"
	"github.com/go-logr/logr"
	"github.com/jacobweinstock/registrar"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/stmcginnis/gofish"
)

const (
	IPMI_PORT    = 623
	SSH_PORT     = 22
	TLS_PORT     = 443
	REDFISH_PORT = 5000
)

type bmcProbeResult struct {
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
	Preferred		string
	Timeout       int
	WithSecureTLS bool
	CertPoolFile  string
	Verbose       bool
	IpmitoolPath string
}

func rawConnect(host string, ports []int, timeout int, keepOpenOnly bool) []bmcProbeResult {
	results := []bmcProbeResult{}
	for _, p := range ports {
		result := bmcProbeResult{
			Host:     host,
			Port:     p,
			Protocol: "tcp",
			State:    false,
		}
		t := time.Second * time.Duration(timeout)
		port := fmt.Sprint(p)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), t)
		if err != nil {
			result.State = false
			// fmt.Println("Connecting error:", err)
		}
		if conn != nil {
			result.State = true
			defer conn.Close()
			// fmt.Println("Opened", net.JoinHostPort(host, port))
		}
		if keepOpenOnly {
			if result.State {
				results = append(results, result)
			}
		} else {
			results = append(results, result)
		}
	}

	return results
}

func GenerateHosts(subnet string, begin uint8, end uint8) []string {
	hosts := []string{}
	ip := net.ParseIP(subnet).To4()
	for i := begin; i < end; i++ {
		ip[3] = byte(i)
		hosts = append(hosts, fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3]))
	}
	return hosts
}

func ScanForAssets(hosts []string, ports []int, threads int, timeout int) []bmcProbeResult {
	states := make([]bmcProbeResult, 0, len(hosts))
	done := make(chan struct{}, threads+1)
	chanHost := make(chan string, threads+1)
	// chanPort := make(chan int, threads+1)
	var wg sync.WaitGroup
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		go func() {
			for {
				host, ok := <-chanHost
				if !ok {
					wg.Done()
					return
				}
				s := rawConnect(host, ports, timeout, true)
				states = append(states, s...)
			}
		}()
	}

	for _, host := range hosts {
		chanHost <- host
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
	close(chanHost)
	wg.Wait()
	close(done)
	return states
}

func StoreStates(path string, states *[]bmcProbeResult) error {
	if states == nil {
		return fmt.Errorf("states == nil")
	}

	// create database if it doesn't already exist
	schema := `
	CREATE TABLE IF NOT EXISTS magellan_scanned_ports (
		host TEXT PRIMARY KEY NOT NULL,
		port INTEGER,
		protocol TEXT,
		state INTEGER
	);
	`
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("could not open database: %v", err)
	}
	db.MustExec(schema)

	// insert all probe states into db
	tx := db.MustBegin()
	for _, state := range *states {
		sql := `INSERT OR REPLACE INTO magellan_scanned_ports (host, port, protocol, state) 
		VALUES (:host, :port, :protocol, :state);`
		_, err := tx.NamedExec(sql, &state)
		if err != nil {
			fmt.Printf("could not execute transaction: %v\n", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %v", err)
	}
	return nil
}

func GetStates(path string) ([]bmcProbeResult, error) {
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %v", err)
	}

	results := []bmcProbeResult{}
	err = db.Select(&results, "SELECT * FROM magellan_scanned_ports ORDER BY host ASC")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve probes: %v", err)
	}
	return results, nil
}

func GetDefaultPorts() []int {
	return []int{SSH_PORT, TLS_PORT, IPMI_PORT, REDFISH_PORT}
}

func NewClient(l *logr.Logger, q *QueryParams) (*bmclib.Client, error) {
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
		// bmclib.WithSecureTLS(),
		// bmclib.WithHTTPClient(&httpClient),
		bmclib.WithLogger(*l),
		// bmclib.WithRedfishHTTPClient(&httpClient),
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

func QueryMetadata(client *bmclib.Client, l *logr.Logger, q *QueryParams) ([]byte, error) {
	// client, err := NewClient(l, q)

	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open BMC client: %v", err)
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
	return []byte(b), nil
}

func QueryInventory(client *bmclib.Client, l *logr.Logger, q *QueryParams) ([]byte, error) {
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
	return []byte(b), nil
}


// func QueryInventoryV2(host string, port int, user string, pass string) ([]byte, error) {
// 	url := fmt.Sprintf("http://%s:%s@%s:%s/redfish/v1/", user, pass, host, fmt.Sprint(port))
// 	res, body, err := api.MakeRequest(url, "GET", nil)
// 	if err != nil {
// 		return nil , fmt.Errorf("could not get endpoint: %v", err)
// 	}
// 	fmt.Println(res)
// 	fmt.Println(string(body))

// 	return body, err
// }

func QueryUsers(client *bmclib.Client, l *logr.Logger, q *QueryParams) ([]byte, error) {
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
		return nil, fmt.Errorf("could not open BMC client: %v", err)
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
	return []byte(b), nil
}

func QueryBios(client *bmclib.Client, l *logr.Logger, q *QueryParams) ([]byte, error) {
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
