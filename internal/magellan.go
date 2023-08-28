package magellan

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	bmclib "github.com/bmc-toolbox/bmclib"
	"github.com/go-logr/logr"
	"github.com/jacobweinstock/registrar"
	"github.com/jmoiron/sqlx"
)

const (
	IPMI_PORT    = 623
	SSH_PORT     = 22
	TLS_PORT     = 443
	REDFISH_PORT = 5000
)

type bmcProbeResult struct {
	Host     string
	Port     int
	Protocol string
	State    bool
}

// NOTE: ...params were getting too long...
type QueryParams struct {
	Host string
	Port int
	User string
	Pass string
	Drivers []string
	Timeout int
	WithSecureTLS bool
	CertPoolFile string
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
	states := []bmcProbeResult{}
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
	CREATE IF NOT EXISTS TABLE scanned_ports (
		host text,
		port integer,
		protocol text,
		state integer
	)
	`
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("could not open database: %v", err)
	}
	db.MustExec(schema)

	// insert all probe states into db
	tx := db.MustBegin()
	for _, state := range *states {
		tx.NamedExec(`INSERT INTO scanned_ports (host, port, protocol, state) 
		VALUES (:Host, :Port, :Protocol, :State)`, &state)
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
	err = db.Select(&results, "SELECT * FROM scanned_ports ORDER BY host ASC")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve probes: %v", err)
	}
	return results, nil
}

func GetDefaultPorts() []int {
	return []int{SSH_PORT, TLS_PORT, IPMI_PORT, REDFISH_PORT}
}

func QueryInventory(l *logr.Logger, q *QueryParams) ([]byte, error) {
	// discover.ScanAndConnect(url, user, pass, clientOpts)
	client, err := makeClient(l, q)
	if err != nil {
		return nil, fmt.Errorf("could not make query: %v", err)
	}

	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err = client.Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open BMC client: %v", err)
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

	// return b, nil
	ctxCancel()
	return []byte(b), nil
}

func QueryUsers(l *logr.Logger, q *QueryParams) ([]byte, error) {
	// discover.ScanAndConnect(url, user, pass, clientOpts)
	client, err := makeClient(l, q)
	if err != nil {
		return nil, fmt.Errorf("could not make query: %v", err)
	}

	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err = client.Open(ctx)
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
	fmt.Printf("users: %v\n", string(b))
	return []byte(b), nil
}

// func QueryBios(l *logr.Logger, q *QueryParams) ([]byte, error){
// 	client, err := makeClient(l, q)
// 	if err != nil {
// 		return nil, fmt.Errorf("could not make query: %v", err)
// 	}
// 	return makeRequest(client, client.GetBiosConfiguration, q.Timeout)
// }

func makeClient(l *logr.Logger, q *QueryParams) (*bmclib.Client, error) {
	// NOTE: bmclib.NewClient(host, port, user, pass)
	// ...seems like the `port` params doesn't work like expected depending on interface

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{
		Transport: tr, 
	}

	// init client
	clientOpts := []bmclib.Option{
		// bmclib.WithSecureTLS(),
		bmclib.WithHTTPClient(&httpClient),
		bmclib.WithLogger(*l),
		// bmclib.WithRedfishHTTPClient(&httpClient),
		// bmclib.WithRedfishPort(fmt.Sprint(q.Port)),
		// bmclib.WithRedfishUseBasicAuth(true),
		// bmclib.WithDellRedfishUseBasicAuth(true),
		// bmclib.WithIpmitoolPort(fmt.Sprint(q.Port)),
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
	url := fmt.Sprintf("https://%s:%s@%s:%d", q.User, q.Pass, q.Host, q.Port)
	fmt.Println("url: ", url)
	client := bmclib.NewClient(url, fmt.Sprint(q.Port), q.User, q.Pass, clientOpts...)
	ds := registrar.Drivers{}
	for _, driver := range q.Drivers {
		ds = append(ds, client.Registry.Using(driver)...) // ipmi, gofish, redfish
	}
	client.Registry.Drivers = ds
	
	return client, nil
}

func makeRequest[T interface{}](client *bmclib.Client, fn func(context.Context) (T, error), timeout int) ([]byte, error){
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.Open(ctx)
	if err != nil {
		ctxCancel()
		return nil, fmt.Errorf("could not open BMC client: %v", err)
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