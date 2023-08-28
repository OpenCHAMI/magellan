package main

import (
	smd "davidallendj/magellan/api/smd"
	magellan "davidallendj/magellan/internal"
	"fmt"

	// smd "github.com/alexlovelltroy/hms-smd/pkg/redfish"

	logrusr "github.com/bombsimon/logrusr/v2"
	"github.com/cznic/mathutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	timeout       int
	threads       int
	ports         []int
	subnets       []string
	hosts         []string
	withSecureTLS bool
	certPoolFile  string
	user          string
	pass          string
	dbpath		string 
	drivers       []string
)

// TODO: discover bmc's on network (dora)
// TODO: query bmc component information and store in db (?)
// TODO: send bmc component information to smd
func main() {
	pflag.StringVar(&user, "user", "root", "set the BMC user")
	pflag.StringVar(&pass, "pass", "root_password", "set the BMC pass")
	pflag.StringSliceVar(&subnets, "subnet", []string{"127.0.0.0"}, "set additional subnets")
	pflag.StringSliceVar(&hosts, "host", []string{}, "set additional hosts")
	pflag.IntVar(&threads, "threads", -1, "set the number of threads")
	pflag.IntVar(&timeout, "timeout", 1, "set the timeout")
	pflag.IntSliceVar(&ports, "port", []int{}, "set the ports to scan")
	pflag.StringSliceVar(&drivers, "driver", []string{"redfish"}, "set the BMC driver to use")
	pflag.StringVar(&dbpath, "dbpath", ":memory:", "set the probe storage path")
	pflag.BoolVar(&withSecureTLS, "secure-tls", false, "enable secure TLS")
	pflag.StringVar(&certPoolFile, "cert-pool", "", "path to an file containing x509 CAs. An empty string uses the system CAs. Only takes effect when --secure-tls=true")
	pflag.Parse()

	// make application logger
	l := logrus.New()
	l.Level = logrus.DebugLevel
	logger := logrusr.New(l)

	// set hosts to use for scanning
	hostsToScan := []string{}
	if len(hosts) > 0 {
		hostsToScan = hosts
	} else {
		for _, subnet := range subnets {
			hostsToScan = append(hostsToScan, magellan.GenerateHosts(subnet, 1, 5)...)
		}
	}

	// set ports to use for scanning
	portsToScan := []int{}
	if len(ports) > 0 {
		portsToScan = ports
	} else {
		portsToScan = append(magellan.GetDefaultPorts(), ports...)
	}

	// scan and store probe data in dbPath
	if threads <= 0 {
		threads = mathutil.Clamp(len(hostsToScan), 1, 255)
	}
	probeStates := magellan.ScanForAssets(hostsToScan, portsToScan, threads, timeout)
	fmt.Printf("probe states: %v\n", probeStates)
	magellan.StoreStates(dbpath, &probeStates)

	// use the found results to query bmc information
	inventories := [][]byte{}
	for _, ps := range probeStates {
		if !ps.State {
			continue
		}
		logrus.Infof("querying bmc %v\n", ps)
		q := magellan.QueryParams{
			Host: ps.Host,
			Port: ps.Port,
			User: user,
			Pass: pass,
			Drivers: drivers,
			Timeout: timeout,
		}
		inventory, err := magellan.QueryInventory(&logger, &q)
		if err != nil {
			logrus.Errorf("could not query BMC information: %v\n", err)
		}
		inventories = append(inventories, inventory)
	}

	// add all endpoints to smd
	for _, inventory := range inventories {
		err := smd.AddRedfishEndpoint(inventory)
		if err != nil {
			logrus.Errorf("could not add redfish endpoint: %v", err)
		}
	}

	// confirm the inventories were added
	err := smd.GetRedfishEndpoints()
	if err != nil {
		logrus.Errorf("could not get redfish endpoints: %v\n", err)
	}
}
