package dora

import (
	"davidallendj/magellan/api"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const (
	Host = "http://localhost"
	DbType = "sqlite3"
	DbPath = "../data/assets.db"
	BaseEndpoint = "/v1"
	Port = 8000
)

type ScannedResult struct {
	id string
	site any
	cidr string 
	ip string
	port int
	protocol string
	scanner string
	state string
	updated string
}

func makeEndpointUrl(endpoint string) string {
	return Host + ":" + fmt.Sprint(Port) + BaseEndpoint + endpoint
}

// Scan for BMC assets uing dora scanner
func ScanForAssets() error {

	return nil
}

// Query dora API to get scanned ports
func QueryScannedPorts() error {
	// Perform scan and collect from dora server
	url := makeEndpointUrl("/scanned_ports")
	_, body, err := api.MakeRequest(url, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("could not discover assets: %v", err)
	}

	// get data from JSON
	var res map[string]any
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("could not unmarshal response body: %v", err)
	}
	data := res["data"]

	fmt.Println(data)

	return nil
}

// Loads scanned ports directly from DB
func LoadScannedPortsFromDB(dbPath string, dbType string) {
	db, _ := sqlx.Open(dbType, dbPath)
	sql := `SELECT * FROM scanned_port WHERE state='open'`
	rows, _ := db.Query(sql)
	for rows.Next() {
		var r ScannedResult
		rows.Scan(
			&r.id, &r.site, &r.cidr, &r.ip, &r.port, &r.protocol, &r.scanner, 
			&r.state, &r.updated,
		)
	}
}