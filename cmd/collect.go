package cmd

import (
	"davidallendj/magellan/api/smd"
	magellan "davidallendj/magellan/internal"

	"github.com/bombsimon/logrusr/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)


var collectCmd = &cobra.Command{
	Use: "collect",
	Short: "Query information about BMC",
	Run: func(cmd *cobra.Command, args []string) {
		// make application logger
	l := logrus.New()
	l.Level = logrus.DebugLevel
	logger := logrusr.New(l)

	// get probe states stored in db from scan
	probeStates, err := magellan.GetStates(dbpath)
	if err != nil {
		l.Errorf("could not get states: %v", err)
	}

	// use the found results to query bmc information
	inventories := [][]byte{}
	// users := [][]byte{}
	for _, ps := range probeStates {
		if !ps.State {
			continue
		}
		logrus.Infof("querying %v\n", ps)
		q := magellan.QueryParams{
			Host: ps.Host,
			Port: ps.Port,
			User: user,
			Pass: pass,
			Drivers: drivers,
			Timeout: timeout,
			Verbose: true,
			WithSecureTLS: withSecureTLS,
		}

		client, err := magellan.NewClient(&logger, &q)
		if err != nil {
			l.Errorf("could not make client: %v", err)
			return 
		}

		// metadata
		_, err = magellan.QueryMetadata(client, &logger, &q)
		if err != nil {
			l.Errorf("could not query metadata: %v\n", err)
		}

		// inventories
		inventory, err := magellan.QueryInventory(client, &logger, &q)
		// inventory, err := magellan.QueryInventoryV2(q.Host, q.Port, q.User, q.Pass)
		if err != nil {
			l.Errorf("could not query inventory: %v\n", err)
		}
		inventories = append(inventories, inventory)

		// users
		// user, err := magellan.QueryUsers(client, &logger, &q)
		// if err != nil {
		// 	l.Errorf("could not query users: %v\n", err)
		// }

		// // bios
		// _, err = magellan.QueryBios(client, &logger, &q)
		// if err != nil {
		// 	l.Errorf("could not query bios: %v\n", err)
		// }
		// users = append(users, user)
	}

	// add all endpoints to smd
	for _, inventory := range inventories {
		err := smd.AddRedfishEndpoint(inventory)
		if err != nil {
			logrus.Errorf("could not add redfish endpoint: %v", err)
		}
	}

	// confirm the inventories were added
	err = smd.GetRedfishEndpoints()
	if err != nil {
		logrus.Errorf("could not get redfish endpoints: %v\n", err)
	}
	},
}

func init(){
	rootCmd.AddCommand(collectCmd)
}