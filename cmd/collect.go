package cmd

import (
	magellan "davidallendj/magellan/internal"
	"davidallendj/magellan/internal/api/smd"
	"davidallendj/magellan/internal/db/sqlite"

	"github.com/cznic/mathutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)


var collectCmd = &cobra.Command{
	Use: "collect",
	Short: "Query information about BMC",
	Run: func(cmd *cobra.Command, args []string) {
	
	// make application logger
	l := magellan.NewLogger(logrus.New(), logrus.DebugLevel)

	// get probe states stored in db from scan
	probeStates, err := sqlite.GetProbeResults(dbpath)
	if err != nil {
		l.Log.Errorf("could not get states: %v", err)
	}

	if threads <= 0 {
		threads = mathutil.Clamp(len(probeStates), 1, 255)
	}
	q := &magellan.QueryParams{
		User: 		user,
		Pass: 		pass,
		Drivers: 	drivers,
		Timeout: 	timeout,
		Threads:	threads,
		Verbose: 	verbose,
		WithSecureTLS: withSecureTLS,
	}
	magellan.CollectInfo(&probeStates, l, q)

	// generate custom xnames for bmcs
	// node := xnames.Node{
	// 	Cabinet:		1000,
	// 	Chassis:		1,
	// 	ComputeModule:	7,
	// 	NodeBMC:		1,
	// 	Node:			0,
	// }

	// // use the found results to query bmc information
	// // users := [][]byte{}
	// probedHosts := []string{}
	// for _, ps := range probeStates {

	// 	// skip if found info from host
	// 	foundHost := slices.Index(probedHosts, ps.Host)
	// 	if !ps.State || foundHost >= 0{
	// 		continue
	// 	}

	// 	logrus.Printf("querying %v:%v (%v)\n", ps.Host, ps.Port, ps.Protocol)
		

	// 	client, err := magellan.NewClient(l, q)
	// 	if err != nil {
	// 		l.Log.Errorf("could not make client: %v", err)
	// 		return
	// 	}

	// 	// metadata
	// 	// _, err = magellan.QueryMetadata(client, l, &q)
	// 	// if err != nil {
	// 	// 	l.Log.Errorf("could not query metadata: %v\n", err)
	// 	// }

	// 	// inventories
	// 	inventory, err := magellan.QueryInventory(client, l, q)
	// 	if err != nil {
	// 		l.Log.Errorf("could not query inventory: %v\n", err)
	// 		continue
	// 	}

	// 	// chassis
	// 	_, err = magellan.QueryChassis(client, l, q)
	// 	if err != nil {
	// 		l.Log.Errorf("could not query chassis: %v\n", err)
	// 		continue
	// 	}

	// 	// got host information, so add to list of already probed hosts
	// 	probedHosts = append(probedHosts, ps.Host)

	// 	node.NodeBMC += 1

	// 	headers := make(map[string]string)
	// 	headers["Content-Type"] = "application/json"

	// 	data := make(map[string]any)
	// 	data["ID"] 					= fmt.Sprintf("%v", node)
	// 	data["Type"]				= ""
	// 	data["Name"]				= ""
	// 	data["FQDN"]				= ps.Host
	// 	data["RediscoverOnUpdate"] 	= false
	// 	data["Inventory"] 			= inventory


	// 	b, err := json.MarshalIndent(data, "", "    ")
	// 	if err != nil {
	// 		l.Log.Errorf("could not marshal JSON: %v\n", err)
	// 		continue
	// 	}

	// 	// add all endpoints to smd
	// 	err = smd.AddRedfishEndpoint(b, headers)
	// 	if err != nil {
	// 		logrus.Errorf("could not add redfish endpoint: %v", err)
	// 		continue
	// 	}

	// 	// confirm the inventories were added
	// 	err = smd.GetRedfishEndpoints()
	// 	if err != nil {
	// 		logrus.Errorf("could not get redfish endpoints: %v\n", err)
	// 		continue
	// 	}

		// users
		// user, err := magellan.QueryUsers(client, l, &q)
		// if err != nil {
		// 	l.Log.Errorf("could not query users: %v\n", err)
		// }
		// users = append(users, user)

		// // bios
		// _, err = magellan.QueryBios(client, l, &q)
		// if err != nil {
		// 	l.Log.Errorf("could not query bios: %v\n", err)
		// }

		// _, err = magellan.QueryPowerState(client, l, &q)
		// if err != nil {
		// 	l.Log.Errorf("could not query power state: %v\n", err)
		// }
		
	// }
	
	},
}

func init(){
	collectCmd.PersistentFlags().StringSliceVar(&drivers, "driver", []string{"redfish"}, "set the driver(s) and fallback drivers to use")
	collectCmd.PersistentFlags().StringVar(&smd.Host, "host", smd.Host, "set the host to the smd API")
	collectCmd.PersistentFlags().IntVar(&smd.Port, "port", smd.Port, "set the port to the smd API")
	collectCmd.PersistentFlags().StringVar(&user, "user", "", "set the BMC user")
	collectCmd.PersistentFlags().StringVar(&pass, "pass", "", "set the BMC password")
	collectCmd.PersistentFlags().StringVar(&pass, "password", "", "set the BMC password")
	collectCmd.PersistentFlags().StringVar(&preferredDriver, "preferred-driver", "ipmi", "set the preferred driver to use")
	collectCmd.PersistentFlags().StringVar(&ipmitoolPath, "ipmitool.path", "/usr/bin/ipmitool", "set the path for ipmitool")
	collectCmd.PersistentFlags().BoolVar(&withSecureTLS, "secure-tls", false, "enable secure TLS")
	collectCmd.PersistentFlags().StringVar(&certPoolFile, "cert-pool", "", "path to CA cert. (defaults to system CAs; used with --secure-tls=true)")
	rootCmd.AddCommand(collectCmd)
}